-- Migration 12: per-registration notification opt-out + per-event late-cancel policy
-- TZ §6: пользователь может отключить уведомления по конкретному мероприятию
-- TZ §5: поведение при отмене после начала мероприятия фиксируется в правилах

-- +goose Up
ALTER TABLE registrations
    ADD COLUMN IF NOT EXISTS notifications_disabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS late_cancel_allowed BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE events
    DROP COLUMN IF EXISTS late_cancel_allowed;

ALTER TABLE registrations
    DROP COLUMN IF EXISTS notifications_disabled;
