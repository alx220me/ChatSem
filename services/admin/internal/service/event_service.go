package service

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// EventService implements business logic for event management.
type EventService struct {
	events domain.EventRepository
	chats  domain.ChatRepository
}

// NewEventService creates an EventService backed by the given repositories.
func NewEventService(events domain.EventRepository, chats domain.ChatRepository) *EventService {
	return &EventService{events: events, chats: chats}
}

// CreateEvent creates a new event with a bcrypt-hashed API secret and a parent chat.
func (s *EventService) CreateEvent(ctx context.Context, name, allowedOrigin, apiSecret string) (*domain.Event, error) {
	slog.Debug("[EventService.CreateEvent] start", "name", name)

	hash, err := bcrypt.GenerateFromPassword([]byte(apiSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("EventService.CreateEvent bcrypt: %w", err)
	}

	e := &domain.Event{
		Name:          name,
		AllowedOrigin: allowedOrigin,
		APISecret:     string(hash),
		Settings:      []byte("{}"),
	}
	if err := s.events.Create(ctx, e); err != nil {
		return nil, fmt.Errorf("EventService.CreateEvent insert: %w", err)
	}
	slog.Info("[EventService.CreateEvent] created", "event_id", e.ID)

	// Create parent chat + seq counter.
	if err := s.createParentChat(ctx, e.ID); err != nil {
		return nil, err
	}

	return e, nil
}

func (s *EventService) createParentChat(ctx context.Context, eventID uuid.UUID) error {
	slog.Debug("[EventService.createParentChat] start", "event_id", eventID)

	// Insert parent chat via GetOrCreateChild stub — admin ChatRepo has its own Create path.
	// Use a direct approach: the domain.ChatRepository doesn't expose a bare Create method,
	// so we rely on GetOrCreateChild to create the child.
	// For parent chats, we need a separate method. We'll expose it via the chat repo directly
	// by using a type assertion or a dedicated interface. Here we use a dedicated insert approach.

	// Insert parent chat directly using the EventRepository's underlying connection.
	// Since ChatRepository doesn't have CreateParent, we add that to admin's ChatRepo.
	chatID, err := s.chats.(interface {
		CreateParent(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)
	}).CreateParent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("EventService.createParentChat insert: %w", err)
	}

	if err := s.chats.InitChatSeq(ctx, chatID); err != nil {
		return fmt.Errorf("EventService.createParentChat init seq: %w", err)
	}

	slog.Info("[EventService.CreateParentChat] parent chat created", "event_id", eventID, "chat_id", chatID)
	return nil
}

// ListChats returns all chats (parent + children) for an event.
func (s *EventService) ListChats(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error) {
	slog.Debug("[EventService.ListChats] start", "event_id", eventID)
	chats, err := s.chats.ListByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("EventService.ListChats: %w", err)
	}
	return chats, nil
}

// UpdateChatSettings applies a JSONB settings patch to a chat.
func (s *EventService) UpdateChatSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error {
	slog.Debug("[EventService.UpdateChatSettings] start", "chat_id", chatID)
	if err := s.chats.UpdateSettings(ctx, chatID, settings); err != nil {
		return fmt.Errorf("EventService.UpdateChatSettings: %w", err)
	}
	slog.Info("[EventService.UpdateChatSettings] updated", "chat_id", chatID)
	return nil
}

// ListEvents returns all events with pagination.
func (s *EventService) ListEvents(ctx context.Context) ([]*domain.Event, error) {
	slog.Debug("[EventService.ListEvents] start")
	events, err := s.events.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("EventService.ListEvents: %w", err)
	}
	return events, nil
}

// GetEvent returns an event by ID.
func (s *EventService) GetEvent(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	slog.Debug("[EventService.GetEvent] start", "event_id", id)
	event, err := s.events.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("EventService.GetEvent: %w", err)
	}
	return event, nil
}
