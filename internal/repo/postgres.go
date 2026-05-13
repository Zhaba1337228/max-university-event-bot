// Package repo содержит репозитории для PostgreSQL и контракты для сервисов.
//
// Архитектурные правила (см. execution_plan.md § 4.2 и executor_prompt.md § 3.1):
//
//   - Все публичные сигнатуры — через интерфейсы; реализация — приватная структура.
//   - SQL только параметризованный (никаких fmt.Sprintf в SQL).
//   - Контекст первым параметром всех методов.
//   - Транзакции — через тип Querier (см. tx.go), чтобы один и тот же
//     репозиторий можно было вызвать как из пула, так и из tx.
package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier — общий интерфейс для pgxpool.Pool и pgx.Tx.
// Используется во всех методах репозиториев, чтобы их можно было вызывать
// и без транзакции, и внутри транзакции (передавая pgx.Tx).
//
// Также интерфейс реализуется pgxmock-объектами, что даёт юнит-тесты без БД.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewPool создаёт pgxpool.Pool с настройками MaxConns/MinConns и проверяет
// connectivity через Ping. Возвращает закрытый пул при ошибке.
func NewPool(ctx context.Context, url string, maxConns, minConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	if minConns >= 0 {
		cfg.MinConns = minConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg ping: %w", err)
	}
	return pool, nil
}

// IsUniqueViolation сообщает, является ли err ошибкой нарушения UNIQUE.
// Используется для отлова дублей (registrations user_event_uk, notif_dedup).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}

// IsNoRows сообщает, что запрос вернул пустой результат. Эквивалент pgx.ErrNoRows.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
