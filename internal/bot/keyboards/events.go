package keyboards

import (
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
func EventList(events []*domain.Event, offset int, hasMore bool) *maxbot.Keyboard {
	kb := newKB()

	for _, e := range events {
		kb.AddRow().AddCallback(e.Title, schemes.DEFAULT, callbacks.EventShow(e.ID))
	}

	// Навигация — отдельным рядом.
	navRow := kb.AddRow()
	if offset >= pageSize {
		navRow.AddCallback("Назад", schemes.DEFAULT, callbacks.EventListPage(offset-pageSize))
	}
	if hasMore {
		navRow.AddCallback("Дальше", schemes.DEFAULT, callbacks.EventListPage(offset+pageSize))
	}

	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// PageSize возвращает размер страницы списка событий. Экспортируется для
// сервисов, которые должны отрезать корректный slice до передачи в EventList.
func PageSize() int { return pageSize }

// EventCard — клавиатура карточки мероприятия.
//
// freeSeats > 0   → «Записаться» (POSITIVE)
// freeSeats == 0  → «Встать в лист ожидания» (только если waitlistEnabled)
//
// Кнопка «Назад» всегда возвращает на ту же страницу списка, где находился
// пользователь до открытия карточки (offset передаётся вызывающим).
func EventCard(eventID int64, freeSeats int, waitlistEnabled bool, backOffset int) *maxbot.Keyboard {
	kb := newKB()
	if freeSeats > 0 {
		kb.AddRow().AddCallback("Записаться", schemes.POSITIVE, callbacks.RegStart(eventID))
	} else if waitlistEnabled {
		kb.AddRow().AddCallback("Встать в лист ожидания", schemes.DEFAULT, callbacks.WaitlistJoin(eventID))
	}
	kb.AddRow().AddCallback("Назад к списку", schemes.NEGATIVE, callbacks.EventListPage(backOffset))
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}
