package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/gigachat"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/scheduler"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
	"github.com/Zhaba1337228/max-university-event-bot/internal/transport/adminapi"
	"github.com/Zhaba1337228/max-university-event-bot/internal/transport/longpoll"
	"github.com/Zhaba1337228/max-university-event-bot/internal/transport/webhook"
)

// App собирает все зависимости и предоставляет Run/Shutdown.
//
// На день 4 поднимаем минимальный конвейер:
//  1. pgxpool (Ping проверяет связь);
//  2. UserStates репозиторий + FSM Manager;
//  3. MAX client (Ping проверяет токен);
//  4. Handlers (Start, Fallback);
//  5. Dispatcher на 32 параллельных хендлера;
//  6. Long-poll Runner.
//
// Дни 5-12 добавят остальные репозитории и сервисы; день 13 добавит admin API
// и scheduler. Здесь сейчас — самый узкий «спинной мозг».
type App struct {
	cfg *Config
	log *slog.Logger

	pool *pgxpool.Pool
	max  *maxclient.Client

	dispatcher *bot.Dispatcher
	longpoll   *longpoll.Runner
	webhook    *webhook.Server
	adminAPI   *adminapi.Server
	scheduler  *scheduler.Scheduler

	updates chan schemes.UpdateInterface
}

// New собирает зависимости. При ошибке возвращает (nil, err), уже освобождая
// уже захваченные ресурсы (pool, max client).
func New(ctx context.Context, cfg *Config, log *slog.Logger) (*App, error) {
	a := &App{cfg: cfg, log: log}

	// 1. PostgreSQL
	pool, err := repo.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns, cfg.DB.MinConns)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	a.pool = pool

	// 2. Repositories
	statesRepo := repo.NewUserStates()
	eventsRepo := repo.NewEvents()
	regsRepo := repo.NewRegistrations()
	usersRepo := repo.NewUsers()
	logsRepo := repo.NewActionLogs()
	notifsRepo := repo.NewNotifications()

	// 3. FSM
	fsmMgr := fsm.NewManager(statesRepo, pool)

	// 4. Services
	eventSvc := service.NewEvent(pool, eventsRepo, regsRepo, logsRepo)
	userSvc := service.NewUser(pool, usersRepo, logsRepo)
	regSvc := service.NewRegistration(pool, eventsRepo, regsRepo, usersRepo, logsRepo,
		cfg.Business.WaitlistEnabled)
	actionLogSvc := service.NewActionLog(pool, logsRepo)
	roleSvc := service.NewRole(pool, usersRepo, eventsRepo, log)

	// Bootstrap ролей из env (organizer/staff/admin user IDs).
	if err := roleSvc.Bootstrap(ctx,
		cfg.Business.OrganizerUserIDs,
		cfg.Business.StaffUserIDs,
		cfg.Business.AdminUserIDs); err != nil {
		log.Warn("role bootstrap failed (continuing without)", "err", err)
	}

	// 5. MAX client + ping
	mc, err := maxclient.New(maxclient.Config{
		Token:       cfg.Max.Token,
		HTTPTimeout: cfg.Max.HTTPTimeout,
		Debug:       cfg.Max.Debug,
	}, log)
	if err != nil {
		a.closeQuietly()
		return nil, fmt.Errorf("maxclient: %w", err)
	}
	a.max = mc

	if cfg.Max.DevSkipPing {
		log.Warn("MAX_BOT_DEV_SKIP_PING=true — MAX API ping/long-poll/scheduler disabled. " +
			"Admin API запустится для локальной отладки веб-админки, но бот не будет принимать апдейты.")
	} else {
		botInfo, err := mc.Ping(ctx)
		if err != nil {
			a.closeQuietly()
			return nil, fmt.Errorf("ping max api: %w", err)
		}
		log.Info("max bot online",
			"bot_id", botInfo.UserId,
			"name", botInfo.Name,
			"username", botInfo.Username,
		)
	}

	// Notification service зависит от mc — конструируем после Ping.
	notifSvc := service.NewNotification(pool, notifsRepo, regsRepo, eventsRepo,
		usersRepo, logsRepo, mc, cfg.Business.NotifyRateLimitRPS, cfg.Business.NotifyBatchSize, log)

	// Day 13: auth + QR + attendance сервисы.
	// Auth работает только если задан ADMIN_SESSION_KEY (см. config.Validate);
	// иначе AdminLogin-handler в боте сам молча скажет "не настроено".
	var authSvc service.Auth
	if cfg.Admin.SessionKey != "" {
		authSvc = service.NewAuth(pool, usersRepo, cfg.Admin.SessionKey)
	}
	// QR-сервис шифрует payload через AES-256-GCM с ключом, производным от
	// ADMIN_SESSION_KEY. Если ключ не задан (один из dev-сценариев — запуск
	// только бота без admin API), включаем legacy plaintext-формат, чтобы
	// QR в чате хотя бы работал; в проде ADMIN_SESSION_KEY обязателен.
	var qrSvc service.QR
	if cfg.Admin.SessionKey != "" {
		qr, err := service.NewQR(cfg.Admin.SessionKey)
		if err != nil {
			a.closeQuietly()
			return nil, fmt.Errorf("init qr service: %w", err)
		}
		qrSvc = qr
	} else {
		log.Warn("ADMIN_SESSION_KEY empty — QR payloads will be plaintext (legacy MAXUEB:event:code). " +
			"Set ADMIN_SESSION_KEY for encrypted QR.")
		qrSvc = service.NewQRPlaintext()
	}
	attendSvc := service.NewAttendance(pool, qrSvc, regsRepo, eventsRepo, usersRepo, roleSvc, logsRepo)

	// Day 16: GigaChat (опционально). Если AuthKey не задан — фасад деградирует
	// в ErrAIUnavailable, и handler уйдёт в fallback.
	var aiSvc service.AI
	if cfg.GigaChat.AuthKey != "" {
		giga := gigachat.New(gigachat.Config{
			AuthKey:      cfg.GigaChat.AuthKey,
			Scope:        cfg.GigaChat.Scope,
			Model:        cfg.GigaChat.Model,
			OAuthURL:     cfg.GigaChat.OAuthURL,
			APIURL:       cfg.GigaChat.APIURL,
			Timeout:      cfg.GigaChat.Timeout,
			CABundleFile: cfg.GigaChat.CABundleFile,
			InsecureTLS:  cfg.GigaChat.InsecureTLS,
			MaxTokens:    cfg.AI.MaxTokens,
		})
		aiSvc = service.NewAI(giga, service.AIConfig{
			RecommenderEnabled: cfg.AI.RecommenderEnabled,
			RewriterEnabled:    cfg.AI.RewriterEnabled,
			SummaryEnabled:     cfg.AI.SummaryEnabled,
			RequestTimeout:     cfg.AI.RequestTimeout,
		}, log)
		log.Info("ai enabled",
			"recommender", cfg.AI.RecommenderEnabled,
			"rewriter", cfg.AI.RewriterEnabled,
			"summary", cfg.AI.SummaryEnabled,
		)
	} else {
		log.Info("ai disabled (GIGACHAT_AUTH_KEY empty)")
	}

	// 6. Handlers + Dispatcher + Long-poll runner
	handlers := bot.NewHandlers(bot.HandlersConfig{
		API:             mc,
		Log:             log,
		FSM:             fsmMgr,
		Events:          eventSvc,
		Users:           userSvc,
		Registration:    regSvc,
		ActionLogs:      actionLogSvc,
		Role:            roleSvc,
		Notification:    notifSvc,
		Auth:            authSvc,
		QR:              qrSvc,
		AI:              aiSvc,
		RegsRepo:        regsRepo,
		EventsRepo:      eventsRepo,
		DB:              pool,
		WaitlistEnabled: cfg.Business.WaitlistEnabled,
		PolicyVersion:   cfg.Policy.PrivacyPolicyVersion,
		WebBaseURL:      cfg.Admin.WebBaseURL,
	})

	// Day 13: admin REST API на отдельном порту, только если задан ADMIN_SESSION_KEY.
	if authSvc != nil {
		a.adminAPI = adminapi.New(adminapi.Config{
			Addr:         cfg.Admin.APIAddr,
			WebBaseURL:   cfg.Admin.WebBaseURL,
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
		}, log, adminapi.Deps{
			Auth:           authSvc,
			Events:         eventSvc,
			Registration:   regSvc,
			Users:          userSvc,
			Role:           roleSvc,
			Notification:   notifSvc,
			Attendance:     attendSvc,
			ActionLogs:     actionLogSvc,
			RegsRepo:       regsRepo,
			UsersRepo:      usersRepo,
			EventsRepo:     eventsRepo,
			ActionLogsRepo: logsRepo,
			DB:             pool,
		})
	}
	a.dispatcher = bot.NewDispatcher(log, handlers, 32)
	a.longpoll = longpoll.New(mc, log)
	a.updates = make(chan schemes.UpdateInterface, 256)

	// Day 18: webhook сервер (опционален, только при MAX_BOT_MODE=webhook).
	if cfg.Max.Mode == "webhook" {
		a.webhook = webhook.New(webhook.Config{
			Addr:         cfg.HTTP.Addr,
			Secret:       cfg.Max.WebhookSecret,
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
		}, log, a.updates)
	}

	// Day 16: scheduler — reminders + dispatch + purge.
	// В DEV_SKIP_PING пропускаем — без рабочего MAX API job'ы будут только
	// логгировать 401 каждую минуту.
	if !cfg.Max.DevSkipPing {
		sched, err := scheduler.New(scheduler.Config{
			ReminderHoursCSV: cfg.Business.ReminderHoursCSV,
		}, log,
			notifsRepo, eventsRepo, regsRepo, usersRepo, statesRepo, pool, mc)
		if err != nil {
			a.closeQuietly()
			return nil, fmt.Errorf("scheduler: %w", err)
		}
		a.scheduler = sched
	}

	return a, nil
}

// Run запускает все рантайм-компоненты. Блокируется до ctx.Done()
// или фатальной ошибки. На день 4 поддерживается только long-polling
// для приёма обновлений MAX. С Дня 13 параллельно поднимается admin REST API.
func (a *App) Run(ctx context.Context) error {
	// Dispatcher всегда нужен — обрабатывает входящие апдейты.
	go a.dispatcher.Run(ctx, a.updates)

	// Day 16: scheduler — reminders + dispatch + purge.
	if a.scheduler != nil {
		if err := a.scheduler.Start(); err != nil {
			return fmt.Errorf("scheduler start: %w", err)
		}
	}

	// Admin REST API (опционален; запускается только при ADMIN_SESSION_KEY).
	apiErrCh := make(chan error, 1)
	if a.adminAPI != nil {
		go func() {
			if err := a.adminAPI.Run(ctx); err != nil {
				apiErrCh <- err
			} else {
				apiErrCh <- nil
			}
		}()
	}

	switch a.cfg.Max.Mode {
	case "longpoll":
		if a.cfg.Max.DevSkipPing {
			a.log.Info("long-poll skipped (DEV_SKIP_PING); blocking until ctx.Done()")
			<-ctx.Done()
		} else {
			// Long-poll работает в основной горутине; завершится при ctx.Done().
			a.longpoll.Run(ctx, a.updates)
		}
	case "webhook":
		// Регистрируем подписку (если ещё нет — Subscribe; если есть — пропускаем).
		if err := a.ensureSubscription(ctx); err != nil {
			a.log.Warn("ensure subscription failed (continue)", "err", err)
		}
		if err := a.webhook.Run(ctx); err != nil {
			return fmt.Errorf("webhook: %w", err)
		}
	default:
		return fmt.Errorf("unsupported mode: %s", a.cfg.Max.Mode)
	}

	// Дождёмся завершения admin API (если он был запущен).
	if a.adminAPI != nil {
		if err := <-apiErrCh; err != nil {
			return fmt.Errorf("admin api: %w", err)
		}
	}
	return nil
}

// Shutdown освобождает ресурсы. Безопасно вызывать многократно.
func (a *App) Shutdown() {
	if a.scheduler != nil {
		a.scheduler.Stop()
		a.scheduler = nil
	}
	a.closeQuietly()
}

func (a *App) closeQuietly() {
	if a.pool != nil {
		a.pool.Close()
		a.pool = nil
	}
}

// ensureSubscription проверяет, что в MAX уже зарегистрирован наш webhook URL
// и при необходимости вызывает Subscribe. Идемпотентно: повторные вызовы
// безопасны.
//
// План §13.4: MAX через 8 ч простоя автоматически отписывает webhook.
// Поэтому при старте всегда переподписываемся, если своей нет.
func (a *App) ensureSubscription(ctx context.Context) error {
	subs, err := a.max.Raw().Subscriptions.GetSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("get subscriptions: %w", err)
	}
	wantURL := a.cfg.Max.WebhookURL
	for _, s := range subs.Subscriptions {
		if s.Url == wantURL {
			a.log.Info("webhook subscription already exists", "url", wantURL)
			return nil
		}
	}

	updateTypes := []string{
		string(schemes.TypeBotStarted),
		string(schemes.TypeMessageCreated),
		string(schemes.TypeMessageCallback),
	}
	if _, err := a.max.Raw().Subscriptions.Subscribe(ctx,
		wantURL, updateTypes, a.cfg.Max.WebhookSecret); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	a.log.Info("webhook subscribed", "url", wantURL, "types", updateTypes)
	return nil
}
