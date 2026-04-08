package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	adminpostgres "chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/ports"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// --- mock EventRepository for unit tests ---

type mockEventRepo struct {
	ports.EventRepository // embed to satisfy unused methods
	updateAPISecretErr    error
	updatedID             uuid.UUID
	updatedHash           string
}

func (m *mockEventRepo) UpdateAPISecret(_ context.Context, id uuid.UUID, hashedSecret string) error {
	m.updatedID = id
	m.updatedHash = hashedSecret
	return m.updateAPISecretErr
}

func (m *mockEventRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
	return nil, nil
}
func (m *mockEventRepo) Create(_ context.Context, _ *domain.Event) error { return nil }
func (m *mockEventRepo) List(_ context.Context) ([]*domain.Event, error)  { return nil, nil }

// --- mock ChatRepository (no-op) ---

type mockChatRepo struct{ ports.ChatRepository }

func (m *mockChatRepo) CreateParent(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (m *mockChatRepo) InitChatSeq(_ context.Context, _ uuid.UUID) error { return nil }

// --- unit tests for RotateAPISecret ---

func TestRotateAPISecret_Success(t *testing.T) {
	eventID := uuid.New()
	repo := &mockEventRepo{}
	svc := service.NewEventService(repo, &mockChatRepo{})

	plainSecret, err := svc.RotateAPISecret(context.Background(), eventID)
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if len(plainSecret) != 64 {
		t.Errorf("[%s] expected 64-char hex secret, got %d chars", t.Name(), len(plainSecret))
	}
	if repo.updatedID != eventID {
		t.Errorf("[%s] expected update called with %s, got %s", t.Name(), eventID, repo.updatedID)
	}
	// Verify stored value is a valid bcrypt hash of the returned plaintext.
	if err := bcrypt.CompareHashAndPassword([]byte(repo.updatedHash), []byte(plainSecret)); err != nil {
		t.Errorf("[%s] stored hash does not match returned plaintext: %v", t.Name(), err)
	}
	t.Logf("[%s] assert: 64-char hex secret, bcrypt hash stored", t.Name())
}

func TestRotateAPISecret_NotFound(t *testing.T) {
	eventID := uuid.New()
	notFoundErr := errors.New("event not found: " + eventID.String())
	repo := &mockEventRepo{updateAPISecretErr: notFoundErr}
	svc := service.NewEventService(repo, &mockChatRepo{})

	_, err := svc.RotateAPISecret(context.Background(), eventID)
	if err == nil {
		t.Fatalf("[%s] expected error, got nil", t.Name())
	}
	if !errors.Is(err, notFoundErr) && !strings.Contains(err.Error(), "not found") {
		t.Errorf("[%s] expected not-found error, got: %v", t.Name(), err)
	}
	t.Logf("[%s] assert: not-found error propagated", t.Name())
}


func TestCreateEvent_CreatesParentChat(t *testing.T) {
	pool := testPool(t)
	t.Logf("[%s] setup: applying migrations (schema assumed present)", t.Name())

	eventRepo := adminpostgres.NewEventRepo(pool)
	chatRepo := adminpostgres.NewChatRepo(pool)
	svc := service.NewEventService(eventRepo, chatRepo)

	event, plainSecret, err := svc.CreateEvent(context.Background(), "test-event-chat", "http://localhost")
	if err != nil {
		t.Fatalf("[%s] CreateEvent: %v", t.Name(), err)
	}
	if len(plainSecret) != 64 {
		t.Errorf("[%s] expected 64-char secret, got %d chars", t.Name(), len(plainSecret))
	}
	t.Logf("[%s] assert: secret is 64-char hex", t.Name())

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM chats WHERE event_id=$1`, event.ID)
		pool.Exec(context.Background(), `DELETE FROM events WHERE id=$1`, event.ID)
	})

	// Assert: parent chat created.
	var chatCount int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM chats WHERE event_id=$1 AND type='parent'`, event.ID).Scan(&chatCount)
	if chatCount != 1 {
		t.Errorf("[%s] expected 1 parent chat, got %d", t.Name(), chatCount)
	}

	// Assert: chat_seqs row created.
	var seqCount int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM chat_seqs cs
		 JOIN chats c ON cs.chat_id = c.id
		 WHERE c.event_id=$1`, event.ID).Scan(&seqCount)
	if seqCount != 1 {
		t.Errorf("[%s] expected 1 chat_seqs row, got %d", t.Name(), seqCount)
	}

	t.Logf("[%s] assert: CreateEvent → parent chat + chat_seq in DB", t.Name())
}
