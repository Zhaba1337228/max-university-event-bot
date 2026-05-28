// Package adminapi реализует JSON REST API для веб-админки.
//
// Сервер поднимается на отдельном порту (по умолчанию :8081) и отдаёт
// только JSON под /api/*. Все endpoint'ы (кроме /api/auth/exchange и
// /api/healthz) защищены сессионным JWT cookie sid.
package adminapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// Config — параметры сервера.
type Config struct {
	Addr         string
	WebBaseURL   string // ADMIN_WEB_BASE_URL — для CORS и Origin guard
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Server держит зависимости и http.Server.
type Server struct {
	cfg  Config
	log  *slog.Logger
	auth service.Auth
	deps Deps
	srv  *http.Server
}

// Deps — все сервисы и репозитории, нужные для handlers.
type Deps struct {
	Auth           service.Auth
	Events         service.Event
	Registration   service.Registration
	Users          service.User
	Role           service.Role
	Notification   service.Notification
	Attendance     service.Attendance
	ActionLogs     service.ActionLog
	RegsRepo       repo.RegistrationRepo
	UsersRepo      repo.UserRepo // нужен для checkin (local id → max id lookup)
	EventsRepo     repo.EventRepo
	ActionLogsRepo repo.ActionLogRepo // прямой Append (CSV export, manual mark вне сервиса)
	DB             repo.Querier
}

// New создаёт сервер. Не запускает — это делает Run.
func New(cfg Config, log *slog.Logger, deps Deps) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	return &Server{
		cfg:  cfg,
		log:  log.With("component", "adminapi"),
		auth: deps.Auth,
		deps: deps,
	}
}

// Run блокируется на http.ListenAndServe до ctx.Done().
func (s *Server) Run(ctx context.Context) error {
	r := s.routes()
	s.srv = &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown при ctx.Done().
	// context.Background() здесь намеренно — основной ctx уже отменён,
	// а Shutdown нуждается в нескольких секундах на доводку in-flight запросов.
	// #nosec G118 -- detached shutdown context is intentional for graceful shutdown after ctx cancellation.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:gosec // G118: graceful shutdown требует независимый ctx
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
	}()

	s.log.Info("admin api listening", "addr", s.cfg.Addr)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// routes — chi-роутер с middleware-цепочкой.
//
// Middleware-порядок (важен):
//  1. RequestID
//  2. recover (без stacktrace в response)
//  3. requestLogger (БЕЗ query/body — там могут быть PII)
//  4. securityHeaders
//  5. cors (только из ADMIN_WEB_BASE_URL)
//  6. originGuard на mutating endpoints (внутри роутера)
//  7. requireSession на /api (кроме /healthz, /auth/exchange)
//  8. requireRoles на /api/users (admin/organizer)
func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(recoverMW(s.log))
	r.Use(slogLogger(s.log))
	r.Use(securityHeaders())
	r.Use(corsMW(s.cfg.WebBaseURL))

	// Глобальный healthcheck — без auth, без CORS preflight.
	r.Get("/api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth — публичный exchange + logout/me.
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(originGuard(s.cfg.WebBaseURL))
		r.Post("/exchange", s.handleAuthExchange)
		r.Post("/logout", s.handleAuthLogout)
		r.With(requireSession(s.auth)).Get("/me", s.handleAuthMe)
	})

	// Защищённые endpoint'ы под сессией.
	r.Route("/api", func(r chi.Router) {
		r.Use(originGuard(s.cfg.WebBaseURL))
		r.Use(requireSession(s.auth))

		// Events.
		r.Get("/events", s.handleListEvents)
		r.Post("/events", s.handleEventCreate)
		r.Get("/events/{id}", s.handleGetEvent)
		r.Patch("/events/{id}", s.handleEventUpdate)
		r.Get("/events/{id}/participants", s.handleListParticipants)
		r.Get("/events/{id}/participants.csv", s.handleExportParticipantsCSV)
		r.Get("/events/{id}/actions", s.handleEventAuditLog)
		r.Post("/events/{id}/close", s.handleEventClose)
		r.Post("/events/{id}/open", s.handleEventOpen)
		r.Post("/events/{id}/cancel", s.handleEventCancel)
		r.Post("/events/{id}/broadcast", s.handleBroadcast)
		r.Post("/events/{id}/registrations/{regID}/mark", s.handleManualMark)

		// Registration lookup by short code (без отметки attended).
		r.Get("/registrations/by-code", s.handleLookupByCode)

		// Check-in (камера телефона организатора).
		r.Post("/checkin", s.handleCheckin)

		// Dashboard.
		r.Get("/dashboard", s.handleDashboard)

		// Users / volunteers (admin + organizer).
		r.With(requireRoles(domain.RoleAdmin, domain.RoleOrganizer)).Get("/users", s.handleListUsers)
		r.With(requireRoles(domain.RoleAdmin, domain.RoleOrganizer)).Patch("/users/{id}/role", s.handleSetUserRole)
	})

	return r
}
