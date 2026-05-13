package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// AdminLoginHandler — обработка команды /admin_login.
//
// Бот выдаёт magic JWT (5 мин) через service.Auth.IssueMagic и шлёт
// inline-кнопку со ссылкой ${ADMIN_WEB_BASE_URL}/auth?t=<jwt>.
// Magic-токен одноразовый (фронт его обменивает на session JWT, после
// чего он уже не нужен; TTL 5 мин = жёсткий backstop против повторного
// использования из shoulder-surfing).
//
// Только organizer/admin могут получить magic — applicant получает
// AdminLoginNoAccess + main menu.
type AdminLoginHandler struct {
	api        *maxclient.Client
	auth       service.Auth
	webBaseURL string
	log        *slog.Logger
}

// NewAdminLoginHandler — конструктор.
func NewAdminLoginHandler(api *maxclient.Client, auth service.Auth,
	webBaseURL string, log *slog.Logger,
) *AdminLoginHandler {
	return &AdminLoginHandler{
		api:        api,
		auth:       auth,
		webBaseURL: webBaseURL,
		log:        log.With("handler", "admin_login"),
	}
}

// OnCmd обрабатывает /admin_login.
func (h *AdminLoginHandler) OnCmd(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userMaxID := upd.Message.Sender.UserId

	token, err := h.auth.IssueMagic(ctx, userMaxID)
	switch {
	case errors.Is(err, service.ErrAccessDenied):
		if err := h.api.SendTextWithKeyboard(ctx, chatID,
			messages.AdminLoginNoAccess(), keyboards.MainMenu()); err != nil {
			h.log.Error("send no access failed", "err", err)
		}
		return
	case errors.Is(err, service.ErrAuthSessionKeyMissing):
		h.log.Warn("ADMIN_SESSION_KEY not configured — magic login disabled")
		if err := h.api.SendText(ctx, chatID,
			"Веб-админка пока не настроена администратором."); err != nil {
			h.log.Error("send not configured failed", "err", err)
		}
		return
	case err != nil:
		h.log.Error("issue magic failed", "err", err)
		if err := h.api.SendText(ctx, chatID,
			messages.ErrorTryLater()); err != nil {
			h.log.Error("send error failed", "err", err)
		}
		return
	}

	link := h.webBaseURL + "/auth?t=" + url.QueryEscape(token)
	if err := h.api.SendTextWithKeyboard(ctx, chatID,
		messages.AdminLoginLink(), keyboards.AdminLoginLink(link)); err != nil {
		h.log.Error("send magic link failed", "err", err)
	}
}
