package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventRepository manages event persistence.
type EventRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Event, error)
	Create(ctx context.Context, e *Event) error
	List(ctx context.Context) ([]*Event, error)
}

// ChatRepository manages chat persistence.
type ChatRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Chat, error)
	GetParentByEventID(ctx context.Context, eventID uuid.UUID) (*Chat, error)
	GetOrCreateChild(ctx context.Context, eventID uuid.UUID, externalRoomID string, parentID uuid.UUID) (*Chat, error)
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*Chat, error)
	GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error)
	UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error
}

// MessageRepository manages message persistence.
type MessageRepository interface {
	Create(ctx context.Context, m *Message) error
	GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*Message, error)
	ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*Message, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*Message, error)
	CountByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time) (int64, error)
}

// UserRepository manages user persistence.
type UserRepository interface {
	Upsert(ctx context.Context, u *User) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByExternalID(ctx context.Context, externalID string, eventID uuid.UUID) (*User, error)
	ListByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]*User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role UserRole) error
}

// BanRepository manages ban persistence.
type BanRepository interface {
	Create(ctx context.Context, b *Ban) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*Ban, error)
	IsUserBanned(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

// MuteRepository manages mute persistence.
type MuteRepository interface {
	Create(ctx context.Context, m *Mute) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*Mute, error)
	IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error)
}
