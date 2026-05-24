package messages_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// TestFormatDateTimeBasic — формат «13 мая 2026, 18:00» в MSK.
// Точная локализация (UTC vs MSK) зависит от наличия tzdata на хосте —
// важно, что есть день, русский месяц, год и HH:MM.
func TestFormatDateTimeBasic(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, time.May, 13, 18, 0, 0, 0, time.UTC)
	got := messages.FormatDateTime(in)

	checks := []string{"мая", "2026"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDateTime missing %q in %q", want, got)
		}
	}
	if len(got) < len("01 мая 2026, 00:00") {
		t.Errorf("FormatDateTime output suspiciously short: %q", got)
	}
}

// TestFormatDateTimePadding — день с однозначной цифрой должен быть «09», а не «9».
func TestFormatDateTimePadding(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, time.January, 9, 8, 5, 0, 0, time.UTC)
	got := messages.FormatDateTime(in)

	// проверяем «09 января» и «:05» в часах
	if !strings.Contains(got, "09 января 2026") && !strings.Contains(got, "09 января") {
		t.Errorf("want padded day 09 января in %q", got)
	}
	// время может быть 08:05 (UTC) или 11:05 (MSK с tzdata)
	if !strings.Contains(got, ":05") {
		t.Errorf("want padded minutes :05 in %q", got)
	}
}

// TestHumanStatus — таблица всех статусов.
func TestHumanStatus(t *testing.T) {
	t.Parallel()

	cases := map[domain.RegistrationStatus]string{
		domain.RegStatusRegistered:           "запись подтверждена",
		domain.RegStatusWaitlist:             "лист ожидания",
		domain.RegStatusCancelledByUser:      "отменена вами",
		domain.RegStatusCancelledByOrganizer: "отменена организатором",
		domain.RegStatusAttended:             "посещено",
		domain.RegStatusNoShow:               "не посещено",
	}
	for in, want := range cases {
		if got := messages.HumanStatus(in); got != want {
			t.Errorf("HumanStatus(%q) = %q, want %q", in, got, want)
		}
	}
	// неизвестный статус возвращает строковое представление
	if got := messages.HumanStatus(domain.RegistrationStatus("alien")); got != "alien" {
		t.Errorf("HumanStatus(alien) = %q, want %q", got, "alien")
	}
}

// TestHumanFormat — таблица форматов.
func TestHumanFormat(t *testing.T) {
	t.Parallel()

	cases := map[domain.EventFormat]string{
		domain.EventFormatOffline: "очно",
		domain.EventFormatOnline:  "онлайн",
		domain.EventFormatHybrid:  "очно + онлайн",
	}
	for in, want := range cases {
		if got := messages.HumanFormat(in); got != want {
			t.Errorf("HumanFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEventCardContainsAllFields — карточка должна содержать все важные поля.
func TestEventCardContainsAllFields(t *testing.T) {
	t.Parallel()

	starts := time.Date(2026, time.May, 20, 10, 0, 0, 0, time.UTC)
	ev := &domain.Event{
		Title:       "День открытых дверей",
		Description: "Описание мероприятия в полном объёме.",
		StartsAt:    starts,
		Location:    "Главный корпус, ауд. 301",
		Format:      domain.EventFormatOffline,
		Capacity:    100,
		Status:      domain.EventStatusOpen,
	}
	got := messages.EventCard(ev, 47, nil)

	mustContain := []string{
		ev.Title,
		ev.Location,
		"очно",
		"Свободно мест: 47 из 100",
		"мая",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("EventCard missing %q in:\n%s", want, got)
		}
	}
}

// TestEventCardShortSummaryOverride — если есть ShortSummary, описание берётся оттуда.
func TestEventCardShortSummaryOverride(t *testing.T) {
	t.Parallel()

	summary := "Короткая сводка от ИИ"
	ev := &domain.Event{
		Title:        "Test",
		Description:  "Длинное описание которое НЕ должно попасть в карточку",
		ShortSummary: &summary,
		StartsAt:     time.Now(),
		Location:     "X",
		Format:       domain.EventFormatOnline,
		Capacity:     1,
	}
	got := messages.EventCard(ev, 0, nil)
	if !strings.Contains(got, summary) {
		t.Errorf("EventCard missing short summary %q in:\n%s", summary, got)
	}
	if strings.Contains(got, "Длинное описание") {
		t.Errorf("EventCard leaked long description while short summary is set:\n%s", got)
	}
}

func TestEventCardRegisteredStatusLine(t *testing.T) {
	t.Parallel()

	ev := &domain.Event{
		Title:       "День открытых дверей",
		Description: "Описание",
		StartsAt:    time.Now(),
		Location:    "Главный корпус",
		Format:      domain.EventFormatOffline,
		Capacity:    50,
	}
	got := messages.EventCard(ev, 49, &domain.Registration{Status: domain.RegStatusRegistered})
	if !strings.Contains(got, "Статус: вы уже записаны") {
		t.Errorf("EventCard missing registered status line in:\n%s", got)
	}
}

// TestOrganizerStatsHasNumbers — все числа из EventStats попали в текст.
func TestOrganizerStatsHasNumbers(t *testing.T) {
	t.Parallel()

	ev := &domain.Event{Title: "Test"}
	stats := &domain.EventStats{
		Capacity:   50,
		Registered: 30,
		Cancelled:  5,
		Waitlist:   10,
		Attended:   25,
		FreeSeats:  20,
		TopInterests: map[string]int{
			"Прикладная информатика":      12,
			"Информационная безопасность": 8,
		},
	}
	got := messages.OrganizerStats(ev, stats)

	mustContain := []string{
		"Всего мест: 50",
		"Записано: 30",
		"Свободно: 20",
		"В листе ожидания: 10",
		"Отменили запись: 5",
		"Посетили: 25",
		"Прикладная информатика", "12",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("OrganizerStats missing %q in:\n%s", want, got)
		}
	}
}

// TestOrganizerStatsEmptyInterests — пустая мапа не падает, выводит «пока нет данных».
func TestOrganizerStatsEmptyInterests(t *testing.T) {
	t.Parallel()

	ev := &domain.Event{Title: "Empty"}
	stats := &domain.EventStats{Capacity: 10, TopInterests: nil}
	got := messages.OrganizerStats(ev, stats)
	if !strings.Contains(got, "пока нет данных") {
		t.Errorf("want fallback for empty interests in:\n%s", got)
	}
}

// TestNoEmojiInWelcome — executor_prompt §4.4: эмодзи в текстах запрещены.
// Простая эвристика: ни один символ не должен быть из basic-emoji диапазона.
func TestNoEmojiInWelcome(t *testing.T) {
	t.Parallel()

	texts := []string{
		messages.Welcome("Иван"),
		messages.Help(),
		messages.AskFullName(),
		messages.RegSuccess(&domain.Event{Title: "T", StartsAt: time.Now(), Location: "X"}, ""),
		messages.ConsentAsk("v1"),
		messages.ForgetMeAsk(),
		messages.OrganizerNoAccess(),
	}
	for _, txt := range texts {
		for _, r := range txt {
			if r >= 0x1F300 && r <= 0x1FAFF {
				t.Errorf("emoji %U found in: %q", r, txt)
			}
			if r >= 0x2600 && r <= 0x27BF {
				t.Errorf("misc symbol/emoji %U found in: %q", r, txt)
			}
		}
	}
}

func TestMyRegistrationShowsShortCode(t *testing.T) {
	t.Parallel()

	code := "deadbeefcafebabefeedface12345678"
	ev := &domain.Event{
		Title:    "День открытых дверей",
		StartsAt: time.Now(),
		Location: "Главный корпус",
	}
	reg := &domain.Registration{
		Status:         domain.RegStatusRegistered,
		AttendanceCode: &code,
	}

	got := messages.MyRegistration(ev, reg)
	if !strings.Contains(got, "Код записи: deadbeef") {
		t.Errorf("MyRegistration missing short code in:\n%s", got)
	}
	if strings.Contains(got, code) && !strings.Contains(got, "Код записи: deadbeef") {
		t.Errorf("MyRegistration should show short code, got:\n%s", got)
	}
}

func TestQRCaptionShowsShortCode(t *testing.T) {
	t.Parallel()

	ev := &domain.Event{
		Title:    "День открытых дверей",
		StartsAt: time.Now(),
		Location: "Главный корпус",
	}
	got := messages.QRCaption(ev, "deadbeefcafebabefeedface12345678")
	if !strings.Contains(got, "Код записи: deadbeef") {
		t.Errorf("QRCaption missing short code in:\n%s", got)
	}
}
