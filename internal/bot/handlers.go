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

	Start    *handlers.StartHandler
	Fallback *handlers.FallbackHandler
	Events   *handlers.EventsHandler
}

// HandlersConfig — групповая инициализация. По мере роста зависимостей удобнее
// передавать конфиг, чем 10 позиционных параметров.
type HandlersConfig struct {
	API             *maxclient.Client
	Log             *slog.Logger
	FSM             *fsm.Manager
	Events          service.Event
	WaitlistEnabled bool
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
	return h
}

// RouteMessage маршрутизирует MessageCreatedUpdate:
//   - команды (/start, /help, /organizer, /admin_login, /forget_me) — приоритет;
//   - иначе — смотрим текущее FSM-состояние; если оно ожидает текст —
//     отдаём соответствующему handler'у; иначе — fallback.
func (h *Handlers) RouteMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	cmd := strings.ToLower(strings.TrimSpace(upd.Message.Body.Text))
	cmd = strings.SplitN(cmd, " ", 2)[0] // отделяем аргументы команды

	switch cmd {
	case "/start":
		h.Start.OnStart(ctx, upd)
		return
	case "/help":
		h.Start.OnHelp(ctx, upd)
		return
	}

	// На День 4 — для текстовых сообщений вне команд показываем fallback.
	// Реальная FSM-маршрутизация (reg_full_name, reg_contact и т.д.)
	// появится в Дне 6 и расширит этот switch.
	h.Fallback.OnText(ctx, upd)
}

// RouteCallback маршрутизирует MessageCallbackUpdate по группе payload'а.
//
// Группы, для которых ещё нет реального handler'а (дни 6-12), уходят в
// Fallback с понятным сообщением «Эта кнопка устарела».
func (h *Handlers) RouteCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	p := callbacks.Parse(upd.Callback.Payload)
	switch p.Group {
	case callbacks.GroupMain:
		h.Start.OnMainMenu(ctx, upd)
	case callbacks.GroupBack:
		h.Start.OnMainMenu(ctx, upd)
	case callbacks.GroupEvent:
		h.Events.OnCallback(ctx, upd, p)
	default:
		// Дни 6-12: появятся reg/my/cancel/wl/org/orglist/orgnotif/orgclose/admin/ai
		h.Fallback.OnCallback(ctx, upd, p)
	}
}
