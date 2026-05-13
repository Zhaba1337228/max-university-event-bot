package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

type actionLogsRepo struct{}

// NewActionLogs создаёт репозиторий action_logs. Реализация stateless —
// пул соединений или транзакция передаются через параметр Querier.
func NewActionLogs() ActionLogRepo { return &actionLogsRepo{} }

const actionLogColumns = `id, actor_user_id, target_user_id, event_id,
    registration_id, action, payload, created_at`

func (r *actionLogsRepo) Append(ctx context.Context, q Querier, log *domain.ActionLog) error {
	if log == nil {
		return fmt.Errorf("action log: nil")
	}
	payload := log.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	const stmt = `
INSERT INTO action_logs (actor_user_id, target_user_id, event_id, registration_id, action, payload)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at`

	return q.QueryRow(ctx, stmt,
		log.ActorUserID, log.TargetUserID, log.EventID, log.RegistrationID,
		string(log.Action), payload,
	).Scan(&log.ID, &log.CreatedAt)
}

func (r *actionLogsRepo) ListByUser(ctx context.Context, q Querier, userID int64, limit int) ([]*domain.ActionLog, error) {
	if limit <= 0 {
		limit = 10
	}
	const stmt = `
SELECT ` + actionLogColumns + `
FROM action_logs
WHERE actor_user_id = $1 OR target_user_id = $1
ORDER BY created_at DESC
LIMIT $2`

	rows, err := q.Query(ctx, stmt, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query action_logs by user: %w", err)
	}
	defer rows.Close()
	return scanActionLogs(rows)
}

func (r *actionLogsRepo) ListByEvent(ctx context.Context, q Querier, eventID int64, limit int) ([]*domain.ActionLog, error) {
	if limit <= 0 {
		limit = 50
	}
	const stmt = `
SELECT ` + actionLogColumns + `
FROM action_logs
WHERE event_id = $1
ORDER BY created_at DESC
LIMIT $2`

	rows, err := q.Query(ctx, stmt, eventID, limit)
	if err != nil {
		return nil, fmt.Errorf("query action_logs by event: %w", err)
	}
	defer rows.Close()
	return scanActionLogs(rows)
}

func scanActionLogs(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*domain.ActionLog, error) {
	out := make([]*domain.ActionLog, 0, 16)
	for rows.Next() {
		l := &domain.ActionLog{}
		var action string
		var payload []byte
		if err := rows.Scan(
			&l.ID, &l.ActorUserID, &l.TargetUserID, &l.EventID,
			&l.RegistrationID, &action, &payload, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan action_log: %w", err)
		}
		l.Action = domain.ActionType(action)
		l.Payload = payload
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter action_logs: %w", err)
	}
	return out, nil
}
