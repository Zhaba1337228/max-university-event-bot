package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// =============================================================================
// In-memory моки для UserRepo / ActionLogRepo.
// =============================================================================

type mockUserRepo struct {
	getByIDFunc func(id int64) (*domain.User, error)
	setRoleFunc func(id int64, role domain.Role) error
	repo.UserRepo
}

func (m *mockUserRepo) GetByID(_ context.Context, _ repo.Querier, id int64) (*domain.User, error) {
	return m.getByIDFunc(id)
}

func (m *mockUserRepo) SetRole(_ context.Context, _ repo.Querier, id int64, r domain.Role) error {
	if m.setRoleFunc == nil {
		return nil
	}
	return m.setRoleFunc(id, r)
}

type mockActionLogRepo struct {
	appendCalls []*domain.ActionLog
	repo.ActionLogRepo
}

func (m *mockActionLogRepo) Append(_ context.Context, _ repo.Querier, l *domain.ActionLog) error {
	m.appendCalls = append(m.appendCalls, l)
	return nil
}

// =============================================================================
// User.SetRole
// =============================================================================

func TestUserSetRole_OK(t *testing.T) {
	t.Parallel()
	logs := &mockActionLogRepo{}
	users := &mockUserRepo{
		getByIDFunc: func(id int64) (*domain.User, error) {
			return &domain.User{ID: id, MaxUserID: 100, Role: domain.RoleApplicant}, nil
		},
	}
	svc := service.NewUser(nil, users, logs)
	u, err := svc.SetRole(context.Background(), 1 /*actor*/, domain.RoleAdmin, 42 /*target*/, domain.RoleOrganizer)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if u.Role != domain.RoleOrganizer {
		t.Errorf("want role=organizer, got %s", u.Role)
	}
	if len(logs.appendCalls) != 1 {
		t.Fatalf("expected 1 audit log entry, got %d", len(logs.appendCalls))
	}
	if logs.appendCalls[0].Action != domain.ActionUserRoleChanged {
		t.Errorf("audit log action want user_role_changed, got %s", logs.appendCalls[0].Action)
	}
}

func TestUserSetRole_InvalidRole(t *testing.T) {
	t.Parallel()
	svc := service.NewUser(nil, &mockUserRepo{}, &mockActionLogRepo{})
	_, err := svc.SetRole(context.Background(), 1, domain.RoleAdmin, 42, domain.Role("bogus"))
	if !errors.Is(err, service.ErrUserInvalidRole) {
		t.Errorf("want ErrUserInvalidRole, got %v", err)
	}
}

func TestUserSetRole_CannotChangeSelf(t *testing.T) {
	t.Parallel()
	svc := service.NewUser(nil, &mockUserRepo{}, &mockActionLogRepo{})
	_, err := svc.SetRole(context.Background(), 42, domain.RoleAdmin, 42, domain.RoleApplicant)
	if !errors.Is(err, service.ErrUserCannotChangeSelf) {
		t.Errorf("want ErrUserCannotChangeSelf, got %v", err)
	}
}

func TestUserSetRole_NotFound(t *testing.T) {
	t.Parallel()
	users := &mockUserRepo{
		getByIDFunc: func(_ int64) (*domain.User, error) { return nil, nil },
	}
	svc := service.NewUser(nil, users, &mockActionLogRepo{})
	_, err := svc.SetRole(context.Background(), 1, domain.RoleAdmin, 999, domain.RoleAdmin)
	if !errors.Is(err, service.ErrUserNotFound) {
		t.Errorf("want ErrUserNotFound, got %v", err)
	}
}

func TestUserSetRole_NoChange_NoAuditLog(t *testing.T) {
	t.Parallel()
	logs := &mockActionLogRepo{}
	users := &mockUserRepo{
		getByIDFunc: func(id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleStaff}, nil
		},
	}
	svc := service.NewUser(nil, users, logs)
	_, err := svc.SetRole(context.Background(), 1, domain.RoleAdmin, 42, domain.RoleStaff)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(logs.appendCalls) != 0 {
		t.Errorf("audit log must not be written when role unchanged; got %d entries", len(logs.appendCalls))
	}
}

func TestUserSetRole_OrganizerCanManageVolunteer(t *testing.T) {
	t.Parallel()

	users := &mockUserRepo{
		getByIDFunc: func(id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleApplicant}, nil
		},
	}
	svc := service.NewUser(nil, users, &mockActionLogRepo{})

	u, err := svc.SetRole(context.Background(), 7, domain.RoleOrganizer, 42, domain.RoleStaff)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if u.Role != domain.RoleStaff {
		t.Errorf("want role=staff, got %s", u.Role)
	}
}

func TestUserSetRole_OrganizerCannotPromoteOrganizer(t *testing.T) {
	t.Parallel()

	users := &mockUserRepo{
		getByIDFunc: func(id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleApplicant}, nil
		},
	}
	svc := service.NewUser(nil, users, &mockActionLogRepo{})

	_, err := svc.SetRole(context.Background(), 7, domain.RoleOrganizer, 42, domain.RoleOrganizer)
	if !errors.Is(err, service.ErrUserRoleChangeDenied) {
		t.Errorf("want ErrUserRoleChangeDenied, got %v", err)
	}
}

func TestUserSetRole_OrganizerCannotTouchAdmin(t *testing.T) {
	t.Parallel()

	users := &mockUserRepo{
		getByIDFunc: func(id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleAdmin}, nil
		},
	}
	svc := service.NewUser(nil, users, &mockActionLogRepo{})

	_, err := svc.SetRole(context.Background(), 7, domain.RoleOrganizer, 99, domain.RoleStaff)
	if !errors.Is(err, service.ErrUserRoleChangeDenied) {
		t.Errorf("want ErrUserRoleChangeDenied, got %v", err)
	}
}
