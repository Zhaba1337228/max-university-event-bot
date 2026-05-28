package bot

import (
	"context"
	"log/slog"
	"strings"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/handlers"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// Handlers — корневой holder всех скоупированных обработчиков.
// Маршрутизация по типу Update реализована в Dispatcher.
//
// По мере прохождения дней дорожной карты сюда добавляются новые обработчики
// (см. план §15.1). Маршрутизатор RouteCallback заранее знает все группы,
// чтобы при появлении хендлера не пришлось переписывать switch.
type Handlers struct {
	Log *slog.Logger
	FSM *fsm.Manager

	Start         *handlers.StartHandler
	Fallback      *handlers.FallbackHandler
	Events        *handlers.EventsHandler
	Registration  *handlers.RegistrationHandler
	MyReg         *handlers.MyRegistrationHandler
	Cancel        *handlers.CancelHandler
	Waitlist      *handlers.WaitlistHandler
	Organizer     *handlers.OrganizerHandler
	OrganizerList *handlers.OrganizerListHandler
	OrgNotify     *handlers.OrganizerNotifyHandler
	AdminLogin    *handlers.AdminLoginHandler
	AIPick        *handlers.AIPickHandler
}

// HandlersConfig — групповая инициализация. По мере роста зависимостей удобнее
// передавать конфиг, чем 10 позиционных параметров.
type HandlersConfig struct {
	API             *maxclient.Client
	Log             *slog.Logger
	FSM             *fsm.Manager
	Events          service.Event
	Users           service.User
	Registration    service.Registration
	ActionLogs      service.ActionLog
	Role            service.Role
	Notification    service.Notification
	Auth            service.Auth
	QR              service.QR
	AI              service.AI
	RegsRepo        repo.RegistrationRepo
	EventsRepo      repo.EventRepo
	DB              repo.Querier
	WaitlistEnabled bool
	PolicyVersion   string
	WebBaseURL      string
}

// NewHandlers собирает Handlers по конфигу.
func NewHandlers(cfg HandlersConfig) *Handlers {
	h := &Handlers{
		Log: cfg.Log.With("component", "handlers"),
		FSM: cfg.FSM,
	}
	h.Start = handlers.NewStartHandler(cfg.API, cfg.FSM, cfg.Log)
	h.Fallback = handlers.NewFallbackHandler(cfg.API, cfg.FSM, cfg.Log)
	h.Events = handlers.NewEventsHandler(cfg.API, cfg.FSM, cfg.Events,
		cfg.Users, cfg.Registration, cfg.Log, cfg.WaitlistEnabled)
	h.Registration = handlers.NewRegistrationHandler(cfg.API, cfg.FSM,
		cfg.Registration, cfg.Users, cfg.Events,
		cfg.QR, cfg.RegsRepo, cfg.DB,
		cfg.Log, cfg.PolicyVersion)
	h.MyReg = handlers.NewMyRegistrationHandler(cfg.API, cfg.FSM,
		cfg.Users, cfg.Registration, cfg.Events, cfg.ActionLogs, cfg.QR,
		cfg.RegsRepo, cfg.DB, cfg.Log)
	h.Cancel = handlers.NewCancelHandler(cfg.API, cfg.FSM,
		cfg.Registration, cfg.Users, cfg.Events, cfg.Log)
	h.Waitlist = handlers.NewWaitlistHandler(cfg.API, cfg.FSM, h.Registration, cfg.Log)
	h.Organizer = handlers.NewOrganizerHandler(cfg.API, cfg.FSM, cfg.Role, cfg.Events, cfg.AI, cfg.Notification,
		cfg.EventsRepo, cfg.DB, cfg.Log)
	h.OrganizerList = handlers.NewOrganizerListHandler(cfg.API, cfg.FSM, cfg.Role,
		cfg.Events, cfg.RegsRepo, cfg.DB, cfg.Log)
	h.OrgNotify = handlers.NewOrganizerNotifyHandler(cfg.API, cfg.FSM, cfg.Role,
		cfg.Events, cfg.Notification, cfg.AI, cfg.RegsRepo, cfg.DB, cfg.Log)
	h.AIPick = handlers.NewAIPickHandler(cfg.API, cfg.FSM, cfg.AI, cfg.Events, cfg.Log)
	if cfg.Auth != nil {
		h.AdminLogin = handlers.NewAdminLoginHandler(cfg.API, cfg.Auth, cfg.WebBaseURL, cfg.Log)
	}
	return h
}

// RouteMessage маршрутизирует MessageCreatedUpdate:
//   - команды (/start, /help, /forget_me, /organizer, /admin_login) — приоритет;
//   - иначе — смотрим текущее FSM-состояние; если оно ожидает текст —
//     отдаём соответствующему handler'у; иначе — fallback.
func (h *Handlers) RouteMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	text := strings.TrimSpace(upd.Message.Body.Text)
	cmd := strings.ToLower(strings.SplitN(text, " ", 2)[0])

	switch cmd {
	case "/start":
		h.Start.OnStart(ctx, upd)
		return
	case "/help":
		h.Start.OnHelp(ctx, upd)
		return
	case "/whoami", "/id", "/myid":
		h.Start.OnWhoami(ctx, upd)
		return
	case "/forget_me":
		h.MyReg.OnForgetMeCmd(ctx, upd)
		return
	case "/organizer":
		h.Organizer.OnEntryCmd(ctx, upd)
		return
	case "/admin_login":
		if h.AdminLogin != nil {
			h.AdminLogin.OnCmd(ctx, upd)
		} else {
			h.Fallback.OnText(ctx, upd)
		}
		return
	}

	// Если текст не команда — смотрим FSM и направляем в ожидающий handler.
	userMaxID := upd.Message.Sender.UserId
	snap, err := h.FSM.Load(ctx, userMaxID)
	if err != nil {
		h.Log.Warn("fsm load failed", "err", err, "user_id", userMaxID)
		h.Fallback.OnText(ctx, upd)
		return
	}

	switch snap.State {
	case fsm.StateRegFullName, fsm.StateRegContact, fsm.StateRegInterest:
		h.Registration.OnText(ctx, upd, snap)
	case fsm.StateOrganizerNotifText:
		h.OrgNotify.OnText(ctx, upd, snap)
	case fsm.StateOrganizerSearchCode:
		h.OrganizerList.OnSearchCodeText(ctx, upd, snap.Context.OrganizerEventID)
	case fsm.StateAIPickIntent:
		if h.AIPick != nil {
			h.AIPick.OnText(ctx, upd, snap)
		} else {
			h.Fallback.OnText(ctx, upd)
		}
	default:
		h.Fallback.OnText(ctx, upd)
	}
}

// RouteCallback маршрутизирует MessageCallbackUpdate по группе payload'а.
//
// Группы, для которых ещё нет реального handler'а (дни 10-12), уходят в
// Fallback с понятным сообщением «Эта кнопка устарела».
func (h *Handlers) RouteCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	p := callbacks.Parse(upd.Callback.Payload)
	switch p.Group {
	case callbacks.GroupMain, callbacks.GroupBack:
		h.Start.OnMainMenu(ctx, upd)
	case callbacks.GroupEvent:
		h.Events.OnCallback(ctx, upd, p)
	case callbacks.GroupReg:
		h.Registration.OnCallback(ctx, upd, p)
	case callbacks.GroupMy:
		h.MyReg.OnCallback(ctx, upd, p)
	case callbacks.GroupCancel:
		h.Cancel.OnCallback(ctx, upd, p)
	case callbacks.GroupWaitlist:
		h.Waitlist.OnCallback(ctx, upd, p)
	case callbacks.GroupOrg:
		h.Organizer.OnCallback(ctx, upd, p)
	case callbacks.GroupOrgList:
		h.OrganizerList.OnCallback(ctx, upd, p)
	case callbacks.GroupOrgNotif:
		h.OrgNotify.OnCallback(ctx, upd, p)
	case callbacks.GroupOrgClose:
		h.Organizer.OnCloseCallback(ctx, upd, p)
	case callbacks.GroupOrgCancel:
		h.Organizer.OnCancelCallback(ctx, upd, p)
	case callbacks.GroupAI:
		if h.AIPick != nil {
			h.AIPick.OnCallback(ctx, upd, p)
		} else {
			h.Fallback.OnCallback(ctx, upd, p)
		}
	default:
		h.Fallback.OnCallback(ctx, upd, p)
	}
}
