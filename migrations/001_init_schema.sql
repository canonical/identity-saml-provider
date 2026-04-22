-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    create_time TIMESTAMPTZ NOT NULL,
    expire_time TIMESTAMPTZ NOT NULL,
    index_val TEXT NOT NULL,
    name_id TEXT NOT NULL,
    user_email TEXT NOT NULL,
    user_common_name TEXT NOT NULL,
    groups TEXT[] DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sessions_expire_time ON sessions(expire_time);

CREATE TABLE IF NOT EXISTS service_providers (
    entity_id TEXT PRIMARY KEY,
    acs_url TEXT NOT NULL,
    acs_binding TEXT NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS service_providers;
DROP INDEX IF EXISTS idx_sessions_expire_time;
DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd

