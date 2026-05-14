package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// =============================================================================
// In-memory моки репозиториев. Сервисы тестируют бизнес-логику, поэтому
// удобнее простые функциональные struct'ы, чем pgxmock с реальным SQL.
// Querier параметр игнорируется — реальный SQL дёргать не нужно.
// =============================================================================

type mockEventRepo struct {
	getFunc      func(ctx context.Context, id int64) (*domain.Event, error)
	listOpenFunc func(ctx context.Context, limit, offset int) ([]*domain.Event, int, error)
	statsFunc    func(ctx context.Context, id int64) (*domain.EventStats, error)
	listOrgFunc  func(ctx context.Context, orgID int64) ([]*domain.Event, error)
	createFunc   func(ctx context.Context, e *domain.Event) (int64, error)
	updateFunc   func(ctx context.Context, e *domain.Event) error
	repo.EventRepo
}

func (m *mockEventRepo) Get(ctx context.Context, _ repo.Querier, id int64) (*domain.Event, error) {
	return m.getFunc(ctx, id)
}

func (m *mockEventRepo) ListOpen(ctx context.Context, _ repo.Querier, limit, offset int) ([]*domain.Event, int, error) {
	return m.listOpenFunc(ctx, limit, offset)
}

func (m *mockEventRepo) Stats(ctx context.Context, _ repo.Querier, id int64) (*domain.EventStats, error) {
	return m.statsFunc(ctx, id)
}

func (m *mockEventRepo) ListByOrganizer(ctx context.Context, _ repo.Querier, orgID int64) ([]*domain.Event, error) {
	return m.listOrgFunc(ctx, orgID)
}

func (m *mockEventRepo) Create(ctx context.Context, _ repo.Querier, e *domain.Event) (int64, error) {
	return m.createFunc(ctx, e)
}

func (m *mockEventRepo) Update(ctx context.Context, _ repo.Querier, e *domain.Event) error {
	return m.updateFunc(ctx, e)
}

type mockRegRepo struct {
	countFunc func(ctx context.Context, eventID int64, status domain.RegistrationStatus) (int, error)
	repo.RegistrationRepo
}

func (m *mockRegRepo) CountByEvent(ctx context.Context, _ repo.Querier, eventID int64, status domain.RegistrationStatus) (int, error) {
	if m.countFunc == nil {
		return 0, nil
	}
	return m.countFunc(ctx, eventID, status)
}

// =============================================================================
// Тесты
// =============================================================================

func TestGetOpenReturnsErrEventNotFound(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
		return nil, nil
	}}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)

	_, err := svc.GetOpen(context.Background(), 999)
	if !errors.Is(err, service.ErrEventNotFound) {
		t.Errorf("want ErrEventNotFound, got %v", err)
	}
}

func TestGetOpenReturnsErrEventClosed(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
		return &domain.Event{ID: 1, Status: domain.EventStatusClosed, Capacity: 10}, nil
	}}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)

	_, err := svc.GetOpen(context.Background(), 1)
	if !errors.Is(err, service.ErrEventClosed) {
		t.Errorf("want ErrEventClosed, got %v", err)
	}
}

func TestGetOpenComputesFreeSeats(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
		return &domain.Event{
			ID: 1, Status: domain.EventStatusOpen,
			Capacity: 100, StartsAt: time.Now().Add(48 * time.Hour),
		}, nil
	}}
	rr := &mockRegRepo{countFunc: func(_ context.Context, _ int64, status domain.RegistrationStatus) (int, error) {
		switch status {
		case domain.RegStatusRegistered:
			return 60, nil
		case domain.RegStatusAttended:
			return 0, nil
		default:
			return 0, nil
		}
	}}
	svc := service.NewEvent(nil, er, rr, nil)

	got, err := svc.GetOpen(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetOpen: %v", err)
	}
	if got.FreeSeats != 40 {
		t.Errorf("FreeSeats: want 40, got %d", got.FreeSeats)
	}
}

func TestGetOpenFreeSeatsNotNegative(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
		return &domain.Event{ID: 1, Status: domain.EventStatusOpen, Capacity: 10}, nil
	}}
	rr := &mockRegRepo{countFunc: func(_ context.Context, _ int64, status domain.RegistrationStatus) (int, error) {
		// перепроданность (registered + attended > capacity)
		if status == domain.RegStatusRegistered {
			return 8, nil
		}
		return 5, nil
	}}
	svc := service.NewEvent(nil, er, rr, nil)

	got, err := svc.GetOpen(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetOpen: %v", err)
	}
	if got.FreeSeats != 0 {
		t.Errorf("FreeSeats: want 0 (not negative), got %d", got.FreeSeats)
	}
}

func TestListOpenAddsFreeSeats(t *testing.T) {
	t.Parallel()
	events := []*domain.Event{
		{ID: 1, Capacity: 10},
		{ID: 2, Capacity: 20},
	}
	er := &mockEventRepo{listOpenFunc: func(_ context.Context, _ int, _ int) ([]*domain.Event, int, error) {
		return events, 2, nil
	}}
	rr := &mockRegRepo{countFunc: func(_ context.Context, eventID int64, status domain.RegistrationStatus) (int, error) {
		switch eventID {
		case 1:
			if status == domain.RegStatusRegistered {
				return 3, nil
			}
		case 2:
			if status == domain.RegStatusRegistered {
				return 20, nil // полная
			}
		}
		return 0, nil
	}}
	svc := service.NewEvent(nil, er, rr, nil)

	items, total, err := svc.ListOpen(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListOpen: %v", err)
	}
	if total != 2 {
		t.Errorf("total: want 2, got %d", total)
	}
	if items[0].FreeSeats != 7 {
		t.Errorf("event 1 free: want 7, got %d", items[0].FreeSeats)
	}
	if items[1].FreeSeats != 0 {
		t.Errorf("event 2 free: want 0 (full), got %d", items[1].FreeSeats)
	}
}

func TestStatsErrEventNotFound(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{statsFunc: func(_ context.Context, _ int64) (*domain.EventStats, error) {
		return nil, nil
	}}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)

	_, err := svc.Stats(context.Background(), 99)
	if !errors.Is(err, service.ErrEventNotFound) {
		t.Errorf("want ErrEventNotFound, got %v", err)
	}
}

// ---- Create / Update validation ----

func validInput() service.EventInput {
	endsAt := time.Now().Add(50 * time.Hour)
	return service.EventInput{
		Title:       "Тестовое мероприятие",
		Description: "Описание",
		StartsAt:    time.Now().Add(48 * time.Hour),
		EndsAt:      &endsAt,
		Location:    "Аудитория 301",
		Format:      domain.EventFormatOffline,
		Capacity:    50,
		Tags:        []string{"ит"},
	}
}

func TestCreate_OK(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{
		createFunc: func(_ context.Context, e *domain.Event) (int64, error) {
			e.ID = 42
			return 42, nil
		},
	}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)
	ev, err := svc.Create(context.Background(), 7, validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ev.ID != 42 {
		t.Errorf("ID: want 42, got %d", ev.ID)
	}
	if ev.CreatedBy == nil || *ev.CreatedBy != 7 {
		t.Errorf("CreatedBy: want 7, got %+v", ev.CreatedBy)
	}
	if ev.Status != domain.EventStatusOpen {
		t.Errorf("Status: want open, got %s", ev.Status)
	}
}

func TestCreate_EmptyTitle(t *testing.T) {
	t.Parallel()
	svc := service.NewEvent(nil, &mockEventRepo{}, &mockRegRepo{}, nil)
	in := validInput()
	in.Title = "   "
	_, err := svc.Create(context.Background(), 7, in)
	if !errors.Is(err, service.ErrEventInvalidTitle) {
		t.Errorf("want ErrEventInvalidTitle, got %v", err)
	}
}

func TestCreate_StartsInPast(t *testing.T) {
	t.Parallel()
	svc := service.NewEvent(nil, &mockEventRepo{}, &mockRegRepo{}, nil)
	in := validInput()
	in.StartsAt = time.Now().Add(-48 * time.Hour)
	_, err := svc.Create(context.Background(), 7, in)
	if !errors.Is(err, service.ErrEventInvalidDates) {
		t.Errorf("want ErrEventInvalidDates, got %v", err)
	}
}

func TestCreate_EndsBeforeStart(t *testing.T) {
	t.Parallel()
	svc := service.NewEvent(nil, &mockEventRepo{}, &mockRegRepo{}, nil)
	in := validInput()
	earlier := in.StartsAt.Add(-time.Hour)
	in.EndsAt = &earlier
	_, err := svc.Create(context.Background(), 7, in)
	if !errors.Is(err, service.ErrEventInvalidDates) {
		t.Errorf("want ErrEventInvalidDates, got %v", err)
	}
}

func TestCreate_ZeroCapacity(t *testing.T) {
	t.Parallel()
	svc := service.NewEvent(nil, &mockEventRepo{}, &mockRegRepo{}, nil)
	in := validInput()
	in.Capacity = 0
	_, err := svc.Create(context.Background(), 7, in)
	if !errors.Is(err, service.ErrEventInvalidCapacity) {
		t.Errorf("want ErrEventInvalidCapacity, got %v", err)
	}
}

func TestCreate_BadFormat(t *testing.T) {
	t.Parallel()
	svc := service.NewEvent(nil, &mockEventRepo{}, &mockRegRepo{}, nil)
	in := validInput()
	in.Format = domain.EventFormat("not-a-format")
	_, err := svc.Create(context.Background(), 7, in)
	if !errors.Is(err, service.ErrEventInvalidFormat) {
		t.Errorf("want ErrEventInvalidFormat, got %v", err)
	}
}

func TestUpdate_OK(t *testing.T) {
	t.Parallel()
	owner := int64(7)
	er := &mockEventRepo{
		getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
			return &domain.Event{ID: 1, CreatedBy: &owner, Status: domain.EventStatusOpen, Capacity: 10}, nil
		},
		updateFunc: func(_ context.Context, _ *domain.Event) error { return nil },
	}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)
	in := validInput()
	in.Title = "Новое название"
	in.Status = domain.EventStatusClosed
	ev, err := svc.Update(context.Background(), 7, 1, in)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if ev.Title != "Новое название" {
		t.Errorf("Title: want 'Новое название', got %q", ev.Title)
	}
	if ev.Status != domain.EventStatusClosed {
		t.Errorf("Status: want closed, got %s", ev.Status)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	er := &mockEventRepo{
		getFunc: func(_ context.Context, _ int64) (*domain.Event, error) { return nil, nil },
	}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)
	_, err := svc.Update(context.Background(), 7, 999, validInput())
	if !errors.Is(err, service.ErrEventNotFound) {
		t.Errorf("want ErrEventNotFound, got %v", err)
	}
}

func TestUpdate_InvalidStatus(t *testing.T) {
	t.Parallel()
	owner := int64(7)
	er := &mockEventRepo{
		getFunc: func(_ context.Context, _ int64) (*domain.Event, error) {
			return &domain.Event{ID: 1, CreatedBy: &owner, Status: domain.EventStatusOpen, Capacity: 10}, nil
		},
	}
	svc := service.NewEvent(nil, er, &mockRegRepo{}, nil)
	in := validInput()
	in.Status = domain.EventStatus("nonsense")
	_, err := svc.Update(context.Background(), 7, 1, in)
	if !errors.Is(err, service.ErrEventInvalidStatus) {
		t.Errorf("want ErrEventInvalidStatus, got %v", err)
	}
}
