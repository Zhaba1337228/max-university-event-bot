package adminapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

func TestParseEventInputParsesOptionalFields(t *testing.T) {
	t.Parallel()

	body := io.NopCloser(strings.NewReader(`{
		"title":"Open Day",
		"description":"Details",
		"starts_at":"2026-05-25T10:00:00Z",
		"ends_at":"2026-05-25T12:30:00Z",
		"location":"Main Campus",
		"format":"offline",
		"capacity":50,
		"status":"closed",
		"tags":["it","open-day"],
		"late_cancel_allowed":true
	}`))

	got, err := parseEventInput(body, true)
	if err != nil {
		t.Fatalf("parseEventInput: %v", err)
	}
	if got.Title != "Open Day" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.EndsAt == nil {
		t.Fatal("EndsAt must be set")
	}
	if got.Status != domain.EventStatusClosed {
		t.Fatalf("status = %s, want %s", got.Status, domain.EventStatusClosed)
	}
	if !got.LateCancelAllowed {
		t.Fatal("LateCancelAllowed must be true")
	}
}

func TestParseEventInputRejectsBadStartsAt(t *testing.T) {
	t.Parallel()

	body := io.NopCloser(strings.NewReader(`{
		"title":"x",
		"starts_at":"not-a-date"
	}`))

	_, err := parseEventInput(body, false)
	if err == nil || !strings.Contains(err.Error(), "bad_starts_at") {
		t.Fatalf("err = %v, want bad_starts_at", err)
	}
}

func TestActionLogToDTOParsesPayload(t *testing.T) {
	t.Parallel()

	actorID := int64(7)
	targetID := int64(42)
	evID := int64(99)
	regID := int64(1001)
	createdAt := time.Date(2026, time.May, 25, 11, 0, 0, 0, time.UTC)

	dto := actionLogToDTO(&domain.ActionLog{
		ID:             1,
		ActorUserID:    &actorID,
		TargetUserID:   &targetID,
		EventID:        &evID,
		RegistrationID: &regID,
		Action:         domain.ActionParticipantsExported,
		Payload:        []byte(`{"rows":15}`),
		CreatedAt:      createdAt,
	})

	payload, ok := dto["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload = %#v, want parsed object", dto["payload"])
	}
	if payload["rows"] != float64(15) {
		t.Fatalf("payload.rows = %v, want 15", payload["rows"])
	}
}

func TestEventToDTOIncludesOptionalFields(t *testing.T) {
	t.Parallel()

	createdBy := int64(17)
	shortSummary := "Short summary"
	endsAt := time.Date(2026, time.May, 25, 12, 0, 0, 0, time.UTC)

	dto := eventToDTO(&domain.Event{
		ID:                5,
		Title:             "IT Day",
		Description:       "Description",
		ShortSummary:      &shortSummary,
		StartsAt:          time.Date(2026, time.May, 25, 10, 0, 0, 0, time.UTC),
		EndsAt:            &endsAt,
		Location:          "Campus A",
		Format:            domain.EventFormatHybrid,
		Capacity:          120,
		Status:            domain.EventStatusOpen,
		CreatedBy:         &createdBy,
		Tags:              []string{"it"},
		LateCancelAllowed: true,
	})

	if dto["short_summary"] != shortSummary {
		t.Fatalf("short_summary = %v, want %s", dto["short_summary"], shortSummary)
	}
	if dto["created_by"] != createdBy {
		t.Fatalf("created_by = %v, want %d", dto["created_by"], createdBy)
	}
	if dto["ends_at"] != endsAt.UTC().Format(time.RFC3339) {
		t.Fatalf("ends_at = %v, want %s", dto["ends_at"], endsAt.UTC().Format(time.RFC3339))
	}
}

func TestStatsToDTOIncludesTopInterests(t *testing.T) {
	t.Parallel()

	dto := statsToDTO(&domain.EventStats{
		Capacity:   50,
		Registered: 40,
		Cancelled:  2,
		Waitlist:   5,
		Attended:   10,
		NoShow:     3,
		FreeSeats:  0,
		TopInterests: map[string]int{
			"Applied Informatics": 8,
		},
	})

	top, ok := dto["top_interests"].(map[string]int)
	if !ok {
		t.Fatalf("top_interests = %#v, want map[string]int", dto["top_interests"])
	}
	if top["Applied Informatics"] != 8 {
		t.Fatalf("top_interests value = %d, want 8", top["Applied Informatics"])
	}
}

func TestMaskHelpers(t *testing.T) {
	t.Parallel()

	if got := maskFullName("Ivan Ivanov Ivanovich"); got != "Ivan I. I." {
		t.Fatalf("maskFullName = %q, want %q", got, "Ivan I. I.")
	}
	if got := maskContactDTO("alex@example.com"); got != "al***@example.com" {
		t.Fatalf("maskContactDTO email = %q", got)
	}
	if got := maskContactDTO("79161234567"); got != "79***67" {
		t.Fatalf("maskContactDTO phone = %q", got)
	}
}

func TestParseIntDefaultClampsBounds(t *testing.T) {
	t.Parallel()

	if got := parseIntDefault("", 50, 1, 100); got != 50 {
		t.Fatalf("empty = %d, want 50", got)
	}
	if got := parseIntDefault("-10", 50, 1, 100); got != 1 {
		t.Fatalf("low clamp = %d, want 1", got)
	}
	if got := parseIntDefault("999", 50, 1, 100); got != 100 {
		t.Fatalf("high clamp = %d, want 100", got)
	}
}

func TestOriginGuardRejectsBadOrigin(t *testing.T) {
	t.Parallel()

	handler := originGuard("https://zhaba1337.eu.cc")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users/42/role", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestOriginGuardAllowsGoodOrigin(t *testing.T) {
	t.Parallel()

	called := false
	handler := originGuard("https://zhaba1337.eu.cc")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users/42/role", nil)
	req.Header.Set("Origin", "https://zhaba1337.eu.cc")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestClaimsFromContextMissing(t *testing.T) {
	t.Parallel()

	if claims, ok := claimsFromContext(context.Background()); ok || claims != nil {
		t.Fatalf("claims = %#v, ok = %v; want nil,false", claims, ok)
	}
}
