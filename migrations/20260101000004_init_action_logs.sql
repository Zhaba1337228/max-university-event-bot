-- +goose Up
-- +goose StatementBegin
CREATE TABLE action_logs (
    id               BIGSERIAL PRIMARY KEY,
    actor_user_id    BIGINT REFERENCES users(id)         ON DELETE SET NULL,
    target_user_id   BIGINT REFERENCES users(id)         ON DELETE SET NULL,
    event_id         BIGINT REFERENCES events(id)        ON DELETE SET NULL,
    registration_id  BIGINT REFERENCES registrations(id) ON DELETE SET NULL,
    action           VARCHAR(64)  NOT NULL,
    -- action: registration_created | registration_cancelled_by_user | ...
    -- (полный список — domain/action_log.go)
    payload          JSONB        NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_logs_event   ON action_logs(event_id,      created_at DESC);
CREATE INDEX idx_action_logs_actor   ON action_logs(actor_user_id, created_at DESC);
CREATE INDEX idx_action_logs_target  ON action_logs(target_user_id,created_at DESC);
CREATE INDEX idx_action_logs_action  ON action_logs(action,         created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE action_logs;
-- +goose StatementEnd
