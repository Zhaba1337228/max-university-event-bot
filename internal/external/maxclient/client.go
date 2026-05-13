// Package maxclient — тонкая обёртка над `github.com/max-messenger/max-bot-api-client-go`.
//
// Назначение:
//   - единственное место, где знают про токен MAX (маскируется в логах);
//   - автоматический retry на 429/5xx через retry.Do;
//   - удобные методы SendText/SendTextKeyboard/AnswerCallback вместо
//     ручной сборки maxbot.NewMessage().SetChat().SetText() каждый раз.
//
// SDK напрямую остаётся доступен через .Raw() — мы не повторяем за ним
// весь API, чтобы не плодить тонны boilerplate.
package maxclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/retry"
	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/secret"
)

// Config — параметры клиента.
type Config struct {
	Token       string
	HTTPTimeout time.Duration
	Debug       bool
}

// Client — обёртка с retry.
type Client struct {
	api   *maxbot.Api
	log   *slog.Logger
	retry retry.Config
}

// New создаёт клиента. Не делает сетевых вызовов — ping вынесен в Ping().
func New(cfg Config, log *slog.Logger) (*Client, error) {
	if cfg.Token == "" {
		return nil, maxbot.ErrEmptyToken
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}

	opts := []maxbot.Option{
		maxbot.WithHTTPClient(&http.Client{Timeout: cfg.HTTPTimeout}),
		maxbot.WithApiTimeout(cfg.HTTPTimeout),
	}
	if cfg.Debug {
		opts = append(opts, maxbot.WithDebugMode())
	}

	api, err := maxbot.New(cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("maxbot new: %w", err)
	}

	c := &Client{
		api:   api,
		log:   log.With("component", "maxclient"),
		retry: retry.DefaultConfig(),
	}
	c.log.Info("max client configured", "token", secret.Mask(cfg.Token), "timeout", cfg.HTTPTimeout)
	return c, nil
}

// Raw возвращает базовый maxbot.Api для прямых вызовов
// (например, NewKeyboardBuilder, Uploads, Subscriptions).
func (c *Client) Raw() *maxbot.Api { return c.api }

// Ping проверяет связь с API через GetBot. Используется в /healthz и на старте.
func (c *Client) Ping(ctx context.Context) (*schemes.BotInfo, error) {
	var info *schemes.BotInfo
	err := retry.Do(ctx, c.retry, func() error {
		var inner error
		info, inner = c.api.Bots.GetBot(ctx)
		return inner
	})
	if err != nil {
		return nil, fmt.Errorf("ping bot: %w", err)
	}
	return info, nil
}

// GetUpdates возвращает канал апдейтов от SDK (long-polling).
// Канал закрывается, когда ctx.Done() закрыт SDK самостоятельно.
func (c *Client) GetUpdates(ctx context.Context) <-chan schemes.UpdateInterface {
	return c.api.GetUpdates(ctx)
}

// SendText — лаконичная отправка текстового сообщения в чат.
func (c *Client) SendText(ctx context.Context, chatID int64, text string) error {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	return c.send(ctx, msg)
}

// SendTextToUser — отправка в личку пользователю по его max_user_id.
func (c *Client) SendTextToUser(ctx context.Context, userID int64, text string) error {
	msg := maxbot.NewMessage().SetUser(userID).SetText(text)
	return c.send(ctx, msg)
}

// SendTextWithKeyboard — отправка с inline-клавиатурой.
func (c *Client) SendTextWithKeyboard(ctx context.Context, chatID int64, text string, kb *maxbot.Keyboard) error {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	if kb != nil {
		msg = msg.AddKeyboard(kb)
	}
	return c.send(ctx, msg)
}

// SendTextWithKeyboardToUser — то же, но через SetUser.
func (c *Client) SendTextWithKeyboardToUser(ctx context.Context, userID int64, text string, kb *maxbot.Keyboard) error {
	msg := maxbot.NewMessage().SetUser(userID).SetText(text)
	if kb != nil {
		msg = msg.AddKeyboard(kb)
	}
	return c.send(ctx, msg)
}

// SendMessage отправляет произвольно собранный *maxbot.Message с retry.
func (c *Client) SendMessage(ctx context.Context, msg *maxbot.Message) error {
	return c.send(ctx, msg)
}

// AnswerCallback закрывает спиннер на нажатой кнопке.
// Если notification != "" — пользователь увидит toast.
func (c *Client) AnswerCallback(ctx context.Context, callbackID string, notification string) error {
	cb := &schemes.CallbackAnswer{}
	if notification != "" {
		cb.Notification = notification
	}
	return retry.Do(ctx, c.retry, func() error {
		_, err := c.api.Messages.AnswerOnCallback(ctx, callbackID, cb)
		return err
	})
}

// send — общий retry-wrapper.
func (c *Client) send(ctx context.Context, msg *maxbot.Message) error {
	if msg == nil {
		return errors.New("send: nil message")
	}
	return retry.Do(ctx, c.retry, func() error {
		return c.api.Messages.Send(ctx, msg)
	})
}
