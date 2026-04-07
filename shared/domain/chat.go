package domain

import (
	"time"

	"github.com/google/uuid"
)

// ChatType distinguishes parent chats (per event) from child chats (per room/hall).
type ChatType string

const (
	TypeParent ChatType = "parent"
	TypeChild  ChatType = "child"
)

// Chat represents a chat room. Parent chats own settings; child chats inherit via SQL JOIN.
type Chat struct {
	ID               uuid.UUID  `json:"id"`
	EventID          uuid.UUID  `json:"eventId"`
	ParentID         *uuid.UUID `json:"parentId,omitempty"` // nil for parent chats
	ExternalRoomID   string     `json:"externalRoomId,omitempty"`
	ExternalRoomName string     `json:"externalRoomName,omitempty"` // human-readable room name set by organizer
	Type             ChatType   `json:"type"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// ChatSettings stores configuration for a parent chat. Child chats read settings via JOIN.
type ChatSettings struct {
	ChatID   uuid.UUID `json:"chatId"`
	Settings []byte    `json:"settings"` // JSONB blob
}
