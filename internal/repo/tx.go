package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxRunner — типизированный helper для запуска бизнес-операций в транзакции.
//
// Используется в сервисах (например, service.Registration.Register с
// SELECT ... FOR UPDATE на event row). Сервис передаёт замыкание, которое
// получает pgx.Tx — внутри замыкания репозитории работают через интерфейс
// Querier (а pgx.Tx его реализует).
//
// При ошибке внутри замыкания транзакция откатывается; при панике —
// откатывается и паника пробрасывается дальше.
type TxRunner struct {
	pool *pgxpool.Pool
}

// NewTxRunner создаёт TxRunner поверх пула соединений.
func NewTxRunner(pool *pgxpool.Pool) *TxRunner {
	return &TxRunner{pool: pool}
}

// InTx запускает fn в новой транзакции. Изоляция по умолчанию — RepeatableRead,
// чтобы SELECT FOR UPDATE на event row защищал счётчик мест от гонок.
//
// Если fn возвращает не-nil ошибку — транзакция откатывается.
// При панике в fn — Rollback + повторная паника.
func (t *TxRunner) InTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return t.InTxWithOptions(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, fn)
}

// InTxWithOptions — то же, что InTx, но с явными опциями (для read-only / serializable).
func (t *TxRunner) InTxWithOptions(ctx context.Context, opts pgx.TxOptions,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	if t == nil || t.pool == nil {
		return errors.New("tx runner not initialized")
	}
	tx, err := t.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			// Rollback вне зависимости от того, ошибка ли это или паника.
			// Игнорируем ErrTxClosed, потому что после успешного Commit
			// rollback бессмысленен.
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}
