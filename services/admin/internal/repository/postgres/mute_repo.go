package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MuteRepo implements domain.MuteRepository for the admin service using pgx.
type MuteRepo struct {
	db *pgxpool.Pool
}

// NewMuteRepo creates a MuteRepo backed by the given connection pool.
func NewMuteRepo(db *pgxpool.Pool) *MuteRepo {
	return &MuteRepo{db: db}
}

// Create inserts a new mute record.
func (r *MuteRepo) Create(ctx context.Context, m *domain.Mute) error {
	slog.Debug("[MuteRepo.Create] creating mute", "user_id", m.UserID, "chat_id", m.ChatID, "expires_at", m.ExpiresAt)
	row := r.db.QueryRow(ctx, `
		INSERT INTO mutes (chat_id, user_id, muted_by, reason, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		m.ChatID, m.UserID, m.MutedBy, m.Reason, m.ExpiresAt)

	if err := row.Scan(&m.ID, &m.CreatedAt); err != nil {
		return fmt.Errorf("MuteRepo.Create: %w", err)
	}
	slog.Debug("[MuteRepo.Create] created", "mute_id", m.ID)
	return nil
}

// Delete removes a mute by ID (unmute).
func (r *MuteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	slog.Debug("[MuteRepo.Delete] deleting mute", "mute_id", id)
	_, err := r.db.Exec(ctx, `DELETE FROM mutes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("MuteRepo.Delete: %w", err)
	}
	slog.Debug("[MuteRepo.Delete] done", "mute_id", id)
	return nil
}

// ListByChatID returns all mutes for a chat.
func (r *MuteRepo) ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*domain.Mute, error) {
	slog.Debug("[MuteRepo.ListByChatID] query", "chat_id", chatID)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, muted_by, reason, created_at, expires_at
		FROM mutes WHERE chat_id = $1 ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, fmt.Errorf("MuteRepo.ListByChatID: %w", err)
	}
	defer rows.Close()

	var mutes []*domain.Mute
	for rows.Next() {
		m := &domain.Mute{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.MutedBy, &m.Reason, &m.CreatedAt, &m.ExpiresAt); err != nil {
			return nil, fmt.Errorf("MuteRepo.ListByChatID scan: %w", err)
		}
		mutes = append(mutes, m)
	}
	slog.Debug("[MuteRepo.ListByChatID] fetched", "count", len(mutes))
	return mutes, rows.Err()
}

// IsUserMuted returns true if the user has an active mute in the chat.
func (r *MuteRepo) IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error) {
	slog.Debug("[MuteRepo.IsUserMuted] query", "user_id", userID, "chat_id", chatID)
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM mutes
		WHERE user_id = $1 AND chat_id = $2 AND expires_at > NOW()`,
		userID, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("MuteRepo.IsUserMuted: %w", err)
	}
	return count > 0, nil
}
