-- +goose Up
-- +goose StatementBegin

-- Shared clips are viewed by unauthenticated guests, so a comment must be
-- able to record a guest-supplied display name instead of a user reference.
-- user_id was already nullable in 003; only the display_name column is new.
ALTER TABLE comments ADD COLUMN display_name VARCHAR(64);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE comments DROP COLUMN display_name;
-- +goose StatementEnd
