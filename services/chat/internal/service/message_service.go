package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"chatsem/services/chat/internal/ports"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// MessageService implements business logic for message sending, listing, and moderation.
type MessageService struct {
	messages ports.MessageRepository
	mutes    ports.MuteRepository
	broker   longpoll.Broker
	rdb      *redis.Client
}

// NewMessageService creates a MessageService backed by the given repositories and broker.
func NewMessageService(
	messages ports.MessageRepository,
	mutes ports.MuteRepository,
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
// replyToID is optional; when non-nil the message is stored as a reply to the referenced message.
func (s *MessageService) SendMessage(ctx context.Context, chatID, userID, eventID uuid.UUID, text string, replyToID *uuid.UUID) (*domain.Message, error) {
	slog.Debug("[MessageService.SendMessage] sending",
		"chat_id", chatID, "user_id", userID, "text_len", len(text), "reply_to_id", replyToID)

	if len(text) == 0 {
		return nil, domain.ErrEmptyMessage
	}
	if len(text) > 4096 {
		return nil, domain.ErrMessageTooLong
	}

	// Ban check via Redis (fast path, fail-open).
	if s.rdb != nil {
		banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
		exists, err := s.rdb.Exists(ctx, banKey).Result()
		if err != nil {
			slog.Warn("[MessageService.SendMessage] redis ban check failed, failing open", "err", err)
		} else if exists > 0 {
			slog.Warn("[MessageService.SendMessage] user is banned", "user_id", userID, "event_id", eventID)
			return nil, domain.ErrUserBanned
		}
	}

	// Mute check via DB.
	muted, err := s.mutes.IsUserMuted(ctx, userID, chatID)
	if err != nil {
		slog.Warn("[MessageService.SendMessage] mute check failed, failing open", "err", err)
	} else if muted {
		slog.Warn("[MessageService.SendMessage] user is muted", "user_id", userID, "chat_id", chatID)
		return nil, domain.ErrUserMuted
	}

	// Validate reply reference when provided.
	var replyToSeq *int64
	if replyToID != nil {
		slog.Debug("[MessageService.SendMessage] validating reply", "reply_to_id", replyToID)
		orig, err := s.messages.GetByID(ctx, *replyToID)
		if err != nil {
			slog.Warn("[MessageService.SendMessage] reply message not found", "reply_to_id", replyToID, "err", err)
			return nil, domain.ErrNotFound
		}
		if orig.ChatID != chatID {
			slog.Warn("[MessageService.SendMessage] reply cross-chat attempt",
				"reply_to_id", replyToID, "orig_chat_id", orig.ChatID, "chat_id", chatID)
			return nil, domain.ErrInvalidReply
		}
		replyToSeq = &orig.Seq
	}

	msg := &domain.Message{
		ChatID:     chatID,
		UserID:     userID,
		Text:       text,
		ReplyToID:  replyToID,
		ReplyToSeq: replyToSeq,
	}
	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("MessageService.SendMessage create: %w", err)
	}
	slog.Info("[MessageService.SendMessage] sent", "chat_id", chatID, "seq", msg.Seq, "id", msg.ID)

	// Update last_seq in Redis — used by PollHandler fast-path to detect missed messages.
	if s.rdb != nil {
		seqKey := fmt.Sprintf("chat:%s:last_seq", chatID)
		if setErr := s.rdb.Set(ctx, seqKey, msg.Seq, 7*24*time.Hour).Err(); setErr != nil {
			slog.Warn("[MessageService.SendMessage] redis last_seq update failed", "err", setErr)
		}
	}

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

	// Append deletion to a sorted set (score = delete_seq) so polls can cursor through deletions.
	if s.rdb != nil {
		counterKey := fmt.Sprintf("chat:%s:delete_seq", msg.ChatID)
		logKey := fmt.Sprintf("chat:%s:delete_log", msg.ChatID)

		deleteSeq, incrErr := s.rdb.Incr(ctx, counterKey).Result()
		if incrErr == nil {
			pipe := s.rdb.Pipeline()
			pipe.ZAdd(ctx, logKey, redis.Z{Score: float64(deleteSeq), Member: msgID.String()})
			pipe.Expire(ctx, counterKey, time.Hour)
			pipe.Expire(ctx, logKey, time.Hour)
			if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
				slog.Warn("[MessageService.SoftDelete] redis delete_log update failed", "err", pipeErr)
			}
		} else {
			slog.Warn("[MessageService.SoftDelete] redis delete_seq incr failed", "err", incrErr)
		}
	}

	if publishErr := s.broker.Publish(ctx, msg.ChatID, []byte(`{"type":"delete"}`)); publishErr != nil {
		slog.Warn("[MessageService.SoftDelete] broker publish failed", "err", publishErr)
	}

	return nil
}
