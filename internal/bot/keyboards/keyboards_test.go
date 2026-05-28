package keyboards_test

import (
	"strings"
	"testing"
	"time"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/keyboards"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// rowsCount возвращает число рядов в built клавиатуре.
// Через Build() переводим в schemes.Keyboard, у которой доступны ряды.
func rowsCount(kb interface {
	Build() schemes.Keyboard
}) int {
	return len(kb.Build().Buttons)
}

func TestMainMenuStructure(t *testing.T) {
	t.Parallel()

	kb := keyboards.MainMenu()
	if rows := rowsCount(kb); rows != 5 {
		t.Errorf("MainMenu: want 5 rows, got %d", rows)
	}
}

func TestEventListEmpty(t *testing.T) {
	t.Parallel()

	kb := keyboards.EventList(nil, 0, false, "")
	rows := kb.Build().Buttons
	// Ряд фильтров + «в главное меню».
	if len(rows) != 2 {
		t.Errorf("want 2 rows for empty list (filter+menu), got %d", len(rows))
	}
}

func TestEventListWithPaging(t *testing.T) {
	t.Parallel()

	events := []*domain.Event{
		{ID: 1, Title: "A"},
		{ID: 2, Title: "B"},
	}
	kb := keyboards.EventList(events, 8, true, "")
	rows := kb.Build().Buttons

	// 2 события + ряд навигации (Back+page+Дальше) + ряд фильтров + ряд "В главное меню" = 5
	if len(rows) != 5 {
		t.Errorf("want 5 rows, got %d", len(rows))
	}
	// offset=8>=pageSize → есть Назад; hasMore=true → есть Вперёд; плюс номер страницы = 3
	navRow := rows[2]
	if len(navRow) != 3 {
		t.Errorf("want 3 nav buttons, got %d", len(navRow))
	}
}

func TestEventCardFreeSeats(t *testing.T) {
	t.Parallel()

	kb := keyboards.EventCard(42, 5, true, 0, nil)
	rows := kb.Build().Buttons

	// «Записаться» + «Назад к списку» + «В главное меню» = 3
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	firstBtn, ok := rows[0][0].(schemes.CallbackButton)
	if !ok {
		t.Fatalf("first button is not callback: %T", rows[0][0])
	}
	if !strings.Contains(firstBtn.Text, "Записаться") {
		t.Errorf("first button text = %q, want contains 'Записаться'", firstBtn.Text)
	}
	if firstBtn.Intent != schemes.POSITIVE {
		t.Errorf("want POSITIVE intent, got %v", firstBtn.Intent)
	}
}

func TestEventCardWaitlist(t *testing.T) {
	t.Parallel()

	kb := keyboards.EventCard(42, 0, true, 0, nil)
	if !hasCallbackText(kb.Build(), "лист ожидания") {
		t.Errorf("want 'лист ожидания' button")
	}
}

func TestEventCardNoWaitlistAndNoSeats(t *testing.T) {
	t.Parallel()

	kb := keyboards.EventCard(42, 0, false, 0, nil)
	rows := kb.Build().Buttons
	// «Подробнее» + «Назад к списку» + «В главное меню»
	if len(rows) != 3 {
		t.Errorf("want 3 rows when no seats and no waitlist, got %d", len(rows))
	}
}

func TestEventCardRegisteredUser(t *testing.T) {
	t.Parallel()

	kb := keyboards.EventCard(42, 5, true, 0, &domain.Registration{Status: domain.RegStatusRegistered})
	rows := kb.Build().Buttons
	if len(rows) != 3 {
		t.Fatalf("want 3 rows for registered user card, got %d", len(rows))
	}
	if !hasCallbackText(kb.Build(), "Моя запись") {
		t.Errorf("registered user card: missing 'Моя запись' button")
	}
	if hasCallbackText(kb.Build(), "Записаться") {
		t.Errorf("registered user card: unexpected 'Записаться' button")
	}
}

func TestRegConsentTwoButtons(t *testing.T) {
	t.Parallel()

	kb := keyboards.RegConsent()
	rows := kb.Build().Buttons
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("RegConsent: want 1 row with 2 buttons, got rows=%d, cols0=%d",
			len(rows), func() int {
				if len(rows) == 0 {
					return 0
				}
				return len(rows[0])
			}())
	}
}

func TestOrganizerEventActionsWithStatus(t *testing.T) {
	t.Parallel()

	// Open event → кнопка «Закрыть регистрацию» присутствует.
	kbOpen := keyboards.OrganizerEventActions(42, domain.EventStatusOpen)
	if !hasCallbackText(kbOpen.Build(), "Закрыть регистрацию") {
		t.Errorf("OrganizerEventActions(open): missing 'Закрыть регистрацию'")
	}

	// Closed event → нет кнопки закрытия.
	kbClosed := keyboards.OrganizerEventActions(42, domain.EventStatusClosed)
	if hasCallbackText(kbClosed.Build(), "Закрыть регистрацию") {
		t.Errorf("OrganizerEventActions(closed): unexpected 'Закрыть регистрацию' present")
	}
}

func TestAdminLoginLink(t *testing.T) {
	t.Parallel()

	url := "https://admin.example.com/auth?t=jwt-here"
	kb := keyboards.AdminLoginLink(url)
	rows := kb.Build().Buttons
	if len(rows) != 1 || len(rows[0]) != 1 {
		t.Fatalf("want 1x1, got %dx%d", len(rows), func() int {
			if len(rows) == 0 {
				return 0
			}
			return len(rows[0])
		}())
	}
	btn, ok := rows[0][0].(schemes.LinkButton)
	if !ok {
		t.Fatalf("want LinkButton, got %T", rows[0][0])
	}
	if btn.Url != url {
		t.Errorf("URL: want %q, got %q", url, btn.Url)
	}
}

func TestYesNoNoSurprises(t *testing.T) {
	t.Parallel()

	kb := keyboards.YesNo("Да", "yes:ok", "Нет", "no:ok")
	rows := kb.Build().Buttons
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("YesNo: want 1x2, got %dx%d", len(rows), len(rows[0]))
	}
	if got := rows[0][0].(schemes.CallbackButton); got.Intent != schemes.POSITIVE {
		t.Errorf("yes intent: want POSITIVE, got %v", got.Intent)
	}
	if got := rows[0][1].(schemes.CallbackButton); got.Intent != schemes.NEGATIVE {
		t.Errorf("no intent: want NEGATIVE, got %v", got.Intent)
	}
}

func TestMyRegistrationConditional(t *testing.T) {
	t.Parallel()

	// regID == 0 → нет QR-кнопки и нет «Отменить»
	kb := keyboards.MyRegistration(0, false)
	if hasCallbackText(kb.Build(), "Показать мой QR") {
		t.Errorf("regID=0: unexpected QR button")
	}
	if hasCallbackText(kb.Build(), "Отменить запись") {
		t.Errorf("regID=0: unexpected cancel button")
	}

	// regID != 0 → есть обе кнопки
	kb2 := keyboards.MyRegistration(77, false)
	if !hasCallbackText(kb2.Build(), "Показать мой QR") {
		t.Errorf("regID=77: missing QR button")
	}
	if !hasCallbackText(kb2.Build(), "Отменить запись") {
		t.Errorf("regID=77: missing cancel button")
	}
}

// hasCallbackText сканирует все callback-кнопки и возвращает true, если
// найдена кнопка с подстрокой substr в тексте.
func hasCallbackText(k schemes.Keyboard, substr string) bool {
	for _, row := range k.Buttons {
		for _, b := range row {
			if cb, ok := b.(schemes.CallbackButton); ok {
				if strings.Contains(cb.Text, substr) {
					return true
				}
			}
		}
	}
	return false
}

var _ = time.Time{} // импорт оставлен для будущих тестов с датами
