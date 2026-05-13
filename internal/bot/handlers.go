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
)

// Handlers — корневой holder всех скоупированных обработчиков.
// Маршрутизация по типу Update реализована в Dispatcher.
//
// На День 4 присутствуют только Start и Fallback; остальные хендлеры
// добавятся в дни 5-12. Маршрутизатор RouteCallback заранее знает все группы,
// чтобы при появлении хендлера не пришлось переписывать switch.
type Handlers struct {
	Log *slog.Logger
	FSM *fsm.Manager

	Start    *handlers.StartHandler
	Fallback *handlers.FallbackHandler
}

// NewHandlers собирает Handlers. По мере добавления сервисов в дни 5-12 здесь
// будет расти количество параметров, либо мы перейдём на struct-конфиг.
func NewHandlers(api *maxclient.Client, log *slog.Logger, fsmMgr *fsm.Manager) *Handlers {
	h := &Handlers{
		Log: log.With("component", "handlers"),
		FSM: fsmMgr,
	}
	h.Start = handlers.NewStartHandler(api, fsmMgr, log)
	h.Fallback = handlers.NewFallbackHandler(api, fsmMgr, log)
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
// На День 4 живых обработчиков ещё мало, но switch покрывает все группы,
// чтобы не пропускать неизвестные payload'ы — для них fallback с logging.
func (h *Handlers) RouteCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	p := callbacks.Parse(upd.Callback.Payload)
	switch p.Group {
	case callbacks.GroupMain:
		h.Start.OnMainMenu(ctx, upd)
	case callbacks.GroupBack:
		h.Start.OnMainMenu(ctx, upd)
	default:
		// День 5+: появятся ev/reg/my/cancel/wl/org/...
		h.Fallback.OnCallback(ctx, upd, p)
	}
}
