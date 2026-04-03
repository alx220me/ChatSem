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

// MessageRepo implements domain.MessageRepository using pgx.
type MessageRepo struct {
	db *pgxpool.Pool
}

// NewMessageRepo creates a MessageRepo backed by the given connection pool.
func NewMessageRepo(db *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{db: db}
}

// Create atomically assigns the next seq for chatID via CTE and inserts the message.
func (r *MessageRepo) Create(ctx context.Context, m *domain.Message) error {
	slog.Debug("[MessageRepo.Create] inserting message", "chat_id", m.ChatID, "user_id", m.UserID)
	row := r.db.QueryRow(ctx, `
		WITH next_seq AS (
			UPDATE chat_seqs SET last_seq = last_seq + 1 WHERE chat_id = $1 RETURNING last_seq
		)
		INSERT INTO messages (id, chat_id, user_id, text, seq, created_at)
		SELECT gen_random_uuid(), $1, $2, $3, next_seq.last_seq, NOW()
		FROM next_seq
		RETURNING id, seq, created_at`,
		m.ChatID, m.UserID, m.Text)

	if err := row.Scan(&m.ID, &m.Seq, &m.CreatedAt); err != nil {
		return fmt.Errorf("MessageRepo.Create: %w", err)
	}
	slog.Debug("[MessageRepo.Create] inserted", "message_id", m.ID, "seq", m.Seq)
	return nil
}

// GetByChatIDAfterSeq returns messages in chatID with seq > afterSeq, ordered ascending.
func (r *MessageRepo) GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
	slog.Debug("[MessageRepo.GetByChatIDAfterSeq] query", "chat_id", chatID, "after_seq", afterSeq, "limit", limit)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, text, seq, created_at, deleted_at
		FROM messages
		WHERE chat_id = $1 AND seq > $2 AND deleted_at IS NULL
		ORDER BY seq ASC
		LIMIT $3`,
		chatID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("MessageRepo.GetByChatIDAfterSeq: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	slog.Debug("[MessageRepo.GetByChatIDAfterSeq] fetched", "chat_id", chatID, "count", len(msgs))
	return msgs, nil
}

// ListByChatID returns messages for chatID in descending order (most recent first).
func (r *MessageRepo) ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, error) {
	slog.Debug("[MessageRepo.ListByChatID] query", "chat_id", chatID, "limit", limit, "offset", offset)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, text, seq, created_at, deleted_at
		FROM messages
		WHERE chat_id = $1 AND deleted_at IS NULL
		ORDER BY seq DESC
		LIMIT $2 OFFSET $3`,
		chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("MessageRepo.ListByChatID: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// SoftDelete marks a message as deleted by setting deleted_at.
func (r *MessageRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	slog.Debug("[MessageRepo.SoftDelete] deleting", "message_id", id)
	_, err := r.db.Exec(ctx, `UPDATE messages SET deleted_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("MessageRepo.SoftDelete: %w", err)
	}
	slog.Debug("[MessageRepo.SoftDelete] done", "message_id", id)
	return nil
}

// GetByChatRange returns messages within the optional time range [from, to], paginated.
func (r *MessageRepo) GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error) {
	slog.Debug("[MessageRepo.GetByChatRange] query", "chat_id", chatID, "from", from, "to", to, "limit", limit, "offset", offset)
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, user_id, text, seq, created_at, deleted_at
		FROM messages
		WHERE chat_id = $1
		  AND deleted_at IS NULL
		  AND ($2::TIMESTAMPTZ IS NULL OR created_at >= $2)
		  AND ($3::TIMESTAMPTZ IS NULL OR created_at <= $3)
		ORDER BY seq ASC
		LIMIT $4 OFFSET $5`,
		chatID, from, to, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("MessageRepo.GetByChatRange: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	slog.Debug("[MessageRepo.GetByChatRange] fetched", "count", len(msgs))
	return msgs, nil
}

// CountByChatRange returns the count of non-deleted messages in the given time range.
func (r *MessageRepo) CountByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time) (int64, error) {
	slog.Debug("[MessageRepo.CountByChatRange] query", "chat_id", chatID)
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE chat_id = $1
		  AND deleted_at IS NULL
		  AND ($2::TIMESTAMPTZ IS NULL OR created_at >= $2)
		  AND ($3::TIMESTAMPTZ IS NULL OR created_at <= $3)`,
		chatID, from, to).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("MessageRepo.CountByChatRange: %w", err)
	}
	return count, nil
}

// GetByID returns a single message by its ID (including soft-deleted).
func (r *MessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	slog.Debug("[MessageRepo.GetByID] query", "message_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, chat_id, user_id, text, seq, created_at, deleted_at
		FROM messages WHERE id = $1`, id)
	m := &domain.Message{}
	if err := row.Scan(&m.ID, &m.ChatID, &m.UserID, &m.Text, &m.Seq, &m.CreatedAt, &m.DeletedAt); err != nil {
		return nil, fmt.Errorf("MessageRepo.GetByID: %w", err)
	}
	return m, nil
}

// scanMessages scans pgx rows into a slice of *domain.Message.
func scanMessages(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*domain.Message, error) {
	var msgs []*domain.Message
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.Text, &m.Seq, &m.CreatedAt, &m.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
