-- +goose Up
CREATE TABLE IF NOT EXISTS bans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    event_id   UUID NOT NULL REFERENCES events(id),
    chat_id    UUID REFERENCES chats(id),                    -- NULL = ban for entire event
    banned_by  UUID NOT NULL REFERENCES users(id),           -- moderator/admin who issued the ban
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ                                    -- NULL = permanent ban
);

-- For ban-check queries: "is user X banned in event Y?"
-- Note: partial index with now() is evaluated at creation time only;
-- use btree on (user_id, event_id, expires_at) and filter in SQL for production correctness.
CREATE INDEX IF NOT EXISTS bans_user_event_idx
    ON bans (user_id, event_id, expires_at);

-- +goose Down
DROP TABLE IF EXISTS bans;
