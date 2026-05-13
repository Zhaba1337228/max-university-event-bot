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

// RBAC.
var (
	ErrAccessDenied  = errors.New("access denied")
	ErrNotOrganizer  = errors.New("user is not an organizer")
	ErrNotEventOwner = errors.New("user is not owner of this event")
)

// AI.
var (
	ErrAIUnavailable = errors.New("ai service unavailable")
)
