package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MessageRepo implements the message read queries needed by admin export.
type MessageRepo struct {
	db *pgxpool.Pool
}

// NewMessageRepo creates a MessageRepo backed by the given connection pool.
func NewMessageRepo(db *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{db: db}
}

// GetByChatRange returns non-deleted messages for chatID within the optional time range,
// ordered by seq ascending, with pagination.
func (r *MessageRepo) GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error) {
	slog.Debug("[AdminMessageRepo.GetByChatRange] query", "chat_id", chatID, "from", from, "to", to, "limit", limit, "offset", offset)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, text, seq, created_at
		FROM messages
		WHERE chat_id = $1
		  AND deleted_at IS NULL
		  AND ($2::TIMESTAMPTZ IS NULL OR created_at >= $2)
		  AND ($3::TIMESTAMPTZ IS NULL OR created_at <= $3)
		ORDER BY seq ASC
		LIMIT $4 OFFSET $5`,
		chatID, from, to, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("AdminMessageRepo.GetByChatRange: %w", err)
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.Text, &m.Seq, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("AdminMessageRepo.GetByChatRange scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("AdminMessageRepo.GetByChatRange rows: %w", err)
	}
	slog.Debug("[AdminMessageRepo.GetByChatRange] fetched", "count", len(msgs))
	return msgs, nil
}

// CountByChatRange returns the count of non-deleted messages for chatID in the optional time range.
func (r *MessageRepo) CountByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time) (int64, error) {
	slog.Debug("[AdminMessageRepo.CountByChatRange] query", "chat_id", chatID)
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE chat_id = $1
		  AND deleted_at IS NULL
		  AND ($2::TIMESTAMPTZ IS NULL OR created_at >= $2)
		  AND ($3::TIMESTAMPTZ IS NULL OR created_at <= $3)`,
		chatID, from, to).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("AdminMessageRepo.CountByChatRange: %w", err)
	}
	return count, nil
}
