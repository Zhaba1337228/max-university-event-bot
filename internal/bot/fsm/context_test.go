package fsm_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
)

// TestMarshalUnmarshalRoundTrip — round-trip без потерь.
func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original := fsm.UserFSMContext{
		CurrentEventID:   42,
		DraftFullName:    "Иванов Иван Иванович",
		DraftInterest:    "Прикладная информатика",
		CancelRegID:      77,
		OrganizerEventID: 5,
		Offset:           20,
	}
	raw := original.Marshal()
	if len(raw) == 0 {
		t.Fatal("Marshal returned empty bytes")
	}
	got := fsm.Unmarshal(raw)
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round-trip mismatch:\nwant: %+v\ngot:  %+v", original, got)
	}
}

// TestMarshalOmitempty — пустые поля не должны попадать в JSON.
func TestMarshalOmitempty(t *testing.T) {
	t.Parallel()

	ctx := fsm.UserFSMContext{CurrentEventID: 1}
	raw := string(ctx.Marshal())
	// должно быть только current_event_id
	if !strings.Contains(raw, `"current_event_id":1`) {
		t.Errorf("want current_event_id in JSON, got: %s", raw)
	}
	if strings.Contains(raw, "draft_full_name") {
		t.Errorf("empty draft_full_name leaked: %s", raw)
	}
	if strings.Contains(raw, "organizer_event_id") {
		t.Errorf("empty organizer_event_id leaked: %s", raw)
	}
}

// TestUnmarshalEmpty — пустые/мусорные входы дают zero value без паники.
func TestUnmarshalEmpty(t *testing.T) {
	t.Parallel()

	cases := [][]byte{
		nil,
		{},
		[]byte("{}"),
		[]byte("garbage"),
		[]byte(`{"current_event_id":"not a number"}`),
	}
	for _, in := range cases {
		got := fsm.Unmarshal(in)
		if got.CurrentEventID != 0 {
			t.Errorf("expected zero context for input %q, got %+v", string(in), got)
		}
	}
}

// TestReset — обнуление.
func TestReset(t *testing.T) {
	t.Parallel()

	ctx := fsm.UserFSMContext{CurrentEventID: 1, DraftFullName: "A B"}
	ctx.Reset()
	if !reflect.DeepEqual(ctx, fsm.UserFSMContext{}) {
		t.Fatalf("Reset did not zero context: %+v", ctx)
	}
}

// TestIsTextInput — таблица состояний.
func TestIsTextInput(t *testing.T) {
	t.Parallel()

	textInputStates := []fsm.State{
		fsm.StateRegFullName,
		fsm.StateRegInterest,
		fsm.StateAIPickIntent,
		fsm.StateOrganizerNotifText,
	}
	for _, s := range textInputStates {
		if !fsm.IsTextInput(s) {
			t.Errorf("IsTextInput(%q): want true", s)
		}
	}

	notTextInput := []fsm.State{
		fsm.StateMainMenu,
		fsm.StateRegConsent,
		fsm.StateRegConfirmation,
		fsm.StateMyRegistration,
		fsm.StateOrganizerMenu,
		"unknown_state",
	}
	for _, s := range notTextInput {
		if fsm.IsTextInput(s) {
			t.Errorf("IsTextInput(%q): want false", s)
		}
	}
}
