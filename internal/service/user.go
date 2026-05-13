package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/ptr"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// User — публичный интерфейс сервиса пользователей.
type User interface {
	// EnsureProfile находит пользователя по maxUserID (создавая если нет)
	// и обновляет ФИО/контакт, если переданы непустые значения.
	EnsureProfile(ctx context.Context, maxUserID int64, fullName, contact string) (*domain.User, error)

	// GetByMaxID — точечный поиск без создания. Возвращает (nil, nil) если нет.
	GetByMaxID(ctx context.Context, maxUserID int64) (*domain.User, error)

	// GrantConsent фиксирует согласие на обработку ПДн (152-ФЗ).
	// policyVer — версия документа на момент клика.
	GrantConsent(ctx context.Context, userID int64, policyVer string) error

	// ForgetMe удаляет пользователя со всеми каскадными данными.
	// Возвращает (deleted bool, err) — false если пользователя не было.
	ForgetMe(ctx context.Context, maxUserID int64) (bool, error)
}

type userService struct {
	pool  repo.Querier
	users repo.UserRepo
	logs  repo.ActionLogRepo
}

// NewUser создаёт сервис.
func NewUser(pool repo.Querier, users repo.UserRepo, logs repo.ActionLogRepo) User {
	return &userService{pool: pool, users: users, logs: logs}
}

func (s *userService) EnsureProfile(ctx context.Context, maxUserID int64, fullName, contact string) (*domain.User, error) {
	u, err := s.users.EnsureByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}

	// Обновляем профиль только если переданы новые значения.
	fullName = strings.TrimSpace(fullName)
	contact = strings.TrimSpace(contact)
	if fullName == "" && contact == "" {
		return u, nil
	}

	var namePtr, contactPtr *string
	if fullName != "" {
		namePtr = ptr.To(fullName)
	}
	if contact != "" {
		contactPtr = ptr.To(contact)
	}
	if err := s.users.UpdateProfile(ctx, s.pool, u.ID, namePtr, contactPtr); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	// Перечитываем, чтобы вернуть актуальные поля (consent, role, обновлённые контакты).
	updated, err := s.users.GetByID(ctx, s.pool, u.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user: %w", err)
	}
	if updated != nil {
		return updated, nil
	}
	return u, nil
}

func (s *userService) GetByMaxID(ctx context.Context, maxUserID int64) (*domain.User, error) {
	u, err := s.users.GetByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return nil, fmt.Errorf("get user by max id: %w", err)
	}
	return u, nil
}

func (s *userService) GrantConsent(ctx context.Context, userID int64, policyVer string) error {
	if err := s.users.GrantConsent(ctx, s.pool, userID, policyVer); err != nil {
		return fmt.Errorf("grant consent: %w", err)
	}
	// Не критичный лог: ошибку записи в action_log не пропагируем как падение
	// бизнес-операции (consent уже зафиксирован), просто оборачиваем.
	_ = s.logs.Append(ctx, s.pool, &domain.ActionLog{
		ActorUserID: &userID,
		Action:      domain.ActionConsentGranted,
	})
	return nil
}

func (s *userService) ForgetMe(ctx context.Context, maxUserID int64) (bool, error) {
	u, err := s.users.GetByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return false, fmt.Errorf("lookup user: %w", err)
	}
	if u == nil {
		return false, nil
	}

	// ActionLog ПЕРЕД удалением — после DELETE CASCADE пользователь исчезнет,
	// а нам нужно сохранить факт удаления для отчётности и compliance-аудита.
	_ = s.logs.Append(ctx, s.pool, &domain.ActionLog{
		TargetUserID: &u.ID,
		Action:       domain.ActionForgetMe,
	})

	if err := s.users.ForgetMe(ctx, s.pool, u.ID); err != nil {
		return false, fmt.Errorf("delete user: %w", err)
	}
	return true, nil
}
