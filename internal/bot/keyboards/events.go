package keyboards

import (
	"fmt"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// pageSize — сколько событий показывать на одной странице списка.
// MAX-лимит 30 рядов на клавиатуру; берём 8 с запасом под пагинацию.
const pageSize = 8

// EventList — клавиатура для постраничного списка мероприятий.
// events — срез, уже отрезанный по странице (т.е. длиной ≤ pageSize).
// offset — текущий offset (нужен для расчёта next/prev).
// hasMore — есть ли страница дальше.
// activeFilter — активный фильтр формата ("offline"/"online"/"hybrid"/"" = все).
func EventList(events []*domain.Event, offset int, hasMore bool, activeFilter string) *maxbot.Keyboard {
	kb := newKB()

	for _, e := range events {
		kb.AddRow().AddCallback(e.Title, schemes.DEFAULT, callbacks.EventShow(e.ID))
	}

	// Навигация — отдельным рядом с номером страницы посередине.
	hasPrev := offset >= pageSize
	if hasPrev || hasMore {
		navRow := kb.AddRow()
		if hasPrev {
			navRow.AddCallback("◀ Назад", schemes.DEFAULT, callbacks.EventListPage(offset-pageSize))
		}
		page := offset/pageSize + 1
		navRow.AddCallback(fmt.Sprintf("· %d ·", page), schemes.DEFAULT, callbacks.EventListPage(offset))
		if hasMore {
			navRow.AddCallback("Вперёд ▶", schemes.DEFAULT, callbacks.EventListPage(offset+pageSize))
		}
	}

	// Фильтры — одна кнопка, открывает отдельный экран.
	filterLabel := "Фильтры"
	filterStyle := schemes.DEFAULT
	if activeFilter != "" {
		filterLabel = "Фильтры: " + humanFilterLabel(activeFilter)
		filterStyle = schemes.POSITIVE
	}
	kb.AddRow().AddCallback(filterLabel, filterStyle, callbacks.EventFiltersOpen())

	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// EventFilterMenu — экран выбора фильтров (формат, время, места, тема).
// Активная опция помечается «✓ » префиксом и стилем POSITIVE.
func EventFilterMenu(formatFilter, timeFilter string, seatsOnly bool, tagFilter string) *maxbot.Keyboard {
	kb := newKB()

	// --- Формат ---
	fmtRow := kb.AddRow()
	for _, f := range []struct{ label, val string }{
		{"Все форматы", ""},
		{"Очно", "offline"},
		{"Онлайн", "online"},
		{"Гибрид", "hybrid"},
	} {
		label, style := filterBtn(f.label, formatFilter == f.val)
		fmtRow.AddCallback(label, style, callbacks.EventFilterSet(f.val))
	}

	// --- Когда ---
	timeRow := kb.AddRow()
	for _, f := range []struct{ label, val string }{
		{"Любое время", ""},
		{"Сегодня", "today"},
		{"На неделю", "week"},
	} {
		label, style := filterBtn(f.label, timeFilter == f.val)
		timeRow.AddCallback(label, style, callbacks.EventFilterTime(f.val))
	}

	// --- Места ---
	seatsRow := kb.AddRow()
	lAll, sAll := filterBtn("Любое кол-во", !seatsOnly)
	lFree, sFree := filterBtn("Только свободные", seatsOnly)
	seatsRow.AddCallback(lAll, sAll, callbacks.EventFilterSeats(""))
	seatsRow.AddCallback(lFree, sFree, callbacks.EventFilterSeats("1"))

	// --- Тема ---
	tagRow := kb.AddRow()
	for _, f := range []struct{ label, val string }{
		{"Все темы", ""},
		{"IT", "it"},
		{"Карьера", "карьера"},
		{"Хакатон", "хакатон"},
	} {
		label, style := filterBtn(f.label, tagFilter == f.val)
		tagRow.AddCallback(label, style, callbacks.EventFilterTag(f.val))
	}
	extraTagRow := kb.AddRow()
	for _, f := range []struct{ label, val string }{
		{"Поступление", "поступление"},
		{"Олимпиада", "олимпиада"},
		{"DevOps", "devops"},
	} {
		label, style := filterBtn(f.label, tagFilter == f.val)
		extraTagRow.AddCallback(label, style, callbacks.EventFilterTag(f.val))
	}

	kb.AddRow().AddCallback("Сбросить все фильтры", schemes.DEFAULT, callbacks.EventFilterReset())
	kb.AddRow().AddCallback("Назад к списку", schemes.NEGATIVE, callbacks.EventListPage(0))
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// filterBtn возвращает метку с «✓ » и стиль POSITIVE для активной опции.
func filterBtn(label string, active bool) (string, schemes.Intent) {
	if active {
		return "✓ " + label, schemes.POSITIVE
	}
	return label, schemes.DEFAULT
}

func humanFilterLabel(f string) string {
	switch f {
	case "offline":
		return "Очно"
	case "online":
		return "Онлайн"
	case "hybrid":
		return "Гибрид"
	default:
		return f
	}
}

// PageSize возвращает размер страницы списка событий. Экспортируется для
// сервисов, которые должны отрезать корректный slice до передачи в EventList.
func PageSize() int { return pageSize }

// EventDetailsBack — клавиатура страницы «Подробнее»: назад к краткой карточке.
func EventDetailsBack(eventID int64, backOffset int) *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("Назад к карточке", schemes.DEFAULT, callbacks.EventShow(eventID))
	kb.AddRow().AddCallback("Назад к списку", schemes.NEGATIVE, callbacks.EventListPage(backOffset))
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// EventCard — клавиатура карточки мероприятия.
//
// freeSeats > 0   → «Записаться» (POSITIVE)
// freeSeats == 0  → «Встать в лист ожидания» (только если waitlistEnabled)
//
// Кнопка «Назад» всегда возвращает на ту же страницу списка, где находился
// пользователь до открытия карточки (offset передаётся вызывающим).
func EventCard(eventID int64, freeSeats int, waitlistEnabled bool, backOffset int, activeReg *domain.Registration) *maxbot.Keyboard {
	kb := newKB()
	if activeReg != nil {
		label := "Моя запись"
		if activeReg.Status == domain.RegStatusWaitlist {
			label = "Моя запись (лист ожидания)"
		}
		kb.AddRow().
			AddCallback(label, schemes.POSITIVE, callbacks.MyShow()).
			AddCallback("Подробнее", schemes.DEFAULT, callbacks.EventDetails(eventID))
	} else if freeSeats > 0 {
		kb.AddRow().
			AddCallback("Записаться", schemes.POSITIVE, callbacks.RegStart(eventID)).
			AddCallback("Подробнее", schemes.DEFAULT, callbacks.EventDetails(eventID))
	} else {
		kb.AddRow().AddCallback("Подробнее", schemes.DEFAULT, callbacks.EventDetails(eventID))
		if waitlistEnabled {
			kb.AddRow().AddCallback("Встать в лист ожидания", schemes.DEFAULT, callbacks.WaitlistJoin(eventID))
		}
	}
	kb.AddRow().AddCallback("Назад к списку", schemes.NEGATIVE, callbacks.EventListPage(backOffset))
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}
