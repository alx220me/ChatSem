package ports

import (
	"context"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// EventRepository is the minimal event store interface needed by the auth service.
type EventRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
}

// UserRepository is the minimal user store interface needed by the auth service.
type UserRepository interface {
	Upsert(ctx context.Context, u *domain.User) (*domain.User, error)
}