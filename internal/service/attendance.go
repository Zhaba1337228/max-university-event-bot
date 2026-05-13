package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Attendance — сервис check-in по QR-коду на странице организатора.
//
// CheckIn срабатывает на каждый успешный скан камерой:
//  1. ParseQRPayload (отсекаем чужие QR);
//  2. GetByCodeForUpdate (блокировка строки регистрации);
//  3. проверка ownership события (organizer/admin);
//  4. проверка статуса регистрации (active);
//  5. проверка окна check-in [starts_at - 2h, ends_at + 4h];
//  6. MarkAttended; ActionLog.checkin_scanned.
type Attendance interface {
	CheckIn(ctx context.Context, organizerMaxUserID int64, qrPayload string) (*CheckInResult, error)
}

// CheckInResult — что произошло.
type CheckInResult struct {
	Registration *domain.Registration
	Event        *domain.Event
	AlreadyDone  bool // если повторный скан того же QR — true, status уже attended
}

type attendanceService struct {
	pool   *pgxpool.Pool
	qr     QR
	regs   repo.RegistrationRepo
	events repo.EventRepo
	users  repo.UserRepo
	role   Role
	logs   repo.ActionLogRepo
}

// NewAttendance создаёт сервис.
func NewAttendance(pool *pgxpool.Pool, qr QR, regs repo.RegistrationRepo,
	events repo.EventRepo, users repo.UserRepo, role Role, logs repo.ActionLogRepo,
) Attendance {
	return &attendanceService{
		pool:   pool,
		qr:     qr,
		regs:   regs,
		events: events,
		users:  users,
		role:   role,
		logs:   logs,
	}
}

// Окно check-in: 2 часа до старта и 4 часа после планового окончания.
const (
	checkinPreWindow  = 2 * time.Hour
	checkinPostWindow = 4 * time.Hour
)

func (s *attendanceService) CheckIn(ctx context.Context, organizerMaxUserID int64, payload string) (*CheckInResult, error) {
	parsed, err := s.qr.ParseQRPayload(payload)
	if err != nil {
		return nil, err
	}

	var result *CheckInResult
	txErr := s.inTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		reg, err := s.regs.GetByCodeForUpdate(ctx, tx, parsed.AttendanceCode)
		if err != nil {
			return fmt.Errorf("get by code: %w", err)
		}
		if reg == nil {
			return ErrNotRegistered
		}
		if reg.EventID != parsed.EventID {
			// QR от другого события — не должен сработать.
			return fmt.Errorf("%w: event mismatch", ErrNotRegistered)
		}

		// Проверка прав организатора на это событие.
		if _, err := s.role.RequireEventOwner(ctx, organizerMaxUserID, reg.EventID); err != nil {
			return err
		}

		// Состояние записи.
		switch reg.Status {
		case domain.RegStatusAttended:
			// Повторный скан — не ошибка, просто отметим.
			ev, _ := s.events.Get(ctx, tx, reg.EventID)
			result = &CheckInResult{Registration: reg, Event: ev, AlreadyDone: true}
			return nil
		case domain.RegStatusRegistered:
			// ok, продолжаем
		default:
			// cancelled / no_show / waitlist — нельзя.
			return fmt.Errorf("%w: status=%s", ErrNotRegistered, reg.Status)
		}

		// Окно check-in.
		ev, err := s.events.Get(ctx, tx, reg.EventID)
		if err != nil {
			return fmt.Errorf("get event: %w", err)
		}
		if ev == nil {
			return ErrEventNotFound
		}
		now := time.Now()
		windowStart := ev.StartsAt.Add(-checkinPreWindow)
		windowEnd := ev.StartsAt.Add(checkinPostWindow)
		if ev.EndsAt != nil {
			windowEnd = ev.EndsAt.Add(checkinPostWindow)
		}
		if now.Before(windowStart) || now.After(windowEnd) {
			return ErrCheckinWindowClosed
		}

		// Кто сканировал — нужен local user_id.
		organizer, err := s.users.GetByMaxID(ctx, tx, organizerMaxUserID)
		if err != nil {
			return fmt.Errorf("lookup organizer: %w", err)
		}
		if organizer == nil {
			return ErrNotOrganizer
		}

		if err := s.regs.MarkAttended(ctx, tx, reg.ID, organizer.ID); err != nil {
			return fmt.Errorf("mark attended: %w", err)
		}

		actorID := organizer.ID
		target := reg.UserID
		evID := ev.ID
		regID := reg.ID
		_ = s.logs.Append(ctx, tx, &domain.ActionLog{
			ActorUserID:    &actorID,
			TargetUserID:   &target,
			EventID:        &evID,
			RegistrationID: &regID,
			Action:         domain.ActionCheckinScanned,
		})

		reg.Status = domain.RegStatusAttended
		nowVal := time.Now()
		reg.CheckinAt = &nowVal
		reg.CheckinBy = &organizer.ID
		result = &CheckInResult{Registration: reg, Event: ev, AlreadyDone: false}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

func (s *attendanceService) inTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

// Защита от unused import.
var _ = errors.New
