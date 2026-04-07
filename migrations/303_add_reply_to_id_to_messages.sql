-- +goose Up
ALTER TABLE messages
    ADD COLUMN reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL;

CREATE INDEX messages_reply_to_idx
    ON messages (chat_id, reply_to_id)
    WHERE reply_to_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS messages_reply_to_idx;
ALTER TABLE messages DROP COLUMN IF EXISTS reply_to_id;
