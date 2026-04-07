package service

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/services/chat/internal/ports"
	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// ChatService implements business logic for chat hierarchy.
type ChatService struct {
	chats ports.ChatRepository
}

// NewChatService creates a ChatService backed by the given repository.
func NewChatService(chats ports.ChatRepository) *ChatService {
	return &ChatService{chats: chats}
}

// GetOrCreateChildResult carries the created/fetched child chat and whether it was newly created.
type GetOrCreateChildResult struct {
	Chat  *domain.Chat
	IsNew bool
}

// GetOrCreateChildChat returns an existing child chat for (eventID, roomID) or creates one.
// Always goes through the upsert path so that external_room_name is kept up to date.
func (s *ChatService) GetOrCreateChildChat(ctx context.Context, eventID uuid.UUID, roomID string, roomName string) (*GetOrCreateChildResult, error) {
	slog.Debug("[ChatService.GetOrCreateChildChat] start", "event_id", eventID, "room_id", roomID, "room_name", roomName)

	parent, err := s.chats.GetParentByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("ChatService.GetOrCreateChildChat get parent: %w", err)
	}

	child, err := s.chats.GetOrCreateChild(ctx, eventID, roomID, roomName, parent.ID)
	if err != nil {
		return nil, fmt.Errorf("ChatService.GetOrCreateChildChat create: %w", err)
	}

	// Init seq counter — idempotent: INSERT ignores duplicate key errors via the repo.
	if err := s.chats.InitChatSeq(ctx, child.ID); err != nil {
		// Non-fatal: seq row may already exist if another process won the race.
		slog.Debug("[ChatService.GetOrCreateChildChat] InitChatSeq skipped (likely already exists)", "chat_id", child.ID, "err", err)
	}

	slog.Info("[ChatService.GetOrCreateChildChat] upserted", "chat_id", child.ID, "room_name", child.ExternalRoomName)
	return &GetOrCreateChildResult{Chat: child, IsNew: false}, nil
}

// GetParentChat returns the parent chat for an event.
func (s *ChatService) GetParentChat(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[ChatService.GetParentChat] start", "event_id", eventID)
	chat, err := s.chats.GetParentByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("ChatService.GetParentChat: %w", err)
	}
	return chat, nil
}

// GetChildren returns all child chats for an event.
func (s *ChatService) GetChildren(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error) {
	slog.Debug("[ChatService.GetChildren] start", "event_id", eventID)
	all, err := s.chats.ListByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("ChatService.GetChildren: %w", err)
	}
	var children []*domain.Chat
	for _, c := range all {
		if c.Type == domain.TypeChild {
			children = append(children, c)
		}
	}
	return children, nil
}

// GetChat returns a single chat by ID.
func (s *ChatService) GetChat(ctx context.Context, chatID uuid.UUID) (*domain.Chat, error) {
	slog.Debug("[ChatService.GetChat] start", "chat_id", chatID)
	chat, err := s.chats.GetByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("ChatService.GetChat: %w", err)
	}
	return chat, nil
}
