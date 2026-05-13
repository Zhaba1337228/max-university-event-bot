package callbacks_test

import (
	"strings"
	"testing"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
)

// TestRoundTrip проверяет, что для каждого конструктора Parse возвращает
// payload с правильными Group/Action/Args. Это самая важная гарантия
// согласованности: если поломаем конструктор, тест упадёт.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		group   string
		action  string
		args    []string
	}{
		{"main_menu", callbacks.MainMenu(), "main", "menu", nil},
		{"event_list_page_0", callbacks.EventListPage(0), "ev", "list", []string{"0"}},
		{"event_list_page_50", callbacks.EventListPage(50), "ev", "list", []string{"50"}},
		{"event_show", callbacks.EventShow(42), "ev", "show", []string{"42"}},

		{"reg_start", callbacks.RegStart(7), "reg", "start", []string{"7"}},
		{"reg_consent_yes", callbacks.RegConsentYes(), "reg", "consent_yes", nil},
		{"reg_consent_no", callbacks.RegConsentNo(), "reg", "consent_no", nil},
		{"reg_confirm", callbacks.RegConfirm(), "reg", "confirm", nil},
		{"reg_edit", callbacks.RegEdit(), "reg", "edit", nil},
		{"reg_cancel", callbacks.RegCancelDraft(), "reg", "cancel", nil},

		{"my_show", callbacks.MyShow(), "my", "show", nil},
		{"my_history", callbacks.MyHistory(), "my", "history", nil},
		{"my_show_qr", callbacks.MyShowQR(123), "my", "qr", []string{"123"}},
		{"forget_me_ask", callbacks.ForgetMeAsk(), "my", "forget_ask", nil},
		{"forget_me_yes", callbacks.ForgetMeYes(), "my", "forget_yes", nil},
		{"forget_me_no", callbacks.ForgetMeNo(), "my", "forget_no", nil},

		{"cancel_ask", callbacks.CancelAsk(99), "cancel", "ask", []string{"99"}},
		{"cancel_yes", callbacks.CancelYes(99), "cancel", "yes", []string{"99"}},
		{"cancel_no", callbacks.CancelNo(99), "cancel", "no", []string{"99"}},

		{"ai_pick", callbacks.AIPickStart(), "ai", "pick", nil},

		{"waitlist_join", callbacks.WaitlistJoin(11), "wl", "join", []string{"11"}},
		{"waitlist_promote_yes", callbacks.WaitlistPromoteYes(22), "wl", "yes", []string{"22"}},
		{"waitlist_promote_no", callbacks.WaitlistPromoteNo(22), "wl", "no", []string{"22"}},

		{"org_entry", callbacks.OrgEntry(), "org", "entry", nil},
		{"org_stats", callbacks.OrgStats(5), "org", "stats", []string{"5"}},
		{"org_ai_summary", callbacks.OrgAISummary(5), "org", "ai_summary", []string{"5"}},
		{"org_list_show", callbacks.OrgListParticipants(5, 10), "orglist", "show", []string{"5", "10"}},
		{"org_list_csv", callbacks.OrgListExport(5), "orglist", "csv", []string{"5"}},
		{"org_notif_start", callbacks.OrgNotifStart(5), "orgnotif", "start", []string{"5"}},
		{"org_notif_ai", callbacks.OrgNotifAIRewrite(), "orgnotif", "ai", nil},
		{"org_notif_send", callbacks.OrgNotifSend(), "orgnotif", "send", nil},
		{"org_notif_cancel", callbacks.OrgNotifCancel(), "orgnotif", "cancel", nil},
		{"org_close_ask", callbacks.OrgCloseAsk(5), "orgclose", "ask", []string{"5"}},
		{"org_close_yes", callbacks.OrgCloseYes(5), "orgclose", "yes", []string{"5"}},

		{"admin_promote", callbacks.AdminPromote(777), "admin", "promote", []string{"777"}},
		{"admin_demote", callbacks.AdminDemote(777), "admin", "demote", []string{"777"}},

		{"back_to_main", callbacks.BackTo("main_menu"), "back", "main_menu", nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := callbacks.Parse(tc.payload)
			if got.Group != tc.group {
				t.Fatalf("Group: want %q, got %q (payload=%q)", tc.group, got.Group, tc.payload)
			}
			if got.Action != tc.action {
				t.Fatalf("Action: want %q, got %q (payload=%q)", tc.action, got.Action, tc.payload)
			}
			if len(got.Args) != len(tc.args) {
				t.Fatalf("Args len: want %d, got %d (payload=%q, args=%v)",
					len(tc.args), len(got.Args), tc.payload, got.Args)
			}
			for i, want := range tc.args {
				if got.Args[i] != want {
					t.Fatalf("Args[%d]: want %q, got %q (payload=%q)", i, want, got.Args[i], tc.payload)
				}
			}
		})
	}
}

// TestParseEdge — пограничные случаи: пустой ввод, ":", двойные двоеточия.
func TestParseEdge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		group   string
		action  string
		argsLen int
	}{
		{"empty", "", "", "", 0},
		{"only_group", "ev", "ev", "", 0},
		{"group_and_action", "ev:list", "ev", "list", 0},
		{"trailing_colon", "ev:list:", "ev", "list", 0},
		{"empty_arg_between", "ev:list::5", "ev", "list", 2},
		{"unknown_group", "xxx:yyy:1", "xxx", "yyy", 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := callbacks.Parse(tc.payload)
			if got.Group != tc.group {
				t.Errorf("Group: want %q, got %q", tc.group, got.Group)
			}
			if got.Action != tc.action {
				t.Errorf("Action: want %q, got %q", tc.action, got.Action)
			}
			if len(got.Args) != tc.argsLen {
				t.Errorf("Args len: want %d, got %d (args=%v)", tc.argsLen, len(got.Args), got.Args)
			}
		})
	}
}

// TestArgInt64Safety проверяет, что доступ к несуществующему/невалидному
// аргументу не паникует и возвращает 0.
func TestArgInt64Safety(t *testing.T) {
	t.Parallel()

	p := callbacks.Parse("ev:show:42:notanumber")
	if got := p.ArgInt64(0); got != 42 {
		t.Fatalf("ArgInt64(0): want 42, got %d", got)
	}
	if got := p.ArgInt64(1); got != 0 {
		t.Fatalf("ArgInt64(1): want 0 for non-numeric, got %d", got)
	}
	if got := p.ArgInt64(2); got != 0 {
		t.Fatalf("ArgInt64(2): want 0 for out of range, got %d", got)
	}
	if got := p.ArgInt64(-1); got != 0 {
		t.Fatalf("ArgInt64(-1): want 0 for negative index, got %d", got)
	}
	if got := p.ArgString(0); got != "42" {
		t.Fatalf("ArgString(0): want \"42\", got %q", got)
	}
	if got := p.ArgString(99); got != "" {
		t.Fatalf("ArgString(99): want \"\" for out of range, got %q", got)
	}
}

// TestNoColonInUserPayload — наши конструкторы не должны порождать
// payload'ы с подряд идущими ":" или превышать разумную длину.
func TestNoColonInUserPayload(t *testing.T) {
	t.Parallel()

	payloads := []string{
		callbacks.MainMenu(),
		callbacks.EventShow(123),
		callbacks.RegStart(123),
		callbacks.OrgListParticipants(123, 10),
	}
	for _, p := range payloads {
		if strings.Contains(p, "::") {
			t.Errorf("payload contains double colon: %q", p)
		}
		if len(p) > 256 {
			t.Errorf("payload too long: %q (len=%d)", p, len(p))
		}
	}
}
