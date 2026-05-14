package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Role — сервис проверок ролей пользователей.
//
// На MVP роль присваивается через env-bootstrap (ORGANIZER_USER_IDS,
// ADMIN_USER_IDS). На дне 10 dynamic-promote делает admin через бот
// или веб-админку (день 13).
type Role interface {
	// Bootstrap проставляет роли organizer/staff/admin пользователям, чьи
	// max_user_id указаны в config.Business.{Organizer|Staff|Admin}UserIDs.
	// Безопасно вызывать многократно — операция идемпотентна (EnsureByMaxID).
	//
	// Иерархия: admin > organizer / staff (равны по приоритету) > applicant.
	// Admin не «понижается» до organizer/staff. Organizer и staff НЕ пересекаются:
	// если user попал в оба списка, побеждает порядок admin > organizer > staff.
	Bootstrap(ctx context.Context, organizerMaxIDs, staffMaxIDs, adminMaxIDs []int64) error

	// RequireOrganizer — false если у пользователя роль applicant/staff
	// (admin тоже считается organizer'ом).
	RequireOrganizer(ctx context.Context, maxUserID int64) (*domain.User, error)

	// RequireStaff — доступ только для staff/admin (organizer/applicant получает ErrNotStaff).
	RequireStaff(ctx context.Context, maxUserID int64) (*domain.User, error)

	// RequireEventOwner — пользователь должен быть admin или создателем события.
	// Это первая строка каждого организаторского handler'а (см. план §19.4).
	RequireEventOwner(ctx context.Context, maxUserID, eventID int64) (*domain.User, error)
}

type roleService struct {
	pool   repo.Querier
	users  repo.UserRepo
	events repo.EventRepo
	log    *slog.Logger
}

// NewRole создаёт сервис.
func NewRole(pool repo.Querier, users repo.UserRepo, events repo.EventRepo, log *slog.Logger) Role {
	return &roleService{
		pool:   pool,
		users:  users,
		events: events,
		log:    log.With("service", "role"),
	}
}

func (s *roleService) Bootstrap(ctx context.Context, organizerMaxIDs, staffMaxIDs, adminMaxIDs []int64) error {
	// Admin'ам — admin (он включает organizer + staff привилегии).
	for _, mid := range adminMaxIDs {
		if mid == 0 {
			continue
		}
		u, err := s.users.EnsureByMaxID(ctx, s.pool, mid)
		if err != nil {
			return fmt.Errorf("ensure admin %d: %w", mid, err)
		}
		if u.Role == domain.RoleAdmin {
			continue
		}
		if err := s.users.SetRole(ctx, s.pool, u.ID, domain.RoleAdmin); err != nil {
			return fmt.Errorf("set admin %d: %w", mid, err)
		}
		s.log.Info("role bootstrapped", "max_user_id", mid, "role", domain.RoleAdmin)
	}

	// Organizer'ам — organizer.
	for _, mid := range organizerMaxIDs {
		if mid == 0 {
			continue
		}
		u, err := s.users.EnsureByMaxID(ctx, s.pool, mid)
		if err != nil {
			return fmt.Errorf("ensure organizer %d: %w", mid, err)
		}
		// Не «понижаем» admin до organizer'а.
		if u.Role == domain.RoleAdmin || u.Role == domain.RoleOrganizer {
			continue
		}
		if err := s.users.SetRole(ctx, s.pool, u.ID, domain.RoleOrganizer); err != nil {
			return fmt.Errorf("set organizer %d: %w", mid, err)
		}
		s.log.Info("role bootstrapped", "max_user_id", mid, "role", domain.RoleOrganizer)
	}

	// Staff'у — staff. Не перебиваем admin/organizer (они «выше» по привилегиям).
	for _, mid := range staffMaxIDs {
		if mid == 0 {
			continue
		}
		u, err := s.users.EnsureByMaxID(ctx, s.pool, mid)
		if err != nil {
			return fmt.Errorf("ensure staff %d: %w", mid, err)
		}
		if u.Role == domain.RoleAdmin || u.Role == domain.RoleOrganizer || u.Role == domain.RoleStaff {
			continue
		}
		if err := s.users.SetRole(ctx, s.pool, u.ID, domain.RoleStaff); err != nil {
			return fmt.Errorf("set staff %d: %w", mid, err)
		}
		s.log.Info("role bootstrapped", "max_user_id", mid, "role", domain.RoleStaff)
	}
	return nil
}

func (s *roleService) RequireStaff(ctx context.Context, maxUserID int64) (*domain.User, error) {
	u, err := s.users.GetByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if u == nil || !u.IsStaff() {
		return nil, ErrNotStaff
	}
	return u, nil
}

func (s *roleService) RequireOrganizer(ctx context.Context, maxUserID int64) (*domain.User, error) {
	u, err := s.users.GetByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if u == nil || !u.IsOrganizer() {
		return nil, ErrNotOrganizer
	}
	return u, nil
}

func (s *roleService) RequireEventOwner(ctx context.Context, maxUserID, eventID int64) (*domain.User, error) {
	u, err := s.RequireOrganizer(ctx, maxUserID)
	if err != nil {
		return nil, err
	}
	// Admin может всё.
	if u.IsAdmin() {
		return u, nil
	}
	ev, err := s.events.Get(ctx, s.pool, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if ev == nil {
		return nil, ErrEventNotFound
	}
	if ev.CreatedBy == nil || *ev.CreatedBy != u.ID {
		return nil, ErrNotEventOwner
	}
	return u, nil
}
