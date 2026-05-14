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

func regCols() []string {
	return []string{
		"id", "user_id", "event_id", "status", "interest_program",
		"full_name_snapshot", "contact_snapshot", "waitlist_position",
		"registered_at", "cancelled_at", "source",
		"attendance_code", "checkin_at", "checkin_by", "qr_sent_message_id",
		"created_at", "updated_at",
	}
}

func TestRegCreateInsertsRegistered(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	r := &domain.Registration{
		UserID:           1,
		EventID:          2,
		Status:           domain.RegStatusRegistered,
		FullNameSnapshot: "Test Name",
		ContactSnapshot:  "test@example.com",
	}
	// 9 параметров (см. registrations.go Create): user_id, event_id, status,
	// interest_program, full_name, contact, waitlist_position, registered_at, source.
	mock.ExpectQuery(`INSERT INTO registrations`).
		WithArgs(
			int64(1), int64(2), "registered",
			(*string)(nil), "Test Name", "test@example.com",
			(*int)(nil), pgxmock.AnyArg(), "bot",
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(int64(99), time.Now(), time.Now()))

	id, err := regs.Create(context.Background(), mock, r)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != 99 {
		t.Fatalf("want id=99, got %d", id)
	}
	if r.RegisteredAt == nil {
		t.Errorf("RegisteredAt must be set for registered status")
	}
	if r.Source != "bot" {
		t.Errorf("source default: want 'bot', got %q", r.Source)
	}
}

func TestRegGetActiveByUserEventReturnsNil(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	mock.ExpectQuery(`SELECT .* FROM registrations WHERE user_id = \$1 AND event_id = \$2`).
		WithArgs(int64(7), int64(10)).
		WillReturnError(pgx.ErrNoRows)

	r, err := regs.GetActiveByUserEvent(context.Background(), mock, 7, 10)
	if err != nil {
		t.Fatalf("GetActiveByUserEvent: %v", err)
	}
	if r != nil {
		t.Errorf("want nil for no active reg, got %+v", r)
	}
}

func TestRegCountByEvent(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM registrations WHERE event_id = \$1 AND status = \$2`).
		WithArgs(int64(5), "registered").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(42)))

	c, err := regs.CountByEvent(context.Background(), mock, 5, domain.RegStatusRegistered)
	if err != nil {
		t.Fatalf("CountByEvent: %v", err)
	}
	if c != 42 {
		t.Errorf("want 42, got %d", c)
	}
}

func TestRegNextWaitlistEmptyQueue(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	mock.ExpectQuery(`status = 'waitlist'`).
		WithArgs(int64(5)).
		WillReturnError(pgx.ErrNoRows)

	r, err := regs.NextWaitlist(context.Background(), mock, 5)
	if err != nil {
		t.Fatalf("NextWaitlist: %v", err)
	}
	if r != nil {
		t.Errorf("want nil for empty queue, got %+v", r)
	}
}

func TestRegNextWaitlistPositionFirstInLine(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	// MAX==NULL → COALESCE возвращает 1.
	mock.ExpectQuery(`COALESCE\(MAX\(waitlist_position\), 0\) \+ 1`).
		WithArgs(int64(5)).
		WillReturnRows(pgxmock.NewRows([]string{"pos"}).AddRow(int(1)))

	pos, err := regs.NextWaitlistPosition(context.Background(), mock, 5)
	if err != nil {
		t.Fatalf("NextWaitlistPosition: %v", err)
	}
	if pos != 1 {
		t.Errorf("want 1, got %d", pos)
	}
}

func TestRegNextWaitlistPositionAfterSeveral(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	mock.ExpectQuery(`COALESCE\(MAX\(waitlist_position\), 0\) \+ 1`).
		WithArgs(int64(5)).
		WillReturnRows(pgxmock.NewRows([]string{"pos"}).AddRow(int(4)))

	pos, err := regs.NextWaitlistPosition(context.Background(), mock, 5)
	if err != nil {
		t.Fatalf("NextWaitlistPosition: %v", err)
	}
	if pos != 4 {
		t.Errorf("want 4, got %d", pos)
	}
}

func TestRegUpdateStatusSetsCancelledAt(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	regs := repo.NewRegistrations()

	const sql = `
UPDATE registrations
SET status = $2,
    cancelled_at = CASE
        WHEN $2 IN ('cancelled_by_user','cancelled_by_organizer') THEN NOW()
        ELSE cancelled_at
    END,
    updated_at = NOW()
WHERE id = $1`

	mock.ExpectExec(sql).
		WithArgs(int64(77), "cancelled_by_user").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := regs.UpdateStatus(context.Background(), mock, 77, domain.RegStatusCancelledByUser)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
}

func TestRegMarkAttended(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	mock.ExpectExec(`UPDATE registrations\s+SET status = 'attended'`).
		WithArgs(int64(11), int64(22)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := regs.MarkAttended(context.Background(), mock, 11, 22); err != nil {
		t.Fatalf("MarkAttended: %v", err)
	}
}

func TestRegMarkNoShow(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	// SQL должен сбрасывать checkin_at в NULL — иначе при коррекции
	// attended → no_show останется устаревший timestamp.
	mock.ExpectExec(`UPDATE registrations\s+SET status = 'no_show',\s+checkin_at = NULL,\s+checkin_by = \$2`).
		WithArgs(int64(11), int64(22)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := regs.MarkNoShow(context.Background(), mock, 11, 22); err != nil {
		t.Fatalf("MarkNoShow: %v", err)
	}
}

func TestRegListByEventAllStatuses(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	now := time.Now()
	rows := pgxmock.NewRows(regCols()).
		AddRow(int64(1), int64(10), int64(5), "registered", (*string)(nil),
			"User A", "+79991110000", (*int)(nil),
			&now, (*time.Time)(nil), "bot",
			(*string)(nil), (*time.Time)(nil), (*int64)(nil), (*int64)(nil),
			now, now).
		AddRow(int64(2), int64(11), int64(5), "cancelled_by_user", (*string)(nil),
			"User B", "userb@x.com", (*int)(nil),
			&now, &now, "bot",
			(*string)(nil), (*time.Time)(nil), (*int64)(nil), (*int64)(nil),
			now, now)

	mock.ExpectQuery(`FROM registrations WHERE event_id = \$1\s+ORDER BY`).
		WithArgs(int64(5), 100, 0).
		WillReturnRows(rows)

	got, err := regs.ListByEventAllStatuses(context.Background(), mock, 5, 100, 0)
	if err != nil {
		t.Fatalf("ListByEventAllStatuses: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].Status != domain.RegStatusRegistered || got[1].Status != domain.RegStatusCancelledByUser {
		t.Errorf("statuses mismatch: %v / %v", got[0].Status, got[1].Status)
	}
}

func TestRegSetAttendanceCode(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	regs := repo.NewRegistrations()

	mock.ExpectExec(`UPDATE registrations SET attendance_code = $2, updated_at = NOW() WHERE id = $1`).
		WithArgs(int64(11), "deadbeefdeadbeefdeadbeefdeadbeef").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := regs.SetAttendanceCode(context.Background(), mock, 11, "deadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatalf("SetAttendanceCode: %v", err)
	}
}

func TestRegGetByCodeForUpdate(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	regs := repo.NewRegistrations()

	// Используем *time.Time для timestamptz-полей которые в БД могут быть NULL.
	now := time.Now()
	regAt := now
	rows := pgxmock.NewRows(regCols()).
		AddRow(int64(11), int64(1), int64(2), "registered", (*string)(nil),
			"Test", "test@x.com", (*int)(nil),
			&regAt, (*time.Time)(nil), "bot",
			pStr("deadbeef"), (*time.Time)(nil), (*int64)(nil), (*int64)(nil),
			now, now)

	mock.ExpectQuery(`attendance_code = \$1 FOR UPDATE`).
		WithArgs("deadbeef").
		WillReturnRows(rows)

	r, err := regs.GetByCodeForUpdate(context.Background(), mock, "deadbeef")
	if err != nil {
		t.Fatalf("GetByCodeForUpdate: %v", err)
	}
	if r == nil || r.ID != 11 {
		t.Errorf("want id=11, got %+v", r)
	}
}

func pStr(s string) *string { return &s }
