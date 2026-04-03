-- +goose Up
CREATE TABLE IF NOT EXISTS chats (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id         UUID NOT NULL REFERENCES events(id),
    parent_id        UUID REFERENCES chats(id),           -- NULL for parent chat
    external_room_id TEXT,                                 -- room ID from organizer system (NULL for parent)
    type             TEXT NOT NULL CHECK (type IN ('parent', 'child')),
    settings         JSONB NOT NULL DEFAULT '{}',          -- only for parent; child reads via JOIN
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chats_type_parent_check CHECK (
        (type = 'parent' AND parent_id IS NULL AND external_room_id IS NULL) OR
        (type = 'child'  AND parent_id IS NOT NULL)
    )
);

-- Unique index for lazy child-chat creation (ON CONFLICT DO NOTHING)
CREATE UNIQUE INDEX IF NOT EXISTS chats_event_room_unique
    ON chats (event_id, external_room_id)
    WHERE type = 'child';

-- For finding parent chat by event_id
CREATE INDEX IF NOT EXISTS chats_event_type_idx ON chats (event_id, type);

-- +goose Down
DROP TABLE IF EXISTS chats;
