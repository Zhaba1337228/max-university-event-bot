package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

func TestNotifScheduleHappyPath(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	notifs := repo.NewNotifications()

	at := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	n := &domain.Notification{
		EventID:     1,
		UserID:      2,
		Type:        domain.NotifReminder24h,
		Text:        "напоминание",
		ScheduledAt: at,
	}
	mock.ExpectQuery(`INSERT INTO notifications`).
		WithArgs(int64(1), int64(2), "reminder_24h", "напоминание", "pending", at).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(55)))

	id, err := notifs.Schedule(context.Background(), mock, n)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if id != 55 {
		t.Fatalf("want id=55, got %d", id)
	}
}

func TestNotifScheduleDedupSilent(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	notifs := repo.NewNotifications()

	at := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	n := &domain.Notification{
		EventID: 1, UserID: 2, Type: domain.NotifReminder24h,
		Text: "x", ScheduledAt: at,
	}
	// Дубль → DO NOTHING → RETURNING пуст → pgx.ErrNoRows.
	mock.ExpectQuery(`INSERT INTO notifications`).
		WithArgs(int64(1), int64(2), "reminder_24h", "x", "pending", at).
		WillReturnError(pgx.ErrNoRows)

	id, err := notifs.Schedule(context.Background(), mock, n)
	if err != nil {
		t.Fatalf("Schedule must not return error on dedup, got %v", err)
	}
	if id != 0 {
		t.Errorf("want id=0 for dedup, got %d", id)
	}
}

func TestNotifPickDue(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	notifs := repo.NewNotifications()

	now := time.Now()
	rows := pgxmock.NewRows([]string{
		"id", "event_id", "user_id", "type", "text", "status",
		"scheduled_at", "sent_at", "error", "created_at",
	}).AddRow(int64(1), int64(2), int64(3), "reminder_24h", "txt", "pending",
		now.Add(-time.Minute), nil, nil, now)

	mock.ExpectQuery(`status = 'pending' AND scheduled_at <= \$1`).
		WithArgs(now, 50).
		WillReturnRows(rows)

	got, err := notifs.PickDue(context.Background(), mock, now, 0)
	if err != nil {
		t.Fatalf("PickDue: %v", err)
	}
	if len(got) != 1 || got[0].Type != domain.NotifReminder24h {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestNotifMarkSent(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	notifs := repo.NewNotifications()

	at := time.Now()
	mock.ExpectExec(`UPDATE notifications SET status = 'sent', sent_at = $2, error = NULL WHERE id = $1`).
		WithArgs(int64(5), at).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := notifs.MarkSent(context.Background(), mock, 5, at); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
}

func TestNotifMarkFailedTruncates(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	notifs := repo.NewNotifications()

	// Длинная ошибка должна быть обрезана до 1024 байт.
	longErr := make([]byte, 2000)
	for i := range longErr {
		longErr[i] = 'x'
	}
	expected := string(longErr[:1021]) + "..."

	mock.ExpectExec(`UPDATE notifications SET status = 'failed', error = $2 WHERE id = $1`).
		WithArgs(int64(5), expected).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := notifs.MarkFailed(context.Background(), mock, 5, string(longErr)); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
}
