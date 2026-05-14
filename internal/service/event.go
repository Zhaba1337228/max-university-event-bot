package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// EventInput — данные для создания/редактирования мероприятия.
//
// Поля валидируются в Create/Update; пустой Title или Capacity <= 0 — отказ.
// EndsAt и Tags опциональны; Status разрешён только open|closed (cancelled и
// finished — внутренние, проставляются переходами).
type EventInput struct {
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      *time.Time
	Location    string
	Format      domain.EventFormat
	Capacity    int
	Status      domain.EventStatus // только для Update; в Create игнорируется (всегда open)
	Tags        []string
}

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

	// Create создаёт мероприятие от лица organizerID и пишет в action_log.
	// Валидация описана в errors.go (ErrEventInvalid*).
	Create(ctx context.Context, organizerID int64, in EventInput) (*domain.Event, error)

	// Update меняет поля мероприятия. Доступ — только владельцу (created_by
	// = organizerID) или admin'у (admin проверяет вызывающий handler).
	Update(ctx context.Context, organizerID int64, eventID int64, in EventInput) (*domain.Event, error)
}

type eventService struct {
	pool    repo.Querier
	events  repo.EventRepo
	regs    repo.RegistrationRepo
	actions repo.ActionLogRepo // для audit log create/update
}

// NewEvent создаёт сервис.
//
// actions опционален: если nil, операции create/update не пишут в audit log.
// Это удобно в unit-тестах, где audit log не критичен.
func NewEvent(pool repo.Querier, events repo.EventRepo, regs repo.RegistrationRepo, actions repo.ActionLogRepo) Event {
	return &eventService{pool: pool, events: events, regs: regs, actions: actions}
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

// Create реализует валидацию и insert в transactional querier.
func (s *eventService) Create(ctx context.Context, organizerID int64, in EventInput) (*domain.Event, error) {
	now := time.Now()
	ev, err := buildEventFromInput(in, now, true)
	if err != nil {
		return nil, err
	}
	ev.CreatedBy = &organizerID
	ev.Status = domain.EventStatusOpen

	if _, err := s.events.Create(ctx, s.pool, ev); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	s.logEventAction(ctx, organizerID, ev.ID, domain.ActionEventCreated, map[string]any{
		"title":     ev.Title,
		"starts_at": ev.StartsAt.UTC().Format(time.RFC3339),
		"capacity":  ev.Capacity,
		"format":    string(ev.Format),
	})
	return ev, nil
}

// Update перезаписывает поля события. status принимается только open|closed.
func (s *eventService) Update(ctx context.Context, organizerID int64, eventID int64, in EventInput) (*domain.Event, error) {
	existing, err := s.events.Get(ctx, s.pool, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if existing == nil {
		return nil, ErrEventNotFound
	}

	now := time.Now()
	up, err := buildEventFromInput(in, now, false)
	if err != nil {
		return nil, err
	}
	existing.Title = up.Title
	existing.Description = up.Description
	existing.StartsAt = up.StartsAt
	existing.EndsAt = up.EndsAt
	existing.Location = up.Location
	existing.Format = up.Format
	existing.Capacity = up.Capacity
	existing.Tags = up.Tags
	if up.Status != "" {
		existing.Status = up.Status
	}

	if err := s.events.Update(ctx, s.pool, existing); err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}
	s.logEventAction(ctx, organizerID, existing.ID, domain.ActionEventUpdated, map[string]any{
		"title":     existing.Title,
		"starts_at": existing.StartsAt.UTC().Format(time.RFC3339),
		"capacity":  existing.Capacity,
		"status":    string(existing.Status),
	})
	return existing, nil
}

// buildEventFromInput валидирует входные данные и собирает доменный Event
// без CreatedBy/ID/CreatedAt (это поля заполняет вызывающий код / БД).
func buildEventFromInput(in EventInput, now time.Time, isCreate bool) (*domain.Event, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || len(title) > 255 {
		return nil, ErrEventInvalidTitle
	}
	descr := strings.TrimSpace(in.Description)
	if len(descr) > 16000 {
		return nil, ErrEventInvalidDescription
	}
	if in.StartsAt.IsZero() {
		return nil, ErrEventInvalidDates
	}
	if isCreate && in.StartsAt.Before(now.Add(-time.Hour)) {
		// при создании дата старта не должна быть глубоко в прошлом
		return nil, ErrEventInvalidDates
	}
	if in.EndsAt != nil && !in.EndsAt.After(in.StartsAt) {
		return nil, ErrEventInvalidDates
	}
	if in.Capacity <= 0 || in.Capacity > 100000 {
		return nil, ErrEventInvalidCapacity
	}
	format := in.Format
	if format == "" {
		format = domain.EventFormatOffline
	}
	switch format {
	case domain.EventFormatOffline, domain.EventFormatOnline, domain.EventFormatHybrid:
	default:
		return nil, ErrEventInvalidFormat
	}
	status := in.Status
	if status != "" && status != domain.EventStatusOpen && status != domain.EventStatusClosed {
		return nil, ErrEventInvalidStatus
	}
	tags := normalizeTags(in.Tags)
	if len(tags) > 20 {
		return nil, ErrEventTooManyTags
	}
	for _, t := range tags {
		if len(t) > 50 {
			return nil, ErrEventTooManyTags
		}
	}
	location := strings.TrimSpace(in.Location)
	if len(location) > 512 {
		location = location[:512]
	}
	return &domain.Event{
		Title:       title,
		Description: descr,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		Location:    location,
		Format:      format,
		Capacity:    in.Capacity,
		Status:      status,
		Tags:        tags,
	}, nil
}

func normalizeTags(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		t = strings.ToLower(t)
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func (s *eventService) logEventAction(ctx context.Context, actorID, eventID int64, action domain.ActionType, payload map[string]any) {
	if s.actions == nil {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{}`)
	}
	eid := eventID
	aid := actorID
	log := &domain.ActionLog{
		ActorUserID: &aid,
		EventID:     &eid,
		Action:      action,
		Payload:     raw,
	}
	_ = s.actions.Append(ctx, s.pool, log)
}
