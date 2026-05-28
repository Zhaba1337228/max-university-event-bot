package handlers

import (
	"context"
	"log/slog"
	"strings"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// AIFAQHandler — «Задать вопрос ИИ» в главном меню.
type AIFAQHandler struct {
	api        *maxclient.Client
	fsm        *fsm.Manager
	ai         service.AI
	events     service.Event
	faqEnabled bool
	log        *slog.Logger
}

func NewAIFAQHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	ai service.AI, events service.Event, faqEnabled bool, log *slog.Logger,
) *AIFAQHandler {
	return &AIFAQHandler{
		api:        api,
		fsm:        fsmMgr,
		ai:         ai,
		events:     events,
		faqEnabled: faqEnabled,
		log:        log.With("handler", "ai_faq"),
	}
}

// Enabled сообщает, включён ли FAQ — используется для показа кнопки в меню.
func (h *AIFAQHandler) Enabled() bool { return h.faqEnabled }

// OnCallback — ai:faq.
func (h *AIFAQHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, _ callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	if !h.faqEnabled || h.ai == nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.AIFAQUnavailable(), keyboards.MainMenu()); err != nil {
			h.log.Error("send faq unavailable failed", "err", err)
		}
		return
	}

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateAIFAQIntent, fsm.UserFSMContext{})
	if err := h.api.SendText(ctx, chatID, messages.AIAskFAQ()); err != nil {
		h.log.Error("send faq ask failed", "err", err)
	}
}

// OnText — обработка вопроса в state ai_faq_intent.
func (h *AIFAQHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate, _ fsm.Snapshot) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	question := strings.TrimSpace(upd.Message.Body.Text)

	_ = h.fsm.Reset(ctx, userMaxID)

	if h.ai == nil || question == "" {
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.AIFAQUnavailable(), keyboards.MainMenu()); err != nil {
			h.log.Error("send faq unavailable failed", "err", err)
		}
		return
	}

	items, _, err := h.events.ListOpen(ctx, 50, 0)
	if err != nil || len(items) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.AIFAQUnavailable(), keyboards.MainMenu()); err != nil {
			h.log.Error("send faq unavailable failed", "err", err)
		}
		return
	}

	pool := make([]*domain.Event, 0, len(items))
	for _, it := range items {
		pool = append(pool, it.Event)
	}

	answer, err := h.ai.AnswerFAQ(ctx, question, pool)
	if err != nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.AIFAQUnavailable(), keyboards.MainMenu()); err != nil {
			h.log.Error("send faq unavailable failed", "err", err)
		}
		return
	}

	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.AIFAQAnswer(answer), keyboards.MainMenu()); err != nil {
		h.log.Error("send faq answer failed", "err", err)
	}
}
