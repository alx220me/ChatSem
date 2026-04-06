package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"chatsem/services/admin/internal/ports"
	"chatsem/shared/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// EventService implements business logic for event management.
type EventService struct {
	events ports.EventRepository
	chats  ports.ChatRepository
}

// NewEventService creates an EventService backed by the given repositories.
func NewEventService(events ports.EventRepository, chats ports.ChatRepository) *EventService {
	return &EventService{events: events, chats: chats}
}

// CreateEvent generates a cryptographically secure API secret, hashes it with bcrypt,
// persists the event, and returns both the event and the plaintext secret (shown once).
func (s *EventService) CreateEvent(ctx context.Context, name, allowedOrigin string) (*domain.Event, string, error) {
	slog.Debug("[EventService.CreateEvent] start", "name", name, "origin", allowedOrigin)

	plainSecret, err := generateSecret()
	if err != nil {
		return nil, "", fmt.Errorf("EventService.CreateEvent generate secret: %w", err)
	}
	slog.Debug("[EventService.CreateEvent] secret generated")

	hash, err := bcrypt.GenerateFromPassword([]byte(plainSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("EventService.CreateEvent bcrypt: %w", err)
	}

	e := &domain.Event{
		Name:          name,
		AllowedOrigin: allowedOrigin,
		APISecret:     string(hash),
		Settings:      []byte("{}"),
	}
	if err := s.events.Create(ctx, e); err != nil {
		return nil, "", fmt.Errorf("EventService.CreateEvent insert: %w", err)
	}
	slog.Info("[EventService.CreateEvent] created", "event_id", e.ID)

	if err := s.createParentChat(ctx, e.ID); err != nil {
		return nil, "", err
	}

	return e, plainSecret, nil
}

// generateSecret returns a 64-character hex string from 32 random bytes.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateSecret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *EventService) createParentChat(ctx context.Context, eventID uuid.UUID) error {
	slog.Debug("[EventService.createParentChat] start", "event_id", eventID)

	chatID, err := s.chats.CreateParent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("EventService.createParentChat insert: %w", err)
	}

	if err := s.chats.InitChatSeq(ctx, chatID); err != nil {
		return fmt.Errorf("EventService.createParentChat init seq: %w", err)
	}

	slog.Info("[EventService.createParentChat] parent chat created", "event_id", eventID, "chat_id", chatID)
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

// ListEvents returns all events.
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
