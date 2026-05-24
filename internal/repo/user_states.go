package repo

import (
	"context"
	"fmt"
	"time"
)

type userStatesRepo struct{}

// NewUserStates создаёт репозиторий user_states (persisted FSM).
func NewUserStates() UserStateRepo { return &userStatesRepo{} }

// defaultState — состояние по умолчанию, если для пользователя нет строки в user_states.
const defaultState = "main_menu"

// Load возвращает state и context по MAX user_id. Для нового пользователя — main_menu и пустой контекст.
func (r *userStatesRepo) Load(ctx context.Context, q Querier, userID int64) (string, []byte, error) {
	const stmt = `SELECT state, context FROM user_states WHERE max_user_id = $1`

	var state string
	var data []byte
	err := q.QueryRow(ctx, stmt, userID).Scan(&state, &data)
	if IsNoRows(err) {
		return defaultState, []byte("{}"), nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("load user state: %w", err)
	}
	if len(data) == 0 {
		data = []byte("{}")
	}
	return state, data, nil
}

// Save UPSERT строки user_states по MAX user_id.
func (r *userStatesRepo) Save(ctx context.Context, q Querier, userID int64, state string, contextJSON []byte) error {
	if state == "" {
		state = defaultState
	}
	if len(contextJSON) == 0 {
		contextJSON = []byte("{}")
	}
	const stmt = `
INSERT INTO user_states (max_user_id, state, context, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (max_user_id) DO UPDATE
SET state = EXCLUDED.state,
    context = EXCLUDED.context,
    updated_at = NOW()`

	_, err := q.Exec(ctx, stmt, userID, state, contextJSON)
	if err != nil {
		return fmt.Errorf("save user state: %w", err)
	}
	return nil
}

// Reset сбрасывает состояние пользователя в main_menu и пустой контекст.
func (r *userStatesRepo) Reset(ctx context.Context, q Querier, userID int64) error {
	return r.Save(ctx, q, userID, defaultState, []byte("{}"))
}

// PurgeStaleBefore удаляет state'ы, которые не обновлялись с указанного момента.
// Возвращает число удалённых строк.
func (r *userStatesRepo) PurgeStaleBefore(ctx context.Context, q Querier, before time.Time) (int, error) {
	const stmt = `DELETE FROM user_states WHERE updated_at < $1`

	tag, err := q.Exec(ctx, stmt, before)
	if err != nil {
		return 0, fmt.Errorf("purge user_states: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
