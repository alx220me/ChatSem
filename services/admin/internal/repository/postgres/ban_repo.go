package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BanRepo implements domain.BanRepository for the admin service using pgx.
type BanRepo struct {
	db *pgxpool.Pool
}

// NewBanRepo creates a BanRepo backed by the given connection pool.
func NewBanRepo(db *pgxpool.Pool) *BanRepo {
	return &BanRepo{db: db}
}

// Create inserts a new ban record.
func (r *BanRepo) Create(ctx context.Context, b *domain.Ban) error {
	slog.Debug("[BanRepo.Create] creating ban", "user_id", b.UserID, "event_id", b.EventID, "expires_at", b.ExpiresAt)
	row := r.db.QueryRow(ctx, `
		INSERT INTO bans (user_id, event_id, chat_id, banned_by, reason, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		b.UserID, b.EventID, b.ChatID, b.BannedBy, b.Reason, b.ExpiresAt)

	if err := row.Scan(&b.ID, &b.CreatedAt); err != nil {
		return fmt.Errorf("BanRepo.Create: %w", err)
	}
	slog.Debug("[BanRepo.Create] created", "ban_id", b.ID)
	return nil
}

// GetByID returns a ban by its ID, or an error if not found.
func (r *BanRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Ban, error) {
	slog.Debug("[BanRepo.GetByID] query", "ban_id", id)
	b := &domain.Ban{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, event_id, chat_id, banned_by, reason, created_at, expires_at
		FROM bans WHERE id = $1`, id).
		Scan(&b.ID, &b.UserID, &b.EventID, &b.ChatID, &b.BannedBy, &b.Reason, &b.CreatedAt, &b.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("BanRepo.GetByID: %w", err)
	}
	return b, nil
}

// Delete removes a ban by ID (expire / unban).
func (r *BanRepo) Delete(ctx context.Context, id uuid.UUID) error {
	slog.Debug("[BanRepo.Delete] deleting ban", "ban_id", id)
	_, err := r.db.Exec(ctx, `DELETE FROM bans WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("BanRepo.Delete: %w", err)
	}
	slog.Debug("[BanRepo.Delete] done", "ban_id", id)
	return nil
}

// ListByEventID returns all active and expired bans for an event.
func (r *BanRepo) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Ban, error) {
	slog.Debug("[BanRepo.ListByEventID] query", "event_id", eventID)
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, event_id, chat_id, banned_by, reason, created_at, expires_at
		FROM bans WHERE event_id = $1 ORDER BY created_at DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("BanRepo.ListByEventID: %w", err)
	}
	defer rows.Close()

	bans := make([]*domain.Ban, 0)
	for rows.Next() {
		b := &domain.Ban{}
		if err := rows.Scan(&b.ID, &b.UserID, &b.EventID, &b.ChatID, &b.BannedBy, &b.Reason, &b.CreatedAt, &b.ExpiresAt); err != nil {
			return nil, fmt.Errorf("BanRepo.ListByEventID scan: %w", err)
		}
		bans = append(bans, b)
	}
	slog.Debug("[BanRepo.ListByEventID] fetched", "count", len(bans))
	return bans, rows.Err()
}

// IsUserBanned returns true if the user has an active ban in the event.
func (r *BanRepo) IsUserBanned(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	slog.Debug("[BanRepo.IsUserBanned] query", "user_id", userID, "event_id", eventID)
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM bans
		WHERE user_id = $1 AND event_id = $2
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		userID, eventID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("BanRepo.IsUserBanned: %w", err)
	}
	return count > 0, nil
}
