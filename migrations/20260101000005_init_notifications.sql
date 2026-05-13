-- +goose Up
-- +goose StatementBegin
CREATE TABLE notifications (
    id            BIGSERIAL PRIMARY KEY,
    event_id      BIGINT       NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id       BIGINT       NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    type          VARCHAR(32)  NOT NULL,
    -- type: reminder_24h | reminder_1h | organizer_broadcast | waitlist_promoted | event_cancelled
    text          TEXT         NOT NULL,
    status        VARCHAR(32)  NOT NULL DEFAULT 'pending',
    -- status: pending | sent | failed | skipped
    scheduled_at  TIMESTAMPTZ  NOT NULL,
    sent_at       TIMESTAMPTZ,
    error         TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT notif_type_chk CHECK (type IN (
        'reminder_24h','reminder_1h','organizer_broadcast','waitlist_promoted','event_cancelled'
    )),
    CONSTRAINT notif_status_chk CHECK (status IN ('pending','sent','failed','skipped'))
);

CREATE INDEX idx_notif_status_scheduled ON notifications(status, scheduled_at);
CREATE INDEX idx_notif_event_user       ON notifications(event_id, user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notifications;
-- +goose StatementEnd
