package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// WaitlistHandler — кнопка «Встать в лист ожидания» на карточке события.
//
// Делегирует основной FSM сценарию registration handler'а: пользователь
// проходит ровно те же шаги (consent → ФИО → контакт → направление →
// подтверждение), а уже сервис.Registration.Register решает, что мест
// нет и переводит запись в waitlist. Этот файл — только подсказка
// «вы будете в листе ожидания».
//
// Также обрабатывает wl:yes / wl:no — ответы на промоушен из waitlist
// (День 8/16). В MVP промоушен auto, без user-confirm, но если в будущем
// захотим with-confirm — payload'ы уже определены.
type WaitlistHandler struct {
	api *maxclient.Client
	fsm *fsm.Manager
	reg *RegistrationHandler // делегируем onStart-логику
	log *slog.Logger
}

// NewWaitlistHandler — конструктор. Принимает уже сконструированный
// RegistrationHandler чтобы переиспользовать его onStart.
func NewWaitlistHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	reg *RegistrationHandler, log *slog.Logger,
) *WaitlistHandler {
	return &WaitlistHandler{
		api: api,
		fsm: fsmMgr,
		reg: reg,
		log: log.With("handler", "waitlist"),
	}
}

// OnCallback маршрутизирует группу "wl:".
func (h *WaitlistHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "join":
		eventID := p.ArgInt64(0)
		if eventID <= 0 {
			h.sendFallback(ctx, chatID)
			return
		}
		// Делегируем onStart — он сам разберётся с consent + ФИО + контактом.
		// service.Registration.Register увидит нулевые места и создаст waitlist.
		h.reg.onStart(ctx, chatID, userMaxID, eventID)

	case "yes", "no":
		// Promote-flow с явным подтверждением — не используется в MVP
		// (promote auto в Cancel), но обработчик молчаливо игнорирует
		// чтобы старые кнопки не вызывали FallbackUnknown.
		_ = errors.New("noop")
		if err := h.api.SendText(ctx, chatID,
			"Подтверждение из листа ожидания пока недоступно."); err != nil {
			h.log.Error("send wl noop failed", "err", err)
		}

	default:
		h.log.Debug("unknown wl action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

// --- helpers ---

func (h *WaitlistHandler) sendFallback(ctx context.Context, chatID int64) {
	_ = chatID
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

// _ — placeholder.
var _ = fsm.StateMainMenu
var _ service.Registration = (service.Registration)(nil)
