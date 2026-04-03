-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT NOT NULL,                               -- user ID in the organizer system
    event_id    UUID NOT NULL REFERENCES events(id),
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user'
                    CHECK (role IN ('user', 'moderator', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Uniqueness: one user = one record per event (upsert on token exchange)
CREATE UNIQUE INDEX IF NOT EXISTS users_external_event_unique
    ON users (external_id, event_id);

-- +goose Down
DROP TABLE IF EXISTS users;
