-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS edited_at;
