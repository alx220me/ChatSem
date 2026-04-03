-- +goose Up
CREATE TABLE IF NOT EXISTS mutes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    muted_by   UUID NOT NULL REFERENCES users(id),           -- moderator/admin who issued the mute
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL                          -- mutes are always temporary
);

-- For mute-check queries: "is user X muted in chat Y?"
CREATE INDEX IF NOT EXISTS mutes_chat_user_idx
    ON mutes (chat_id, user_id, expires_at);

-- +goose Down
DROP TABLE IF EXISTS mutes;
