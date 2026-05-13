-- +goose Up
-- +goose StatementBegin
-- Persisted FSM state. См. internal/bot/fsm/.
-- Очистка устаревших состояний раз в сутки — scheduler.purgeStaleStates (День 16).
CREATE TABLE user_states (
    user_id     BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    state       VARCHAR(64)  NOT NULL DEFAULT 'main_menu',
    context     JSONB        NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_states_updated_at ON user_states(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_states;
-- +goose StatementEnd
