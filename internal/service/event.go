package service

import (
	"context"
	"fmt"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// EventWithFree — событие плюс число свободных мест.
// Удобно для рендера списка и карточки — handler не делает SQL сам.
type EventWithFree struct {
	Event     *domain.Event
	FreeSeats int
}

// Event — публичный интерфейс сервиса событий.
//
// Сервис тонкий: оборачивает репозитории + считает free seats. Бизнес-правила
// (capacity, waitlist, статусы) принимаются в этих методах декларативно.
type Event interface {
	// ListOpen возвращает открытые мероприятия (status=open),
	// отсортированные по starts_at; total — общее число для пагинации.
	ListOpen(ctx context.Context, limit, offset int) (items []EventWithFree, total int, err error)

	// GetOpen возвращает событие по id ВМЕСТЕ с количеством свободных мест.
	// Возвращает ErrEventNotFound если события нет, ErrEventClosed если status != open.
	GetOpen(ctx context.Context, id int64) (*EventWithFree, error)

	// Get — без бизнес-проверок (используется организатором).
	Get(ctx context.Context, id int64) (*domain.Event, error)

	// Stats — управленческая сводка по событию.
	Stats(ctx context.Context, eventID int64) (*domain.EventStats, error)

	// ListByOrganizer — события, созданные данным organizer'ом.
	ListByOrganizer(ctx context.Context, organizerID int64) ([]*domain.Event, error)
}

type eventService struct {
	pool   repo.Querier
	events repo.EventRepo
	regs   repo.RegistrationRepo
}

// NewEvent создаёт сервис.
func NewEvent(pool repo.Querier, events repo.EventRepo, regs repo.RegistrationRepo) Event {
	return &eventService{pool: pool, events: events, regs: regs}
}

func (s *eventService) ListOpen(ctx context.Context, limit, offset int) ([]EventWithFree, int, error) {
	events, total, err := s.events.ListOpen(ctx, s.pool, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list open events: %w", err)
	}
	out := make([]EventWithFree, 0, len(events))
	for _, e := range events {
		free, err := s.freeSeats(ctx, e)
		if err != nil {
			return nil, 0, fmt.Errorf("free seats for event %d: %w", e.ID, err)
		}
		out = append(out, EventWithFree{Event: e, FreeSeats: free})
	}
	return out, total, nil
}

func (s *eventService) GetOpen(ctx context.Context, id int64) (*EventWithFree, error) {
	e, err := s.events.Get(ctx, s.pool, id)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if e == nil {
		return nil, ErrEventNotFound
	}
	if !e.IsOpenForRegistration() {
		return nil, ErrEventClosed
	}
	free, err := s.freeSeats(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("free seats: %w", err)
	}
	return &EventWithFree{Event: e, FreeSeats: free}, nil
}

func (s *eventService) Get(ctx context.Context, id int64) (*domain.Event, error) {
	e, err := s.events.Get(ctx, s.pool, id)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if e == nil {
		return nil, ErrEventNotFound
	}
	return e, nil
}

func (s *eventService) Stats(ctx context.Context, eventID int64) (*domain.EventStats, error) {
	stats, err := s.events.Stats(ctx, s.pool, eventID)
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	if stats == nil {
		return nil, ErrEventNotFound
	}
	return stats, nil
}

func (s *eventService) ListByOrganizer(ctx context.Context, organizerID int64) ([]*domain.Event, error) {
	out, err := s.events.ListByOrganizer(ctx, s.pool, organizerID)
	if err != nil {
		return nil, fmt.Errorf("list by organizer: %w", err)
	}
	return out, nil
}

// freeSeats — реальный остаток мест = capacity - count(registered+attended).
// attended считаем «занявшим место» — он уже был на мероприятии.
func (s *eventService) freeSeats(ctx context.Context, e *domain.Event) (int, error) {
	registered, err := s.regs.CountByEvent(ctx, s.pool, e.ID, domain.RegStatusRegistered)
	if err != nil {
		return 0, err
	}
	attended, err := s.regs.CountByEvent(ctx, s.pool, e.ID, domain.RegStatusAttended)
	if err != nil {
		return 0, err
	}
	free := e.Capacity - registered - attended
	if free < 0 {
		return 0, nil
	}
	return free, nil
}
