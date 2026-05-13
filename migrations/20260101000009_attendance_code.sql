-- +goose Up
-- +goose StatementBegin
-- attendance_code = uuid v4 hex (128 бит энтропии). Используется в QR-приглашении
-- и проверяется на странице check-in (День 15).
-- qr_sent_message_id — id PNG-сообщения в MAX, чтобы кнопка «Показать мой QR»
-- могла переиспользовать его без перегенерации.
ALTER TABLE registrations
    ADD COLUMN attendance_code     CHAR(32) UNIQUE,
    ADD COLUMN checkin_at          TIMESTAMPTZ,
    ADD COLUMN checkin_by          BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN qr_sent_message_id  BIGINT;

CREATE INDEX idx_reg_checkin_at ON registrations(event_id, checkin_at)
    WHERE checkin_at IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE registrations
    DROP COLUMN attendance_code,
    DROP COLUMN checkin_at,
    DROP COLUMN checkin_by,
    DROP COLUMN qr_sent_message_id;
-- +goose StatementEnd
