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
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// RegistrationHandler — FSM сценарий «согласие на ПДн → ФИО → контакт → направление → подтверждение».
//
// Состояния (см. internal/bot/fsm/states.go):
//
//	StateRegConsent    — пользователь не давал consent, ждём «Согласен/Отмена»
//	StateRegFullName   — ждём текст ФИО
//	StateRegContact    — ждём текст контакт (телефон/email)
//	StateRegInterest   — ждём направление (текст)
//	StateRegConfirmation — отрисована карточка с кнопками «Подтвердить/Изменить/Отменить»
type RegistrationHandler struct {
	api           *maxclient.Client
	fsm           *fsm.Manager
	reg           service.Registration
	users         service.User
	events        service.Event
	log           *slog.Logger
	policyVersion string
}

// NewRegistrationHandler — конструктор.
func NewRegistrationHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	reg service.Registration, users service.User, events service.Event,
	log *slog.Logger, policyVersion string,
) *RegistrationHandler {
	return &RegistrationHandler{
		api:           api,
		fsm:           fsmMgr,
		reg:           reg,
		users:         users,
		events:        events,
		log:           log.With("handler", "registration"),
		policyVersion: policyVersion,
	}
}

// OnCallback — обработка callback'ов группы "reg:".
func (h *RegistrationHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	// Закрываем спиннер сразу.
	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "start":
		eventID := p.ArgInt64(0)
		h.onStart(ctx, chatID, userMaxID, eventID)
	case "consent_yes":
		h.onConsentYes(ctx, chatID, userMaxID)
	case "consent_no":
		h.onConsentNo(ctx, chatID, userMaxID)
	case "confirm":
		h.onConfirm(ctx, chatID, userMaxID)
	case "edit":
		h.onEdit(ctx, chatID, userMaxID)
	case "cancel":
		h.onCancel(ctx, chatID, userMaxID)
	default:
		h.log.Debug("unknown reg action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

// OnText — обработка текста в reg_full_name / reg_contact / reg_interest.
func (h *RegistrationHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate, snap fsm.Snapshot) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	text := strings.TrimSpace(upd.Message.Body.Text)

	switch snap.State {
	case fsm.StateRegFullName:
		h.onFullName(ctx, chatID, userMaxID, snap, text)
	case fsm.StateRegContact:
		h.onContact(ctx, chatID, userMaxID, snap, text)
	case fsm.StateRegInterest:
		h.onInterest(ctx, chatID, userMaxID, snap, text)
	default:
		h.sendFallback(ctx, chatID)
	}
}

// --- callback handlers ---

// onStart — пользователь нажал «Записаться» на карточке мероприятия.
// Если согласия нет — показываем ConsentAsk; если есть — переходим к ФИО.
func (h *RegistrationHandler) onStart(ctx context.Context, chatID, userMaxID, eventID int64) {
	// Убедимся что event ещё открыт.
	withFree, err := h.events.GetOpen(ctx, eventID)
	switch {
	case errors.Is(err, service.ErrEventNotFound):
		h.sendText(ctx, chatID, messages.EventNotAvailable())
		return
	case errors.Is(err, service.ErrEventClosed):
		h.sendText(ctx, chatID, messages.EventClosedNow())
		return
	case err != nil:
		h.log.Error("get event for reg start failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if withFree.FreeSeats == 0 {
		// Свободных мест нет — карточка должна была показать кнопку «лист ожидания»,
		// но если пользователь сюда попал через старую кнопку — продолжаем,
		// service.Register сам решит (waitlist если включён).
		h.log.Info("registration start with no seats",
			"user_id", userMaxID, "event_id", eventID)
	}

	// Гарантируем что пользователь есть в БД (нужен user.ID для GrantConsent).
	user, err := h.users.EnsureProfile(ctx, userMaxID, "", "")
	if err != nil {
		h.log.Error("ensure profile failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	// Сохраняем текущее событие в FSM.
	snap, _ := h.fsm.Load(ctx, userMaxID)
	snap.Context.CurrentEventID = eventID

	if !user.HasConsent() {
		// Шаг согласия — без него запись запрещена (§19.10).
		_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegConsent, snap.Context)
		text := messages.ConsentAsk(h.policyVersion)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, text, keyboards.RegConsent()); err != nil {
			h.log.Error("send consent failed", "err", err)
		}
		return
	}

	// Согласие есть — сразу к ФИО.
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegFullName, snap.Context)
	h.sendText(ctx, chatID, messages.AskFullName())
}

func (h *RegistrationHandler) onConsentYes(ctx context.Context, chatID, userMaxID int64) {
	user, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil || user == nil {
		h.log.Error("consent: user not found", "err", err, "user_id", userMaxID)
		h.sendError(ctx, chatID)
		return
	}
	if err := h.users.GrantConsent(ctx, user.ID, h.policyVersion); err != nil {
		h.log.Error("grant consent failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	snap, _ := h.fsm.Load(ctx, userMaxID)
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegFullName, snap.Context)
	h.sendText(ctx, chatID, messages.ConsentRecorded())
	h.sendText(ctx, chatID, messages.AskFullName())
}

func (h *RegistrationHandler) onConsentNo(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ConsentDeclined(), keyboards.MainMenu()); err != nil {
		h.log.Error("send consent declined failed", "err", err)
	}
}

func (h *RegistrationHandler) onConfirm(ctx context.Context, chatID, userMaxID int64) {
	snap, err := h.fsm.Load(ctx, userMaxID)
	if err != nil || snap.State != fsm.StateRegConfirmation {
		// Защита от устаревших кнопок «Подтвердить» из старого сообщения.
		h.sendFallback(ctx, chatID)
		return
	}
	user, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil || user == nil {
		h.log.Error("confirm: user not found", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	res, err := h.reg.Register(ctx, service.RegisterInput{
		UserID:          user.ID,
		EventID:         snap.Context.CurrentEventID,
		FullName:        snap.Context.DraftFullName,
		Contact:         snap.Context.DraftContact,
		InterestProgram: snap.Context.DraftInterest,
	})
	switch {
	case errors.Is(err, service.ErrConsentRequired):
		// На этом этапе consent уже должен быть, но если нет — назад в начало.
		h.sendText(ctx, chatID, messages.ConsentDeclined())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case errors.Is(err, service.ErrAlreadyRegistered):
		h.sendText(ctx, chatID, messages.AlreadyRegistered())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case errors.Is(err, service.ErrEventClosed):
		h.sendText(ctx, chatID, messages.EventClosedNow())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case errors.Is(err, service.ErrEventNotFound):
		h.sendText(ctx, chatID, messages.EventNotAvailable())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case errors.Is(err, service.ErrNoSeats):
		h.sendText(ctx, chatID, "К сожалению, мест не осталось, и лист ожидания закрыт.")
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case err != nil:
		h.log.Error("register failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}

	// Успех. Получаем событие для красивого ответа.
	event, _ := h.events.Get(ctx, snap.Context.CurrentEventID)
	_ = h.fsm.Reset(ctx, userMaxID)

	if res.IsWaitlist {
		text := messages.WaitlistConfirmed(res.Position)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, text, keyboards.AfterWaitlist()); err != nil {
			h.log.Error("send waitlist confirmation failed", "err", err)
		}
		return
	}

	if event == nil {
		h.sendText(ctx, chatID, "Запись подтверждена.")
		return
	}
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.RegSuccess(event), keyboards.AfterRegistration()); err != nil {
		h.log.Error("send success failed", "err", err)
	}
}

func (h *RegistrationHandler) onEdit(ctx context.Context, chatID, userMaxID int64) {
	snap, _ := h.fsm.Load(ctx, userMaxID)
	snap.Context.DraftFullName = ""
	snap.Context.DraftContact = ""
	snap.Context.DraftInterest = ""
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegFullName, snap.Context)
	h.sendText(ctx, chatID, messages.AskFullName())
}

func (h *RegistrationHandler) onCancel(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.RegCancelledDraft(), keyboards.MainMenu()); err != nil {
		h.log.Error("send reg cancelled failed", "err", err)
	}
}

// --- text handlers ---

func (h *RegistrationHandler) onFullName(ctx context.Context, chatID, userMaxID int64, snap fsm.Snapshot, text string) {
	if !validFullName(text) {
		h.sendText(ctx, chatID, messages.InvalidFullName())
		return
	}
	snap.Context.DraftFullName = text
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegContact, snap.Context)
	h.sendText(ctx, chatID, messages.AskContact())
}

func (h *RegistrationHandler) onContact(ctx context.Context, chatID, userMaxID int64, snap fsm.Snapshot, text string) {
	if !validContact(text) {
		h.sendText(ctx, chatID, messages.InvalidContact())
		return
	}
	snap.Context.DraftContact = text
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegInterest, snap.Context)
	h.sendText(ctx, chatID, messages.AskInterest())
}

func (h *RegistrationHandler) onInterest(ctx context.Context, chatID, userMaxID int64, snap fsm.Snapshot, text string) {
	// Направление — свободный текст. Не валидируем строго, чтобы пользователь
	// мог написать «не знаю». Длину ограничим, чтобы не упереться в лимит
	// 4000 символов сообщения.
	if len(text) > 200 {
		text = text[:200]
	}
	snap.Context.DraftInterest = text
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegConfirmation, snap.Context)

	event, err := h.events.Get(ctx, snap.Context.CurrentEventID)
	if err != nil || event == nil {
		h.sendText(ctx, chatID, messages.EventNotAvailable())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	}
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.RegConfirmation(event, snap.Context), keyboards.RegConfirm()); err != nil {
		h.log.Error("send reg confirmation failed", "err", err)
	}
}

// --- helpers ---

func (h *RegistrationHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}

func (h *RegistrationHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

func (h *RegistrationHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}

// validFullName — простая валидация ФИО: хотя бы 2 слова и разумная длина.
// Тест на это поведение — в registration_test.go.
func validFullName(s string) bool {
	if len(s) < 5 || len(s) > 200 {
		return false
	}
	return strings.Contains(strings.TrimSpace(s), " ")
}

// validContact — email (@ + .) ИЛИ ≥7 цифр (телефон).
func validContact(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return false
	}
	if strings.Contains(s, "@") && strings.Contains(s, ".") {
		return true
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 7
}
