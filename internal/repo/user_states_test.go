package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

func TestUserStatesLoadDefaultIfMissing(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	st := repo.NewUserStates()

	mock.ExpectQuery(`SELECT state, context FROM user_states WHERE user_id = $1`).
		WithArgs(int64(7)).
		WillReturnError(pgx.ErrNoRows)

	state, data, err := st.Load(context.Background(), mock, 7)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state != "main_menu" {
		t.Errorf("state: want main_menu, got %s", state)
	}
	if string(data) != "{}" {
		t.Errorf("context: want '{}', got %q", string(data))
	}
}

func TestUserStatesLoadExisting(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	st := repo.NewUserStates()

	mock.ExpectQuery(`SELECT state, context FROM user_states WHERE user_id = $1`).
		WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows([]string{"state", "context"}).
			AddRow("reg_full_name", []byte(`{"current_event_id":1}`)))

	state, data, err := st.Load(context.Background(), mock, 7)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state != "reg_full_name" {
		t.Errorf("state: want reg_full_name, got %s", state)
	}
	if string(data) != `{"current_event_id":1}` {
		t.Errorf("context: %q", string(data))
	}
}

func TestUserStatesSaveUpsert(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	st := repo.NewUserStates()

	mock.ExpectExec(`INSERT INTO user_states`).
		WithArgs(int64(7), "reg_contact", []byte(`{"current_event_id":1}`)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := st.Save(context.Background(), mock, 7, "reg_contact", []byte(`{"current_event_id":1}`))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestUserStatesSaveDefaultsEmptyValues(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	st := repo.NewUserStates()

	// Empty state -> defaults to main_menu; empty context -> "{}"
	mock.ExpectExec(`INSERT INTO user_states`).
		WithArgs(int64(7), "main_menu", []byte("{}")).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := st.Save(context.Background(), mock, 7, "", nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestUserStatesPurgeStaleBeforeReturnsRows(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	st := repo.NewUserStates()

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	mock.ExpectExec(`DELETE FROM user_states WHERE updated_at < $1`).
		WithArgs(cutoff).
		WillReturnResult(pgxmock.NewResult("DELETE", 5))

	n, err := st.PurgeStaleBefore(context.Background(), mock, cutoff)
	if err != nil {
		t.Fatalf("PurgeStaleBefore: %v", err)
	}
	if n != 5 {
		t.Errorf("want 5 rows, got %d", n)
	}
}
