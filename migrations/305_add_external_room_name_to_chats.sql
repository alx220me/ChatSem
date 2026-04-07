-- +goose Up
ALTER TABLE chats ADD COLUMN IF NOT EXISTS external_room_name TEXT;

-- +goose Down
ALTER TABLE chats DROP COLUMN IF EXISTS external_room_name;
