package handlers

import (
	"context"
	"errors"
	"fmt"
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

const aiPageSize = 3

// AIPickHandler — «Подобрать через AI» в главном меню.
//
// UX:
//  1. main → callback ai:pick → AIAskInterest + state=ai_pick_intent.
//  2. text input → RecommendEvents(text, openEvents) → список рекомендаций
//     с inline-кнопками «Записаться» на каждое, постраничная навигация.
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

// OnCallback — ai:pick и ai:page.
func (h *AIPickHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "pick":
		if h.ai == nil {
			h.fallbackList(ctx, chatID)
			return
		}
		_ = h.fsm.Save(ctx, userMaxID, fsm.StateAIPickIntent, fsm.UserFSMContext{})
		if err := h.api.SendText(ctx, chatID, messages.AIAskInterest()); err != nil {
			h.log.Error("send ai ask failed", "err", err)
		}

	case "page":
		offset := int(p.ArgInt64(0))
		snap, _ := h.fsm.Load(ctx, userMaxID)
		if len(snap.Context.AIRecommIDs) == 0 {
			// Рекомендации протухли — показываем главное меню.
			if err := h.api.SendTextWithKeyboard(ctx, chatID,
				messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
				h.log.Error("send fallback failed", "err", err)
			}
			return
		}
		h.showAIPage(ctx, chatID, userMaxID, snap.Context.AIRecommIDs, offset)
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

	if len(recs) == 0 {
		_ = h.fsm.Reset(ctx, userMaxID)
		h.sendText(ctx, chatID, "По вашему запросу ничего не найдено. Покажу общий список.")
		h.fallbackList(ctx, chatID)
		return
	}

	// Сохраняем ID рекомендаций в FSM для постраничной навигации.
	ids := make([]int64, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.EventID)
	}
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateAIResults, fsm.UserFSMContext{
		AIRecommIDs: ids,
		AIOffset:    0,
	})

	h.showAIPage(ctx, chatID, userMaxID, ids, 0)
}

// showAIPage показывает страницу AI-рекомендаций с пагинацией.
func (h *AIPickHandler) showAIPage(ctx context.Context, chatID, userMaxID int64, ids []int64, offset int) {
	total := len(ids)
	end := offset + aiPageSize
	if end > total {
		end = total
	}
	pageIDs := ids[offset:end]
	hasMore := end < total
	hasPrev := offset > 0

	totalPages := (total + aiPageSize - 1) / aiPageSize
	currentPage := offset/aiPageSize + 1

	var sb strings.Builder
	kb := &maxbot.Keyboard{}

	for _, id := range pageIDs {
		ev, err := h.events.Get(ctx, id)
		if err != nil || ev == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("• %s\n", ev.Title))
		kb.AddRow().AddCallback(ev.Title, schemes.POSITIVE, callbacks.EventShow(id))
	}

	// Навигационный ряд.
	if hasPrev || hasMore {
		navRow := kb.AddRow()
		if hasPrev {
			navRow.AddCallback("◀ Назад", schemes.DEFAULT, callbacks.AIPage(offset-aiPageSize))
		}
		if hasMore {
			navRow.AddCallback("Вперёд ▶", schemes.DEFAULT, callbacks.AIPage(offset+aiPageSize))
		}
	}
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())

	text := messages.AIRecommendationPage(currentPage, totalPages, sb.String())
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
		h.log.Error("send ai page failed", "err", err)
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
	kb := keyboards.EventList(events, 0, total > 8, "")
	if err := h.api.SendTextWithKeyboard(ctx, chatID, sb.String(), kb); err != nil {
		h.log.Error("send fallback list failed", "err", err)
	}
}

func (h *AIPickHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}
