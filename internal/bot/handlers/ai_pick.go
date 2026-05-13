package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// AIPickHandler — «Подобрать через AI» в главном меню.
//
// UX:
//  1. main → callback ai:pick → AIAskInterest + state=ai_pick_intent.
//  2. text input → RecommendEvents(text, openEvents) → список рекомендаций
//     с inline-кнопками «Записаться» на каждое.
//  3. На fallback (AI off / parse fail) → AIUnavailable + обычный список.
type AIPickHandler struct {
	api    *maxclient.Client
	fsm    *fsm.Manager
	ai     service.AI // опционально
	events service.Event
	log    *slog.Logger
}

// NewAIPickHandler — конструктор. ai может быть nil — handler сразу уйдёт в fallback.
func NewAIPickHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	ai service.AI, events service.Event, log *slog.Logger,
) *AIPickHandler {
	return &AIPickHandler{
		api:    api,
		fsm:    fsmMgr,
		ai:     ai,
		events: events,
		log:    log.With("handler", "ai_pick"),
	}
}

// OnCallback — ai:pick.
func (h *AIPickHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}
	if p.Action != "pick" {
		return
	}

	if h.ai == nil {
		// AI выключен — сразу обычный список.
		h.fallbackList(ctx, chatID)
		return
	}

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateAIPickIntent, fsm.UserFSMContext{})
	if err := h.api.SendText(ctx, chatID, messages.AIAskInterest()); err != nil {
		h.log.Error("send ai ask failed", "err", err)
	}
}

// OnText — обработка интереса в state ai_pick_intent.
func (h *AIPickHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate, _ fsm.Snapshot) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	interest := strings.TrimSpace(upd.Message.Body.Text)

	if h.ai == nil || interest == "" {
		_ = h.fsm.Reset(ctx, userMaxID)
		h.fallbackList(ctx, chatID)
		return
	}

	// Берём первые 50 открытых событий.
	items, _, err := h.events.ListOpen(ctx, 50, 0)
	if err != nil || len(items) == 0 {
		_ = h.fsm.Reset(ctx, userMaxID)
		h.fallbackList(ctx, chatID)
		return
	}

	pool := make([]*domain.Event, 0, len(items))
	for _, it := range items {
		pool = append(pool, it.Event)
	}

	recs, err := h.ai.RecommendEvents(ctx, interest, pool)
	if errors.Is(err, service.ErrAIUnavailable) || err != nil {
		_ = h.fsm.Reset(ctx, userMaxID)
		h.sendText(ctx, chatID, messages.AIUnavailable())
		h.fallbackList(ctx, chatID)
		return
	}

	_ = h.fsm.Reset(ctx, userMaxID)

	// Собираем текст + клавиатуру с кнопкой «Записаться» на каждую рекомендацию.
	var sb strings.Builder
	sb.WriteString("Подобрал для вас:\n\n")
	kb := &maxbot.Keyboard{}
	for i, r := range recs {
		sb.WriteString(strings.TrimSpace(r.Title))
		sb.WriteString("\n")
		if r.Reason != "" {
			sb.WriteString(r.Reason)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		_ = i
		kb.AddRow().AddCallback(r.Title, schemes.POSITIVE, callbacks.EventShow(r.EventID))
	}
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())

	if err := h.api.SendTextWithKeyboard(ctx, chatID, sb.String(), kb); err != nil {
		h.log.Error("send ai recs failed", "err", err)
	}
}

// fallbackList — обычный список событий (когда AI недоступен).
func (h *AIPickHandler) fallbackList(ctx context.Context, chatID int64) {
	items, total, err := h.events.ListOpen(ctx, 8, 0)
	if err != nil || len(items) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.EventListEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send empty fallback failed", "err", err)
		}
		return
	}

	var sb strings.Builder
	sb.WriteString(messages.EventListHeader())
	sb.WriteString("\n\n")
	events := make([]*domain.Event, 0, len(items))
	for i, it := range items {
		sb.WriteString(messages.EventListItem(i, it.Event))
		sb.WriteString("\n")
		events = append(events, it.Event)
	}
	kb := keyboards.EventList(events, 0, total > 8)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, sb.String(), kb); err != nil {
		h.log.Error("send fallback list failed", "err", err)
	}
}

func (h *AIPickHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}
