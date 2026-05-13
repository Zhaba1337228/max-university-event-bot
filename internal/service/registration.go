package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/ptr"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// RegisterInput — параметры регистрации.
type RegisterInput struct {
	UserID          int64 // локальный id из users
	EventID         int64
	FullName        string
	Contact         string
	InterestProgram string
}

// RegisterResult — что произошло в результате Register.
type RegisterResult struct {
	RegistrationID int64
	IsWaitlist     bool
	Position       int // позиция в очереди (только для IsWaitlist=true)
}

// Registration — сервис регистрации на мероприятия.
type Registration interface {
	// Register выполняет полный сценарий:
	//   1. проверка HasConsent       → ErrConsentRequired
	//   2. проверка статуса события  → ErrEventClosed / ErrEventNotFound
	//   3. проверка дубля            → ErrAlreadyRegistered
	//   4. транзакция (FOR UPDATE):
	//        4.1 count = CountByEvent(registered)
	//        4.2 если есть места     → Create(status=registered) + ActionLog
	//        4.3 если включён waitlist → Create(status=waitlist, position) + ActionLog
	//        4.4 иначе               → ErrNoSeats
	Register(ctx context.Context, in RegisterInput) (*RegisterResult, error)
}

type registrationService struct {
	pool            *pgxpool.Pool
	events          repo.EventRepo
	regs            repo.RegistrationRepo
	users           repo.UserRepo
	logs            repo.ActionLogRepo
	waitlistEnabled bool
}

// NewRegistration создаёт сервис.
// Принимает именно *pgxpool.Pool (а не Querier), потому что внутри Register
// открывается транзакция; pgxmock не подойдёт — сервисы тестируются
// через struct mocks.
func NewRegistration(
	pool *pgxpool.Pool,
	events repo.EventRepo,
	regs repo.RegistrationRepo,
	users repo.UserRepo,
	logs repo.ActionLogRepo,
	waitlistEnabled bool,
) Registration {
	return &registrationService{
		pool:            pool,
		events:          events,
		regs:            regs,
		users:           users,
		logs:            logs,
		waitlistEnabled: waitlistEnabled,
	}
}

func (s *registrationService) Register(ctx context.Context, in RegisterInput) (*RegisterResult, error) {
	if in.UserID <= 0 || in.EventID <= 0 {
		return nil, errors.New("registration: empty user_id or event_id")
	}

	// Шаг 1: проверка согласия.
	user, err := s.users.GetByID(ctx, s.pool, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("registration: user %d not found", in.UserID)
	}
	if !user.HasConsent() {
		return nil, ErrConsentRequired
	}

	// Шаг 2: предварительная проверка статуса события без блокировки.
	// Финальная проверка — внутри транзакции с FOR UPDATE.
	event, err := s.events.Get(ctx, s.pool, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("load event: %w", err)
	}
	if event == nil {
		return nil, ErrEventNotFound
	}
	if !event.IsOpenForRegistration() {
		return nil, ErrEventClosed
	}

	// Шаг 3: дубль (без транзакции — для быстрого UX, в транзакции UNIQUE-индекс
	// нам страхует от гонки).
	existing, err := s.regs.GetActiveByUserEvent(ctx, s.pool, in.UserID, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		return nil, ErrAlreadyRegistered
	}

	// Шаг 4: транзакция с SELECT FOR UPDATE на event row.
	var result *RegisterResult
	err = s.inTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// 4.1: блокируем строку события.
		ev, err := s.events.GetForUpdate(ctx, tx, in.EventID)
		if err != nil {
			return fmt.Errorf("for update event: %w", err)
		}
		if ev == nil {
			return ErrEventNotFound
		}
		if !ev.IsOpenForRegistration() {
			return ErrEventClosed
		}

		// 4.2: считаем зарегистрированных + посетивших.
		regCount, err := s.regs.CountByEvent(ctx, tx, ev.ID, domain.RegStatusRegistered)
		if err != nil {
			return fmt.Errorf("count registered: %w", err)
		}
		attCount, err := s.regs.CountByEvent(ctx, tx, ev.ID, domain.RegStatusAttended)
		if err != nil {
			return fmt.Errorf("count attended: %w", err)
		}
		occupied := regCount + attCount

		// 4.3: если есть места — создаём registered.
		if occupied < ev.Capacity {
			reg := &domain.Registration{
				UserID:           in.UserID,
				EventID:          in.EventID,
				Status:           domain.RegStatusRegistered,
				FullNameSnapshot: in.FullName,
				ContactSnapshot:  in.Contact,
				InterestProgram:  optionalStr(in.InterestProgram),
				Source:           "bot",
			}
			id, err := s.regs.Create(ctx, tx, reg)
			if err != nil {
				return fmt.Errorf("create registration: %w", err)
			}
			s.logAction(ctx, tx, domain.ActionRegistrationCreated, in.UserID, in.EventID, id, nil)
			result = &RegisterResult{RegistrationID: id, IsWaitlist: false}
			return nil
		}

		// 4.4: мест нет — пробуем waitlist.
		if !s.waitlistEnabled {
			return ErrNoSeats
		}

		pos, err := s.regs.NextWaitlistPosition(ctx, tx, in.EventID)
		if err != nil {
			return fmt.Errorf("next waitlist position: %w", err)
		}
		reg := &domain.Registration{
			UserID:           in.UserID,
			EventID:          in.EventID,
			Status:           domain.RegStatusWaitlist,
			FullNameSnapshot: in.FullName,
			ContactSnapshot:  in.Contact,
			InterestProgram:  optionalStr(in.InterestProgram),
			WaitlistPosition: ptr.To(pos),
			Source:           "bot",
		}
		id, err := s.regs.Create(ctx, tx, reg)
		if err != nil {
			return fmt.Errorf("create waitlist: %w", err)
		}
		s.logAction(ctx, tx, domain.ActionWaitlistAdded, in.UserID, in.EventID, id,
			map[string]any{"position": pos})
		result = &RegisterResult{RegistrationID: id, IsWaitlist: true, Position: pos}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// inTx — открывает транзакцию RepeatableRead для гарантии того, что
// SELECT FOR UPDATE на event row блокирует параллельные регистрации.
func (s *registrationService) inTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
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

func (s *registrationService) logAction(ctx context.Context, q repo.Querier, action domain.ActionType,
	userID, eventID, regID int64, payload map[string]any,
) {
	var raw json.RawMessage
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			raw = b
		}
	}
	if err := s.logs.Append(ctx, q, &domain.ActionLog{
		ActorUserID:    &userID,
		EventID:        &eventID,
		RegistrationID: &regID,
		Action:         action,
		Payload:        raw,
	}); err != nil {
		// ActionLog не блокирует основную операцию — она уже совершена,
		// но логируем «не получилось залогировать», чтобы видеть в проде.
		// Поскольку у сервиса нет logger'а на текущем этапе, проглатываем тихо.
		_ = err
	}
}

// optionalStr возвращает указатель на строку или nil для пустой.
func optionalStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
