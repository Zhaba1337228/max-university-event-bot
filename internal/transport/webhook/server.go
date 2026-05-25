package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/secret"
)

// Server — HTTP-сервер webhook'а (см. план §13.3, §19.3).
//
// Endpoint'ы:
//
//	GET  /healthz     — для k8s/docker healthcheck.
//	POST /webhook/max — приём апдейтов от MAX. Проверяет X-Max-Bot-Api-Secret
//	                    (constant-time), парсит, кладёт в out-канал.
//
// Особенности (плана §19):
//   - Body лимит 1 MiB через http.MaxBytesReader.
//   - ReadHeaderTimeout / ReadTimeout / WriteTimeout — против slowloris.
//   - LRU-дедуп update_id (1024 элемента, TTL 10 мин).
//   - На любую ошибку парсинга — 200 OK (иначе MAX будет ретраить и через
//     8 ч отпишет нас от webhook'а).
type Server struct {
	addr         string
	secret       string
	log          *slog.Logger
	updates      chan<- schemes.UpdateInterface
	readTimeout  time.Duration
	writeTimeout time.Duration

	srv *http.Server

	// LRU-дедуп update_id'ов (на самом деле — простой map с TTL).
	dedupMu sync.Mutex
	dedup   map[int64]time.Time
}

// Config — параметры сервера.
type Config struct {
	Addr         string
	Secret       string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// New создаёт сервер.
func New(cfg Config, log *slog.Logger, out chan<- schemes.UpdateInterface) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	return &Server{
		addr:         cfg.Addr,
		secret:       cfg.Secret,
		log:          log.With("component", "webhook"),
		updates:      out,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		dedup:        make(map[int64]time.Time),
	}
}

// Run запускает HTTP-сервер до ctx.Done(). Graceful shutdown 5 секунд.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/webhook/max", s.handleWebhook)

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// #nosec G118 -- detached shutdown context is intentional for graceful shutdown after ctx cancellation.
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:gosec // G118: graceful shutdown
		defer cancel()
		_ = s.srv.Shutdown(shCtx)
	}()

	s.log.Info("webhook listening", "addr", s.addr)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Constant-time compare на secret (§19.3).
	if s.secret != "" {
		got := r.Header.Get("X-Max-Bot-Api-Secret")
		if !secret.ConstantTimeEqual(got, s.secret) {
			s.log.Warn("webhook secret mismatch", "remote", r.RemoteAddr)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.log.Warn("webhook body read failed", "err", err)
		// Возвращаем 200 — иначе MAX будет ретраить, что нам только хуже.
		w.WriteHeader(http.StatusOK)
		return
	}
	defer func() { _ = r.Body.Close() }()

	upd, err := ParseUpdate(body)
	if err != nil {
		s.log.Warn("webhook parse failed", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	if upd == nil {
		// Неизвестный тип update'а — игнорируем.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Дедуп: по update_id (если он есть в payload — у каждого Update есть Timestamp,
	// но нет явного update_id; используем композицию для message_created/callback).
	// На MVP сделаем простой дедуп по telemetry-метке: timestamp + sender id.
	if s.isDuplicate(upd) {
		s.log.Debug("webhook duplicate, skipped")
		w.WriteHeader(http.StatusOK)
		return
	}

	select {
	case s.updates <- upd:
	case <-r.Context().Done():
	}
	w.WriteHeader(http.StatusOK)
}

// isDuplicate возвращает true если update уже был обработан в последние 10 мин.
// Использует «ключ» из timestamp+sender пользователя — этого достаточно
// для real-world ретраев MAX.
func (s *Server) isDuplicate(u schemes.UpdateInterface) bool {
	key := dedupKey(u)
	if key == 0 {
		return false
	}

	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)

	s.dedupMu.Lock()
	defer s.dedupMu.Unlock()

	// Ленивая чистка: при размере > 1024 убираем устаревшие.
	if len(s.dedup) > 1024 {
		for k, v := range s.dedup {
			if v.Before(cutoff) {
				delete(s.dedup, k)
			}
		}
	}

	if _, seen := s.dedup[key]; seen {
		return true
	}
	s.dedup[key] = now
	return false
}

// dedupKey собирает 64-бит ключ из timestamp+user_id.
// Для разных типов update — разный набор полей.
func dedupKey(u schemes.UpdateInterface) int64 {
	switch upd := u.(type) {
	case *schemes.MessageCreatedUpdate:
		return upd.Message.Timestamp ^ upd.Message.Sender.UserId
	case *schemes.MessageCallbackUpdate:
		return upd.Callback.Timestamp ^ upd.Callback.User.UserId
	case *schemes.BotStartedUpdate:
		return upd.User.UserId
	}
	return 0
}
