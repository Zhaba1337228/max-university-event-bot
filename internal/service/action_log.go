package service

import (
	"context"
	"fmt"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// ActionLog — публичный сервис для чтения audit-логов.
// Запись осуществляется внутри других сервисов (registration, user, etc.).
type ActionLog interface {
	ListByUser(ctx context.Context, userID int64, limit int) ([]*domain.ActionLog, error)
	ListByEvent(ctx context.Context, eventID int64, limit int) ([]*domain.ActionLog, error)
}

type actionLogService struct {
	pool repo.Querier
	logs repo.ActionLogRepo
}

// NewActionLog создаёт сервис.
func NewActionLog(pool repo.Querier, logs repo.ActionLogRepo) ActionLog {
	return &actionLogService{pool: pool, logs: logs}
}

func (s *actionLogService) ListByUser(ctx context.Context, userID int64, limit int) ([]*domain.ActionLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	out, err := s.logs.ListByUser(ctx, s.pool, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by user: %w", err)
	}
	return out, nil
}

func (s *actionLogService) ListByEvent(ctx context.Context, eventID int64, limit int) ([]*domain.ActionLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	out, err := s.logs.ListByEvent(ctx, s.pool, eventID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by event: %w", err)
	}
	return out, nil
}
