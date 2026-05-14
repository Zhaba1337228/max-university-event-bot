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

	// GetByID — точечный поиск по локальному id.
	GetByID(ctx context.Context, id int64) (*domain.User, error)

	// GrantConsent фиксирует согласие на обработку ПДн (152-ФЗ).
	// policyVer — версия документа на момент клика.
	GrantConsent(ctx context.Context, userID int64, policyVer string) error

	// ForgetMe удаляет пользователя со всеми каскадными данными.
	// Возвращает (deleted bool, err) — false если пользователя не было.
	ForgetMe(ctx context.Context, maxUserID int64) (bool, error)

	// List возвращает страницу пользователей (для админ-UI).
	// roleFilter == "" — без фильтра. query — case-insensitive подстрока.
	List(ctx context.Context, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error)

	// SetRole меняет роль пользователя. Доступно только admin'у (проверка в handler).
	// actorID нужен для записи в action_log; newRole валидируется (applicant/organizer/staff/admin).
	// Возвращает обновлённого пользователя.
	SetRole(ctx context.Context, actorID, targetUserID int64, newRole domain.Role) (*domain.User, error)
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

func (s *userService) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u, err := s.users.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (s *userService) List(ctx context.Context, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error) {
	users, total, err := s.users.List(ctx, s.pool, roleFilter, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

// SetRole меняет роль и пишет audit_log. Валидация:
//   - newRole ∈ {applicant, organizer, staff, admin}
//   - actor не может сам себя «понизить» из admin (защита от случайного локаута)
func (s *userService) SetRole(ctx context.Context, actorID, targetUserID int64, newRole domain.Role) (*domain.User, error) {
	if !isValidRole(newRole) {
		return nil, ErrUserInvalidRole
	}
	if actorID == targetUserID {
		return nil, ErrUserCannotChangeSelf
	}
	target, err := s.users.GetByID(ctx, s.pool, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("get target user: %w", err)
	}
	if target == nil {
		return nil, ErrUserNotFound
	}
	if target.Role == newRole {
		return target, nil
	}
	if err := s.users.SetRole(ctx, s.pool, targetUserID, newRole); err != nil {
		return nil, fmt.Errorf("set role: %w", err)
	}

	payload := fmt.Sprintf(`{"target_user_id":%d,"old_role":%q,"new_role":%q}`,
		targetUserID, string(target.Role), string(newRole))
	_ = s.logs.Append(ctx, s.pool, &domain.ActionLog{
		ActorUserID:  &actorID,
		TargetUserID: &targetUserID,
		Action:       domain.ActionUserRoleChanged,
		Payload:      []byte(payload),
	})

	target.Role = newRole
	return target, nil
}

func isValidRole(r domain.Role) bool {
	switch r {
	case domain.RoleApplicant, domain.RoleOrganizer, domain.RoleStaff, domain.RoleAdmin:
		return true
	}
	return false
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
