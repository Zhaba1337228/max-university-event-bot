// Package domain содержит чистые доменные типы.
// Здесь нет зависимостей на БД, HTTP, SDK или внешние клиенты.
package domain

import "time"

// Role — роль пользователя в системе.
type Role string

// Возможные роли. Применяется RBAC.
//
//   - applicant — обычный посетитель/абитуриент, только бот
//   - organizer — может создавать и вести события (web-админка), но НЕ сканирует QR
//   - staff — волонтёр на входе: имеет доступ только к /checkin (сканеру), не видит организаторских разделов
//   - admin — все права: organizer + staff + управление пользователями
const (
	RoleApplicant Role = "applicant"
	RoleOrganizer Role = "organizer"
	RoleStaff     Role = "staff"
	RoleAdmin     Role = "admin"
)

// User — модель пользователя.
//
// MaxUserID — внешний id из MAX-мессенджера; локальный ID — суррогатный BIGSERIAL.
// ConsentAt = nil означает, что пользователь не давал согласие на обработку ПДн (152-ФЗ),
// поэтому запись на мероприятие должна быть запрещена до получения согласия.
type User struct {
	ID               int64
	MaxUserID        int64
	FullName         *string
	Phone            *string
	Email            *string
	Role             Role
	Locale           string
	ConsentAt        *time.Time
	ConsentPolicyVer *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// HasConsent сообщает, дал ли пользователь согласие на обработку ПДн.
func (u *User) HasConsent() bool {
	return u != nil && u.ConsentAt != nil
}

// IsOrganizer сообщает, может ли пользователь работать с организаторской панелью.
// Admin всегда имеет доступ. Staff НЕ имеет.
func (u *User) IsOrganizer() bool {
	return u != nil && (u.Role == RoleOrganizer || u.Role == RoleAdmin)
}

// IsStaff сообщает, может ли пользователь делать check-in (сканировать QR).
// Admin всегда имеет доступ. Organizer — нет: он не сканирует QR-коды гостей.
func (u *User) IsStaff() bool {
	return u != nil && (u.Role == RoleStaff || u.Role == RoleAdmin)
}

// IsAdmin сообщает, является ли пользователь администратором.
func (u *User) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}

// CanAccessAdminPanel — может ли пользователь войти в веб-админку
// (любая роль кроме applicant).
func (u *User) CanAccessAdminPanel() bool {
	return u != nil && (u.IsOrganizer() || u.IsStaff())
}
