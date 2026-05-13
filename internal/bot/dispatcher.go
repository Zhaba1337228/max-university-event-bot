// Package bot содержит маршрутизацию входящих апдейтов MAX SDK.
//
// Dispatcher читает Update'ы из канала (long-poll или webhook), ограничивает
// параллелизм через семафор, заворачивает обработку в recover() и направляет
// каждый Update в соответствующий Handler.
package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// Dispatcher — главный цикл бота.
type Dispatcher struct {
	log      *slog.Logger
	handlers *Handlers
	pool     chan struct{}
}

// NewDispatcher создаёт диспетчер.
// parallelism — максимум одновременных in-flight обработок;
// для одного хоста 32-64 — разумно.
func NewDispatcher(log *slog.Logger, h *Handlers, parallelism int) *Dispatcher {
	if parallelism <= 0 {
		parallelism = 32
	}
	return &Dispatcher{
		log:      log.With("component", "dispatcher"),
		handlers: h,
		pool:     make(chan struct{}, parallelism),
	}
}

// Run блокируется и обрабатывает Update'ы из канала updates до ctx.Done()
// либо закрытия канала. Безопасен к параллельным обработкам.
func (d *Dispatcher) Run(ctx context.Context, updates <-chan schemes.UpdateInterface) {
	d.log.Info("dispatcher started")
	defer d.log.Info("dispatcher stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			// Резервируем слот, ждём если перегружены — это back-pressure.
			select {
			case d.pool <- struct{}{}:
			case <-ctx.Done():
				return
			}
			go func(u schemes.UpdateInterface) {
				defer func() { <-d.pool }()
				d.handle(ctx, u)
			}(upd)
		}
	}
}

// handle обрабатывает один Update под защитой recover().
// Любая паника логируется и не валит бота. Хендлеры сами решают, что отправить
// пользователю в случае ошибки (обычно — messages.ErrorTryLater).
func (d *Dispatcher) handle(ctx context.Context, u schemes.UpdateInterface) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("handler panic", "panic", fmt.Sprintf("%v", r), "update_type", fmt.Sprintf("%T", u))
		}
	}()

	switch upd := u.(type) {
	case *schemes.BotStartedUpdate:
		d.handlers.Start.OnBotStarted(ctx, upd)
	case *schemes.MessageCreatedUpdate:
		d.handlers.RouteMessage(ctx, upd)
	case *schemes.MessageCallbackUpdate:
		d.handlers.RouteCallback(ctx, upd)
	default:
		// Не реагируем на ChatTitleChanged, BotRemoved и т.п. — для MVP не нужно.
		d.log.Debug("update ignored", "type", fmt.Sprintf("%T", upd))
	}
}
