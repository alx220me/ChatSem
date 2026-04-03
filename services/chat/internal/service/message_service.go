package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// MessageService implements business logic for message sending, listing, and moderation.
type MessageService struct {
	messages domain.MessageRepository
	mutes    domain.MuteRepository
	broker   longpoll.Broker
	rdb      *redis.Client
}

// NewMessageService creates a MessageService backed by the given repositories and broker.
func NewMessageService(
	messages domain.MessageRepository,
	mutes domain.MuteRepository,
	broker longpoll.Broker,
	rdb *redis.Client,
) *MessageService {
	return &MessageService{
		messages: messages,
		mutes:    mutes,
		broker:   broker,
		rdb:      rdb,
	}
}

// SendMessage validates, checks ban/mute, inserts the message, and publishes to the broker.
func (s *MessageService) SendMessage(ctx context.Context, chatID, userID, eventID uuid.UUID, text string) (*domain.Message, error) {
	slog.Debug("[MessageService.SendMessage] sending", "chat_id", chatID, "user_id", userID, "text_len", len(text))

	if len(text) == 0 {
		return nil, domain.ErrEmptyMessage
	}
	if len(text) > 4096 {
		return nil, domain.ErrMessageTooLong
	}

	// Ban check via Redis (fast path, fail-open).
	banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
	exists, err := s.rdb.Exists(ctx, banKey).Result()
	if err != nil {
		slog.Warn("[MessageService.SendMessage] redis ban check failed, failing open", "err", err)
	} else if exists > 0 {
		slog.Warn("[MessageService.SendMessage] user is banned", "user_id", userID, "event_id", eventID)
		return nil, domain.ErrUserBanned
	}

	// Mute check via DB.
	muted, err := s.mutes.IsUserMuted(ctx, userID, chatID)
	if err != nil {
		slog.Warn("[MessageService.SendMessage] mute check failed, failing open", "err", err)
	} else if muted {
		slog.Warn("[MessageService.SendMessage] user is muted", "user_id", userID, "chat_id", chatID)
		return nil, domain.ErrUserMuted
	}

	msg := &domain.Message{ChatID: chatID, UserID: userID, Text: text}
	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("MessageService.SendMessage create: %w", err)
	}
	slog.Info("[MessageService.SendMessage] sent", "chat_id", chatID, "seq", msg.Seq, "id", msg.ID)

	// Publish to broker — fail safe: message is already persisted.
	payload, _ := json.Marshal(msg)
	if publishErr := s.broker.Publish(ctx, chatID, payload); publishErr != nil {
		slog.Warn("[MessageService.SendMessage] broker publish failed, message saved", "err", publishErr)
	}

	return msg, nil
}

// GetMessages returns messages in chatID with seq > afterSeq, capped at 100.
func (s *MessageService) GetMessages(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
	slog.Debug("[MessageService.GetMessages] query", "chat_id", chatID, "after_seq", afterSeq, "limit", limit)
	if limit > 100 {
		limit = 100
	}
	msgs, err := s.messages.GetByChatIDAfterSeq(ctx, chatID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("MessageService.GetMessages: %w", err)
	}
	return msgs, nil
}

// SoftDelete soft-deletes a message if the requestor is the owner or has a moderator/admin role.
func (s *MessageService) SoftDelete(ctx context.Context, msgID, requestorID uuid.UUID, role string) error {
	slog.Debug("[MessageService.SoftDelete] deleting", "msg_id", msgID, "by", requestorID, "role", role)

	msg, err := s.messages.GetByID(ctx, msgID)
	if err != nil {
		return fmt.Errorf("MessageService.SoftDelete get: %w", err)
	}

	isOwner := msg.UserID == requestorID
	isMod := role == "moderator" || role == "admin"
	if !isOwner && !isMod {
		slog.Warn("[MessageService.SoftDelete] forbidden", "msg_id", msgID, "requestor", requestorID, "role", role)
		return domain.ErrForbidden
	}

	if err := s.messages.SoftDelete(ctx, msgID); err != nil {
		return fmt.Errorf("MessageService.SoftDelete: %w", err)
	}
	slog.Info("[MessageService.SoftDelete] deleted", "msg_id", msgID, "by", requestorID)
	return nil
}
