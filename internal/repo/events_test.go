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

func eventCols() []string {
	return []string{
		"id", "title", "description", "short_summary", "starts_at", "ends_at",
		"location", "format", "capacity", "status", "created_by", "tags", "late_cancel_allowed",
		"created_at", "updated_at",
	}
}

func TestEventsCreate(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	events := repo.NewEvents()

	starts := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	ev := &domain.Event{
		Title:       "Test",
		Description: "Desc",
		StartsAt:    starts,
		Location:    "loc",
		Format:      domain.EventFormatOffline,
		Capacity:    50,
		Status:      domain.EventStatusOpen,
		Tags:        []string{"x"},
	}

	mock.ExpectQuery(`INSERT INTO events`).
		WithArgs(ev.Title, ev.Description, (*string)(nil), starts, (*time.Time)(nil),
			ev.Location, "offline", 50, "open", (*int64)(nil), []string{"x"}, false).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(int64(42), time.Now(), time.Now()))

	id, err := events.Create(context.Background(), mock, ev)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != 42 || ev.ID != 42 {
		t.Fatalf("want id=42, got id=%d ev.ID=%d", id, ev.ID)
	}
}

func TestEventsGetReturnsEvent(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	events := repo.NewEvents()

	rows := pgxmock.NewRows(eventCols()).
		AddRow(int64(1), "Test", "Desc", nil, time.Now(), nil,
			"loc", "online", 100, "open", nil, []string{"a"}, false,
			time.Now(), time.Now())

	mock.ExpectQuery(`SELECT id, title, description`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ev, err := events.Get(context.Background(), mock, 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ev == nil {
		t.Fatal("got nil event")
		return
	}
	if ev.Format != domain.EventFormatOnline {
		t.Errorf("format: want online, got %s", ev.Format)
	}
	if ev.Status != domain.EventStatusOpen {
		t.Errorf("status: want open, got %s", ev.Status)
	}
}

func TestEventsGetNoRows(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	events := repo.NewEvents()

	mock.ExpectQuery(`SELECT id, title, description`).
		WithArgs(int64(999)).
		WillReturnError(pgx.ErrNoRows)

	ev, err := events.Get(context.Background(), mock, 999)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ev != nil {
		t.Errorf("want nil for missing event, got %+v", ev)
	}
}

func TestEventsUpdateStatus(t *testing.T) {
	t.Parallel()
	mock := newMock(t)
	events := repo.NewEvents()

	mock.ExpectExec(`UPDATE events SET status = $2, updated_at = NOW() WHERE id = $1`).
		WithArgs(int64(5), "closed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := events.UpdateStatus(context.Background(), mock, 5, domain.EventStatusClosed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
}

func TestEventsStatsHappyPath(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	events := repo.NewEvents()

	// 1) counts
	mock.ExpectQuery(`COUNT.*registered`).
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{
			"capacity", "registered", "cancelled", "waitlist", "attended", "no_show",
		}).AddRow(100, 67, 5, 10, 7, 0))

	// 2) top interests
	mock.ExpectQuery(`interest_program, COUNT`).
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"interest_program", "cnt"}).
			AddRow("Прикладная информатика", 30).
			AddRow("Безопасность", 15))

	stats, err := events.Stats(context.Background(), mock, 1)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Capacity != 100 || stats.Registered != 67 || stats.Attended != 7 || stats.FreeSeats != 33 {
		t.Errorf("counts wrong: %+v", stats)
	}
	if stats.TopInterests["Прикладная информатика"] != 30 {
		t.Errorf("top interests wrong: %+v", stats.TopInterests)
	}
}

func TestEventsStatsNoEvent(t *testing.T) {
	t.Parallel()
	mock := newMockRegex(t)
	events := repo.NewEvents()

	mock.ExpectQuery(`COUNT.*registered`).
		WithArgs(int64(404)).
		WillReturnError(pgx.ErrNoRows)

	stats, err := events.Stats(context.Background(), mock, 404)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats != nil {
		t.Errorf("want nil stats for missing event, got %+v", stats)
	}
}
