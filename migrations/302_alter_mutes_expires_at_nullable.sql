-- +goose Up
-- Allow permanent mutes: nil expires_at means the mute never expires.
ALTER TABLE mutes ALTER COLUMN expires_at DROP NOT NULL;

-- Rebuild index to cover the NULL case used by GetActive / IsUserMuted queries.
DROP INDEX IF EXISTS mutes_chat_user_idx;
CREATE INDEX IF NOT EXISTS mutes_chat_user_idx
    ON mutes (chat_id, user_id)
    WHERE expires_at IS NULL OR expires_at > now();

-- +goose Down
-- Revert: expire any permanent mutes before re-adding NOT NULL.
UPDATE mutes SET expires_at = now() WHERE expires_at IS NULL;
ALTER TABLE mutes ALTER COLUMN expires_at SET NOT NULL;

DROP INDEX IF EXISTS mutes_chat_user_idx;
CREATE INDEX IF NOT EXISTS mutes_chat_user_idx
    ON mutes (chat_id, user_id, expires_at);
