-- +goose Up
-- +goose StatementBegin

CREATE TABLE pending_requests (
    request_id TEXT PRIMARY KEY,
    saml_request TEXT NOT NULL,
    relay_state TEXT,
    client_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expire_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_pending_requests_expire_at ON pending_requests(expire_at);

ALTER TABLE pending_requests SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_vacuum_threshold = 100
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS pending_requests;

-- +goose StatementEnd
