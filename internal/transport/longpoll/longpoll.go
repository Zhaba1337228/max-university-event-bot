// Package longpoll — транспорт обновлений MAX SDK через long-polling.
//
// На каждом тике SDK сам делает HTTP запрос с long-poll'ом до 30 секунд.
// Наша задача — пробросить апдейты из SDK-канала в общий канал бота.
//
// На день 4 — единственный поддерживаемый транспорт.
// Webhook появится в дне 18.
package longpoll

import (
	"context"
	"log/slog"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
)

// Runner пробрасывает Update'ы из MAX SDK в out-канал.
type Runner struct {
	api *maxclient.Client
	log *slog.Logger
}

// New создаёт Runner.
func New(api *maxclient.Client, log *slog.Logger) *Runner {
	return &Runner{
		api: api,
		log: log.With("component", "longpoll"),
	}
}

// Run блокируется и проталкивает Update'ы до закрытия ctx или out.
// Если потребитель медленный — на out пишем select+ctx, не блокируясь насмерть.
func (r *Runner) Run(ctx context.Context, out chan<- schemes.UpdateInterface) {
	r.log.Info("long-polling started")
	defer r.log.Info("long-polling stopped")

	updates := r.api.GetUpdates(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			select {
			case out <- upd:
			case <-ctx.Done():
				return
			}
		}
	}
}
