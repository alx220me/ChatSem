package jwt

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims holds the parsed fields from a ChatSem JWT.
type Claims struct {
	UserID     uuid.UUID `json:"user_id"`
	ExternalID string    `json:"external_id"`
	EventID    uuid.UUID `json:"event_id"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type rawClaims struct {
	UserID     string `json:"user_id"`
	ExternalID string `json:"external_id"`
	EventID    string `json:"event_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

// ValidateToken parses and validates a JWT, returning the extracted claims.
func ValidateToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &rawClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}

	raw, ok := token.Claims.(*rawClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt: invalid claims")
	}

	userID, err := uuid.Parse(raw.UserID)
	if err != nil {
		return nil, fmt.Errorf("jwt: invalid user_id: %w", err)
	}
	eventID, err := uuid.Parse(raw.EventID)
	if err != nil {
		return nil, fmt.Errorf("jwt: invalid event_id: %w", err)
	}

	var exp time.Time
	if raw.ExpiresAt != nil {
		exp = raw.ExpiresAt.Time
	}

	c := &Claims{
		UserID:     userID,
		ExternalID: raw.ExternalID,
		EventID:    eventID,
		Name:       raw.Name,
		Role:       raw.Role,
		ExpiresAt:  exp,
	}
	slog.Debug("jwt: token validated", "user_id", c.UserID, "event_id", c.EventID, "role", c.Role)
	return c, nil
}

// CreateToken signs a new JWT with the given claims.
func CreateToken(c *Claims, secret string, ttl time.Duration) (string, error) {
	raw := &rawClaims{
		UserID:     c.UserID.String(),
		ExternalID: c.ExternalID,
		EventID:    c.EventID.String(),
		Name:       c.Name,
		Role:       c.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, raw)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}
	slog.Debug("jwt: token created", "user_id", c.UserID, "event_id", c.EventID, "role", c.Role, "ttl", ttl)
	return signed, nil
}
