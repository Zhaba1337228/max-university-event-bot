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
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// OrganizerHandler — /organizer menu, список своих событий, карточка
// организатора (статистика + кнопки рассылки/закрытия/AI-сводки).
//
// Каждый обработчик начинается с RequireOrganizer / RequireEventOwner —
// см. план §19.4 (RBAC). Если проверка не прошла → OrganizerNoAccess
// + main menu без раскрытия деталей.
type OrganizerHandler struct {
	api        *maxclient.Client
	fsm        *fsm.Manager
	role       service.Role
	events     service.Event
	ai         service.AI // опционально (для org:ai_summary)
	eventsRepo repo.EventRepo
	db         repo.Querier
	log        *slog.Logger
}

// NewOrganizerHandler — конструктор. ai, eventsRepo, db опциональны.
func NewOrganizerHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	role service.Role, events service.Event, ai service.AI,
	eventsRepo repo.EventRepo, db repo.Querier,
	log *slog.Logger,
) *OrganizerHandler {
	return &OrganizerHandler{
		api:        api,
		fsm:        fsmMgr,
		role:       role,
		events:     events,
		ai:         ai,
		eventsRepo: eventsRepo,
		db:         db,
		log:        log.With("handler", "organizer"),
	}
}

// OnEntryCmd — команда /organizer.
func (h *OrganizerHandler) OnEntryCmd(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	h.showMenu(ctx, chatID, userMaxID)
}

// OnCallback маршрутизирует org:* колбэки.
func (h *OrganizerHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "entry":
		h.showMenu(ctx, chatID, userMaxID)
	case "stats":
		eventID := p.ArgInt64(0)
		h.showStats(ctx, chatID, userMaxID, eventID)
	case "ai_summary":
		eventID := p.ArgInt64(0)
		h.showAISummary(ctx, chatID, userMaxID, eventID)
	default:
		h.log.Debug("unknown org action", "action", p.Action)
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
			h.log.Error("send fallback failed", "err", err)
		}
	}
}

// showMenu — список своих мероприятий организатора.
func (h *OrganizerHandler) showMenu(ctx context.Context, chatID, userMaxID int64) {
	user, err := h.role.RequireOrganizer(ctx, userMaxID)
	if errors.Is(err, service.ErrNotOrganizer) {
		// PII-safe лог: только id, без имени/почты.
		h.log.Warn("organizer access denied", "user_id", userMaxID)
		h.sendNoAccess(ctx, chatID)
		return
	}
	if err != nil {
		h.log.Error("require organizer failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	events, err := h.events.ListByOrganizer(ctx, user.ID)
	if err != nil {
		h.log.Error("list by organizer failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerMenu, fsm.UserFSMContext{})

	if len(events) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.OrganizerNoEvents(), keyboards.MainMenu()); err != nil {
			h.log.Error("send organizer no events failed", "err", err)
		}
		return
	}
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.OrganizerMenu(), keyboards.OrganizerEventList(events)); err != nil {
		h.log.Error("send organizer menu failed", "err", err)
	}
}

// showStats — карточка организатора для конкретного eventID:
// сводка + клавиатура «Участники/CSV/Рассылка/Закрыть/AI».
func (h *OrganizerHandler) showStats(ctx context.Context, chatID, userMaxID, eventID int64) {
	if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}
	ev, err := h.events.Get(ctx, eventID)
	if err != nil {
		h.log.Error("get event failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	stats, err := h.events.Stats(ctx, eventID)
	if err != nil {
		h.log.Error("stats failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerEventList,
		fsm.UserFSMContext{OrganizerEventID: eventID})

	text := messages.OrganizerStats(ev, stats)
	kb := keyboards.OrganizerEventActions(eventID, ev.Status)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
		h.log.Error("send organizer stats failed", "err", err)
	}
}

// showAISummary — AI-сводка по событию.
// Если AI недоступен — graceful fallback в обычную текстовую статистику.
func (h *OrganizerHandler) showAISummary(ctx context.Context, chatID, userMaxID, eventID int64) {
	if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}
	ev, err := h.events.Get(ctx, eventID)
	if err != nil || ev == nil {
		h.handleAccessErr(ctx, chatID, service.ErrEventNotFound)
		return
	}
	stats, err := h.events.Stats(ctx, eventID)
	if err != nil {
		h.sendError(ctx, chatID)
		return
	}

	if h.ai == nil {
		h.sendText(ctx, chatID, messages.AIUnavailable())
		// показываем обычную статистику как fallback
		text := messages.OrganizerStats(ev, stats)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, text,
			keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
			h.log.Error("send fallback stats failed", "err", err)
		}
		return
	}

	summary, err := h.ai.OrganizerSummary(ctx, ev, stats)
	if errors.Is(err, service.ErrAIUnavailable) || err != nil {
		h.sendText(ctx, chatID, messages.AIUnavailable())
		text := messages.OrganizerStats(ev, stats)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, text,
			keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
			h.log.Error("send fallback stats failed", "err", err)
		}
		return
	}

	text := "AI-сводка по «" + ev.Title + "»:\n\n" + summary
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text,
		keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
		h.log.Error("send ai summary failed", "err", err)
	}
}

func (h *OrganizerHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}

func (h *OrganizerHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

// handleAccessErr — единая трактовка ошибок RequireEventOwner.
func (h *OrganizerHandler) handleAccessErr(ctx context.Context, chatID int64, err error) {
	switch {
	case errors.Is(err, service.ErrNotOrganizer), errors.Is(err, service.ErrNotEventOwner):
		h.sendNoAccess(ctx, chatID)
	case errors.Is(err, service.ErrEventNotFound):
		if e := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.EventNotAvailable(), keyboards.MainMenu()); e != nil {
			h.log.Error("send event not avail failed", "err", e)
		}
	default:
		h.log.Error("organizer access check failed", "err", err)
		h.sendError(ctx, chatID)
	}
}

// OnCloseCallback обрабатывает orgclose:* — закрытие и открытие регистрации из бота.
// Ранее эти callback'и уходили в fallback (в RouteCallback был TODO).
func (h *OrganizerHandler) OnCloseCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	eventID := p.ArgInt64(0)
	if eventID <= 0 {
		h.sendFallback(ctx, chatID)
		return
	}

	switch p.Action {
	case "ask":
		if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
			h.handleAccessErr(ctx, chatID, err)
			return
		}
		ev, err := h.events.Get(ctx, eventID)
		if err != nil || ev == nil {
			h.sendError(ctx, chatID)
			return
		}
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.OrganizerCloseAsk(ev), keyboards.OrganizerCloseConfirm(eventID)); err != nil {
			h.log.Error("send close ask failed", "err", err)
		}

	case "yes":
		if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
			h.handleAccessErr(ctx, chatID, err)
			return
		}
		if h.eventsRepo != nil && h.db != nil {
			if err := h.eventsRepo.UpdateStatus(ctx, h.db, eventID, domain.EventStatusClosed); err != nil {
				h.log.Error("close event failed", "err", err)
				h.sendError(ctx, chatID)
				return
			}
		}
		ev, _ := h.events.Get(ctx, eventID)
		if ev == nil {
			h.sendText(ctx, chatID, messages.OrganizerClosed())
			return
		}
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.OrganizerClosed(), keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
			h.log.Error("send closed failed", "err", err)
		}

	case "open_ask":
		if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
			h.handleAccessErr(ctx, chatID, err)
			return
		}
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			"Открыть регистрацию снова?", keyboards.OrganizerOpenConfirm(eventID)); err != nil {
			h.log.Error("send open ask failed", "err", err)
		}

	case "open_yes":
		if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
			h.handleAccessErr(ctx, chatID, err)
			return
		}
		if h.eventsRepo != nil && h.db != nil {
			if err := h.eventsRepo.UpdateStatus(ctx, h.db, eventID, domain.EventStatusOpen); err != nil {
				h.log.Error("open event failed", "err", err)
				h.sendError(ctx, chatID)
				return
			}
		}
		ev, _ := h.events.Get(ctx, eventID)
		if ev == nil {
			h.sendText(ctx, chatID, messages.OrganizerOpened())
			return
		}
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.OrganizerOpened(), keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
			h.log.Error("send opened failed", "err", err)
		}

	default:
		h.sendFallback(ctx, chatID)
	}
}

func (h *OrganizerHandler) sendNoAccess(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.OrganizerNoAccess(), keyboards.MainMenu()); err != nil {
		h.log.Error("send no access failed", "err", err)
	}
}

func (h *OrganizerHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}
