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

// RegistrationHandler — FSM сценарий «согласие на ПДн → ФИО → направление → подтверждение».
//
// Состояния (см. internal/bot/fsm/states.go):
//
//	StateRegConsent    — пользователь не давал consent, ждём «Согласен/Отмена»
//	StateRegFullName     — ждём текст ФИО
//	StateRegContact      — legacy-совместимость для старых диалогов, переводим на reg_interest
//	StateRegInterest     — ждём направление (текст)
//	StateRegConfirmation — отрисована карточка с кнопками «Подтвердить/Изменить/Отменить»
type RegistrationHandler struct {
	api           *maxclient.Client
	fsm           *fsm.Manager
	reg           service.Registration
	users         service.User
	events        service.Event
	qr            service.QR
	regsRepo      repo.RegistrationRepo
	db            repo.Querier
	log           *slog.Logger
	policyVersion string
}

// NewRegistrationHandler — конструктор.
//
// qr/regsRepo/db опциональны (могут быть nil) — без них handler работает
// в режиме «без QR»; с ними после успешной регистрации генерирует и
// отправляет PNG-код отдельным сообщением + сохраняет attendance_code в БД.
func NewRegistrationHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	reg service.Registration, users service.User, events service.Event,
	qr service.QR, regsRepo repo.RegistrationRepo, db repo.Querier,
	log *slog.Logger, policyVersion string,
) *RegistrationHandler {
	return &RegistrationHandler{
		api:           api,
		fsm:           fsmMgr,
		reg:           reg,
		users:         users,
		events:        events,
		qr:            qr,
		regsRepo:      regsRepo,
		db:            db,
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

// OnText — обработка текста в reg_full_name / reg_interest.
func (h *RegistrationHandler) OnText(ctx context.Context, upd *schemes.MessageCreatedUpdate, snap fsm.Snapshot) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId
	text := strings.TrimSpace(upd.Message.Body.Text)

	switch snap.State {
	case fsm.StateRegFullName:
		h.onFullName(ctx, chatID, userMaxID, snap, text)
	case fsm.StateRegContact:
		h.onLegacyContactStep(ctx, chatID, userMaxID, snap)
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
	if existing, err := h.reg.GetActive(ctx, user.ID, eventID); err != nil {
		h.log.Error("check active registration failed", "err", err)
		h.sendError(ctx, chatID)
		return
	} else if existing != nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.AlreadyRegistered(), keyboards.AfterRegistration()); err != nil {
			h.log.Error("send already registered failed", "err", err)
		}
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

	// Согласие есть — проверяем сохранённое ФИО.
	h.proceedToFullNameOrInterest(ctx, chatID, userMaxID, snap, user)
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
	h.sendText(ctx, chatID, messages.ConsentRecorded())
	h.proceedToFullNameOrInterest(ctx, chatID, userMaxID, snap, user)
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
		InterestProgram: snap.Context.DraftInterest,
	})
	switch {
	case errors.Is(err, service.ErrConsentRequired):
		// На этом этапе consent уже должен быть, но если нет — назад в начало.
		h.sendText(ctx, chatID, messages.ConsentDeclined())
		_ = h.fsm.Reset(ctx, userMaxID)
		return
	case errors.Is(err, service.ErrAlreadyRegistered):
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.AlreadyRegistered(), keyboards.AfterRegistration()); err != nil {
			h.log.Error("send already registered failed", "err", err)
		}
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

	// Успех. Сохраняем ФИО в профиль пользователя для будущих записей.
	if snap.Context.DraftFullName != "" {
		if _, err := h.users.EnsureProfile(ctx, userMaxID, snap.Context.DraftFullName, ""); err != nil {
			h.log.Warn("save full name to profile failed", "err", err)
		}
	}

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

	// Получаем attendance_code для отображения пользователю (TZ §2: код записи).
	var attendanceCode string
	if h.qr != nil && h.regsRepo != nil && h.db != nil {
		if _, code, err := ensureAttendanceCode(ctx, h.regsRepo, h.db, h.qr, res.RegistrationID); err != nil {
			h.log.Warn("prepare attendance code failed", "err", err, "reg_id", res.RegistrationID)
		} else {
			attendanceCode = code
		}
	}

	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.RegSuccess(event, attendanceCode), keyboards.AfterRegistration()); err != nil {
		h.log.Error("send success failed", "err", err)
	}

	// Day 15 — QR-код приглашения отдельным сообщением.
	// QR опционален: если qrSvc не передан в конструктор — пропускаем без ошибки.
	if h.qr != nil && h.regsRepo != nil && h.db != nil {
		h.sendQRCode(ctx, chatID, res.RegistrationID, event)
	}
}

// sendQRCode генерирует attendance_code (если ещё нет), сохраняет в БД и шлёт PNG.
//
// Алгоритм:
//  1. Get(regID) — актуальная запись;
//  2. если attendance_code пустой → NewAttendanceCode + SetAttendanceCode;
//  3. GenerateQRPNG → пишем во временный файл (Uploads принимает filepath);
//  4. UploadPhotoFromFile → AddPhoto → SendWithResult.
//
// Если что-то пошло не так — логируем и продолжаем; пользователь уже получил
// текстовое подтверждение, отсутствие QR не блокирует регистрацию.
func (h *RegistrationHandler) sendQRCode(ctx context.Context, chatID, regID int64, event *domain.Event) {
	if err := deliverRegistrationQRCode(ctx, h.api, h.qr, h.regsRepo, h.db, h.log, chatID, regID, event); err != nil {
		h.log.Warn("qr delivery failed", "err", err, "reg_id", regID)
	}
}

func (h *RegistrationHandler) onEdit(ctx context.Context, chatID, userMaxID int64) {
	snap, _ := h.fsm.Load(ctx, userMaxID)
	snap.Context.DraftFullName = ""
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
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegInterest, snap.Context)
	h.sendText(ctx, chatID, messages.AskInterest())
}

func (h *RegistrationHandler) onLegacyContactStep(ctx context.Context, chatID, userMaxID int64, snap fsm.Snapshot) {
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegInterest, snap.Context)
	h.sendText(ctx, chatID, "Контакт больше не нужен. Напишите, какое направление вам интересно.")
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

// proceedToFullNameOrInterest — переходит к шагу ФИО или сразу к интересу,
// если ФИО уже сохранено в профиле пользователя.
func (h *RegistrationHandler) proceedToFullNameOrInterest(
	ctx context.Context, chatID, userMaxID int64,
	snap fsm.Snapshot, user *domain.User,
) {
	if user != nil && user.FullName != nil && *user.FullName != "" {
		// ФИО уже сохранено — пропускаем шаг и переходим к интересу.
		snap.Context.DraftFullName = *user.FullName
		_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegInterest, snap.Context)
		h.sendText(ctx, chatID, messages.AskFullNameWithSaved(*user.FullName))
		return
	}
	// ФИО не сохранено — запрашиваем.
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateRegFullName, snap.Context)
	h.sendText(ctx, chatID, messages.AskFullName())
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
