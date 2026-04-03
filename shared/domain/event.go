package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event represents an organizer's event (conference, webinar, etc.).
type Event struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Settings      []byte    `json:"settings"`      // JSONB
	AllowedOrigin string    `json:"allowedOrigin"` // CORS allowed origin for host website
	APISecret     string    `json:"-"`             // bcrypt hash of the pre-shared secret
	CreatedAt     time.Time `json:"createdAt"`
}
