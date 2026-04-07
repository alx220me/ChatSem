package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BanRepo provides read-only ban checks for the chat service.
// Bans are written by the admin service; the chat service only reads them as a fallback
// when the Redis ban cache misses.
type BanRepo struct {
	db *pgxpool.Pool
}

// NewBanRepo creates a BanRepo backed by the given connection pool.
func NewBanRepo(db *pgxpool.Pool) *BanRepo {
	return &BanRepo{db: db}
}

// IsUserBanned returns true if the user has an active (non-expired) ban in the event.
func (r *BanRepo) IsUserBanned(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	slog.Debug("[ChatBanRepo.IsUserBanned] query", "user_id", userID, "event_id", eventID)
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM bans
		WHERE user_id = $1 AND event_id = $2
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		userID, eventID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("ChatBanRepo.IsUserBanned: %w", err)
	}
	return count > 0, nil
}