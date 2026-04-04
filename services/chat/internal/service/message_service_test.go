package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"chatsem/services/chat/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
)

// --- mock MessageRepository ---

type mockMessageRepo struct {
	create   func(ctx context.Context, m *domain.Message) error
	getByID  func(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	softDel  func(ctx context.Context, id uuid.UUID) error
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
func (m *mockMessageRepo) GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
	if m.getByChatIDAfterSeq != nil {
		return m.getByChatIDAfterSeq(ctx, chatID, afterSeq, limit)
	}
	return nil, nil
}
func (m *mockMessageRepo) ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *mockMessageRepo) GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *mockMessageRepo) CountByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time) (int64, error) {
	return 0, nil
}

// --- mock MuteRepository ---

type mockMuteRepo struct {
	isUserMuted func(ctx context.Context, userID, chatID uuid.UUID) (bool, error)
}

func (m *mockMuteRepo) Create(ctx context.Context, mute *domain.Mute) error { return nil }
func (m *mockMuteRepo) Delete(ctx context.Context, id uuid.UUID) error       { return nil }
func (m *mockMuteRepo) GetActive(ctx context.Context, chatID, userID uuid.UUID) (*domain.Mute, error) {
	return nil, domain.ErrNotFound
}
func (m *mockMuteRepo) Expire(ctx context.Context, muteID uuid.UUID) error { return nil }
func (m *mockMuteRepo) ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*domain.Mute, error) {
	return nil, nil
}
func (m *mockMuteRepo) IsUserMuted(ctx context.Context, userID, chatID uuid.UUID) (bool, error) {
	if m.isUserMuted != nil {
		return m.isUserMuted(ctx, userID, chatID)
	}
	return false, nil
}

// newSvc creates a MessageService with in-memory broker and nil Redis (ban check will fail-open).
func newSvc(msgs domain.MessageRepository, mutes domain.MuteRepository) *service.MessageService {
	broker := longpoll.NewInMemoryBroker()
	return service.NewMessageService(msgs, mutes, broker, nil)
}

func TestSendMessage_EmptyText(t *testing.T) {
	svc := newSvc(&mockMessageRepo{}, &mockMuteRepo{})
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "")
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
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), string(long))
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
	_, err := svc.SendMessage(context.Background(), chatID, uuid.New(), uuid.New(), "hello")
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
	msg, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "hello")
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
	msg, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), uuid.New(), "test")
	if err != nil {
		t.Fatalf("[%s] broker failure should not propagate: %v", t.Name(), err)
	}
	if msg == nil {
		t.Errorf("[%s] expected message, got nil", t.Name())
	}
	t.Logf("[%s] assert: broker failure is fail-safe, message returned", t.Name())
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
