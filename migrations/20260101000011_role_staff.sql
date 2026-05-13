-- Добавляем роль staff — волонтёр на входе для check-in.
-- Organizer создаёт мероприятия в web-админке, но НЕ сканирует QR-коды гостей.
-- Сканированием QR на входе занимаются отдельные люди с ролью staff (например,
-- студенты-волонтёры). Admin продолжает иметь полные права (организатор + staff).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users
    ADD CONSTRAINT users_role_chk
    CHECK (role IN ('applicant','organizer','staff','admin'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Перед откатом разжалуем staff в applicant, иначе восстановленный CHECK упадёт.
UPDATE users SET role = 'applicant' WHERE role = 'staff';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users
    ADD CONSTRAINT users_role_chk
    CHECK (role IN ('applicant','organizer','admin'));
-- +goose StatementEnd
