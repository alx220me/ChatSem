package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatRepo implements domain.ChatRepository using pgx.
type ChatRepo struct {
	db *pgxpool.Pool
}

// NewChatRepo creates a ChatRepo backed by the given connection pool.
func NewChatRepo(db *pgxpool.Pool) *ChatRepo {
	return &ChatRepo{db: db}
}

// GetByID returns the chat with the given ID.
func (r *ChatRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[ChatRepo.GetByID] query", "chat_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE id = $1`, id)

	c := &domain.Chat{}
	var parentID *uuid.UUID
	var externalRoomID *string
	if err := row.Scan(&c.ID, &c.EventID, &parentID, &externalRoomID, &c.Type, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("ChatRepo.GetByID: %w", err)
	}
	c.ParentID = parentID
	if externalRoomID != nil {
		c.ExternalRoomID = *externalRoomID
	}
	slog.Debug("[ChatRepo.GetByID] found", "chat_id", id, "type", c.Type)
	return c, nil
}

// GetParentByEventID returns the parent chat for an event.
func (r *ChatRepo) GetParentByEventID(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[ChatRepo.GetParentByEventID] query", "event_id", eventID)
	row := r.db.QueryRow(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE event_id = $1 AND type = 'parent'`, eventID)

	c := &domain.Chat{}
	var parentID *uuid.UUID
	var externalRoomID *string
	if err := row.Scan(&c.ID, &c.EventID, &parentID, &externalRoomID, &c.Type, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("ChatRepo.GetParentByEventID: %w", err)
	}
	c.ParentID = parentID
	if externalRoomID != nil {
		c.ExternalRoomID = *externalRoomID
	}
	slog.Debug("[ChatRepo.GetParentByEventID] found", "chat_id", c.ID, "event_id", eventID)
	return c, nil
}

// GetOrCreateChild returns the child chat for the given (eventID, externalRoomID) pair,
// creating it if it does not exist (idempotent — ON CONFLICT DO NOTHING).
func (r *ChatRepo) GetOrCreateChild(ctx context.Context, eventID uuid.UUID, externalRoomID string, parentID uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[ChatRepo.GetOrCreateChild] upsert", "event_id", eventID, "room_id", externalRoomID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO chats (event_id, parent_id, external_room_id, type)
		VALUES ($1, $2, $3, 'child')
		ON CONFLICT (event_id, external_room_id) WHERE type = 'child' DO NOTHING`,
		eventID, parentID, externalRoomID)
	if err != nil {
		return nil, fmt.Errorf("ChatRepo.GetOrCreateChild insert: %w", err)
	}

	row := r.db.QueryRow(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE event_id=$1 AND external_room_id=$2 AND type='child'`,
		eventID, externalRoomID)

	c := &domain.Chat{}
	var pID *uuid.UUID
	var extRoomID *string
	if err := row.Scan(&c.ID, &c.EventID, &pID, &extRoomID, &c.Type, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("ChatRepo.GetOrCreateChild select: %w", err)
	}
	c.ParentID = pID
	if extRoomID != nil {
		c.ExternalRoomID = *extRoomID
	}
	slog.Debug("[ChatRepo.GetOrCreateChild] done", "chat_id", c.ID, "room_id", externalRoomID)
	return c, nil
}

// ListByEventID returns all chats for an event.
func (r *ChatRepo) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error) {
	slog.Debug("[ChatRepo.ListByEventID] query", "event_id", eventID)
	rows, err := r.db.Query(ctx, `
		SELECT id, event_id, parent_id, external_room_id, type, created_at
		FROM chats WHERE event_id = $1 ORDER BY created_at ASC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("ChatRepo.ListByEventID: %w", err)
	}
	defer rows.Close()

	var chats []*domain.Chat
	for rows.Next() {
		c := &domain.Chat{}
		var parentID *uuid.UUID
		var extRoomID *string
		if err := rows.Scan(&c.ID, &c.EventID, &parentID, &extRoomID, &c.Type, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("ChatRepo.ListByEventID scan: %w", err)
		}
		c.ParentID = parentID
		if extRoomID != nil {
			c.ExternalRoomID = *extRoomID
		}
		chats = append(chats, c)
	}
	slog.Debug("[ChatRepo.ListByEventID] fetched", "event_id", eventID, "count", len(chats))
	return chats, rows.Err()
}

// GetSettings returns the JSONB settings of the given chat (always from parent).
func (r *ChatRepo) GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error) {
	slog.Debug("[ChatRepo.GetSettings] query", "chat_id", chatID)
	var settings []byte
	err := r.db.QueryRow(ctx, `SELECT settings FROM chats WHERE id=$1`, chatID).Scan(&settings)
	if err != nil {
		return nil, fmt.Errorf("ChatRepo.GetSettings: %w", err)
	}
	return settings, nil
}

// UpdateSettings sets the JSONB settings of the given chat.
func (r *ChatRepo) UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error {
	slog.Debug("[ChatRepo.UpdateSettings] update", "chat_id", chatID)
	_, err := r.db.Exec(ctx, `UPDATE chats SET settings=$2 WHERE id=$1`, chatID, settings)
	if err != nil {
		return fmt.Errorf("ChatRepo.UpdateSettings: %w", err)
	}
	slog.Debug("[ChatRepo.UpdateSettings] done", "chat_id", chatID)
	return nil
}

// InitChatSeq inserts the initial seq counter row for a newly created chat.
func (r *ChatRepo) InitChatSeq(ctx context.Context, chatID uuid.UUID) error {
	slog.Debug("[ChatRepo.InitChatSeq] init", "chat_id", chatID)
	_, err := r.db.Exec(ctx, `INSERT INTO chat_seqs (chat_id, last_seq) VALUES ($1, 0)`, chatID)
	if err != nil {
		return fmt.Errorf("ChatRepo.InitChatSeq: %w", err)
	}
	return nil
}
