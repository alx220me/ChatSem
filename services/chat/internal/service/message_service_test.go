package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"chatsem/services/chat/internal/ports"
	"chatsem/services/chat/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
)

// --- mock MessageRepository ---

type mockMessageRepo struct {
	create              func(ctx context.Context, m *domain.Message) error
	getByID             func(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	softDel             func(ctx context.Context, id uuid.UUID) error
	update              func(ctx context.Context, id uuid.UUID, newText string) error
	getByChatIDAfterSeq func(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error)
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *domain.Message) error {
	return m.create(ctx, msg)
}
func (m *mockMessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	return m.getByID(ctx, id)
}
func (m *mockMessageRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.softDel(ctx, id)
}
func (m *mockMessageRepo) Update(ctx context.Context, id uuid.UUID, newText string) error {
	if m.update != nil {
		return m.update(ctx, id, newText)
	}
	return nil
}
func (m *mockMessageRepo) GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
	if m.getByChatIDAfterSeq != nil {
		return m.getByChatIDAfterSeq(ctx, chatID, afterSeq, limit)
	}
	return nil, nil
}
func (m *mockMessageRepo) ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, error) {
	return nil, nil
}
// --- mock MuteRepository ---

type mockMuteRepo struct {
	isUserMuted func(ctx context.Context, userID, chatID uuid.UUID) (bool, error)
}

func (m *mockMuteRepo) Create(ctx context.Context, mute *domain.Mute) error { return nil }
func (m *mockMuteRepo) GetActive(ctx context.Context, chatID, userID uuid.UUID) (*domain.Mute, error) {
	return nil, domain.ErrNotFound
}
func (m *mockMuteRepo) IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error) {
	if m.isUserMuted != nil {
		return m.isUserMuted(ctx, userID, chatID)
	}
	return false, nil
}

// newSvc creates a MessageService with in-memory broker and nil Redis (ban check will fail-open).
func newSvc(msgs ports.MessageRepository, mutes ports.MuteRepository) *service.MessageService {
	broker := longpoll.NewInMemoryBroker()
	return service.NewMessageService(msgs, mutes, broker, nil)
}

func TestSendMessage_EmptyText(t *testing.T) {
	svc := newSvc(&mockMessageRepo{}, &mockMuteRepo{})
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "", nil)
	if !errors.Is(err, domain.ErrEmptyMessage) {
		t.Errorf("[%s] expected ErrEmptyMessage, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: empty text → ErrEmptyMessage", t.Name())
}

func TestSendMessage_TooLongText(t *testing.T) {
	long := make([]byte, 4097)
	for i := range long {
		long[i] = 'a'
	}
	svc := newSvc(&mockMessageRepo{}, &mockMuteRepo{})
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), string(long), nil)
	if !errors.Is(err, domain.ErrMessageTooLong) {
		t.Errorf("[%s] expected ErrMessageTooLong, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: 4097-char text → ErrMessageTooLong", t.Name())
}

func TestSendMessage_UserMuted(t *testing.T) {
	chatID := uuid.New()
	muteRepo := &mockMuteRepo{
		isUserMuted: func(ctx context.Context, userID, cID uuid.UUID) (bool, error) {
			if cID == chatID {
				return true, nil
			}
			return false, nil
		},
	}
	svc := newSvc(&mockMessageRepo{}, muteRepo)
	_, err := svc.SendMessage(context.Background(), chatID, uuid.New(), uuid.New(), "hello", nil)
	if !errors.Is(err, domain.ErrUserMuted) {
		t.Errorf("[%s] expected ErrUserMuted, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: muted user → ErrUserMuted", t.Name())
}

func TestSendMessage_Success(t *testing.T) {
	msgRepo := &mockMessageRepo{
		create: func(ctx context.Context, m *domain.Message) error {
			m.ID = uuid.New()
			m.Seq = 42
			m.CreatedAt = time.Now()
			return nil
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	msg, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "hello", nil)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if msg.Seq != 42 {
		t.Errorf("[%s] expected seq=42, got %d", t.Name(), msg.Seq)
	}
	t.Logf("[%s] assert: message sent, seq=%d, id=%s", t.Name(), msg.Seq, msg.ID)
}

func TestSendMessage_BrokerFailureSafe(t *testing.T) {
	msgRepo := &mockMessageRepo{
		create: func(ctx context.Context, m *domain.Message) error {
			m.ID = uuid.New()
			m.Seq = 1
			m.CreatedAt = time.Now()
			return nil
		},
	}
	// Broker will fail because Redis is nil — service must still return the message.
	svc := newSvc(msgRepo, &mockMuteRepo{})
	msg, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "test", nil)
	if err != nil {
		t.Fatalf("[%s] broker failure should not propagate: %v", t.Name(), err)
	}
	if msg == nil {
		t.Errorf("[%s] expected message, got nil", t.Name())
	}
	t.Logf("[%s] assert: broker failure is fail-safe, message returned", t.Name())
}

func TestSendMessage_WithReply_Success(t *testing.T) {
	chatID := uuid.New()
	replyID := uuid.New()
	replySeq := int64(7)

	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			if id == replyID {
				return &domain.Message{ID: replyID, ChatID: chatID, Seq: replySeq}, nil
			}
			return nil, domain.ErrNotFound
		},
		create: func(ctx context.Context, m *domain.Message) error {
			m.ID = uuid.New()
			m.Seq = 10
			m.CreatedAt = time.Now()
			return nil
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	msg, err := svc.SendMessage(context.Background(), chatID, uuid.New(), uuid.New(), "reply text", &replyID)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if msg.ReplyToID == nil || *msg.ReplyToID != replyID {
		t.Errorf("[%s] expected ReplyToID=%s, got %v", t.Name(), replyID, msg.ReplyToID)
	}
	if msg.ReplyToSeq == nil || *msg.ReplyToSeq != replySeq {
		t.Errorf("[%s] expected ReplyToSeq=%d, got %v", t.Name(), replySeq, msg.ReplyToSeq)
	}
	t.Logf("[%s] assert: reply message sent, reply_to_id=%s, reply_to_seq=%d", t.Name(), replyID, replySeq)
}

func TestSendMessage_WithReply_CrossChat(t *testing.T) {
	chatID := uuid.New()
	otherChatID := uuid.New()
	replyID := uuid.New()

	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			// reply message belongs to a different chat
			return &domain.Message{ID: replyID, ChatID: otherChatID, Seq: 1}, nil
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	_, err := svc.SendMessage(context.Background(), chatID, uuid.New(), uuid.New(), "text", &replyID)
	if !errors.Is(err, domain.ErrInvalidReply) {
		t.Errorf("[%s] expected ErrInvalidReply, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: cross-chat reply → ErrInvalidReply", t.Name())
}

func TestSendMessage_WithReply_NotFound(t *testing.T) {
	replyID := uuid.New()

	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return nil, domain.ErrNotFound
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "text", &replyID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("[%s] expected ErrNotFound, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: reply not found → ErrNotFound", t.Name())
}

func TestSoftDelete_OwnerCanDelete(t *testing.T) {
	ownerID := uuid.New()
	msgID := uuid.New()
	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return &domain.Message{ID: msgID, UserID: ownerID}, nil
		},
		softDel: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	if err := svc.SoftDelete(context.Background(), msgID, ownerID, "user"); err != nil {
		t.Fatalf("[%s] owner should be able to delete: %v", t.Name(), err)
	}
	t.Logf("[%s] assert: owner deleted message", t.Name())
}

func TestSoftDelete_ModeratorCanDelete(t *testing.T) {
	modID := uuid.New()
	msgID := uuid.New()
	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return &domain.Message{ID: msgID, UserID: uuid.New()}, nil // different owner
		},
		softDel: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	if err := svc.SoftDelete(context.Background(), msgID, modID, "moderator"); err != nil {
		t.Fatalf("[%s] moderator should be able to delete: %v", t.Name(), err)
	}
	t.Logf("[%s] assert: moderator deleted message", t.Name())
}

func TestSoftDelete_ForeignUserForbidden(t *testing.T) {
	foreignID := uuid.New()
	msgID := uuid.New()
	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return &domain.Message{ID: msgID, UserID: uuid.New()}, nil // different owner
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	err := svc.SoftDelete(context.Background(), msgID, foreignID, "user")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("[%s] expected ErrForbidden, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: foreign user → ErrForbidden", t.Name())
}

// --- EditMessage tests ---

func TestEditMessage_OwnerCanEdit(t *testing.T) {
	ownerID := uuid.New()
	msgID := uuid.New()
	chatID := uuid.New()
	now := time.Now()
	updatedText := "updated text"

	callCount := 0
	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			callCount++
			msg := &domain.Message{ID: msgID, ChatID: chatID, UserID: ownerID, Text: "original"}
			if callCount > 1 {
				// second call after update — return updated text and edited_at
				msg.Text = updatedText
				msg.EditedAt = &now
			}
			return msg, nil
		},
		update: func(ctx context.Context, id uuid.UUID, newText string) error {
			if id != msgID {
				t.Errorf("[%s] unexpected msgID: %s", t.Name(), id)
			}
			if newText != updatedText {
				t.Errorf("[%s] unexpected text: %s", t.Name(), newText)
			}
			return nil
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	msg, err := svc.EditMessage(context.Background(), msgID, ownerID, updatedText)
	if err != nil {
		t.Fatalf("[%s] owner should be able to edit: %v", t.Name(), err)
	}
	if msg.Text != updatedText {
		t.Errorf("[%s] expected text=%q, got %q", t.Name(), updatedText, msg.Text)
	}
	t.Logf("[%s] assert: owner edited message, text=%q", t.Name(), msg.Text)
}

func TestEditMessage_NonOwnerForbidden(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	msgID := uuid.New()

	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return &domain.Message{ID: msgID, UserID: ownerID, Text: "hello"}, nil
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	_, err := svc.EditMessage(context.Background(), msgID, otherID, "new text")
	if !errors.Is(err, domain.ErrEditForbidden) {
		t.Errorf("[%s] expected ErrEditForbidden, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: non-owner → ErrEditForbidden", t.Name())
}

func TestEditMessage_NotFound(t *testing.T) {
	msgRepo := &mockMessageRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
			return nil, domain.ErrNotFound
		},
	}
	svc := newSvc(msgRepo, &mockMuteRepo{})
	_, err := svc.EditMessage(context.Background(), uuid.New(), uuid.New(), "text")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("[%s] expected ErrNotFound, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: message not found → ErrNotFound", t.Name())
}

func TestEditMessage_EmptyText(t *testing.T) {
	svc := newSvc(&mockMessageRepo{}, &mockMuteRepo{})
	_, err := svc.EditMessage(context.Background(), uuid.New(), uuid.New(), "")
	if !errors.Is(err, domain.ErrEmptyMessage) {
		t.Errorf("[%s] expected ErrEmptyMessage, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: empty text → ErrEmptyMessage", t.Name())
}

func TestEditMessage_TooLongText(t *testing.T) {
	long := make([]byte, 4097)
	for i := range long {
		long[i] = 'a'
	}
	svc := newSvc(&mockMessageRepo{}, &mockMuteRepo{})
	_, err := svc.EditMessage(context.Background(), uuid.New(), uuid.New(), string(long))
	if !errors.Is(err, domain.ErrMessageTooLong) {
		t.Errorf("[%s] expected ErrMessageTooLong, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: 4097-char text → ErrMessageTooLong", t.Name())
}
