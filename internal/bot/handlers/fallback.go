package handlers

import (
	"context"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
)

// FallbackHandler — обработчик «не понял команду».
// Срабатывает на:
//   - текстовые сообщения, не попавшие ни в одну команду и не ожидаемые
//     текущим FSM-состоянием;
//   - callback'и с неизвестной группой/действием.
//
// Назначение: не оставлять пользователя в подвисшем состоянии и
// возвращать его в главное меню.
type FallbackHandler struct {
	api *maxclient.Client
	fsm *fsm.Manager
	log *slog.Logger
}

// NewFallbackHandler — конструктор.
func NewFallbackHandler(api *maxclient.Client, fsmMgr *fsm.Manager, log *slog.Logger) *FallbackHandler {
	return &FallbackHandler{
		api: api,
		fsm: fsmMgr,
		log: log.With("handler", "fallback"),
	}
}

// OnText — текст, не распознанный как команда и не ожидаемый FSM.
func (h *FallbackHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userID := upd.Message.Sender.UserId
	h.log.Debug("fallback text",
		"user_id", userID,
		"text_len", len(upd.Message.Body.Text))

	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

// OnCallback — callback из неизвестной группы либо устаревший payload.
//
// Закрываем спиннер коротким уведомлением, чтобы пользователь не остался
// в подвешенном состоянии «кнопка нажата но ничего не происходит».
func (h *FallbackHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	h.log.Debug("fallback callback",
		"user_id", upd.Callback.User.UserId,
		"group", p.Group,
		"action", p.Action,
		"raw", p.Raw)

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, "Эта кнопка устарела"); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}
	chatID := upd.Message.Recipient.ChatId
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}
