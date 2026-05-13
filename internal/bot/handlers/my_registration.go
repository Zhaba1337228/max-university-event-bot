package handlers

import (
	"context"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// MyRegistrationHandler — обработчик «Моя запись» и /forget_me (152-ФЗ).
//
// Полный список регистраций пользователя, история действий, кнопка
// «Показать мой QR» (QR появится в дне 15), отмена записи (день 8) —
// будут добавлены по мере прохождения дорожной карты.
//
// На день 6 здесь реализованы:
//   - /forget_me (двухшаговое подтверждение → ForgetMe сервис);
//   - заглушка «Моя запись» с сообщением «у вас нет активных записей»
//     (полноценный список — после дня 7-8, когда добавятся active regs queries).
type MyRegistrationHandler struct {
	api   *maxclient.Client
	fsm   *fsm.Manager
	users service.User
	log   *slog.Logger
}

// NewMyRegistrationHandler — конструктор.
func NewMyRegistrationHandler(api *maxclient.Client, fsmMgr *fsm.Manager,
	users service.User, log *slog.Logger,
) *MyRegistrationHandler {
	return &MyRegistrationHandler{
		api:   api,
		fsm:   fsmMgr,
		users: users,
		log:   log.With("handler", "my_registration"),
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
		// День 9.
		h.sendText(ctx, chatID, messages.HistoryEmpty())
	default:
		h.log.Debug("unknown my action", "action", p.Action)
		h.sendFallback(ctx, chatID)
	}
}

// onShow — «Моя запись». На день 6 — заглушка с предложением вернуться в меню.
// Реальный список появится в дне 8 после реализации Cancel handler.
func (h *MyRegistrationHandler) onShow(ctx context.Context, chatID, userMaxID int64) {
	_ = userMaxID
	// День 8: загрузить service.Registration.ListActive(userMaxID),
	// показать MyRegistration(event, reg) с клавиатурой MyRegistration(regID).
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.MyRegistrationEmpty(), keyboards.MainMenu()); err != nil {
		h.log.Error("send my reg empty failed", "err", err)
	}
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
		// Защита от устаревшей кнопки. Без подтверждения данные не удаляем.
		h.log.Warn("forget me without confirm state",
			"state", snap.State, "user_id", userMaxID)
		h.sendFallback(ctx, chatID)
		return
	}

	deleted, err := h.users.ForgetMe(ctx, userMaxID)
	if err != nil {
		h.log.Error("forget me failed", "err", err, "user_id", userMaxID)
		h.sendError(ctx, chatID)
		return
	}

	// После удаления FSM нет — Reset бессмысленен (user_states каскадно удалён).
	if !deleted {
		// Пользователя и так не было в БД — сообщаем «нечего удалять».
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
