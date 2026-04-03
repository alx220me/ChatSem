package service_test

import (
	"context"
	"errors"
	"testing"

	"chatsem/services/chat/internal/service"
	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// --- mock ChatRepository ---

type mockChatRepo struct {
	listByEventID      func(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error)
	getParentByEventID func(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error)
	getOrCreateChild   func(ctx context.Context, eventID uuid.UUID, roomID string, parentID uuid.UUID) (*domain.Chat, error)
	getByID            func(ctx context.Context, id uuid.UUID) (*domain.Chat, error)
	initChatSeq        func(ctx context.Context, chatID uuid.UUID) error
}

func (m *mockChatRepo) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*domain.Chat, error) {
	return m.listByEventID(ctx, eventID)
}
func (m *mockChatRepo) GetParentByEventID(ctx context.Context, eventID uuid.UUID) (*domain.Chat, error) {
	return m.getParentByEventID(ctx, eventID)
}
func (m *mockChatRepo) GetOrCreateChild(ctx context.Context, eventID uuid.UUID, roomID string, parentID uuid.UUID) (*domain.Chat, error) {
	return m.getOrCreateChild(ctx, eventID, roomID, parentID)
}
func (m *mockChatRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	return m.getByID(ctx, id)
}
func (m *mockChatRepo) InitChatSeq(ctx context.Context, chatID uuid.UUID) error {
	if m.initChatSeq != nil {
		return m.initChatSeq(ctx, chatID)
	}
	return nil
}
func (m *mockChatRepo) GetSettings(ctx context.Context, chatID uuid.UUID) ([]byte, error) {
	return nil, nil
}
func (m *mockChatRepo) UpdateSettings(ctx context.Context, chatID uuid.UUID, settings []byte) error {
	return nil
}

func TestGetOrCreateChildChat_FastPath(t *testing.T) {
	eventID := uuid.New()
	roomID := "room-A"
	existingChat := &domain.Chat{
		ID:             uuid.New(),
		EventID:        eventID,
		Type:           domain.TypeChild,
		ExternalRoomID: roomID,
	}

	getParentCalled := false
	repo := &mockChatRepo{
		listByEventID: func(ctx context.Context, eID uuid.UUID) ([]*domain.Chat, error) {
			return []*domain.Chat{existingChat}, nil
		},
		getParentByEventID: func(ctx context.Context, eID uuid.UUID) (*domain.Chat, error) {
			getParentCalled = true
			return nil, errors.New("should not be called")
		},
	}

	svc := service.NewChatService(repo)
	result, err := svc.GetOrCreateChildChat(context.Background(), eventID, roomID)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if result.Chat.ID != existingChat.ID {
		t.Errorf("[%s] expected chat_id=%s, got %s", t.Name(), existingChat.ID, result.Chat.ID)
	}
	if result.IsNew {
		t.Errorf("[%s] expected IsNew=false on fast path", t.Name())
	}
	if getParentCalled {
		t.Errorf("[%s] GetParentByEventID should not be called on fast path", t.Name())
	}
	t.Logf("[%s] assert: fast path returned chat_id=%s, IsNew=false", t.Name(), result.Chat.ID)
}

func TestGetOrCreateChildChat_SlowPath(t *testing.T) {
	eventID := uuid.New()
	parentID := uuid.New()
	roomID := "room-B"
	newChild := &domain.Chat{
		ID:             uuid.New(),
		EventID:        eventID,
		Type:           domain.TypeChild,
		ExternalRoomID: roomID,
	}

	repo := &mockChatRepo{
		listByEventID: func(ctx context.Context, eID uuid.UUID) ([]*domain.Chat, error) {
			// No children yet
			return []*domain.Chat{}, nil
		},
		getParentByEventID: func(ctx context.Context, eID uuid.UUID) (*domain.Chat, error) {
			return &domain.Chat{ID: parentID, Type: domain.TypeParent}, nil
		},
		getOrCreateChild: func(ctx context.Context, eID uuid.UUID, rID string, pID uuid.UUID) (*domain.Chat, error) {
			if pID != parentID {
				t.Errorf("[%s] expected parentID=%s, got %s", t.Name(), parentID, pID)
			}
			return newChild, nil
		},
	}

	svc := service.NewChatService(repo)
	result, err := svc.GetOrCreateChildChat(context.Background(), eventID, roomID)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if result.Chat.ID != newChild.ID {
		t.Errorf("[%s] expected chat_id=%s, got %s", t.Name(), newChild.ID, result.Chat.ID)
	}
	if !result.IsNew {
		t.Errorf("[%s] expected IsNew=true on slow path", t.Name())
	}
	t.Logf("[%s] assert: slow path created chat_id=%s, IsNew=true", t.Name(), result.Chat.ID)
}

func TestGetOrCreateChildChat_ConflictFallback(t *testing.T) {
	eventID := uuid.New()
	parentID := uuid.New()
	roomID := "room-C"
	winner := &domain.Chat{ID: uuid.New(), EventID: eventID, Type: domain.TypeChild, ExternalRoomID: roomID}

	createCalls := 0
	repo := &mockChatRepo{
		listByEventID: func(ctx context.Context, eID uuid.UUID) ([]*domain.Chat, error) {
			return []*domain.Chat{}, nil
		},
		getParentByEventID: func(ctx context.Context, eID uuid.UUID) (*domain.Chat, error) {
			return &domain.Chat{ID: parentID, Type: domain.TypeParent}, nil
		},
		getOrCreateChild: func(ctx context.Context, eID uuid.UUID, rID string, pID uuid.UUID) (*domain.Chat, error) {
			createCalls++
			// Simulate race: GetOrCreateChild still returns the winner (ON CONFLICT DO NOTHING + SELECT).
			return winner, nil
		},
	}

	svc := service.NewChatService(repo)
	result, err := svc.GetOrCreateChildChat(context.Background(), eventID, roomID)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if result.Chat.ID != winner.ID {
		t.Errorf("[%s] expected winner chat_id=%s, got %s", t.Name(), winner.ID, result.Chat.ID)
	}
	t.Logf("[%s] assert: conflict resolved to winner chat_id=%s, createCalls=%d", t.Name(), result.Chat.ID, createCalls)
}

func TestGetParentChat_NotFound(t *testing.T) {
	eventID := uuid.New()
	repo := &mockChatRepo{
		getParentByEventID: func(ctx context.Context, eID uuid.UUID) (*domain.Chat, error) {
			return nil, domain.ErrChatNotFound
		},
	}

	svc := service.NewChatService(repo)
	_, err := svc.GetParentChat(context.Background(), eventID)
	if err == nil {
		t.Fatalf("[%s] expected error, got nil", t.Name())
	}
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Errorf("[%s] expected ErrChatNotFound, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: ErrChatNotFound propagated correctly", t.Name())
}
