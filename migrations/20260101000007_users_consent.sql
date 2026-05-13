-- +goose Up
-- +goose StatementBegin
-- 152-ФЗ: явное согласие на обработку ПДн.
-- НЕТ согласия → запись на мероприятие запрещена (service.Registration вернёт ErrConsentRequired).
ALTER TABLE users
    ADD COLUMN consent_at         TIMESTAMPTZ,
    ADD COLUMN consent_policy_ver VARCHAR(16);

CREATE INDEX idx_users_consent ON users(consent_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN consent_at,
    DROP COLUMN consent_policy_ver;
-- +goose StatementEnd
