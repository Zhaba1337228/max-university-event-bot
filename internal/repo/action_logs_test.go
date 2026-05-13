package repo_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

func TestActionLogsAppendDefaultPayload(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	logs := repo.NewActionLogs()

	uid := int64(7)
	evid := int64(5)
	log := &domain.ActionLog{
		ActorUserID: &uid,
		EventID:     &evid,
		Action:      domain.ActionRegistrationCreated,
		// Payload nil — должно превратиться в "{}"
	}
	mock.ExpectQuery(`INSERT INTO action_logs`).
		WithArgs(&uid, (*int64)(nil), &evid, (*int64)(nil), "registration_created", json.RawMessage(`{}`)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(123), time.Now()))

	if err := logs.Append(context.Background(), mock, log); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if log.ID != 123 {
		t.Errorf("want id=123, got %d", log.ID)
	}
}

func TestActionLogsListByUserDefaultLimit(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	logs := repo.NewActionLogs()

	uid := int64(7)
	rows := pgxmock.NewRows([]string{
		"id", "actor_user_id", "target_user_id", "event_id",
		"registration_id", "action", "payload", "created_at",
	}).AddRow(int64(1), &uid, (*int64)(nil), (*int64)(nil), (*int64)(nil),
		"registration_created", []byte(`{}`), time.Now())

	mock.ExpectQuery(`actor_user_id = \$1 OR target_user_id = \$1`).
		WithArgs(uid, 10).
		WillReturnRows(rows)

	got, err := logs.ListByUser(context.Background(), mock, uid, 0) // 0 → defaults to 10
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 log, got %d", len(got))
	}
	if got[0].Action != domain.ActionRegistrationCreated {
		t.Errorf("action: want registration_created, got %s", got[0].Action)
	}
}
