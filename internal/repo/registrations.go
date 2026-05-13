package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

type registrationsRepo struct{}

// NewRegistrations создаёт репозиторий registrations.
func NewRegistrations() RegistrationRepo { return &registrationsRepo{} }

const regColumns = `id, user_id, event_id, status, interest_program,
    full_name_snapshot, contact_snapshot, waitlist_position,
    registered_at, cancelled_at, source,
    attendance_code, checkin_at, checkin_by, qr_sent_message_id,
    created_at, updated_at`

// Create вставляет или «реактивирует» запись (если до этого она была cancelled).
//
// ON CONFLICT (user_id, event_id) гарантирует one-active-per-pair: повторный
// Register после отмены обновляет ту же строку, очищая cancelled_at.
// service.Registration сам проверяет статус ДО Create и кидает ErrAlreadyRegistered,
// если уже есть активная запись (см. план §15.3).
func (r *registrationsRepo) Create(ctx context.Context, q Querier, reg *domain.Registration) (int64, error) {
	if reg == nil {
		return 0, fmt.Errorf("registration: nil")
	}
	if reg.Source == "" {
		reg.Source = "bot"
	}
	if reg.RegisteredAt == nil && reg.Status == domain.RegStatusRegistered {
		now := time.Now()
		reg.RegisteredAt = &now
	}

	const stmt = `
INSERT INTO registrations
  (user_id, event_id, status, interest_program, full_name_snapshot,
   contact_snapshot, waitlist_position, registered_at, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, event_id) DO UPDATE
SET status              = EXCLUDED.status,
    interest_program    = EXCLUDED.interest_program,
    full_name_snapshot  = EXCLUDED.full_name_snapshot,
    contact_snapshot    = EXCLUDED.contact_snapshot,
    waitlist_position   = EXCLUDED.waitlist_position,
    registered_at       = COALESCE(registrations.registered_at, EXCLUDED.registered_at),
    cancelled_at        = NULL,
    updated_at          = NOW()
RETURNING id, created_at, updated_at`

	err := q.QueryRow(ctx, stmt,
		reg.UserID, reg.EventID, string(reg.Status), reg.InterestProgram,
		reg.FullNameSnapshot, reg.ContactSnapshot, reg.WaitlistPosition,
		reg.RegisteredAt, reg.Source,
	).Scan(&reg.ID, &reg.CreatedAt, &reg.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("upsert registration: %w", err)
	}
	return reg.ID, nil
}

func (r *registrationsRepo) Get(ctx context.Context, q Querier, id int64) (*domain.Registration, error) {
	const stmt = `SELECT ` + regColumns + ` FROM registrations WHERE id = $1`
	reg := &domain.Registration{}
	if err := scanRegistration(q.QueryRow(ctx, stmt, id), reg); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get registration: %w", err)
	}
	return reg, nil
}

func (r *registrationsRepo) GetByCode(ctx context.Context, q Querier, code string) (*domain.Registration, error) {
	const stmt = `SELECT ` + regColumns + ` FROM registrations WHERE attendance_code = $1`
	reg := &domain.Registration{}
	if err := scanRegistration(q.QueryRow(ctx, stmt, code), reg); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get registration by code: %w", err)
	}
	return reg, nil
}

// GetByCodeForUpdate — для check-in транзакции. Блокирует строку, чтобы
// два одновременных скана QR-кода не привели к двойной отметке attended.
func (r *registrationsRepo) GetByCodeForUpdate(ctx context.Context, q Querier, code string) (*domain.Registration, error) {
	const stmt = `SELECT ` + regColumns + ` FROM registrations WHERE attendance_code = $1 FOR UPDATE`
	reg := &domain.Registration{}
	if err := scanRegistration(q.QueryRow(ctx, stmt, code), reg); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get registration by code for update: %w", err)
	}
	return reg, nil
}

// GetActiveByUserEvent возвращает активную (registered/waitlist) запись пары.
func (r *registrationsRepo) GetActiveByUserEvent(ctx context.Context, q Querier, userID, eventID int64) (*domain.Registration, error) {
	const stmt = `
SELECT ` + regColumns + `
FROM registrations
WHERE user_id = $1 AND event_id = $2 AND status IN ('registered', 'waitlist')`

	reg := &domain.Registration{}
	if err := scanRegistration(q.QueryRow(ctx, stmt, userID, eventID), reg); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active reg: %w", err)
	}
	return reg, nil
}

// UpdateStatus меняет статус и проставляет cancelled_at автоматически,
// если переходим в любую из cancelled-веток.
func (r *registrationsRepo) UpdateStatus(ctx context.Context, q Querier, id int64, st domain.RegistrationStatus) error {
	const stmt = `
UPDATE registrations
SET status = $2,
    cancelled_at = CASE
        WHEN $2 IN ('cancelled_by_user','cancelled_by_organizer') THEN NOW()
        ELSE cancelled_at
    END,
    updated_at = NOW()
WHERE id = $1`

	_, err := q.Exec(ctx, stmt, id, string(st))
	if err != nil {
		return fmt.Errorf("update reg status: %w", err)
	}
	return nil
}

// SetAttendanceCode присваивает UUID-код. Используется сразу после Create в той же транзакции.
func (r *registrationsRepo) SetAttendanceCode(ctx context.Context, q Querier, id int64, code string) error {
	const stmt = `UPDATE registrations SET attendance_code = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, code)
	if err != nil {
		return fmt.Errorf("set attendance code: %w", err)
	}
	return nil
}

// SetQRMessageID сохраняет id PNG-сообщения, чтобы переотдавать.
func (r *registrationsRepo) SetQRMessageID(ctx context.Context, q Querier, id int64, messageID int64) error {
	const stmt = `UPDATE registrations SET qr_sent_message_id = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, messageID)
	if err != nil {
		return fmt.Errorf("set qr message id: %w", err)
	}
	return nil
}

// MarkAttended проставляет статус 'attended' + checkin_at = NOW + checkin_by = byUserID.
// Это финальная операция check-in (см. День 15, service.Attendance.CheckIn).
func (r *registrationsRepo) MarkAttended(ctx context.Context, q Querier, id int64, byUserID int64) error {
	const stmt = `
UPDATE registrations
SET status = 'attended',
    checkin_at = NOW(),
    checkin_by = $2,
    updated_at = NOW()
WHERE id = $1`

	_, err := q.Exec(ctx, stmt, id, byUserID)
	if err != nil {
		return fmt.Errorf("mark attended: %w", err)
	}
	return nil
}

func (r *registrationsRepo) ListByEvent(ctx context.Context, q Querier, eventID int64, status domain.RegistrationStatus, limit, offset int) ([]*domain.Registration, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// pgx.Rows используем как конкретный тип (без анонимного интерфейса),
	// чтобы sqlclosecheck корректно отследил defer Close().
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		const stmt = `
SELECT ` + regColumns + `
FROM registrations
WHERE event_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3`
		rows, err = q.Query(ctx, stmt, eventID, limit, offset)
	} else {
		const stmt = `
SELECT ` + regColumns + `
FROM registrations
WHERE event_id = $1 AND status = $2
ORDER BY created_at ASC
LIMIT $3 OFFSET $4`
		rows, err = q.Query(ctx, stmt, eventID, string(status), limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list by event: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Registration, 0, limit)
	for rows.Next() {
		reg := &domain.Registration{}
		if err := scanRegistration(rows, reg); err != nil {
			return nil, fmt.Errorf("scan reg: %w", err)
		}
		out = append(out, reg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter regs: %w", err)
	}
	return out, nil
}

func (r *registrationsRepo) ListByUser(ctx context.Context, q Querier, userID int64, activeOnly bool) ([]*domain.Registration, error) {
	var stmt string
	if activeOnly {
		stmt = `SELECT ` + regColumns + ` FROM registrations
WHERE user_id = $1 AND status IN ('registered','waitlist')
ORDER BY created_at DESC`
	} else {
		stmt = `SELECT ` + regColumns + ` FROM registrations
WHERE user_id = $1
ORDER BY created_at DESC`
	}

	rows, err := q.Query(ctx, stmt, userID)
	if err != nil {
		return nil, fmt.Errorf("list by user: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Registration, 0, 8)
	for rows.Next() {
		reg := &domain.Registration{}
		if err := scanRegistration(rows, reg); err != nil {
			return nil, fmt.Errorf("scan reg: %w", err)
		}
		out = append(out, reg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter regs: %w", err)
	}
	return out, nil
}

func (r *registrationsRepo) CountByEvent(ctx context.Context, q Querier, eventID int64, status domain.RegistrationStatus) (int, error) {
	const stmt = `SELECT COUNT(*) FROM registrations WHERE event_id = $1 AND status = $2`
	var c int
	if err := q.QueryRow(ctx, stmt, eventID, string(status)).Scan(&c); err != nil {
		return 0, fmt.Errorf("count by event: %w", err)
	}
	return c, nil
}

// NextWaitlist — следующий за registered (минимальный waitlist_position).
func (r *registrationsRepo) NextWaitlist(ctx context.Context, q Querier, eventID int64) (*domain.Registration, error) {
	const stmt = `
SELECT ` + regColumns + `
FROM registrations
WHERE event_id = $1 AND status = 'waitlist'
ORDER BY waitlist_position ASC NULLS LAST, created_at ASC
LIMIT 1`

	reg := &domain.Registration{}
	if err := scanRegistration(q.QueryRow(ctx, stmt, eventID), reg); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("next waitlist: %w", err)
	}
	return reg, nil
}

// NextWaitlistPosition — какое значение присвоить новому waitlist-участнику.
// Возвращает MAX(position) + 1 или 1 если очередь пустая.
func (r *registrationsRepo) NextWaitlistPosition(ctx context.Context, q Querier, eventID int64) (int, error) {
	const stmt = `
SELECT COALESCE(MAX(waitlist_position), 0) + 1
FROM registrations
WHERE event_id = $1 AND status = 'waitlist'`

	var pos int
	if err := q.QueryRow(ctx, stmt, eventID).Scan(&pos); err != nil {
		return 0, fmt.Errorf("next waitlist position: %w", err)
	}
	return pos, nil
}

func (r *registrationsRepo) AssignWaitlistPosition(ctx context.Context, q Querier, registrationID int64, pos int) error {
	const stmt = `UPDATE registrations SET waitlist_position = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, registrationID, pos)
	if err != nil {
		return fmt.Errorf("assign waitlist position: %w", err)
	}
	return nil
}

func scanRegistration(row interface{ Scan(...any) error }, r *domain.Registration) error {
	var status string
	if err := row.Scan(
		&r.ID, &r.UserID, &r.EventID, &status, &r.InterestProgram,
		&r.FullNameSnapshot, &r.ContactSnapshot, &r.WaitlistPosition,
		&r.RegisteredAt, &r.CancelledAt, &r.Source,
		&r.AttendanceCode, &r.CheckinAt, &r.CheckinBy, &r.QRSentMessageID,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return err
	}
	r.Status = domain.RegistrationStatus(status)
	return nil
}
