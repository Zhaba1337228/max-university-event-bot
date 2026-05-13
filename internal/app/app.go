package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
	"github.com/Zhaba1337228/max-university-event-bot/internal/transport/longpoll"
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
	eventSvc := service.NewEvent(pool, eventsRepo, regsRepo)
	userSvc := service.NewUser(pool, usersRepo, logsRepo)
	regSvc := service.NewRegistration(pool, eventsRepo, regsRepo, usersRepo, logsRepo,
		cfg.Business.WaitlistEnabled)
	actionLogSvc := service.NewActionLog(pool, logsRepo)
	roleSvc := service.NewRole(pool, usersRepo, eventsRepo, log)

	// Bootstrap ролей из env (organizer/admin user IDs).
	if err := roleSvc.Bootstrap(ctx,
		cfg.Business.OrganizerUserIDs, cfg.Business.AdminUserIDs); err != nil {
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

	// Notification service зависит от mc — конструируем после Ping.
	notifSvc := service.NewNotification(pool, notifsRepo, regsRepo, eventsRepo,
		usersRepo, logsRepo, mc, cfg.Business.NotifyRateLimitRPS, cfg.Business.NotifyBatchSize, log)

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
		RegsRepo:        regsRepo,
		DB:              pool,
		WaitlistEnabled: cfg.Business.WaitlistEnabled,
		PolicyVersion:   cfg.Policy.PrivacyPolicyVersion,
	})
	a.dispatcher = bot.NewDispatcher(log, handlers, 32)
	a.longpoll = longpoll.New(mc, log)
	a.updates = make(chan schemes.UpdateInterface, 256)

	return a, nil
}

// Run запускает все рантайм-компоненты. Блокируется до ctx.Done()
// или фатальной ошибки. На день 4 поддерживается только long-polling.
func (a *App) Run(ctx context.Context) error {
	switch a.cfg.Max.Mode {
	case "longpoll":
		// Dispatcher и Long-poll Runner работают параллельно.
		// Когда ctx закроется — оба завершатся, longpoll закроет за нас канал.
		go a.dispatcher.Run(ctx, a.updates)
		a.longpoll.Run(ctx, a.updates)
		return nil
	case "webhook":
		// День 18 — webhook сервер на :8080.
		return errors.New("webhook mode is not implemented yet (день 18)")
	default:
		return fmt.Errorf("unsupported mode: %s", a.cfg.Max.Mode)
	}
}

// Shutdown освобождает ресурсы. Безопасно вызывать многократно.
func (a *App) Shutdown() {
	a.closeQuietly()
}

func (a *App) closeQuietly() {
	if a.pool != nil {
		a.pool.Close()
		a.pool = nil
	}
}
