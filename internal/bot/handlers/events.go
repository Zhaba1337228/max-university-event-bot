package handlers

import (
	"context"
	"errors"
	"fmt"
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

// EventsHandler — обработчик кнопок «список мероприятий» и «карточка».
type EventsHandler struct {
	api    *maxclient.Client
	fsm    *fsm.Manager
	events service.Event
	users  service.User
	regs   service.Registration
	log    *slog.Logger
	// businessWaitlistEnabled определяет, рисовать ли «Встать в лист ожидания»
	// в карточке когда мест нет. Берётся из cfg.Business.WaitlistEnabled.
	businessWaitlistEnabled bool
}

// NewEventsHandler — конструктор.
func NewEventsHandler(api *maxclient.Client, fsmMgr *fsm.Manager, ev service.Event,
	users service.User, regs service.Registration, log *slog.Logger, waitlistEnabled bool,
) *EventsHandler {
	return &EventsHandler{
		api:                     api,
		fsm:                     fsmMgr,
		events:                  ev,
		users:                   users,
		regs:                    regs,
		log:                     log.With("handler", "events"),
		businessWaitlistEnabled: waitlistEnabled,
	}
}

// OnCallback маршрутизирует callback'и группы "ev:" — list, show, details.
func (h *EventsHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	// Закрываем спиннер «крутится» сразу, чтобы UI отвечал быстро.
	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	chatID := upd.Message.Recipient.ChatId
	userID := upd.Callback.User.UserId

	switch p.Action {
	case "list":
		offset := int(p.ArgInt64(0))
		h.showList(ctx, chatID, userID, offset)
	case "show":
		eventID := p.ArgInt64(0)
		h.showCard(ctx, chatID, userID, eventID)
	case "details":
		eventID := p.ArgInt64(0)
		h.showDetails(ctx, chatID, userID, eventID)
	case "filter":
		filterFormat := p.ArgString(0)
		snap, _ := h.fsm.Load(ctx, userID)
		snap.Context.EventFilter = filterFormat
		snap.Context.Offset = 0
		_ = h.fsm.Save(ctx, userID, fsm.StateEventList, snap.Context)
		h.showList(ctx, chatID, userID, 0)
	default:
		h.log.Debug("unknown ev action", "action", p.Action)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
			h.log.Error("send fallback failed", "err", err)
		}
	}
}

func (h *EventsHandler) showList(ctx context.Context, chatID, userID int64, offset int) {
	pageSize := keyboards.PageSize()

	// Загружаем FSM для получения активного фильтра.
	snap, _ := h.fsm.Load(ctx, userID)
	filter := snap.Context.EventFilter

	var pageItems []service.EventWithFree
	var hasMore bool

	if filter != "" {
		// Фильтруем по формату: грузим большой батч, режем в памяти.
		all, _, err := h.events.ListOpen(ctx, 200, 0)
		if err != nil {
			h.log.Error("list events failed", "err", err)
			if sendErr := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); sendErr != nil {
				h.log.Error("send error msg failed", "err", sendErr)
			}
			return
		}
		var filtered []service.EventWithFree
		for _, it := range all {
			if string(it.Event.Format) == filter {
				filtered = append(filtered, it)
			}
		}
		start := offset
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + pageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		pageItems = filtered[start:end]
		hasMore = end < len(filtered)
	} else {
		items, total, err := h.events.ListOpen(ctx, pageSize, offset)
		if err != nil {
			h.log.Error("list events failed", "err", err)
			if sendErr := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); sendErr != nil {
				h.log.Error("send error msg failed", "err", sendErr)
			}
			return
		}
		pageItems = items
		hasMore = offset+pageSize < total
	}

	if len(pageItems) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.EventListEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send empty list failed", "err", err)
		}
		return
	}

	// Сохраняем offset в FSM, чтобы из карточки кнопка «Назад к списку»
	// возвращала пользователя на ту же страницу.
	snap.Context.Offset = offset
	_ = h.fsm.Save(ctx, userID, fsm.StateEventList, snap.Context)

	// Текстовый список + клавиатура.
	var b strings.Builder
	b.WriteString(messages.EventListHeader())
	b.WriteString("\n\n")
	events := make([]*domain.Event, 0, len(pageItems))
	for i, it := range pageItems {
		b.WriteString(messages.EventListItem(i+offset, it.Event))
		if it.FreeSeats == 0 {
			b.WriteString(" (мест нет)")
		}
		b.WriteString("\n")
		events = append(events, it.Event)
	}

	kb := keyboards.EventList(events, offset, hasMore, filter)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, b.String(), kb); err != nil {
		h.log.Error("send list failed", "err", err)
	}
}

func (h *EventsHandler) showCard(ctx context.Context, chatID, userID int64, eventID int64) {
	withFree, err := h.events.GetOpen(ctx, eventID)
	switch {
	case errors.Is(err, service.ErrEventNotFound):
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.EventNotAvailable(), keyboards.MainMenu()); err != nil {
			h.log.Error("send not-found failed", "err", err)
		}
		return
	case errors.Is(err, service.ErrEventClosed):
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.EventClosedNow(), keyboards.MainMenu()); err != nil {
			h.log.Error("send closed failed", "err", err)
		}
		return
	case err != nil:
		h.log.Error("get event failed", "err", err, "event_id", eventID)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
			h.log.Error("send error msg failed", "err", err)
		}
		return
	}

	// FSM: запомним текущее событие, чтобы reg-handler понял с чего стартовать.
	snap, _ := h.fsm.Load(ctx, userID)
	snap.Context.CurrentEventID = withFree.Event.ID
	_ = h.fsm.Save(ctx, userID, fsm.StateEventDetails, snap.Context)

	activeReg, err := h.activeRegistration(ctx, userID, withFree.Event.ID)
	if err != nil {
		h.log.Warn("lookup active registration failed", "err", err, "user_id", userID, "event_id", eventID)
	}

	text := messages.EventCard(withFree.Event, withFree.FreeSeats, activeReg)
	kb := keyboards.EventCard(withFree.Event.ID, withFree.FreeSeats, h.businessWaitlistEnabled, snap.Context.Offset, activeReg)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
		h.log.Error("send card failed", "err", err)
	}
}

func (h *EventsHandler) activeRegistration(ctx context.Context, userMaxID, eventID int64) (*domain.Registration, error) {
	if h.users == nil || h.regs == nil {
		return nil, nil
	}

	user, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil {
		return nil, fmt.Errorf("get user by max id: %w", err)
	}
	if user == nil {
		return nil, nil
	}

	reg, err := h.regs.GetActive(ctx, user.ID, eventID)
	if err != nil {
		return nil, fmt.Errorf("get active registration: %w", err)
	}
	return reg, nil
}

// showDetails — кнопка «Подробнее»: расширенная карточка со всем описанием и условиями.
func (h *EventsHandler) showDetails(ctx context.Context, chatID, userID int64, eventID int64) {
	ev, err := h.events.Get(ctx, eventID)
	if err != nil {
		h.log.Error("get event for details failed", "err", err, "event_id", eventID)
		if sendErr := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); sendErr != nil {
			h.log.Error("send error msg failed", "err", sendErr)
		}
		return
	}
	snap, _ := h.fsm.Load(ctx, userID)
	text := messages.EventDetails(ev)
	// Возвращаемся к карточке (краткой) — кнопка «Назад к карточке»
	kb := keyboards.EventDetailsBack(eventID, snap.Context.Offset)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
		h.log.Error("send details failed", "err", err)
	}
}
