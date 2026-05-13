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
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// CancelHandler обрабатывает callback'и группы "cancel:" (двухшаговая отмена).
//
// UX:
//  1. Из «Моя запись» → «Отменить запись» → callback cancel:ask:<regID>
//     → CancelAsk + кнопки Yes/No.
//  2. cancel:yes:<regID> → проверка ownership → service.Registration.Cancel
//     → CancelDone + MainMenu. Если promote состоялся — promoted-user
//     получит уведомление через scheduler (День 12/16).
//  3. cancel:no:<regID> → возврат в меню.
//
// FSM: на шаге ask сохраняем CancelRegID в Context и переводим в
// StateCancelConfirmation — это защищает yes-callback от устаревших кнопок
// или подмены regID злоумышленником.
type CancelHandler struct {
	api    *maxclient.Client
	fsm    *fsm.Manager
	reg    service.Registration
	users  service.User
	events service.Event
	log    *slog.Logger
}

// NewCancelHandler — конструктор.
func NewCancelHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	reg service.Registration, users service.User, events service.Event, log *slog.Logger,
) *CancelHandler {
	return &CancelHandler{
		api:    api,
		fsm:    fsmMgr,
		reg:    reg,
		users:  users,
		events: events,
		log:    log.With("handler", "cancel"),
	}
}

// OnCallback маршрутизирует ask/yes/no.
func (h *CancelHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	regID := p.ArgInt64(0)
	if regID <= 0 {
		h.sendFallback(ctx, chatID)
		return
	}

	switch p.Action {
	case "ask":
		h.onAsk(ctx, chatID, userMaxID, regID)
	case "yes":
		h.onYes(ctx, chatID, userMaxID, regID)
	case "no":
		h.onNo(ctx, chatID, userMaxID)
	default:
		h.log.Debug("unknown cancel action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

func (h *CancelHandler) onAsk(ctx context.Context, chatID, userMaxID, regID int64) {
	reg, err := h.findUserReg(ctx, userMaxID, regID)
	if err != nil {
		h.log.Error("cancel ask: lookup failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if reg == nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.MyRegistrationEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send no reg failed", "err", err)
		}
		return
	}
	ev, err := h.events.Get(ctx, reg.EventID)
	if err != nil {
		h.log.Error("cancel ask: get event failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	snap, _ := h.fsm.Load(ctx, userMaxID)
	snap.Context.CancelRegID = regID
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateCancelConfirmation, snap.Context)

	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.CancelAsk(ev), keyboards.CancelConfirm(regID)); err != nil {
		h.log.Error("send cancel ask failed", "err", err)
	}
}

func (h *CancelHandler) onYes(ctx context.Context, chatID, userMaxID, regID int64) {
	snap, err := h.fsm.Load(ctx, userMaxID)
	if err != nil || snap.State != fsm.StateCancelConfirmation || snap.Context.CancelRegID != regID {
		// Защита от устаревшей кнопки или подмены regID.
		h.sendFallback(ctx, chatID)
		return
	}
	reg, err := h.findUserReg(ctx, userMaxID, regID)
	if err != nil {
		h.log.Error("cancel yes: lookup failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if reg == nil {
		h.sendFallback(ctx, chatID)
		return
	}

	_, err = h.reg.Cancel(ctx, regID, service.CancelByUser)
	switch {
	case errors.Is(err, service.ErrNotRegistered):
		h.sendText(ctx, chatID, messages.MyRegistrationEmpty())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case err != nil:
		h.log.Error("cancel failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.CancelDone(), keyboards.MainMenu()); err != nil {
		h.log.Error("send cancel done failed", "err", err)
	}
}

func (h *CancelHandler) onNo(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		"Хорошо, оставляем запись. Возвращаемся в меню.", keyboards.MainMenu()); err != nil {
		h.log.Error("send cancel no failed", "err", err)
	}
}

// findUserReg ищет активную запись regID среди записей пользователя по maxUserID.
// Возвращает nil если запись не найдена или принадлежит другому пользователю
// (защита от подмены regID в payload).
func (h *CancelHandler) findUserReg(ctx context.Context, maxUserID, regID int64) (*domain.Registration, error) {
	u, err := h.users.GetByMaxID(ctx, maxUserID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	regs, err := h.reg.ListActiveByUser(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range regs {
		if r.ID == regID {
			return r, nil
		}
	}
	return nil, nil
}

// --- helpers ---

func (h *CancelHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}

func (h *CancelHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

func (h *CancelHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}
