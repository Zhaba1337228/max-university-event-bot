-- +goose Up
-- +goose StatementBegin
-- Защита от дублей при многократном вызове DispatchDue.
-- Уникальность по (user_id, event_id, type, минута scheduled_at):
-- два reminder_24h одному пользователю на одно событие в одну минуту невозможны.
--
-- ВНИМАНИЕ: date_trunc('minute', timestamptz) НЕ IMMUTABLE — PostgreSQL ругается
-- "functions in index expression must be marked IMMUTABLE" (SQLSTATE 42P17).
-- Используем чистую integer-арифметику над EXTRACT(EPOCH FROM ...) — она immutable,
-- даёт ту же гарантию (минутный bucket) и не зависит от session timezone.
CREATE UNIQUE INDEX uniq_notif_dedup
    ON notifications (user_id, event_id, type,
                      ((EXTRACT(EPOCH FROM scheduled_at)::bigint) / 60));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX uniq_notif_dedup;
-- +goose StatementEnd
