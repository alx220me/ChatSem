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
// Reply preview fields (ReplyToSeq, ReplyToText) are populated via LEFT JOIN when reply_to_id is set.
func (r *MessageRepo) GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error) {
	slog.Debug("[AdminMessageRepo.GetByChatRange] query", "chat_id", chatID, "from", from, "to", to, "limit", limit, "offset", offset)
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.chat_id, m.user_id, m.text, m.seq, m.created_at,
		       m.reply_to_id, rm.seq, COALESCE(LEFT(rm.text, 100), ''), COALESCE(ru.name, '')
		FROM messages m
		LEFT JOIN messages rm ON rm.id = m.reply_to_id
		LEFT JOIN users ru    ON ru.id = rm.user_id
		WHERE m.chat_id = $1
		  AND m.deleted_at IS NULL
		  AND ($2::TIMESTAMPTZ IS NULL OR m.created_at >= $2)
		  AND ($3::TIMESTAMPTZ IS NULL OR m.created_at <= $3)
		ORDER BY m.seq ASC
		LIMIT $4 OFFSET $5`,
		chatID, from, to, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("AdminMessageRepo.GetByChatRange: %w", err)
	}
	defer rows.Close()

	var msgs []*domain.Message
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Text, &m.Seq, &m.CreatedAt,
			&m.ReplyToID, &m.ReplyToSeq, &m.ReplyToText, &m.ReplyToUserName,
		); err != nil {
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
