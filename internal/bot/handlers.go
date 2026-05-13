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

	Start        *handlers.StartHandler
	Fallback     *handlers.FallbackHandler
	Events       *handlers.EventsHandler
	Registration *handlers.RegistrationHandler
	MyReg        *handlers.MyRegistrationHandler
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
	WaitlistEnabled bool
	PolicyVersion   string
}

// NewHandlers собирает Handlers по конфигу.
func NewHandlers(cfg HandlersConfig) *Handlers {
	h := &Handlers{
		Log: cfg.Log.With("component", "handlers"),
		FSM: cfg.FSM,
	}
	h.Start = handlers.NewStartHandler(cfg.API, cfg.FSM, cfg.Log)
	h.Fallback = handlers.NewFallbackHandler(cfg.API, cfg.FSM, cfg.Log)
	h.Events = handlers.NewEventsHandler(cfg.API, cfg.FSM, cfg.Events, cfg.Log, cfg.WaitlistEnabled)
	h.Registration = handlers.NewRegistrationHandler(cfg.API, cfg.FSM,
		cfg.Registration, cfg.Users, cfg.Events, cfg.Log, cfg.PolicyVersion)
	h.MyReg = handlers.NewMyRegistrationHandler(cfg.API, cfg.FSM, cfg.Users, cfg.Log)
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
	case "/forget_me":
		h.MyReg.OnForgetMeCmd(ctx, upd)
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
	default:
		h.Fallback.OnText(ctx, upd)
	}
}

// RouteCallback маршрутизирует MessageCallbackUpdate по группе payload'а.
//
// Группы, для которых ещё нет реального handler'а (дни 7-12), уходят в
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
	default:
		// Дни 7-12: cancel/wl/org/orglist/orgnotif/orgclose/admin/ai
		h.Fallback.OnCallback(ctx, upd, p)
	}
}
