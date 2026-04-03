-- +goose Up
CREATE TABLE IF NOT EXISTS messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    text       TEXT NOT NULL,
    seq        BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,                                  -- soft delete
    UNIQUE (chat_id, seq)                                    -- seq is unique within a chat
);

-- Primary long-poll pattern: WHERE chat_id=$1 AND seq > $2 ORDER BY seq ASC
CREATE INDEX IF NOT EXISTS messages_chat_seq_idx
    ON messages (chat_id, seq ASC);

-- History export pattern: WHERE chat_id=$1 AND created_at BETWEEN $2 AND $3
CREATE INDEX IF NOT EXISTS messages_chat_created_at_idx
    ON messages (chat_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS messages;
