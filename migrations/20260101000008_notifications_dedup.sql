-- +goose Up
-- +goose StatementBegin
-- Защита от дублей при многократном вызове DispatchDue.
-- Уникальность по (user_id, event_id, type, минута scheduled_at):
-- два reminder_24h одному пользователю на одно событие в одну минуту невозможны.
--
-- ВНИМАНИЕ про IMMUTABLE (SQLSTATE 42P17):
--   date_trunc('minute', timestamptz)        — STABLE (зависит от timezone сессии)
--   EXTRACT(EPOCH FROM timestamptz)          — STABLE (та же причина)
--   EXTRACT(EPOCH FROM timestamp)            — IMMUTABLE
--   timestamptz AT TIME ZONE 'UTC' (literal) — IMMUTABLE → даёт timestamp без TZ
--
-- Поэтому делаем кастомную IMMUTABLE-функцию и используем её. Это самый
-- надёжный способ, не зависящий от внутренних меток volatility конкретных версий.
CREATE OR REPLACE FUNCTION notif_minute_bucket(ts timestamptz)
RETURNS bigint
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
    SELECT FLOOR(EXTRACT(EPOCH FROM (ts AT TIME ZONE 'UTC')) / 60)::bigint;
$$;

CREATE UNIQUE INDEX uniq_notif_dedup
    ON notifications (user_id, event_id, type, notif_minute_bucket(scheduled_at));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX uniq_notif_dedup;
DROP FUNCTION notif_minute_bucket(timestamptz);
-- +goose StatementEnd
