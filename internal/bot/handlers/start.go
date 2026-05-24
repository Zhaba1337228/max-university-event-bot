// Package handlers содержит обработчики сценариев бота.
//
// Каждый файл — один логический сценарий. Хендлеры зависят только от
// интерфейсов сервисов и maxclient, не знают про БД/SDK напрямую.
package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
)

// StartHandler обрабатывает /start, /help, главное меню и BotStarted.
//
// На день 4 это вся «приветственная» функциональность; в дни 5-16 хендлер
// расширится callback-обработкой главного меню (переходы на «Записаться»,
// «Моя запись», «Подобрать через AI»).
type StartHandler struct {
	api *maxclient.Client
	fsm *fsm.Manager
	log *slog.Logger
}

// NewStartHandler — конструктор.
func NewStartHandler(api *maxclient.Client, fsmMgr *fsm.Manager, log *slog.Logger) *StartHandler {
	return &StartHandler{
		api: api,
		fsm: fsmMgr,
		log: log.With("handler", "start"),
	}
}

// OnBotStarted — событие первого запуска бота пользователем
// (нажал «Запустить» в карточке бота в MAX). Дублирует /start, но без команды.
func (h *StartHandler) OnBotStarted(ctx context.Context, upd *schemes.BotStartedUpdate) {
	chatID := upd.ChatId
	userID := upd.User.UserId
	h.greet(ctx, chatID, userID, upd.User.FirstName)
}

// OnStart — команда /start.
func (h *StartHandler) OnStart(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userID := upd.Message.Sender.UserId
	name := upd.Message.Sender.FirstName
	h.greet(ctx, chatID, userID, name)
}

// OnWhoami — команда /whoami. Показывает MAX user_id отправителя.
// Нужно, чтобы организатор мог быстро добавить себя в ADMIN_USER_IDS / ORGANIZER_USER_IDS.
func (h *StartHandler) OnWhoami(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userID := upd.Message.Sender.UserId
	name := upd.Message.Sender.FirstName
	username := upd.Message.Sender.Username

	text := fmt.Sprintf("🆔 Ваш MAX user_id:\n\n`%d`\n\n", userID)
	if name != "" {
		text += fmt.Sprintf("Имя: %s\n", name)
	}
	if username != "" {
		text += fmt.Sprintf("Username: @%s\n", username)
	}
	text += "\nЧтобы получить доступ к админке — попроси администратора добавить этот ID в роли organizer/admin."

	if err := h.api.SendText(ctx, chatID, text); err != nil {
		h.log.Error("send whoami failed", "err", err, "chat_id", chatID)
	}
}

// OnHelp — команда /help.
func (h *StartHandler) OnHelp(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	if err := h.api.SendTextWithKeyboard(ctx, chatID, messages.Help(), keyboards.MainMenu()); err != nil {
		h.log.Error("send help failed", "err", err, "chat_id", chatID)
	}
}

// OnMainMenu — callback из любой кнопки «В главное меню».
func (h *StartHandler) OnMainMenu(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userID := upd.Callback.User.UserId

	// Закрываем спиннер на кнопке.
	if err := h.api.AnswerCallback(ctx, upd.Callback.CallbackID, ""); err != nil {
		h.log.Warn("answer callback failed", "err", err)
	}
	h.greet(ctx, chatID, userID, upd.Callback.User.FirstName)
}

// greet — общий код для приветствия. Сбрасывает FSM, отправляет welcome.
func (h *StartHandler) greet(ctx context.Context, chatID, userID int64, name string) {
	if err := h.fsm.Reset(ctx, userID); err != nil {
		// FSM-ошибка не должна валить отправку приветствия — просто логируем.
		h.log.Warn("fsm reset failed", "err", err, "user_id", userID)
	}

	text := messages.Welcome(name)
	if err := h.api.SendTextWithKeyboard(ctx, chatID, text, keyboards.MainMenu()); err != nil {
		h.log.Error("send welcome failed", "err", err, "chat_id", chatID)
	}
}
