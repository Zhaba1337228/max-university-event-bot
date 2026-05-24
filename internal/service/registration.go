package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

// CancelBy — кто инициирует отмену.
type CancelBy string

const (
	CancelByUser      CancelBy = "user"
	CancelByOrganizer CancelBy = "organizer"
)

// PromoteResult — что произошло при попытке продвинуть очередь.
type PromoteResult struct {
	PromotedRegistrationID int64 // 0 если очередь пуста / нет места
	PromotedUserID         int64
	Promoted               bool
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

	// JoinWaitlist напрямую добавляет в лист ожидания, минуя проверку capacity.
	// Используется когда карточка события показала «мест нет, встать в очередь».
	// Сам Register сделает это автоматически если capacity исчерпана и
	// waitlist включён; но если пользователь дошёл до карточки до consent —
	// он сначала пройдёт consent, потом сразу попадёт в waitlist.
	//
	// Возвращает ErrWaitlistDisabled, если бизнес-флаг отключён.
	JoinWaitlist(ctx context.Context, in RegisterInput) (*RegisterResult, error)

	// Cancel — отмена активной (registered/waitlist) записи.
	// by определяет статус: cancelled_by_user | cancelled_by_organizer.
	// Возвращает ErrNotRegistered если активной записи нет.
	// После успешной отмены registered-записи пытается продвинуть waitlist:
	// promote-результат — отдельный возврат (может быть Promoted=false).
	Cancel(ctx context.Context, regID int64, by CancelBy) (*PromoteResult, error)

	// ListActiveByUser возвращает активные записи пользователя (для «Моя запись»).
	ListActiveByUser(ctx context.Context, userID int64) ([]*domain.Registration, error)

	// GetActive возвращает активную запись пары (user, event) или nil.
	GetActive(ctx context.Context, userID, eventID int64) (*domain.Registration, error)
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

		// 4.1.1: повторно проверяем дубль уже внутри сериализованной секции.
		// Это закрывает быстрые/параллельные повторные подтверждения с одного аккаунта:
		// первый запрос создаёт запись, второй после ожидания event-lock увидит её здесь
		// и вернёт ErrAlreadyRegistered вместо "успешного" upsert той же строки.
		existing, err := s.regs.GetActiveByUserEvent(ctx, tx, in.UserID, in.EventID)
		if err != nil {
			return fmt.Errorf("check duplicate in tx: %w", err)
		}
		if existing != nil {
			return ErrAlreadyRegistered
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

// JoinWaitlist — пользователь нажал «Встать в лист ожидания».
// Делает то же что и Register, но если есть свободные места — всё равно
// создаёт registered (пользователю это лучше, чем стоять в очереди впустую).
func (s *registrationService) JoinWaitlist(ctx context.Context, in RegisterInput) (*RegisterResult, error) {
	if !s.waitlistEnabled {
		return nil, ErrWaitlistDisabled
	}
	return s.Register(ctx, in)
}

// Cancel — отмена активной записи. by определяет статус.
//
// Если отменяемая запись была registered и места освободились — пытаемся
// продвинуть очередь: переводим первого waitlist-пользователя в registered
// (т.е. промоут «без подтверждения»). Это упрощение MVP — план §23 День 8
// допускает оба варианта (auto или с подтверждением). Здесь auto.
func (s *registrationService) Cancel(ctx context.Context, regID int64, by CancelBy) (*PromoteResult, error) {
	var promoted *PromoteResult

	err := s.inTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		reg, err := s.regs.Get(ctx, tx, regID)
		if err != nil {
			return fmt.Errorf("load registration: %w", err)
		}
		if reg == nil || !reg.Status.IsActive() {
			return ErrNotRegistered
		}

		// Блокируем строку события на запись (чтобы capacity-счётчик не съехал).
		ev, err := s.events.GetForUpdate(ctx, tx, reg.EventID)
		if err != nil {
			return fmt.Errorf("for update event: %w", err)
		}
		if ev == nil {
			return ErrEventNotFound
		}

		// Меняем статус на cancelled_by_user / cancelled_by_organizer.
		newStatus := domain.RegStatusCancelledByUser
		actionType := domain.ActionRegistrationCancelledUser
		if by == CancelByOrganizer {
			newStatus = domain.RegStatusCancelledByOrganizer
			actionType = domain.ActionRegistrationCancelledOrg
		}

		// TZ §5: если мероприятие уже началось, проверяем политику поздней отмены.
		if by == CancelByUser && ev.StartsAt.Before(time.Now()) {
			if !ev.LateCancelAllowed {
				return ErrLateCancelForbidden
			}
			// Поздняя отмена разрешена — фиксируем особым статусом.
			newStatus = domain.RegStatusLateCancel
		}

		if err := s.regs.UpdateStatus(ctx, tx, reg.ID, newStatus); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		s.logAction(ctx, tx, actionType, reg.UserID, reg.EventID, reg.ID, nil)

		// Если отмена waitlist-записи — продвигать ничего не нужно (мест не освободилось).
		if reg.Status != domain.RegStatusRegistered {
			return nil
		}
		if !s.waitlistEnabled {
			return nil
		}

		// Пробуем продвинуть первого waitlist-пользователя в registered.
		next, err := s.regs.NextWaitlist(ctx, tx, reg.EventID)
		if err != nil {
			return fmt.Errorf("next waitlist: %w", err)
		}
		if next == nil {
			return nil
		}

		// Проверим что место реально освободилось (defensive — конкурирующая
		// регистрация могла занять его до нас).
		registered, err := s.regs.CountByEvent(ctx, tx, ev.ID, domain.RegStatusRegistered)
		if err != nil {
			return fmt.Errorf("count registered: %w", err)
		}
		attended, err := s.regs.CountByEvent(ctx, tx, ev.ID, domain.RegStatusAttended)
		if err != nil {
			return fmt.Errorf("count attended: %w", err)
		}
		if registered+attended >= ev.Capacity {
			return nil
		}

		if err := s.regs.UpdateStatus(ctx, tx, next.ID, domain.RegStatusRegistered); err != nil {
			return fmt.Errorf("promote update: %w", err)
		}
		// Снимаем waitlist_position, чтобы счётчик очереди не считал его.
		// repo.AssignWaitlistPosition с nil не поддерживается — пишем SQL напрямую.
		if _, err := tx.Exec(ctx,
			`UPDATE registrations SET waitlist_position = NULL,
                registered_at = COALESCE(registered_at, NOW()),
                updated_at = NOW() WHERE id = $1`, next.ID); err != nil {
			return fmt.Errorf("clear waitlist position: %w", err)
		}

		s.logAction(ctx, tx, domain.ActionWaitlistPromoted, next.UserID, ev.ID, next.ID, nil)
		promoted = &PromoteResult{
			PromotedRegistrationID: next.ID,
			PromotedUserID:         next.UserID,
			Promoted:               true,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if promoted == nil {
		return &PromoteResult{Promoted: false}, nil
	}
	return promoted, nil
}

// ListActiveByUser — список активных записей пользователя (registered+waitlist).
func (s *registrationService) ListActiveByUser(ctx context.Context, userID int64) ([]*domain.Registration, error) {
	out, err := s.regs.ListByUser(ctx, s.pool, userID, true)
	if err != nil {
		return nil, fmt.Errorf("list active by user: %w", err)
	}
	return out, nil
}

// GetActive — точечный поиск активной записи пары.
func (s *registrationService) GetActive(ctx context.Context, userID, eventID int64) (*domain.Registration, error) {
	r, err := s.regs.GetActiveByUserEvent(ctx, s.pool, userID, eventID)
	if err != nil {
		return nil, fmt.Errorf("get active: %w", err)
	}
	return r, nil
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
