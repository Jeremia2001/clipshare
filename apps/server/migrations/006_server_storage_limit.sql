-- +goose Up
-- +goose StatementBegin

-- Server-wide storage limit in bytes. 0 means unlimited (the admin hasn't set a cap).
ALTER TABLE instance_config ADD COLUMN storage_limit_bytes BIGINT NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE instance_config DROP COLUMN IF EXISTS storage_limit_bytes;
-- +goose StatementEnd
