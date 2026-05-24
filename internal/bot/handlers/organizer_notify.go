package handlers

import (
	"context"
	"errors"
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

// OrganizerNotifyHandler — рассылка сообщений участникам мероприятия.
//
// FSM:
//
//	StateOrganizerNotifText    — ждём текст
//	StateOrganizerNotifConfirm — показан preview + кнопки Send/Cancel/AI
//
// AI-улучшение (План §17.1 RewriteNotification) — заглушка на День 16.
type OrganizerNotifyHandler struct {
	api    *maxclient.Client
	fsm    *fsm.Manager
	role   service.Role
	events service.Event
	notif  service.Notification
	ai     service.AI // опционально (для orgnotif:ai)
	regs   repo.RegistrationRepo
	q      repo.Querier
	log    *slog.Logger
}

// NewOrganizerNotifyHandler — конструктор. ai может быть nil — в этом случае
// orgnotif:ai вернёт «AI недоступен», текст рассылки не меняется.
func NewOrganizerNotifyHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	role service.Role, events service.Event, notif service.Notification, ai service.AI,
	regs repo.RegistrationRepo, q repo.Querier, log *slog.Logger,
) *OrganizerNotifyHandler {
	return &OrganizerNotifyHandler{
		api:    api,
		fsm:    fsmMgr,
		role:   role,
		events: events,
		notif:  notif,
		ai:     ai,
		regs:   regs,
		q:      q,
		log:    log.With("handler", "organizer_notify"),
	}
}

// OnCallback маршрутизирует orgnotif:* (start, ai, send, cancel).
func (h *OrganizerNotifyHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "start":
		eventID := p.ArgInt64(0)
		h.onStart(ctx, chatID, userMaxID, eventID)
	case "send":
		h.onSend(ctx, chatID, userMaxID)
	case "cancel":
		h.onCancel(ctx, chatID, userMaxID)
	case "ai":
		h.onAIRewrite(ctx, chatID, userMaxID)
	default:
		h.log.Debug("unknown orgnotif action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

// OnText — текстовый ввод в state organizer_notif_text.
func (h *OrganizerNotifyHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate, snap fsm.Snapshot) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	text := strings.TrimSpace(upd.Message.Body.Text)

	if snap.State != fsm.StateOrganizerNotifText {
		h.sendFallback(ctx, chatID)
		return
	}
	if text == "" || len(text) > 4000 {
		h.sendText(ctx, chatID, "Текст пустой или слишком длинный (>4000 символов).")
		return
	}

	eventID := snap.Context.OrganizerEventID
	if eventID == 0 {
		h.sendFallback(ctx, chatID)
		return
	}
	if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}

	// Считаем кол-во получателей для preview.
	count, err := h.regs.CountByEvent(ctx, h.q, eventID, domain.RegStatusRegistered)
	if err != nil {
		h.log.Error("count recipients failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	// Сохраняем черновик в FSM.
	snap.Context.NotificationDraft = text
	snap.Context.NotificationFinal = text
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerNotifConfirm, snap.Context)

	preview := messages.OrganizerNotifPreview(text, count)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, preview, keyboards.OrganizerNotifConfirm()); err != nil {
		h.log.Error("send preview failed", "err", err)
	}
}

func (h *OrganizerNotifyHandler) onStart(ctx context.Context, chatID, userMaxID, eventID int64) {
	if _, err := h.role.RequireEventOwner(ctx, userMaxID, eventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerNotifText,
		fsm.UserFSMContext{OrganizerEventID: eventID})
	h.sendText(ctx, chatID, messages.OrganizerAskNotifText())
}

func (h *OrganizerNotifyHandler) onSend(ctx context.Context, chatID, userMaxID int64) {
	snap, err := h.fsm.Load(ctx, userMaxID)
	if err != nil || snap.State != fsm.StateOrganizerNotifConfirm || snap.Context.OrganizerEventID == 0 {
		h.sendFallback(ctx, chatID)
		return
	}
	if _, err := h.role.RequireEventOwner(ctx, userMaxID, snap.Context.OrganizerEventID); err != nil {
		h.handleAccessErr(ctx, chatID, err)
		return
	}

	text := snap.Context.NotificationFinal
	if text == "" {
		text = snap.Context.NotificationDraft
	}
	if text == "" {
		h.sendFallback(ctx, chatID)
		return
	}

	sent, err := h.notif.SendBroadcast(ctx, snap.Context.OrganizerEventID, text)
	if err != nil {
		h.log.Error("broadcast failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.OrganizerNotifSent(sent), keyboards.MainMenu()); err != nil {
		h.log.Error("send broadcast result failed", "err", err)
	}
}

func (h *OrganizerNotifyHandler) onCancel(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.OrganizerNotifCancelled(), keyboards.MainMenu()); err != nil {
		h.log.Error("send cancel notif failed", "err", err)
	}
}

// onAIRewrite — улучшение текста рассылки через GigaChat.
// На fallback (AI недоступен / парсинг JSON упал) — оставляем оригинальный
// текст, показываем preview снова + сообщение «AI недоступен».
func (h *OrganizerNotifyHandler) onAIRewrite(ctx context.Context, chatID, userMaxID int64) {
	snap, err := h.fsm.Load(ctx, userMaxID)
	if err != nil || snap.State != fsm.StateOrganizerNotifConfirm || snap.Context.OrganizerEventID == 0 {
		h.sendFallback(ctx, chatID)
		return
	}
	if h.ai == nil {
		h.sendText(ctx, chatID, messages.AIUnavailable())
		return
	}

	draft := snap.Context.NotificationFinal
	if draft == "" {
		draft = snap.Context.NotificationDraft
	}

	ev, err := h.events.Get(ctx, snap.Context.OrganizerEventID)
	if err != nil || ev == nil {
		h.sendError(ctx, chatID)
		return
	}

	improved, err := h.ai.RewriteNotification(ctx, draft, ev)
	if err != nil {
		h.sendText(ctx, chatID, messages.AIUnavailable())
		return
	}

	// Сохраняем улучшенный текст и показываем новый preview.
	snap.Context.NotificationFinal = improved
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateOrganizerNotifConfirm, snap.Context)

	count, _ := h.regs.CountByEvent(ctx, h.q, snap.Context.OrganizerEventID, domain.RegStatusRegistered)
	preview := messages.AIOrganizerNotifPreview(improved, count)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, preview, keyboards.OrganizerNotifConfirm()); err != nil {
		h.log.Error("send ai preview failed", "err", err)
	}
}

// --- helpers ---

func (h *OrganizerNotifyHandler) handleAccessErr(ctx context.Context, chatID int64, err error) {
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

func (h *OrganizerNotifyHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}

func (h *OrganizerNotifyHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

func (h *OrganizerNotifyHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}
