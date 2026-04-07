package ports

import (
	"context"
	"time"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// EventRepository is the minimal event store interface needed by the admin service.
type EventRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	Create(ctx context.Context, e *domain.Event) error
	List(ctx context.Context) ([]*domain.Event, error)
}

// ChatRepository is the minimal chat store interface needed by the admin service.
// CreateParent is included here directly, eliminating the type assertion in EventService.
type ChatRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error)
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error)
	GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error)
	UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error
	InitChatSeq(ctx context.Context, chatID uuid.UUID) error
	CreateParent(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)
}

// UserRepository is the minimal user store interface needed by the admin service.
type UserRepository interface {
	ListByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]*domain.User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role domain.UserRole) error
}

// BanRepository is the minimal ban store interface needed by the admin service.
type BanRepository interface {
	Create(ctx context.Context, b *domain.Ban) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Ban, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Ban, error)
}

// MuteRepository is the minimal mute store interface needed by the admin service.
type MuteRepository interface {
	Create(ctx context.Context, m *domain.Mute) error
	GetActive(ctx context.Context, chatID, userID uuid.UUID) (*domain.Mute, error)
	Expire(ctx context.Context, muteID uuid.UUID) error
	ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*domain.Mute, error)
}

// MessageReader is the minimal message store interface needed by ExportService.
type MessageReader interface {
	GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error)
}