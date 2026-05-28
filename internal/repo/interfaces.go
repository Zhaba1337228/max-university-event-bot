package repo

import (
	"context"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// UserRepo — операции над пользователями.
type UserRepo interface {
	// EnsureByMaxID возвращает существующего пользователя по max_user_id
	// или создаёт нового с ролью applicant.
	EnsureByMaxID(ctx context.Context, q Querier, maxUserID int64) (*domain.User, error)

	// GetByID — точечный поиск по локальному id.
	GetByID(ctx context.Context, q Querier, id int64) (*domain.User, error)

	// GetByMaxID — точечный поиск по внешнему max_user_id.
	// Возвращает (nil, nil) если пользователя нет.
	GetByMaxID(ctx context.Context, q Querier, maxUserID int64) (*domain.User, error)

	// UpdateProfile обновляет ФИО и контакт (телефон/email распознаются автоматически).
	UpdateProfile(ctx context.Context, q Querier, id int64, fullName, contact *string) error

	// SetRole меняет роль пользователя.
	SetRole(ctx context.Context, q Querier, id int64, role domain.Role) error

	// GrantConsent фиксирует согласие на обработку ПДн с версией политики.
	GrantConsent(ctx context.Context, q Querier, id int64, policyVer string) error

	// ForgetMe удаляет пользователя со всеми каскадными зависимостями
	// (registrations, action_logs, user_states, notifications).
	ForgetMe(ctx context.Context, q Querier, id int64) error

	// List возвращает страницу пользователей с фильтром по роли и подстрокой.
	// roleFilter == "" — без фильтра по роли. query сравнивается с full_name/phone/email
	// case-insensitive (ILIKE). Возвращает users + total для пагинации.
	List(ctx context.Context, q Querier, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error)
}

// EventRepo — операции над мероприятиями.
type EventRepo interface {
	Create(ctx context.Context, q Querier, e *domain.Event) (int64, error)
	Get(ctx context.Context, q Querier, id int64) (*domain.Event, error)
	GetForUpdate(ctx context.Context, q Querier, id int64) (*domain.Event, error)
	ListOpen(ctx context.Context, q Querier, limit, offset int) ([]*domain.Event, int, error)
	ListByOrganizer(ctx context.Context, q Querier, organizerID int64) ([]*domain.Event, error)
	ListUpcomingForReminders(ctx context.Context, q Querier, within time.Duration) ([]*domain.Event, error)
	UpdateStatus(ctx context.Context, q Querier, id int64, st domain.EventStatus) error
	Update(ctx context.Context, q Querier, e *domain.Event) error
	UpdateShortSummary(ctx context.Context, q Querier, id int64, summary string) error
	Delete(ctx context.Context, q Querier, id int64) error
	Stats(ctx context.Context, q Querier, eventID int64) (*domain.EventStats, error)
}

// RegistrationRepo — операции над записями.
//
// Get/GetActiveByUserEvent — точечный поиск; NextWaitlist — для promote.
// CountByEvent — для проверки capacity (вызывается в транзакции с FOR UPDATE на event).
type RegistrationRepo interface {
	Get(ctx context.Context, q Querier, id int64) (*domain.Registration, error)
	GetByCode(ctx context.Context, q Querier, code string) (*domain.Registration, error)
	GetByCodeForUpdate(ctx context.Context, q Querier, code string) (*domain.Registration, error)
	GetActiveByUserEvent(ctx context.Context, q Querier, userID, eventID int64) (*domain.Registration, error)
	Create(ctx context.Context, q Querier, r *domain.Registration) (int64, error)
	UpdateStatus(ctx context.Context, q Querier, id int64, status domain.RegistrationStatus) error
	SetAttendanceCode(ctx context.Context, q Querier, id int64, code string) error
	SetQRMessageID(ctx context.Context, q Querier, id int64, messageID int64) error
	MarkAttended(ctx context.Context, q Querier, id int64, byUserID int64) error
	MarkNoShow(ctx context.Context, q Querier, id int64, byUserID int64) error
	ListByEvent(ctx context.Context, q Querier, eventID int64, status domain.RegistrationStatus, limit, offset int) ([]*domain.Registration, error)
	ListByEventAllStatuses(ctx context.Context, q Querier, eventID int64, limit, offset int) ([]*domain.Registration, error)
	ListByUser(ctx context.Context, q Querier, userID int64, activeOnly bool) ([]*domain.Registration, error)
	CountByEvent(ctx context.Context, q Querier, eventID int64, status domain.RegistrationStatus) (int, error)
	NextWaitlist(ctx context.Context, q Querier, eventID int64) (*domain.Registration, error)
	NextWaitlistPosition(ctx context.Context, q Querier, eventID int64) (int, error)
	AssignWaitlistPosition(ctx context.Context, q Querier, registrationID int64, pos int) error
	SetNotificationsDisabled(ctx context.Context, q Querier, id int64, disabled bool) error
}

// ActionLogRepo — операции над audit log.
type ActionLogRepo interface {
	Append(ctx context.Context, q Querier, log *domain.ActionLog) error
	ListByUser(ctx context.Context, q Querier, userID int64, limit int) ([]*domain.ActionLog, error)
	ListByEvent(ctx context.Context, q Querier, eventID int64, limit int) ([]*domain.ActionLog, error)
}

// NotificationRepo — операции над уведомлениями.
//
// PickDue/MarkSent/MarkFailed используются scheduler'ом.
// Schedule — точечная вставка; если такая уже есть (notif_dedup) —
// репозиторий возвращает (0, nil), пропуская без ошибки.
type NotificationRepo interface {
	Schedule(ctx context.Context, q Querier, n *domain.Notification) (int64, error)
	PickDue(ctx context.Context, q Querier, now time.Time, limit int) ([]*domain.Notification, error)
	MarkSent(ctx context.Context, q Querier, id int64, at time.Time) error
	MarkFailed(ctx context.Context, q Querier, id int64, errMsg string) error
	MarkSkipped(ctx context.Context, q Querier, id int64, reason string) error
}

// UserStateRepo — FSM persistence.
type UserStateRepo interface {
	Load(ctx context.Context, q Querier, userID int64) (state string, contextJSON []byte, err error)
	Save(ctx context.Context, q Querier, userID int64, state string, contextJSON []byte) error
	Reset(ctx context.Context, q Querier, userID int64) error
	PurgeStaleBefore(ctx context.Context, q Querier, before time.Time) (int, error)
}
