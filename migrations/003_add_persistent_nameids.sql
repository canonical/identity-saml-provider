-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS persistent_nameids (
    entity_id     TEXT        NOT NULL
        REFERENCES service_providers (entity_id) ON DELETE CASCADE,
    user_subject  TEXT        NOT NULL,
    persistent_id TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entity_id, user_subject)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS persistent_nameids;

-- +goose StatementEnd
