-- +goose Up
-- +goose StatementBegin
-- Защита от дублей при многократном вызове DispatchDue.
-- Уникальность по (user_id, event_id, type, минута scheduled_at):
-- два reminder_24h одному пользователю на одно событие в один час невозможны.
CREATE UNIQUE INDEX uniq_notif_dedup
    ON notifications (user_id, event_id, type, date_trunc('minute', scheduled_at));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX uniq_notif_dedup;
-- +goose StatementEnd
