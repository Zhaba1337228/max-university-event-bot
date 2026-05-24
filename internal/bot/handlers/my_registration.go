package handlers

import (
	"context"
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

// MyRegistrationHandler — обработчик «Моя запись», /forget_me и истории.
type MyRegistrationHandler struct {
	api      *maxclient.Client
	fsm      *fsm.Manager
	users    service.User
	reg      service.Registration
	events   service.Event
	logs     service.ActionLog
	regsRepo repo.RegistrationRepo
	db       repo.Querier
	log      *slog.Logger
}

// NewMyRegistrationHandler — конструктор.
// regsRepo и db опциональны (могут быть nil); без них toggle уведомлений не работает.
func NewMyRegistrationHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	users service.User, reg service.Registration, events service.Event,
	logs service.ActionLog,
	regsRepo repo.RegistrationRepo, db repo.Querier,
	log *slog.Logger,
) *MyRegistrationHandler {
	return &MyRegistrationHandler{
		api:      api,
		fsm:      fsmMgr,
		users:    users,
		reg:      reg,
		events:   events,
		logs:     logs,
		regsRepo: regsRepo,
		db:       db,
		log:      log.With("handler", "my_registration"),
	}
}

// OnForgetMeCmd — обработка текстовой команды /forget_me.
func (h *MyRegistrationHandler) OnForgetMeCmd(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId

	_ = h.fsm.Save(ctx, userMaxID, fsm.StateForgetMeConfirm, fsm.UserFSMContext{})
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ForgetMeAsk(), keyboards.ForgetMeConfirm()); err != nil {
		h.log.Error("send forget me ask failed", "err", err)
	}
}

// OnCallback — обработка callback'ов группы "my:".
func (h *MyRegistrationHandler) OnCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate, p callbacks.Payload) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Callback.User.UserId

	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}

	switch p.Action {
	case "show":
		h.onShow(ctx, chatID, userMaxID)
	case "forget_ask":
		h.onForgetAsk(ctx, chatID, userMaxID)
	case "forget_yes":
		h.onForgetYes(ctx, chatID, userMaxID)
	case "forget_no":
		h.onForgetNo(ctx, chatID, userMaxID)
	case "history":
		h.onHistory(ctx, chatID, userMaxID)
	case "toggle_notif":
		regID := p.ArgInt64(0)
		h.onToggleNotif(ctx, chatID, userMaxID, regID)
	case "qr":
		// День 15.
		h.sendText(ctx, chatID, messages.QRNotAvailable())
	default:
		h.log.Debug("unknown my action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

// onShow — «Моя запись».
//
// Если активных регистраций несколько — показываем список + клавиатуру «выбрать».
// Если одна — сразу карточку с кнопками «Отменить»/«История»/«Меню».
// Если ноль — empty + меню.
func (h *MyRegistrationHandler) onShow(ctx context.Context, chatID, userMaxID int64) {
	user, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil {
		h.log.Error("my show: get user failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if user == nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.MyRegistrationEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send empty failed", "err", err)
		}
		return
	}

	regs, err := h.reg.ListActiveByUser(ctx, user.ID)
	if err != nil {
		h.log.Error("my show: list failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if len(regs) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.MyRegistrationEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send empty failed", "err", err)
		}
		return
	}

	// Для каждой регистрации подгружаем event (не оптимально — N+1, но
	// у пользователя обычно 1-3 активные записи, так что норм).
	for _, r := range regs {
		ev, err := h.events.Get(ctx, r.EventID)
		if err != nil {
			h.log.Warn("my show: get event failed", "err", err, "event_id", r.EventID)
			continue
		}
		if ev == nil {
			continue
		}
		text := messages.MyRegistration(ev, r)
		kb := keyboards.MyRegistration(r.ID, r.NotificationsDisabled)
		if err := h.api.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
			h.log.Error("send my reg failed", "err", err)
		}
	}
}

// onToggleNotif — переключает флаг notifications_disabled для регистрации (TZ §6).
func (h *MyRegistrationHandler) onToggleNotif(ctx context.Context, chatID, userMaxID, regID int64) {
	if regID <= 0 {
		h.sendFallback(ctx, chatID)
		return
	}
	if h.regsRepo == nil || h.db == nil {
		h.sendText(ctx, chatID, "Управление уведомлениями временно недоступно.")
		return
	}

	// Проверяем, что запись принадлежит этому пользователю.
	u, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil || u == nil {
		h.sendError(ctx, chatID)
		return
	}
	regs, err := h.reg.ListActiveByUser(ctx, u.ID)
	if err != nil {
		h.sendError(ctx, chatID)
		return
	}
	var targetReg *domain.Registration
	for _, r := range regs {
		if r.ID == regID {
			targetReg = r
			break
		}
	}
	if targetReg == nil {
		h.sendFallback(ctx, chatID)
		return
	}

	newDisabled := !targetReg.NotificationsDisabled
	if err := h.regsRepo.SetNotificationsDisabled(ctx, h.db, regID, newDisabled); err != nil {
		h.log.Error("toggle notif failed", "err", err, "reg_id", regID)
		h.sendError(ctx, chatID)
		return
	}

	if newDisabled {
		h.sendText(ctx, chatID, messages.NotifDisabledDone())
	} else {
		h.sendText(ctx, chatID, messages.NotifEnabledDone())
	}
}

// onHistory — последние 10 записей audit log пользователя.
func (h *MyRegistrationHandler) onHistory(ctx context.Context, chatID, userMaxID int64) {
	user, err := h.users.GetByMaxID(ctx, userMaxID)
	if err != nil {
		h.log.Error("history: get user failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if user == nil {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.HistoryEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send history empty failed", "err", err)
		}
		return
	}

	logs, err := h.logs.ListByUser(ctx, user.ID, 10)
	if err != nil {
		h.log.Error("history: list failed", "err", err)
		h.sendError(ctx, chatID)
		return
	}
	if len(logs) == 0 {
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.HistoryEmpty(), keyboards.MainMenu()); err != nil {
			h.log.Error("send history empty failed", "err", err)
		}
		return
	}

	// Собираем текстовый список.
	lines := []string{messages.HistoryHeader(), ""}
	for _, l := range logs {
		lines = append(lines, messages.HistoryLine(l))
	}
	text := joinLines(lines)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, keyboards.MainMenu()); err != nil {
		h.log.Error("send history failed", "err", err)
	}
}

// joinLines — локальная альтернатива strings.Join (избегаем лишний импорт).
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	out := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return string(out)
}

func (h *MyRegistrationHandler) onForgetAsk(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Save(ctx, userMaxID, fsm.StateForgetMeConfirm, fsm.UserFSMContext{})
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ForgetMeAsk(), keyboards.ForgetMeConfirm()); err != nil {
		h.log.Error("send forget me ask failed", "err", err)
	}
}

func (h *MyRegistrationHandler) onForgetYes(ctx context.Context, chatID, userMaxID int64) {
	snap, err := h.fsm.Load(ctx, userMaxID)
	if err != nil || snap.State != fsm.StateForgetMeConfirm {
		h.log.Warn("forget me without confirm state", "state", snap.State, "user_id", userMaxID)
		h.sendFallback(ctx, chatID)
		return
	}

	deleted, err := h.users.ForgetMe(ctx, userMaxID)
	if err != nil {
		h.log.Error("forget me failed", "err", err, "user_id", userMaxID)
		h.sendError(ctx, chatID)
		return
	}

	if !deleted {
		h.sendText(ctx, chatID, "Данных не было — удалять нечего.")
		return
	}

	if err := h.api.SendText(ctx, chatID, messages.ForgetMeDone()); err != nil {
		h.log.Error("send forget done failed", "err", err)
	}
}

func (h *MyRegistrationHandler) onForgetNo(ctx context.Context, chatID, userMaxID int64) {
	_ = h.fsm.Reset(ctx, userMaxID)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ForgetMeCancelled(), keyboards.MainMenu()); err != nil {
		h.log.Error("send forget cancelled failed", "err", err)
	}
}

// --- helpers ---

func (h *MyRegistrationHandler) sendText(ctx context.Context, chatID int64, text string) {
	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send text failed", "err", err)
	}
}

func (h *MyRegistrationHandler) sendFallback(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.FallbackUnknown(), keyboards.MainMenu()); err != nil {
		h.log.Error("send fallback failed", "err", err)
	}
}

func (h *MyRegistrationHandler) sendError(ctx context.Context, chatID int64) {
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.ErrorTryLater(), keyboards.MainMenu()); err != nil {
		h.log.Error("send error msg failed", "err", err)
	}
}

// _ — placeholder, чтобы не падал import при будущих чистках.
var _ = (*domain.Registration)(nil)
