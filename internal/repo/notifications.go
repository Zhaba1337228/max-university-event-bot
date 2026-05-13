package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

type notificationsRepo struct{}

// NewNotifications создаёт репозиторий notifications.
func NewNotifications() NotificationRepo { return &notificationsRepo{} }

const notifColumns = `id, event_id, user_id, type, text, status,
    scheduled_at, sent_at, error, created_at`

// Schedule вставляет уведомление. При нарушении уникального индекса
// uniq_notif_dedup (миграция 8) возвращает (0, nil) — это штатный
// дедуп при многократном вызове ScheduleUpcomingReminders.
func (r *notificationsRepo) Schedule(ctx context.Context, q Querier, n *domain.Notification) (int64, error) {
	if n == nil {
		return 0, fmt.Errorf("notification: nil")
	}
	if n.Status == "" {
		n.Status = domain.NotifStatusPending
	}
	// ON CONFLICT использует то же выражение, что и uniq_notif_dedup
	// (см. миграция 8): integer-минутный bucket из EPOCH. date_trunc нельзя —
	// он не IMMUTABLE и Postgres откажется создать индекс.
	const stmt = `
INSERT INTO notifications (event_id, user_id, type, text, status, scheduled_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, event_id, type, ((EXTRACT(EPOCH FROM scheduled_at)::bigint) / 60))
DO NOTHING
RETURNING id`

	var id int64
	err := q.QueryRow(ctx, stmt,
		n.EventID, n.UserID, string(n.Type), n.Text, string(n.Status), n.ScheduledAt,
	).Scan(&id)
	if IsNoRows(err) {
		// Дубль — это нормально.
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("insert notification: %w", err)
	}
	n.ID = id
	return id, nil
}

// PickDue достаёт пачку уведомлений готовых к отправке.
// Без транзакции, потому что DispatchDue сам управляет состоянием
// через MarkSent/MarkFailed.
func (r *notificationsRepo) PickDue(ctx context.Context, q Querier, now time.Time, limit int) ([]*domain.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	const stmt = `
SELECT ` + notifColumns + `
FROM notifications
WHERE status = 'pending' AND scheduled_at <= $1
ORDER BY scheduled_at ASC
LIMIT $2`

	rows, err := q.Query(ctx, stmt, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query due notifications: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Notification, 0, limit)
	for rows.Next() {
		n := &domain.Notification{}
		var nType, status string
		if err := rows.Scan(
			&n.ID, &n.EventID, &n.UserID, &nType, &n.Text, &status,
			&n.ScheduledAt, &n.SentAt, &n.Error, &n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		n.Type = domain.NotificationType(nType)
		n.Status = domain.NotificationStatus(status)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter notifications: %w", err)
	}
	return out, nil
}

func (r *notificationsRepo) MarkSent(ctx context.Context, q Querier, id int64, at time.Time) error {
	const stmt = `UPDATE notifications SET status = 'sent', sent_at = $2, error = NULL WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, at)
	if err != nil {
		return fmt.Errorf("mark notif sent: %w", err)
	}
	return nil
}

func (r *notificationsRepo) MarkFailed(ctx context.Context, q Querier, id int64, errMsg string) error {
	const stmt = `UPDATE notifications SET status = 'failed', error = $2 WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, truncate(errMsg, 1024))
	if err != nil {
		return fmt.Errorf("mark notif failed: %w", err)
	}
	return nil
}

func (r *notificationsRepo) MarkSkipped(ctx context.Context, q Querier, id int64, reason string) error {
	const stmt = `UPDATE notifications SET status = 'skipped', error = $2 WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, truncate(reason, 1024))
	if err != nil {
		return fmt.Errorf("mark notif skipped: %w", err)
	}
	return nil
}

// truncate обрезает строку до n байт, добавляя многоточие.
// Применяется к описаниям ошибок перед записью в БД (защита от
// неконтролируемо длинных сообщений из внешних API).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
