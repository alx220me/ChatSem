package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatRepo implements domain.ChatRepository for the admin service using pgx.
type ChatRepo struct {
	db *pgxpool.Pool
}

// NewChatRepo creates a ChatRepo backed by the given connection pool.
func NewChatRepo(db *pgxpool.Pool) *ChatRepo {
	return &ChatRepo{db: db}
}

// GetByID returns the chat with the given ID.
func (r *ChatRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[AdminChatRepo.GetByID] query", "chat_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE id = $1`, id)

	c := &domain.Chat{}
	var parentID *uuid.UUID
	if err := row.Scan(&c.ID, &c.EventID, &parentID, &c.ExternalRoomID, &c.Type, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("AdminChatRepo.GetByID: %w", err)
	}
	c.ParentID = parentID
	return c, nil
}

// GetParentByEventID returns the parent chat for an event.
func (r *ChatRepo) GetParentByEventID(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[AdminChatRepo.GetParentByEventID] query", "event_id", eventID)
	row := r.db.QueryRow(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE event_id = $1 AND type = 'parent'`, eventID)

	c := &domain.Chat{}
	var parentID *uuid.UUID
	if err := row.Scan(&c.ID, &c.EventID, &parentID, &c.ExternalRoomID, &c.Type, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("AdminChatRepo.GetParentByEventID: %w", err)
	}
	c.ParentID = parentID
	return c, nil
}

// GetOrCreateChild is not used in admin service — required by interface.
func (r *ChatRepo) GetOrCreateChild(ctx context.Context, eventID uuid.UUID, externalRoomID string, parentID uuid.UUID) (*domain.Chat, error) {
	return nil, fmt.Errorf("AdminChatRepo.GetOrCreateChild: not implemented for admin service")
}

// ListByEventID returns all chats for an event.
func (r *ChatRepo) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error) {
	slog.Debug("[AdminChatRepo.ListByEventID] query", "event_id", eventID)
	rows, err := r.db.Query(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE event_id = $1 ORDER BY type DESC, created_at ASC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("AdminChatRepo.ListByEventID: %w", err)
	}
	defer rows.Close()

	var chats []*domain.Chat
	for rows.Next() {
		c := &domain.Chat{}
		var parentID *uuid.UUID
		if err := rows.Scan(&c.ID, &c.EventID, &parentID, &c.ExternalRoomID, &c.Type, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("AdminChatRepo.ListByEventID scan: %w", err)
		}
		c.ParentID = parentID
		chats = append(chats, c)
	}
	slog.Debug("[AdminChatRepo.ListByEventID] fetched", "count", len(chats))
	return chats, rows.Err()
}

// GetSettings returns the JSONB settings of the given chat.
func (r *ChatRepo) GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error) {
	slog.Debug("[AdminChatRepo.GetSettings] query", "chat_id", chatID)
	var settings []byte
	err := r.db.QueryRow(ctx, `SELECT settings FROM chats WHERE id=$1`, chatID).Scan(&settings)
	if err != nil {
		return nil, fmt.Errorf("AdminChatRepo.GetSettings: %w", err)
	}
	return settings, nil
}

// UpdateSettings sets the JSONB settings of a parent chat.
func (r *ChatRepo) UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error {
	slog.Debug("[AdminChatRepo.UpdateSettings] update", "chat_id", chatID)
	_, err := r.db.Exec(ctx, `UPDATE chats SET settings=$2 WHERE id=$1`, chatID, settings)
	if err != nil {
		return fmt.Errorf("AdminChatRepo.UpdateSettings: %w", err)
	}
	slog.Debug("[AdminChatRepo.UpdateSettings] done", "chat_id", chatID)
	return nil
}

// InitChatSeq inserts the initial seq counter row for a newly created chat.
func (r *ChatRepo) InitChatSeq(ctx context.Context, chatID uuid.UUID) error {
	slog.Debug("[AdminChatRepo.InitChatSeq] init", "chat_id", chatID)
	_, err := r.db.Exec(ctx, `INSERT INTO chat_seqs (chat_id, last_seq) VALUES ($1, 0)`, chatID)
	if err != nil {
		return fmt.Errorf("AdminChatRepo.InitChatSeq: %w", err)
	}
	return nil
}

// CreateParent inserts a new parent chat for an event and returns its ID.
func (r *ChatRepo) CreateParent(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	slog.Debug("[AdminChatRepo.CreateParent] insert", "event_id", eventID)
	var chatID uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("AdminChatRepo.CreateParent: %w", err)
	}
	slog.Debug("[AdminChatRepo.CreateParent] created", "chat_id", chatID)
	return chatID, nil
}
