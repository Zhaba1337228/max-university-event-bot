-- +goose Up
-- +goose StatementBegin
CREATE TABLE events (
    id              BIGSERIAL PRIMARY KEY,
    title           VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    short_summary   TEXT,                    -- опциональная AI-сгенерированная короткая версия
    starts_at       TIMESTAMPTZ  NOT NULL,
    ends_at         TIMESTAMPTZ,
    location        VARCHAR(512) NOT NULL DEFAULT '',
    format          VARCHAR(32)  NOT NULL DEFAULT 'offline',
    -- format: offline | online | hybrid
    capacity        INTEGER      NOT NULL CHECK (capacity > 0),
    status          VARCHAR(32)  NOT NULL DEFAULT 'open',
    -- status: open | closed | cancelled | finished
    created_by      BIGINT       REFERENCES users(id) ON DELETE SET NULL,
    tags            TEXT[]       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT events_format_chk CHECK (format IN ('offline','online','hybrid')),
    CONSTRAINT events_status_chk CHECK (status IN ('open','closed','cancelled','finished'))
);

CREATE INDEX idx_events_status_starts ON events(status, starts_at);
CREATE INDEX idx_events_created_by    ON events(created_by);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE events;
-- +goose StatementEnd
