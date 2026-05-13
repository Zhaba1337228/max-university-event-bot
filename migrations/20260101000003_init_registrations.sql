-- +goose Up
-- +goose StatementBegin
CREATE TABLE registrations (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT       NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    event_id            BIGINT       NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    status              VARCHAR(32)  NOT NULL,
    -- status: registered | waitlist | cancelled_by_user | cancelled_by_organizer | attended | no_show
    interest_program    VARCHAR(255),
    full_name_snapshot  VARCHAR(255) NOT NULL,
    contact_snapshot    VARCHAR(255) NOT NULL,
    waitlist_position   INTEGER,
    registered_at       TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    source              VARCHAR(32)  NOT NULL DEFAULT 'bot',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- На одну пару (user, event) ровно одна запись:
    -- при отмене обновляем существующую строку, не создаём новую.
    CONSTRAINT registrations_user_event_uk UNIQUE (user_id, event_id),
    CONSTRAINT registrations_status_chk CHECK (status IN (
        'registered','waitlist','cancelled_by_user','cancelled_by_organizer','attended','no_show'
    ))
);

CREATE INDEX idx_reg_event_status ON registrations(event_id, status);
CREATE INDEX idx_reg_user_status  ON registrations(user_id, status);
CREATE INDEX idx_reg_waitlist     ON registrations(event_id, waitlist_position)
    WHERE status = 'waitlist';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE registrations;
-- +goose StatementEnd
