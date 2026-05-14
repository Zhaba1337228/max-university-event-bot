// Package service содержит бизнес-логику приложения.
//
// Сервисы не знают про MAX SDK и HTTP. Они принимают/возвращают доменные
// типы (internal/domain) и зависят от интерфейсов репозиториев (internal/repo).
//
// Все доменные ошибки централизованы в errors.go и должны проверяться
// в handler'ах через errors.Is. Это позволяет:
//   - переводить технические ошибки в понятные пользователю сообщения;
//   - не давать сервису решать «как показать ошибку» (это работа UI-слоя).
package service

import "errors"

// Регистрация / общая бизнес-логика.
var (
	ErrEventNotFound       = errors.New("event not found")
	ErrEventClosed         = errors.New("event is not open for registration")
	ErrAlreadyRegistered   = errors.New("user already registered")
	ErrNoSeats             = errors.New("no seats available")
	ErrWaitlistDisabled    = errors.New("waitlist is disabled for this event")
	ErrConsentRequired     = errors.New("user consent required (152-FZ)")
	ErrNotRegistered       = errors.New("user has no active registration")
	ErrCheckinWindowClosed = errors.New("check-in window closed")
	ErrAlreadyCheckedIn    = errors.New("already checked in")
)

// Профили / валидация ввода.
var (
	ErrInvalidFullName = errors.New("invalid full name")
	ErrInvalidContact  = errors.New("invalid contact (phone or email)")
)

// Event admin operations.
var (
	ErrEventInvalidTitle       = errors.New("event title is empty or too long")
	ErrEventInvalidDescription = errors.New("event description is too long")
	ErrEventInvalidDates       = errors.New("event dates are invalid (starts_at in past or ends_at <= starts_at)")
	ErrEventInvalidCapacity    = errors.New("event capacity must be > 0")
	ErrEventInvalidFormat      = errors.New("event format must be offline, online or hybrid")
	ErrEventInvalidStatus      = errors.New("event status must be open or closed")
	ErrEventTooManyTags        = errors.New("event tags: max 20 strings, each <= 50 chars")
)

// RBAC.
var (
	ErrAccessDenied  = errors.New("access denied")
	ErrNotOrganizer  = errors.New("user is not an organizer")
	ErrNotStaff      = errors.New("user is not staff/admin")
	ErrNotEventOwner = errors.New("user is not owner of this event")
)

// User admin operations.
var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUserInvalidRole      = errors.New("invalid role (allowed: applicant, organizer, staff, admin)")
	ErrUserCannotChangeSelf = errors.New("admin cannot change own role")
)

// Manual attendance marking (web admin).
var (
	ErrManualMarkInvalidStatus = errors.New("manual mark status must be attended or no_show")
	ErrRegistrationNotFound    = errors.New("registration not found")
	ErrRegNotForEvent          = errors.New("registration does not belong to this event")
	ErrRegNotActive            = errors.New("registration is cancelled and cannot be marked")
)

// AI.
var (
	ErrAIUnavailable = errors.New("ai service unavailable")
)
