package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserRole defines the permission level of a user within an event.
type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleModerator UserRole = "moderator"
	RoleAdmin     UserRole = "admin"
)

// User represents a participant in an event. ExternalID comes from the organizer's system.
type User struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"externalId"` // ID from organizer system
	EventID    uuid.UUID `json:"eventId"`
	Name       string    `json:"name"`
	Role       UserRole  `json:"role"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Ban records a ban of a user within an event (optionally scoped to a chat).
type Ban struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"userId"`
	EventID   uuid.UUID  `json:"eventId"`
	ChatID    *uuid.UUID `json:"chatId,omitempty"` // nil = event-wide ban
	BannedBy  uuid.UUID  `json:"bannedBy"`
	Reason    string     `json:"reason,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"` // nil = permanent
}

// Mute records a mute of a user within a chat. ExpiresAt nil means permanent.
type Mute struct {
	ID        uuid.UUID  `json:"id"`
	ChatID    uuid.UUID  `json:"chatId"`
	UserID    uuid.UUID  `json:"userId"`
	MutedBy   uuid.UUID  `json:"mutedBy"`
	Reason    string     `json:"reason,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"` // nil = permanent
}
