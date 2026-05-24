package domain

import "time"

// RegistrationStatus — состояние конкретной записи пользователя на мероприятие.
type RegistrationStatus string

// Возможные статусы записи.
const (
	RegStatusRegistered           RegistrationStatus = "registered"
	RegStatusWaitlist             RegistrationStatus = "waitlist"
	RegStatusCancelledByUser      RegistrationStatus = "cancelled_by_user"
	RegStatusCancelledByOrganizer RegistrationStatus = "cancelled_by_organizer"
	RegStatusLateCancel           RegistrationStatus = "late_cancel"
	RegStatusAttended             RegistrationStatus = "attended"
	RegStatusNoShow               RegistrationStatus = "no_show"
)

// Registration — запись пользователя на мероприятие.
//
// FullNameSnapshot / ContactSnapshot — слепок данных на момент записи,
// чтобы организатор видел корректную пару даже если пользователь потом
// поменяет профиль или удалится через /forget_me (в этом случае запись
// удалится каскадно, но снапшот остаётся в action_logs).
//
// AttendanceCode — uuid v4 hex (128 бит), нужен для QR check-in (см. День 15).
// QRSentMessageID — id сообщения с PNG, чтобы кнопка «Показать мой QR» в
// напоминании могла дать ссылку без перегенерации.
type Registration struct {
	ID                    int64
	UserID                int64
	EventID               int64
	Status                RegistrationStatus
	InterestProgram       *string
	FullNameSnapshot      string
	ContactSnapshot       string
	WaitlistPosition      *int
	RegisteredAt          *time.Time
	CancelledAt           *time.Time
	Source                string
	AttendanceCode        *string
	CheckinAt             *time.Time
	CheckinBy             *int64
	QRSentMessageID       *int64
	NotificationsDisabled bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// IsActive сообщает, является ли запись «живой» (registered либо waitlist).
func (s RegistrationStatus) IsActive() bool {
	return s == RegStatusRegistered || s == RegStatusWaitlist
}

// IsCancelled сообщает, отменена ли запись (любой стороной).
func (s RegistrationStatus) IsCancelled() bool {
	return s == RegStatusCancelledByUser || s == RegStatusCancelledByOrganizer || s == RegStatusLateCancel
}
