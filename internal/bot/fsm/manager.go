package fsm

import (
	"context"
	"fmt"

	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Snapshot — текущее состояние пользователя + контекст.
// Возвращается из Load и передаётся в handler'ы.
type Snapshot struct {
	State   State
	Context UserFSMContext
}

// Manager — обёртка над UserStateRepo, которая работает с типизированным
// UserFSMContext вместо сырого jsonb.
type Manager struct {
	states repo.UserStateRepo
	q      repo.Querier
}

// NewManager создаёт менеджер. q — pool/querier для всех операций
// (управление транзакциями этому слою не нужно, FSM-операции единичные).
func NewManager(states repo.UserStateRepo, q repo.Querier) *Manager {
	return &Manager{states: states, q: q}
}

// Load возвращает Snapshot. Для пользователя без записи в БД — Snapshot{StateMainMenu, empty}.
func (m *Manager) Load(ctx context.Context, userID int64) (Snapshot, error) {
	state, raw, err := m.states.Load(ctx, m.q, userID)
	if err != nil {
		return Snapshot{State: StateMainMenu}, fmt.Errorf("fsm load: %w", err)
	}
	return Snapshot{State: state, Context: Unmarshal(raw)}, nil
}

// Save сохраняет состояние и контекст.
func (m *Manager) Save(ctx context.Context, userID int64, state State, c UserFSMContext) error {
	if state == "" {
		state = StateMainMenu
	}
	if err := m.states.Save(ctx, m.q, userID, state, c.Marshal()); err != nil {
		return fmt.Errorf("fsm save: %w", err)
	}
	return nil
}

// SaveState — сокращение, когда контекст не меняется.
func (m *Manager) SaveState(ctx context.Context, userID int64, snap Snapshot, newState State) error {
	return m.Save(ctx, userID, newState, snap.Context)
}

// Reset обнуляет state в main_menu и контекст в {}.
func (m *Manager) Reset(ctx context.Context, userID int64) error {
	if err := m.states.Reset(ctx, m.q, userID); err != nil {
		return fmt.Errorf("fsm reset: %w", err)
	}
	return nil
}
