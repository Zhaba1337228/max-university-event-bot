package service

import (
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

func TestCheckinWindowForEventWithoutEndsAtUsesMoscowDay(t *testing.T) {
	t.Parallel()

	ev := &domain.Event{
		StartsAt: time.Date(2026, time.May, 25, 0, 12, 0, 0, time.UTC), // 03:12 MSK
	}

	start, end := checkinWindowForEvent(ev)

	wantStart := time.Date(2026, time.May, 24, 21, 0, 0, 0, time.UTC) // 00:00 MSK
	wantEnd := time.Date(2026, time.May, 26, 1, 0, 0, 0, time.UTC)    // 04:00 MSK next day
	if !start.Equal(wantStart) {
		t.Fatalf("window start = %s, want %s", start.UTC().Format(time.RFC3339), wantStart.Format(time.RFC3339))
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("window end = %s, want %s", end.UTC().Format(time.RFC3339), wantEnd.Format(time.RFC3339))
	}
}

func TestCheckinWindowForEventWithEndsAtKeepsExplicitWindow(t *testing.T) {
	t.Parallel()

	endsAt := time.Date(2026, time.May, 25, 15, 0, 0, 0, time.UTC)
	ev := &domain.Event{
		StartsAt: time.Date(2026, time.May, 25, 10, 0, 0, 0, time.UTC),
		EndsAt:   &endsAt,
	}

	start, end := checkinWindowForEvent(ev)
	if !start.Equal(ev.StartsAt.Add(-checkinPreWindow)) {
		t.Fatalf("explicit event start window = %s, want %s", start.UTC().Format(time.RFC3339), ev.StartsAt.Add(-checkinPreWindow).UTC().Format(time.RFC3339))
	}
	if !end.Equal(endsAt.Add(checkinPostWindow)) {
		t.Fatalf("explicit event end window = %s, want %s", end.UTC().Format(time.RFC3339), endsAt.Add(checkinPostWindow).UTC().Format(time.RFC3339))
	}
}
