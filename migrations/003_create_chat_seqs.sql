-- +goose Up
-- Sequence counters: one row per chat, created together with the chat
CREATE TABLE IF NOT EXISTS chat_seqs (
    chat_id  UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    last_seq BIGINT NOT NULL DEFAULT 0  -- monotonic counter per chat
);

-- +goose Down
DROP TABLE IF EXISTS chat_seqs;
