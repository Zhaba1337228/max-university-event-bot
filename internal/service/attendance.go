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
//  3. проверка прав: сканирующий должен быть staff/admin (organizer-овнер события не сканирует QR-коды);
//  4. проверка статуса регистрации (active);
//  5. проверка окна check-in [starts_at - 2h, ends_at + 4h];
//  6. MarkAttended; ActionLog.checkin_scanned.
type Attendance interface {
	// CheckIn — отметка участника пришёдшим. scannerMaxUserID — max_user_id того,
	// кто сканирует; он обязан иметь роль staff или admin (organizer получит ErrNotStaff).
	CheckIn(ctx context.Context, scannerMaxUserID int64, qrPayload string) (*CheckInResult, error)
}

// CheckInResult — что произошло.
//
// Поля Registration/Event/Participant заполняются ВСЕГДА, когда регистрация
// нашлась — в т.ч. для ошибок ErrRegistrationCancelled, ErrRegistrationOnWaitlist,
// ErrRegistrationNoShow, ErrCheckinWindowClosed, ErrAlreadyCheckedIn. Это
// нужно, чтобы admin web мог показать волонтёру **подробную карточку** даже
// в случае «нельзя отметить» — с информацией о мероприятии и участнике.
// Scanner заполняется только при успешном check-in (и AlreadyDone).
type CheckInResult struct {
	Registration *domain.Registration
	Event        *domain.Event
	Participant  *domain.User // владелец регистрации (для отображения телефона/email)
	Scanner      *domain.User // кто сканировал (заполнен на success/AlreadyDone)
	AlreadyDone  bool         // если повторный скан того же QR — true, status уже attended
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

func (s *attendanceService) CheckIn(ctx context.Context, scannerMaxUserID int64, payload string) (*CheckInResult, error) {
	parsed, err := s.qr.ParseQRPayload(payload)
	if err != nil {
		return nil, err
	}

	// Права на check-in: только staff/admin (organizer сканерит не должен).
	if _, err := s.role.RequireStaff(ctx, scannerMaxUserID); err != nil {
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

		// Регистрация нашлась — подгружаем event + participant для
		// подробной карточки (нужна и в success-, и в error-ответах).
		ev, err := s.events.Get(ctx, tx, reg.EventID)
		if err != nil {
			return fmt.Errorf("get event: %w", err)
		}
		if ev == nil {
			return ErrEventNotFound
		}
		participant, err := s.users.GetByID(ctx, tx, reg.UserID)
		if err != nil {
			return fmt.Errorf("get participant: %w", err)
		}

		// Состояние записи.
		switch reg.Status {
		case domain.RegStatusAttended:
			// Повторный скан — не ошибка, просто отметим.
			// Подтягиваем того, кто отметил первоначально, чтобы карточка
			// на already_done показала строку «Отметил: …».
			var origScanner *domain.User
			if reg.CheckinBy != nil {
				origScanner, err = s.users.GetByID(ctx, tx, *reg.CheckinBy)
				if err != nil {
					return fmt.Errorf("get original scanner: %w", err)
				}
			}
			result = &CheckInResult{Registration: reg, Event: ev, Participant: participant, Scanner: origScanner, AlreadyDone: true}
			return nil
		case domain.RegStatusRegistered:
			// ok, продолжаем
		case domain.RegStatusCancelledByUser, domain.RegStatusCancelledByOrganizer:
			result = &CheckInResult{Registration: reg, Event: ev, Participant: participant}
			return ErrRegistrationCancelled
		case domain.RegStatusWaitlist:
			result = &CheckInResult{Registration: reg, Event: ev, Participant: participant}
			return ErrRegistrationOnWaitlist
		case domain.RegStatusNoShow:
			result = &CheckInResult{Registration: reg, Event: ev, Participant: participant}
			return ErrRegistrationNoShow
		default:
			return fmt.Errorf("%w: status=%s", ErrNotRegistered, reg.Status)
		}

		// Окно check-in. Если закрыто — отдаём подробную карточку для UI.
		now := time.Now()
		windowStart := ev.StartsAt.Add(-checkinPreWindow)
		windowEnd := ev.StartsAt.Add(checkinPostWindow)
		if ev.EndsAt != nil {
			windowEnd = ev.EndsAt.Add(checkinPostWindow)
		}
		if now.Before(windowStart) || now.After(windowEnd) {
			result = &CheckInResult{Registration: reg, Event: ev, Participant: participant}
			return ErrCheckinWindowClosed
		}

		// Кто сканировал — нужен local user_id.
		scanner, err := s.users.GetByMaxID(ctx, tx, scannerMaxUserID)
		if err != nil {
			return fmt.Errorf("lookup scanner: %w", err)
		}
		if scanner == nil {
			return ErrNotStaff
		}

		if err := s.regs.MarkAttended(ctx, tx, reg.ID, scanner.ID); err != nil {
			return fmt.Errorf("mark attended: %w", err)
		}

		actorID := scanner.ID
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
		reg.CheckinBy = &scanner.ID
		result = &CheckInResult{Registration: reg, Event: ev, Participant: participant, Scanner: scanner, AlreadyDone: false}
		return nil
	})
	if txErr != nil {
		// Если результат уже заполнен (статус-ветки/окно) — отдаём его + ошибку.
		// Handler решит, что показать.
		if result != nil {
			return result, txErr
		}
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
