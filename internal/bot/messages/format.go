// Package messages содержит ВСЕ пользовательские тексты бота на русском.
//
// Принцип: один файл — один язык. Меняем формулировки в одном месте,
// не разбираясь по handler'ам. Эмодзи запрещены (см. executor_prompt.md §4.4).
package messages

import (
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// Названия месяцев в родительном падеже — для красивого формата
// «13 мая 2026, 18:00» вместо «May 13».
var monthNamesRU = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// FormatDateTime возвращает «02 января 2006, 15:04» в Europe/Moscow.
// Если переданное время уже в нужной зоне (например, из БД с TIMESTAMPTZ),
// конверсия безопасна и идемпотентна.
func FormatDateTime(t time.Time) string {
	t = inMSK(t)
	return formatDay(t) + " " + monthNamesRU[t.Month()-1] + " " +
		itoa(t.Year()) + ", " + formatHM(t)
}

// FormatDate возвращает «02 января 2006» без времени.
func FormatDate(t time.Time) string {
	t = inMSK(t)
	return formatDay(t) + " " + monthNamesRU[t.Month()-1] + " " + itoa(t.Year())
}

// FormatTime возвращает «15:04».
func FormatTime(t time.Time) string { return formatHM(inMSK(t)) }

// HumanStatus переводит RegistrationStatus в строку для пользователя.
func HumanStatus(s domain.RegistrationStatus) string {
	switch s {
	case domain.RegStatusRegistered:
		return "запись подтверждена"
	case domain.RegStatusWaitlist:
		return "лист ожидания"
	case domain.RegStatusCancelledByUser:
		return "отменена вами"
	case domain.RegStatusCancelledByOrganizer:
		return "отменена организатором"
	case domain.RegStatusAttended:
		return "посещено"
	case domain.RegStatusNoShow:
		return "не посещено"
	}
	return string(s)
}

// HumanFormat переводит формат мероприятия на русский.
func HumanFormat(f domain.EventFormat) string {
	switch f {
	case domain.EventFormatOffline:
		return "очно"
	case domain.EventFormatOnline:
		return "онлайн"
	case domain.EventFormatHybrid:
		return "очно + онлайн"
	}
	return string(f)
}

// HumanEventStatus переводит статус мероприятия на русский (для админки/организатора).
func HumanEventStatus(s domain.EventStatus) string {
	switch s {
	case domain.EventStatusOpen:
		return "открыта запись"
	case domain.EventStatusClosed:
		return "регистрация закрыта"
	case domain.EventStatusCancelled:
		return "отменено"
	case domain.EventStatusFinished:
		return "завершено"
	}
	return string(s)
}

// --- внутреннее форматирование (без выделения в отдельный пакет — на 5 функций) ---

var moscow = mustLoadLocation("Europe/Moscow")

func inMSK(t time.Time) time.Time {
	if moscow == nil {
		return t.UTC()
	}
	return t.In(moscow)
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// На windows-host без tzdata fallback на UTC. План на проде —
		// alpine + tzdata в Dockerfile (см. раздел 21.1).
		return nil
	}
	return loc
}

func formatDay(t time.Time) string {
	d := t.Day()
	if d < 10 {
		return "0" + itoa(d)
	}
	return itoa(d)
}

func formatHM(t time.Time) string {
	h, m := t.Hour(), t.Minute()
	return pad2(h) + ":" + pad2(m)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// itoa — strconv.Itoa, локальный обёртке-импорт чтобы format.go был самодостаточен.
func itoa(n int) string {
	// маленький, без зависимости от strconv для случая отрицательных не нужен.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
