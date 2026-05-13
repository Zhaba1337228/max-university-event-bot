package domain_test

import (
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

func TestUserHasConsent(t *testing.T) {
	t.Parallel()

	if (*domain.User)(nil).HasConsent() {
		t.Error("nil user must not have consent")
	}
	u := &domain.User{}
	if u.HasConsent() {
		t.Error("user without consent_at must not have consent")
	}
	now := time.Now()
	u.ConsentAt = &now
	if !u.HasConsent() {
		t.Error("user with consent_at must have consent")
	}
}

func TestUserRoles(t *testing.T) {
	t.Parallel()

	if (*domain.User)(nil).IsOrganizer() {
		t.Error("nil user must not be organizer")
	}
	if (*domain.User)(nil).IsAdmin() {
		t.Error("nil user must not be admin")
	}

	applicant := &domain.User{Role: domain.RoleApplicant}
	if applicant.IsOrganizer() {
		t.Error("applicant must not be organizer")
	}
	if applicant.IsAdmin() {
		t.Error("applicant must not be admin")
	}

	org := &domain.User{Role: domain.RoleOrganizer}
	if !org.IsOrganizer() {
		t.Error("organizer must be organizer")
	}
	if org.IsAdmin() {
		t.Error("organizer must not be admin")
	}

	admin := &domain.User{Role: domain.RoleAdmin}
	if !admin.IsOrganizer() {
		t.Error("admin must also be organizer (RBAC inclusion)")
	}
	if !admin.IsAdmin() {
		t.Error("admin must be admin")
	}
}

func TestRegistrationStatusActive(t *testing.T) {
	t.Parallel()

	active := []domain.RegistrationStatus{
		domain.RegStatusRegistered,
		domain.RegStatusWaitlist,
	}
	for _, s := range active {
		if !s.IsActive() {
			t.Errorf("%q must be active", s)
		}
	}
	inactive := []domain.RegistrationStatus{
		domain.RegStatusCancelledByUser,
		domain.RegStatusCancelledByOrganizer,
		domain.RegStatusAttended,
		domain.RegStatusNoShow,
	}
	for _, s := range inactive {
		if s.IsActive() {
			t.Errorf("%q must NOT be active", s)
		}
	}
}

func TestRegistrationStatusCancelled(t *testing.T) {
	t.Parallel()

	cancelled := []domain.RegistrationStatus{
		domain.RegStatusCancelledByUser,
		domain.RegStatusCancelledByOrganizer,
	}
	for _, s := range cancelled {
		if !s.IsCancelled() {
			t.Errorf("%q must be cancelled", s)
		}
	}
	notCancelled := []domain.RegistrationStatus{
		domain.RegStatusRegistered,
		domain.RegStatusWaitlist,
		domain.RegStatusAttended,
		domain.RegStatusNoShow,
	}
	for _, s := range notCancelled {
		if s.IsCancelled() {
			t.Errorf("%q must NOT be cancelled", s)
		}
	}
}

func TestEventIsOpenForRegistration(t *testing.T) {
	t.Parallel()

	if (*domain.Event)(nil).IsOpenForRegistration() {
		t.Error("nil event must not be open for registration")
	}

	cases := map[domain.EventStatus]bool{
		domain.EventStatusOpen:      true,
		domain.EventStatusClosed:    false,
		domain.EventStatusCancelled: false,
		domain.EventStatusFinished:  false,
	}
	for st, want := range cases {
		e := &domain.Event{Status: st}
		if got := e.IsOpenForRegistration(); got != want {
			t.Errorf("status=%q: want %v, got %v", st, want, got)
		}
	}
}
