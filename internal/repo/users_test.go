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

// newMock возвращает pgxmock пула с QueryMatcherEqual (строгое совпадение SQL).
// cleanup-функция автоматически проверит ExpectationsWereMet.
func newMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	return newMockWith(t, pgxmock.QueryMatcherEqual)
}

// newMockRegex — то же самое, но матчер regex (для длинных SQL, где
// удобнее сматчить ключевую подстроку).
func newMockRegex(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	return newMockWith(t, pgxmock.QueryMatcherRegexp)
}

func newMockWith(t *testing.T, matcher pgxmock.QueryMatcher) pgxmock.PgxPoolIface {
	t.Helper()
	mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
		mock.Close()
	})
	return mock
}

func TestUsersEnsureByMaxIDInserts(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	users := repo.NewUsers()

	const sql = `
INSERT INTO users (max_user_id, role, locale)
VALUES ($1, 'applicant', 'ru')
ON CONFLICT (max_user_id) DO UPDATE
SET updated_at = users.updated_at
RETURNING id, max_user_id, full_name, phone, email, role, locale,
    consent_at, consent_policy_ver, created_at, updated_at`

	rows := pgxmock.NewRows([]string{
		"id", "max_user_id", "full_name", "phone", "email", "role", "locale",
		"consent_at", "consent_policy_ver", "created_at", "updated_at",
	}).AddRow(int64(1), int64(42), nil, nil, nil, "applicant", "ru",
		nil, nil, time.Now(), time.Now())

	mock.ExpectQuery(sql).WithArgs(int64(42)).WillReturnRows(rows)

	u, err := users.EnsureByMaxID(context.Background(), mock, 42)
	if err != nil {
		t.Fatalf("EnsureByMaxID: %v", err)
	}
	if u.ID != 1 || u.MaxUserID != 42 || u.Role != domain.RoleApplicant {
		t.Fatalf("unexpected user: %+v", u)
	}
	if u.HasConsent() {
		t.Errorf("new user must not have consent")
	}
}

func TestUsersGetByMaxIDNoRows(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	users := repo.NewUsers()

	mock.ExpectQuery(`SELECT id, max_user_id, full_name, phone, email, role, locale,
    consent_at, consent_policy_ver, created_at, updated_at FROM users WHERE max_user_id = $1`).
		WithArgs(int64(99)).
		WillReturnError(pgx.ErrNoRows)

	u, err := users.GetByMaxID(context.Background(), mock, 99)
	if err != nil {
		t.Fatalf("GetByMaxID: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil user for missing max_user_id, got %+v", u)
	}
}

func TestUsersGrantConsent(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	users := repo.NewUsers()

	const sql = `
UPDATE users
SET consent_at = NOW(),
    consent_policy_ver = $2,
    updated_at = NOW()
WHERE id = $1`

	mock.ExpectExec(sql).WithArgs(int64(7), "v1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := users.GrantConsent(context.Background(), mock, 7, "v1"); err != nil {
		t.Fatalf("GrantConsent: %v", err)
	}
}

func TestUsersForgetMeDeletes(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	users := repo.NewUsers()

	mock.ExpectExec(`DELETE FROM users WHERE id = $1`).
		WithArgs(int64(7)).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	if err := users.ForgetMe(context.Background(), mock, 7); err != nil {
		t.Fatalf("ForgetMe: %v", err)
	}
}

func TestUsersSetRole(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	users := repo.NewUsers()

	mock.ExpectExec(`UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1`).
		WithArgs(int64(7), "organizer").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := users.SetRole(context.Background(), mock, 7, domain.RoleOrganizer); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
}
