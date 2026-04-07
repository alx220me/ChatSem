package ports

import (
	"context"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// EventRepository is the minimal event store interface needed by the chat service.
type EventRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
}

// ChatRepository is the minimal chat store interface needed by the chat service.
type ChatRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error)
	GetParentByEventID(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error)
	GetOrCreateChild(ctx context.Context, eventID uuid.UUID, externalRoomID string, roomName string, parentID uuid.UUID) (*domain.Chat, error)
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error)
	GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error)
	UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error
	InitChatSeq(ctx context.Context, chatID uuid.UUID) error
}

// MessageRepository is the minimal message store interface needed by the chat service.
type MessageRepository interface {
	Create(ctx context.Context, m *domain.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error)
	GetByChatIDBeforeSeq(ctx context.Context, chatID uuid.UUID, beforeSeq int64, limit int) ([]*domain.Message, error)
	ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, newText string) error
}

// MuteRepository is the minimal mute store interface needed by the chat service.
type MuteRepository interface {
	IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error)
	Create(ctx context.Context, m *domain.Mute) error
	GetActive(ctx context.Context, chatID, userID uuid.UUID) (*domain.Mute, error)
}