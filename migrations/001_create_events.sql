-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    settings       JSONB NOT NULL DEFAULT '{}',
    allowed_origin TEXT NOT NULL,          -- CORS: allowed domain of the host website
    api_secret     TEXT NOT NULL,          -- pre-shared secret for token exchange (store bcrypt hash)
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS events;
