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
	svc := service.NewEvent(nil, er, &mockRegRepo{})

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
	svc := service.NewEvent(nil, er, &mockRegRepo{})

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
	svc := service.NewEvent(nil, er, rr)

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
	svc := service.NewEvent(nil, er, rr)

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
	svc := service.NewEvent(nil, er, rr)

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
	svc := service.NewEvent(nil, er, &mockRegRepo{})

	_, err := svc.Stats(context.Background(), 99)
	if !errors.Is(err, service.ErrEventNotFound) {
		t.Errorf("want ErrEventNotFound, got %v", err)
	}
}
