package domain

import (
	"time"

	"github.com/google/uuid"
)

// Message is a chat message. Seq is a monotonic counter per chat (not a global ID).
type Message struct {
	ID        uuid.UUID  `json:"id"`
	ChatID    uuid.UUID  `json:"chatId"`
	UserID    uuid.UUID  `json:"userId"`
	UserName  string     `json:"userName,omitempty"`
	Text      string     `json:"text"`
	Seq       int64      `json:"seq"`
	CreatedAt time.Time  `json:"createdAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"` // soft delete

	// Reply fields — populated via LEFT JOIN in the repository; not stored as separate columns.
	ReplyToID       *uuid.UUID `json:"replyToId,omitempty"`
	ReplyToSeq      *int64     `json:"replyToSeq,omitempty"`
	ReplyToText     string     `json:"replyToText,omitempty"`
	ReplyToUserName string     `json:"replyToUserName,omitempty"`
}
