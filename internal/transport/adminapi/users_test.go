package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

type stubUserService struct {
	ensureProfileFunc func(context.Context, int64, string, string) (*domain.User, error)
	getByMaxIDFunc    func(context.Context, int64) (*domain.User, error)
	getByIDFunc       func(context.Context, int64) (*domain.User, error)
	grantConsentFunc  func(context.Context, int64, string) error
	forgetMeFunc      func(context.Context, int64) (bool, error)
	listFunc          func(context.Context, domain.Role, string, int, int) ([]*domain.User, int, error)
	setRoleFunc       func(context.Context, int64, domain.Role, int64, domain.Role) (*domain.User, error)
}

func (s *stubUserService) EnsureProfile(ctx context.Context, maxUserID int64, fullName, contact string) (*domain.User, error) {
	if s.ensureProfileFunc == nil {
		panic("unexpected EnsureProfile call")
	}
	return s.ensureProfileFunc(ctx, maxUserID, fullName, contact)
}

func (s *stubUserService) GetByMaxID(ctx context.Context, maxUserID int64) (*domain.User, error) {
	if s.getByMaxIDFunc == nil {
		panic("unexpected GetByMaxID call")
	}
	return s.getByMaxIDFunc(ctx, maxUserID)
}

func (s *stubUserService) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	if s.getByIDFunc == nil {
		panic("unexpected GetByID call")
	}
	return s.getByIDFunc(ctx, id)
}

func (s *stubUserService) GrantConsent(ctx context.Context, userID int64, policyVer string) error {
	if s.grantConsentFunc == nil {
		panic("unexpected GrantConsent call")
	}
	return s.grantConsentFunc(ctx, userID, policyVer)
}

func (s *stubUserService) ForgetMe(ctx context.Context, maxUserID int64) (bool, error) {
	if s.forgetMeFunc == nil {
		panic("unexpected ForgetMe call")
	}
	return s.forgetMeFunc(ctx, maxUserID)
}

func (s *stubUserService) List(ctx context.Context, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error) {
	if s.listFunc == nil {
		panic("unexpected List call")
	}
	return s.listFunc(ctx, roleFilter, query, limit, offset)
}

func (s *stubUserService) SetRole(ctx context.Context, actorID int64, actorRole domain.Role, targetUserID int64, newRole domain.Role) (*domain.User, error) {
	if s.setRoleFunc == nil {
		panic("unexpected SetRole call")
	}
	return s.setRoleFunc(ctx, actorID, actorRole, targetUserID, newRole)
}

func TestRequireRolesAllowsOrganizer(t *testing.T) {
	t.Parallel()

	handler := requireRoles(domain.RoleAdmin, domain.RoleOrganizer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := requestWithClaims(t, httptest.NewRequest(http.MethodGet, "/api/users", nil), 10, domain.RoleOrganizer)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestRequireRolesRejectsStaff(t *testing.T) {
	t.Parallel()

	handler := requireRoles(domain.RoleAdmin, domain.RoleOrganizer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := requestWithClaims(t, httptest.NewRequest(http.MethodGet, "/api/users", nil), 10, domain.RoleStaff)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	resp := decodeMap(t, rr.Body.Bytes())
	if resp["error"] != "role_required" {
		t.Fatalf("error = %v, want role_required", resp["error"])
	}
}

func TestHandleSetUserRoleOrganizerCanGrantStaff(t *testing.T) {
	t.Parallel()

	users := &stubUserService{
		setRoleFunc: func(_ context.Context, actorID int64, actorRole domain.Role, targetUserID int64, newRole domain.Role) (*domain.User, error) {
			if actorID != 10 {
				t.Fatalf("actorID = %d, want 10", actorID)
			}
			if actorRole != domain.RoleOrganizer {
				t.Fatalf("actorRole = %s, want %s", actorRole, domain.RoleOrganizer)
			}
			if targetUserID != 42 {
				t.Fatalf("targetUserID = %d, want 42", targetUserID)
			}
			if newRole != domain.RoleStaff {
				t.Fatalf("newRole = %s, want %s", newRole, domain.RoleStaff)
			}
			return &domain.User{
				ID:        42,
				MaxUserID: 232513363,
				Role:      domain.RoleStaff,
				CreatedAt: time.Date(2026, time.May, 25, 10, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	server := newUserTestServer(users)
	req := httptest.NewRequest(http.MethodPatch, "/api/users/42/role", bytes.NewBufferString(`{"role":"staff"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParam(requestWithClaims(t, req, 10, domain.RoleOrganizer), "id", "42")
	rr := httptest.NewRecorder()

	server.handleSetUserRole(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeMap(t, rr.Body.Bytes())
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("user payload missing or wrong type: %#v", resp["user"])
	}
	if user["role"] != string(domain.RoleStaff) {
		t.Fatalf("role = %v, want %s", user["role"], domain.RoleStaff)
	}
}

func TestHandleSetUserRoleMapsRoleChangeDenied(t *testing.T) {
	t.Parallel()

	users := &stubUserService{
		setRoleFunc: func(_ context.Context, _ int64, _ domain.Role, _ int64, _ domain.Role) (*domain.User, error) {
			return nil, service.ErrUserRoleChangeDenied
		},
	}

	server := newUserTestServer(users)
	req := httptest.NewRequest(http.MethodPatch, "/api/users/42/role", bytes.NewBufferString(`{"role":"organizer"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParam(requestWithClaims(t, req, 10, domain.RoleOrganizer), "id", "42")
	rr := httptest.NewRecorder()

	server.handleSetUserRole(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	resp := decodeMap(t, rr.Body.Bytes())
	if resp["error"] != "role_change_forbidden" {
		t.Fatalf("error = %v, want role_change_forbidden", resp["error"])
	}
}

func TestHandleListUsersReturnsMaskedContacts(t *testing.T) {
	t.Parallel()

	fullName := "Ivan Ivanov"
	phone := "79161234567"
	email := "alex@example.com"
	now := time.Date(2026, time.May, 25, 10, 0, 0, 0, time.UTC)
	users := &stubUserService{
		listFunc: func(_ context.Context, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error) {
			if roleFilter != domain.Role("") {
				t.Fatalf("roleFilter = %q, want empty", roleFilter)
			}
			if query != "232513363" {
				t.Fatalf("query = %q, want 232513363", query)
			}
			if limit != 10 {
				t.Fatalf("limit = %d, want 10", limit)
			}
			if offset != 5 {
				t.Fatalf("offset = %d, want 5", offset)
			}
			return []*domain.User{{
				ID:        42,
				MaxUserID: 232513363,
				FullName:  &fullName,
				Phone:     &phone,
				Email:     &email,
				Role:      domain.RoleStaff,
				CreatedAt: now,
			}}, 1, nil
		},
	}

	server := newUserTestServer(users)
	req := httptest.NewRequest(http.MethodGet, "/api/users?query=232513363&limit=10&offset=5", nil)
	rr := httptest.NewRecorder()

	server.handleListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeMap(t, rr.Body.Bytes())
	if resp["total"] != float64(1) {
		t.Fatalf("total = %v, want 1", resp["total"])
	}

	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v, want single item", resp["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %#v", items[0])
	}
	if item["phone_masked"] != "79***67" {
		t.Fatalf("phone_masked = %v, want 79***67", item["phone_masked"])
	}
	if item["email_masked"] != "al***@example.com" {
		t.Fatalf("email_masked = %v, want al***@example.com", item["email_masked"])
	}
}

func TestHandleListUsersRejectsInvalidRoleFilter(t *testing.T) {
	t.Parallel()

	server := newUserTestServer(&stubUserService{
		listFunc: func(context.Context, domain.Role, string, int, int) ([]*domain.User, int, error) {
			t.Fatal("list should not be called for invalid role")
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/users?role=hacker", nil)
	rr := httptest.NewRecorder()

	server.handleListUsers(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	resp := decodeMap(t, rr.Body.Bytes())
	if resp["error"] != "bad_role" {
		t.Fatalf("error = %v, want bad_role", resp["error"])
	}
}

func newUserTestServer(users service.User) *Server {
	return &Server{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		deps: Deps{
			Users: users,
		},
	}
}

func requestWithClaims(t *testing.T, req *http.Request, userID int64, role domain.Role) *http.Request {
	t.Helper()

	claims := &service.Claims{UserID: userID, Role: role}
	ctx := context.WithValue(req.Context(), claimsKey{}, claims)
	return req.WithContext(ctx)
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	return req.WithContext(ctx)
}

func decodeMap(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return out
}
