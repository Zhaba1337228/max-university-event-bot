-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_states DROP CONSTRAINT IF EXISTS user_states_user_id_fkey;

UPDATE user_states us
SET user_id = u.max_user_id
FROM users u
WHERE u.id = us.user_id;

ALTER TABLE user_states RENAME COLUMN user_id TO max_user_id;

ALTER TABLE user_states
    ADD CONSTRAINT user_states_max_user_id_fkey
    FOREIGN KEY (max_user_id) REFERENCES users(max_user_id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_states DROP CONSTRAINT IF EXISTS user_states_max_user_id_fkey;

UPDATE user_states us
SET max_user_id = u.id
FROM users u
WHERE u.max_user_id = us.max_user_id;

ALTER TABLE user_states RENAME COLUMN max_user_id TO user_id;

ALTER TABLE user_states
    ADD CONSTRAINT user_states_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
