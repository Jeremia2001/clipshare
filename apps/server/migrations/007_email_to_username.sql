-- +goose Up
-- +goose StatementBegin

-- Self-hosted ClipShare never sends email, so the `email` column was always
-- just "the login handle". Promote `username` to be the single required
-- identifier and drop the email-shaped parts of the schema.

-- Widen username so it can hold handles people actually want to type.
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(64);

-- Backfill username from email for any existing rows (dev/admin) that
-- joined before this migration.
UPDATE users SET username = email WHERE username IS NULL OR username = '';

ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users DROP COLUMN email;

-- Email verification no longer applies.
ALTER TABLE users DROP COLUMN IF EXISTS is_verified;

-- Dead flag on instance_config — we don't verify anything anymore.
ALTER TABLE instance_config DROP COLUMN IF EXISTS require_email_verification;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE instance_config ADD COLUMN require_email_verification BOOLEAN DEFAULT true;
ALTER TABLE users ADD COLUMN is_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN email VARCHAR(255) UNIQUE;
UPDATE users SET email = username;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(32);
ALTER TABLE users ALTER COLUMN username DROP NOT NULL;
-- +goose StatementEnd
