-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    max_user_id  BIGINT       NOT NULL UNIQUE,
    full_name    VARCHAR(255),
    phone        VARCHAR(64),
    email        VARCHAR(255),
    role         VARCHAR(32)  NOT NULL DEFAULT 'applicant',
    -- роль: applicant | organizer | admin
    locale       VARCHAR(8)   NOT NULL DEFAULT 'ru',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT users_role_chk CHECK (role IN ('applicant','organizer','admin'))
);

CREATE INDEX idx_users_role ON users(role);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
