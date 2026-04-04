package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MuteRepo implements domain.MuteRepository for the chat service using pgx.
type MuteRepo struct {
	db *pgxpool.Pool
}

// NewMuteRepo creates a MuteRepo backed by the given connection pool.
func NewMuteRepo(db *pgxpool.Pool) *MuteRepo {
	return &MuteRepo{db: db}
}

// Create inserts a new mute record.
func (r *MuteRepo) Create(ctx context.Context, m *domain.Mute) error {
	slog.Debug("[ChatMuteRepo.Create] creating mute", "user_id", m.UserID, "chat_id", m.ChatID)
	row := r.db.QueryRow(ctx, `
		INSERT INTO mutes (chat_id, user_id, muted_by, reason, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		m.ChatID, m.UserID, m.MutedBy, m.Reason, m.ExpiresAt)
	if err := row.Scan(&m.ID, &m.CreatedAt); err != nil {
		return fmt.Errorf("ChatMuteRepo.Create: %w", err)
	}
	return nil
}

// Delete removes a mute by ID.
func (r *MuteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	slog.Debug("[ChatMuteRepo.Delete] deleting mute", "mute_id", id)
	_, err := r.db.Exec(ctx, `DELETE FROM mutes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("ChatMuteRepo.Delete: %w", err)
	}
	return nil
}

// ListByChatID returns all mutes for a chat.
func (r *MuteRepo) ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*domain.Mute, error) {
	slog.Debug("[ChatMuteRepo.ListByChatID] query", "chat_id", chatID)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, muted_by, reason, created_at, expires_at
		FROM mutes WHERE chat_id = $1 ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, fmt.Errorf("ChatMuteRepo.ListByChatID: %w", err)
	}
	defer rows.Close()

	var mutes []*domain.Mute
	for rows.Next() {
		m := &domain.Mute{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.MutedBy, &m.Reason, &m.CreatedAt, &m.ExpiresAt); err != nil {
			return nil, fmt.Errorf("ChatMuteRepo.ListByChatID scan: %w", err)
		}
		mutes = append(mutes, m)
	}
	return mutes, rows.Err()
}

// GetActive returns the active mute for user in chat, or domain.ErrNotFound if none.
func (r *MuteRepo) GetActive(ctx context.Context, chatID, userID uuid.UUID) (*domain.Mute, error) {
	slog.Debug("[ChatMuteRepo.GetActive] checking", "chat_id", chatID, "user_id", userID)
	row := r.db.QueryRow(ctx, `
		SELECT id, chat_id, user_id, muted_by, reason, created_at, expires_at
		FROM mutes
		WHERE chat_id = $1 AND user_id = $2 AND (expires_at IS NULL OR expires_at > NOW())
		LIMIT 1`, chatID, userID)

	m := &domain.Mute{}
	if err := row.Scan(&m.ID, &m.ChatID, &m.UserID, &m.MutedBy, &m.Reason, &m.CreatedAt, &m.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("ChatMuteRepo.GetActive: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("ChatMuteRepo.GetActive: %w", err)
	}
	return m, nil
}

// Expire sets expires_at = NOW() for the given mute (soft expiry).
func (r *MuteRepo) Expire(ctx context.Context, muteID uuid.UUID) error {
	slog.Debug("[ChatMuteRepo.Expire] expiring", "mute_id", muteID)
	_, err := r.db.Exec(ctx, `UPDATE mutes SET expires_at = NOW() WHERE id = $1`, muteID)
	if err != nil {
		return fmt.Errorf("ChatMuteRepo.Expire: %w", err)
	}
	return nil
}

// IsUserMuted returns true if the user has an active mute in the chat.
func (r *MuteRepo) IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error) {
	slog.Debug("[ChatMuteRepo.IsUserMuted] query", "user_id", userID, "chat_id", chatID)
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM mutes
		WHERE user_id = $1 AND chat_id = $2 AND expires_at > NOW()`,
		userID, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("ChatMuteRepo.IsUserMuted: %w", err)
	}
	return count > 0, nil
}
