-- +goose Up
-- +goose StatementBegin

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS user_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS raw_oidc_claims JSONB;

ALTER TABLE service_providers
    ADD COLUMN IF NOT EXISTS attribute_mapping JSONB;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE service_providers
    DROP COLUMN IF EXISTS attribute_mapping;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS raw_oidc_claims,
    DROP COLUMN IF EXISTS user_name;

-- +goose StatementEnd
