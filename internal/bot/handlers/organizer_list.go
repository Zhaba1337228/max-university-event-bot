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
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// OrganizerListHandler — список участников события + (заглушка) CSV-экспорт.
//
// На MVP CSV отдаётся в виде текста (несколько сообщений), потому что
// api.Uploads.UploadMediaFromFile требует временный файл и тестируется
// сложнее. Реальный CSV-экспорт сделает веб-админка (День 13/14).
//
// Каждый handler-метод начинается с RequireEventOwner — RBAC §19.4.
type OrganizerListHandler struct {
	api    *maxclient.Client
	fsm    *fsm.Manager
	role   service.Role
	events service.Event
	// regsRepo нужен напрямую — service.Registration возвращает только
	// активные пары (user, event) пользователя, а здесь нужен список ВСЕХ
	// для админа. Прокидываем repo напрямую — это слабая связь, но проще,
	// чем тянуть отдельный «admin-only» сервис.
	regs    repo.RegistrationRepo
	querier repo.Querier
	log     *slog.Logger
}

// NewOrganizerListHandler — конструктор.
func NewOrganizerListHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	role service.Role, events service.Event,
	regs repo.RegistrationRepo, q repo.Querier, log *slog.Logger,
) *OrganizerListHandler {
	return &OrganizerListHandler{
		api:     api,
		fsm:     fsmMgr,
		role:    role,
		events:  events,
		regs:    regs,
		querier: q,
		log:     log.With("handler", "organizer_list"),
	}
}

// participantsPerPage — сколько участников показывать одним сообщением.
const participantsPerPage = 10

// OnCallback маршрутизирует orglist:* колбэки.
func (h *OrganizerListHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
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

	if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}

	switch p.Action {
	case "show":
		offset := int(p.ArgInt64(1))
		h.showPage(ctx, chatID, userMaxID, eventID, offset)
	case "csv":
		h.exportCSV(ctx, chatID, eventID)
	default:
		h.log.Debug("unknown orglist action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

func (h *OrganizerListHandler) showPage(ctx context.Context, chatID, userMaxID, eventID int64, offset int) {
	if offset < 0 {
		offset = 0
	}
	ev, err := h.events.Get(ctx, eventID)
	if err != nil {
		h.log.Error("get event failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	regs, err := h.regs.ListByEvent(ctx, h.querier, eventID, domain.RegStatusRegistered,
		participantsPerPage, offset)
	if err != nil {
		h.log.Error("list participants failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	if len(regs) == 0 && offset == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			"Записанных участников пока нет.",
			keyboards.OrganizerEventActions(eventID, ev.Status)); err != nil {
			h.log.Error("send empty participants failed", "err", err)
		}
		return
	}

	// Считаем total через CountByEvent — для навигации.
	total, err := h.regs.CountByEvent(ctx, h.querier, eventID, domain.RegStatusRegistered)
	if err != nil {
		h.log.Warn("count participants failed", "err", err)
		total = offset + len(regs)
	}

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerParticipants,
		fsm.UserFSMContext{OrganizerEventID: eventID, Offset: offset})

	var b strings.Builder
	fmt.Fprintf(&b, "Участники: %s\n\n", ev.Title)
	for i, r := range regs {
		fmt.Fprintf(&b, "%d. %s — %s\n", offset+i+1,
			r.FullNameSnapshot, maskContact(r.ContactSnapshot))
	}
	fmt.Fprintf(&b, "\nВсего записано: %d", total)

	kb := keyboards.OrganizerParticipants(eventID, offset, total, participantsPerPage)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, b.String(), kb); err != nil {
		h.log.Error("send participants failed", "err", err)
	}
}

// exportCSV — на MVP отправляет CSV-текст в чат (без attachment).
// Реальный файл-аттач делается через api.Uploads.UploadMediaFromFile —
// добавим, когда понадобится организаторам массово.
func (h *OrganizerListHandler) exportCSV(ctx context.Context, chatID, eventID int64) {
	regs, err := h.regs.ListByEvent(ctx, h.querier, eventID, "", 1000, 0)
	if err != nil {
		h.log.Error("export csv: list failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if len(regs) == 0 {
		if err := h.api.SendText(ctx, chatID, "Участников нет — экспортировать нечего."); err != nil {
			h.log.Error("send empty csv failed", "err", err)
		}
		return
	}

	var b strings.Builder
	b.WriteString("id,status,full_name,contact,interest_program,registered_at\n")
	for _, r := range regs {
		interest := ""
		if r.InterestProgram != nil {
			interest = *r.InterestProgram
		}
		regAt := ""
		if r.RegisteredAt != nil {
			regAt = r.RegisteredAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		fmt.Fprintf(&b, "%d,%s,%s,%s,%s,%s\n",
			r.ID, r.Status,
			csvEscape(r.FullNameSnapshot),
			csvEscape(r.ContactSnapshot),
			csvEscape(interest),
			regAt)
	}

	text := b.String()
	if len(text) > 3500 {
		// MAX лимит 4000 символов сообщения. Если CSV больше — обрезаем
		// и предупреждаем (полный CSV — через веб-админку, День 13).
		text = text[:3500] + "\n... (обрезано, полный экспорт — через веб-админку)"
	}
	if err := h.api.SendText(ctx, chatID, "CSV (текст):\n\n"+text); err != nil {
		h.log.Error("send csv failed", "err", err)
	}
}

// maskContact — для UI организатора показываем контакт частично замаскированным.
// Полный контакт виден только при отдельном кликe (на MVP — не реализован, см. план §19.5).
func maskContact(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "***"
	}
	// email: показываем 2 символа + *** + домен
	if i := strings.Index(s, "@"); i > 0 {
		head := s[:i]
		if len(head) > 2 {
			head = head[:2] + "***"
		}
		return head + s[i:]
	}
	// телефон: показываем 2 первых + 2 последних
	if len(s) >= 4 {
		return s[:2] + "***" + s[len(s)-2:]
	}
	return "***"
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func (h *OrganizerListHandler) handleAccessErr(ctx context.Context, chatID int64, err error) {
	switch {
	case errors.Is(err, service.ErrNotOrganizer), errors.Is(err, service.ErrNotEventOwner):
		if e := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.OrganizerNoAccess(), keyboards.MainMenu()); e != nil {
			h.log.Error("send no access failed", "err", e)
		}
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

func (h *OrganizerListHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}

func (h *OrganizerListHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}
