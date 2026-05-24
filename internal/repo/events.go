package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

type eventsRepo struct{}

// NewEvents создаёт репозиторий events.
func NewEvents() EventRepo { return &eventsRepo{} }

const eventColumns = `id, title, description, short_summary, starts_at, ends_at,
    location, format, capacity, status, created_by, tags, late_cancel_allowed, created_at, updated_at`

func (r *eventsRepo) Create(ctx context.Context, q Querier, e *domain.Event) (int64, error) {
	if e == nil {
		return 0, fmt.Errorf("event: nil")
	}
	if e.Status == "" {
		e.Status = domain.EventStatusOpen
	}
	if e.Format == "" {
		e.Format = domain.EventFormatOffline
	}
	if e.Tags == nil {
		e.Tags = []string{}
	}

	const stmt = `
INSERT INTO events (title, description, short_summary, starts_at, ends_at,
    location, format, capacity, status, created_by, tags, late_cancel_allowed)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, created_at, updated_at`

	err := q.QueryRow(ctx, stmt,
		e.Title, e.Description, e.ShortSummary, e.StartsAt, e.EndsAt,
		e.Location, string(e.Format), e.Capacity, string(e.Status), e.CreatedBy, e.Tags,
		e.LateCancelAllowed,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	return e.ID, nil
}

func (r *eventsRepo) Get(ctx context.Context, q Querier, id int64) (*domain.Event, error) {
	const stmt = `SELECT ` + eventColumns + ` FROM events WHERE id = $1`
	e := &domain.Event{}
	if err := scanEvent(q.QueryRow(ctx, stmt, id), e); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get event: %w", err)
	}
	return e, nil
}

// GetForUpdate — ровно тот же SELECT, но с FOR UPDATE.
// Используется в транзакции регистрации, чтобы заблокировать строку
// события и безопасно посчитать registered под capacity.
func (r *eventsRepo) GetForUpdate(ctx context.Context, q Querier, id int64) (*domain.Event, error) {
	const stmt = `SELECT ` + eventColumns + ` FROM events WHERE id = $1 FOR UPDATE`
	e := &domain.Event{}
	if err := scanEvent(q.QueryRow(ctx, stmt, id), e); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get event for update: %w", err)
	}
	return e, nil
}

// ListOpen возвращает открытые мероприятия, отсортированные по starts_at.
// total — общее число открытых событий (для пагинации в боте/админке).
func (r *eventsRepo) ListOpen(ctx context.Context, q Querier, limit, offset int) ([]*domain.Event, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	const stmt = `
SELECT ` + eventColumns + `,
       (SELECT COUNT(*) FROM events WHERE status = 'open') AS total
FROM events
WHERE status = 'open'
ORDER BY starts_at ASC
LIMIT $1 OFFSET $2`

	rows, err := q.Query(ctx, stmt, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list open events: %w", err)
	}
	defer rows.Close()

	var total int
	out := make([]*domain.Event, 0, limit)
	for rows.Next() {
		e := &domain.Event{}
		if err := scanEventWithTotal(rows, e, &total); err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter events: %w", err)
	}
	return out, total, nil
}

func (r *eventsRepo) ListByOrganizer(ctx context.Context, q Querier, organizerID int64) ([]*domain.Event, error) {
	const stmt = `
SELECT ` + eventColumns + `
FROM events
WHERE created_by = $1
ORDER BY starts_at DESC`

	rows, err := q.Query(ctx, stmt, organizerID)
	if err != nil {
		return nil, fmt.Errorf("list by organizer: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Event, 0, 16)
	for rows.Next() {
		e := &domain.Event{}
		if err := scanEvent(rows, e); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter events: %w", err)
	}
	return out, nil
}

// ListUpcomingForReminders возвращает open-события, которые стартуют в пределах within.
// Используется scheduler'ом для планирования reminder_24h и reminder_1h.
func (r *eventsRepo) ListUpcomingForReminders(ctx context.Context, q Querier, within time.Duration) ([]*domain.Event, error) {
	const stmt = `
SELECT ` + eventColumns + `
FROM events
WHERE status = 'open' AND starts_at BETWEEN NOW() AND NOW() + ($1::interval)
ORDER BY starts_at ASC`

	// pgx умеет передавать time.Duration как interval, но мы конвертируем
	// сами для предсказуемости: «3 hours» легче дебажить, чем «3h0m0s».
	intervalStr := fmt.Sprintf("%d seconds", int(within.Seconds()))

	rows, err := q.Query(ctx, stmt, intervalStr)
	if err != nil {
		return nil, fmt.Errorf("list upcoming events: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Event, 0, 16)
	for rows.Next() {
		e := &domain.Event{}
		if err := scanEvent(rows, e); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter events: %w", err)
	}
	return out, nil
}

func (r *eventsRepo) UpdateStatus(ctx context.Context, q Querier, id int64, st domain.EventStatus) error {
	const stmt = `UPDATE events SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, string(st))
	if err != nil {
		return fmt.Errorf("update event status: %w", err)
	}
	return nil
}

func (r *eventsRepo) Update(ctx context.Context, q Querier, e *domain.Event) error {
	if e == nil {
		return fmt.Errorf("event: nil")
	}
	const stmt = `
UPDATE events
SET title = $2,
    description = $3,
    short_summary = $4,
    starts_at = $5,
    ends_at = $6,
    location = $7,
    format = $8,
    capacity = $9,
    status = $10,
    tags = $11,
    late_cancel_allowed = $12,
    updated_at = NOW()
WHERE id = $1
RETURNING updated_at`

	return q.QueryRow(ctx, stmt,
		e.ID, e.Title, e.Description, e.ShortSummary, e.StartsAt, e.EndsAt,
		e.Location, string(e.Format), e.Capacity, string(e.Status), e.Tags,
		e.LateCancelAllowed,
	).Scan(&e.UpdatedAt)
}

func (r *eventsRepo) UpdateShortSummary(ctx context.Context, q Querier, id int64, summary string) error {
	const stmt = `UPDATE events SET short_summary = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, summary)
	if err != nil {
		return fmt.Errorf("update short summary: %w", err)
	}
	return nil
}

// Stats — управленческая сводка по событию.
func (r *eventsRepo) Stats(ctx context.Context, q Querier, eventID int64) (*domain.EventStats, error) {
	stats := &domain.EventStats{TopInterests: map[string]int{}}

	// Одним запросом — счётчики по статусам.
	const counts = `
SELECT
  e.capacity,
  COUNT(*) FILTER (WHERE r.status = 'registered')             AS registered,
  COUNT(*) FILTER (WHERE r.status IN ('cancelled_by_user','cancelled_by_organizer')) AS cancelled,
  COUNT(*) FILTER (WHERE r.status = 'waitlist')               AS waitlist,
  COUNT(*) FILTER (WHERE r.status = 'attended')               AS attended,
  COUNT(*) FILTER (WHERE r.status = 'no_show')                AS no_show
FROM events e
LEFT JOIN registrations r ON r.event_id = e.id
WHERE e.id = $1
GROUP BY e.id, e.capacity`

	err := q.QueryRow(ctx, counts, eventID).Scan(
		&stats.Capacity, &stats.Registered, &stats.Cancelled,
		&stats.Waitlist, &stats.Attended, &stats.NoShow,
	)
	if err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stats counts: %w", err)
	}
	stats.FreeSeats = stats.Capacity - stats.Registered
	if stats.FreeSeats < 0 {
		stats.FreeSeats = 0
	}

	// Второй запрос — топ-5 интересов.
	const topInterests = `
SELECT interest_program, COUNT(*) AS cnt
FROM registrations
WHERE event_id = $1 AND interest_program IS NOT NULL AND interest_program <> ''
GROUP BY interest_program
ORDER BY cnt DESC, interest_program ASC
LIMIT 5`

	rows, err := q.Query(ctx, topInterests, eventID)
	if err != nil {
		return nil, fmt.Errorf("stats top interests: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k string
		var v int
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan interest: %w", err)
		}
		stats.TopInterests[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter top interests: %w", err)
	}
	return stats, nil
}

func scanEvent(row interface{ Scan(...any) error }, e *domain.Event) error {
	var status, format string
	if err := row.Scan(
		&e.ID, &e.Title, &e.Description, &e.ShortSummary, &e.StartsAt, &e.EndsAt,
		&e.Location, &format, &e.Capacity, &status, &e.CreatedBy, &e.Tags,
		&e.LateCancelAllowed,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return err
	}
	e.Status = domain.EventStatus(status)
	e.Format = domain.EventFormat(format)
	return nil
}

func scanEventWithTotal(row interface{ Scan(...any) error }, e *domain.Event, total *int) error {
	var status, format string
	if err := row.Scan(
		&e.ID, &e.Title, &e.Description, &e.ShortSummary, &e.StartsAt, &e.EndsAt,
		&e.Location, &format, &e.Capacity, &status, &e.CreatedBy, &e.Tags,
		&e.LateCancelAllowed,
		&e.CreatedAt, &e.UpdatedAt, total,
	); err != nil {
		return err
	}
	e.Status = domain.EventStatus(status)
	e.Format = domain.EventFormat(format)
	return nil
}
