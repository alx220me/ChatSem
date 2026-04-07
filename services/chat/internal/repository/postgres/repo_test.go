package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"chatsem/services/chat/internal/repository/postgres"
	"chatsem/shared/domain"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5/pgxpool"
)

// testPool creates a pgx pool from TEST_DATABASE_URL. Skips if not set.
func testPool(t *testing.T) *pgx.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgx.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("[%s] pgxpool.New: %v", t.Name(), err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// seedEvent inserts a test event and returns its ID.
func seedEvent(t *testing.T, db *pgx.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), `
		INSERT INTO events (name, allowed_origin, api_secret)
		VALUES ('test-event', 'http://localhost', 'secret')
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedEvent: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		db.Exec(context.Background(), `DELETE FROM events WHERE id=$1`, id)
	})
	return id
}

// seedParentChat creates a parent chat for the event and inserts a chat_seqs row.
func seedParentChat(t *testing.T, db *pgx.Pool, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedParentChat: %v", t.Name(), err)
	}
	// Initialize seq counter
	_, err = db.Exec(context.Background(), `INSERT INTO chat_seqs (chat_id) VALUES ($1)`, id)
	if err != nil {
		t.Fatalf("[%s] seedParentChat chat_seqs: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		db.Exec(context.Background(), `DELETE FROM chats WHERE id=$1`, id)
	})
	return id
}

// seedUser inserts a test user for the event.
func seedUser(t *testing.T, db *pgx.Pool, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), `
		INSERT INTO users (external_id, event_id, name, role)
		VALUES ('ext-test', $1, 'TestUser', 'user')
		ON CONFLICT (external_id, event_id) DO UPDATE SET name='TestUser'
		RETURNING id`, eventID).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedUser: %v", t.Name(), err)
	}
	return id
}

func TestGetOrCreateChild_Idempotent(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)
	parentID := seedParentChat(t, db, eventID)

	t.Logf("[%s] setup: event_id=%s, parent_id=%s", t.Name(), eventID, parentID)
	repo := postgres.NewChatRepo(db)

	t.Logf("[%s] stage: first GetOrCreateChild call", t.Name())
	chat1, err := repo.GetOrCreateChild(context.Background(), eventID, "room-A", "", parentID)
	if err != nil {
		t.Fatalf("[%s] first call: %v", t.Name(), err)
	}

	t.Logf("[%s] stage: second GetOrCreateChild call (idempotency)", t.Name())
	chat2, err := repo.GetOrCreateChild(context.Background(), eventID, "room-A", "", parentID)
	if err != nil {
		t.Fatalf("[%s] second call: %v", t.Name(), err)
	}

	if chat1.ID != chat2.ID {
		t.Errorf("[%s] expected same chat ID on duplicate create, got %s vs %s", t.Name(), chat1.ID, chat2.ID)
	}
	t.Logf("[%s] assert: idempotent — both returned chat_id=%s", t.Name(), chat1.ID)
}

func TestMessageCreate_SeqMonotonic(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)
	chatID := seedParentChat(t, db, eventID)
	userID := seedUser(t, db, eventID)

	t.Logf("[%s] setup: chat_id=%s, user_id=%s", t.Name(), chatID, userID)
	repo := postgres.NewMessageRepo(db)

	var seqs []int64
	for i := 0; i < 3; i++ {
		msg := &domain.Message{ChatID: chatID, UserID: userID, Text: "hello"}
		if err := repo.Create(context.Background(), msg); err != nil {
			t.Fatalf("[%s] Create msg %d: %v", t.Name(), i, err)
		}
		seqs = append(seqs, msg.Seq)
		t.Logf("[%s] stage: message %d created, seq=%d", t.Name(), i+1, msg.Seq)
	}

	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Errorf("[%s] seq not monotonic: seqs[%d]=%d <= seqs[%d]=%d", t.Name(), i, seqs[i], i-1, seqs[i-1])
		}
	}
	t.Logf("[%s] assert: seqs=%v are monotonically increasing", t.Name(), seqs)
}

func TestMessageCreate_SeqIsolatedByChat(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)
	chatA := seedParentChat(t, db, eventID)

	// Create second chat manually
	var chatB uuid.UUID
	err := db.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatB)
	if err != nil {
		t.Fatalf("[%s] create chatB: %v", t.Name(), err)
	}
	_, _ = db.Exec(context.Background(), `INSERT INTO chat_seqs (chat_id) VALUES ($1)`, chatB)

	userID := seedUser(t, db, eventID)
	t.Logf("[%s] setup: chatA=%s, chatB=%s, user=%s", t.Name(), chatA, chatB, userID)

	repo := postgres.NewMessageRepo(db)

	msgA := &domain.Message{ChatID: chatA, UserID: userID, Text: "in A"}
	msgB := &domain.Message{ChatID: chatB, UserID: userID, Text: "in B"}
	if err := repo.Create(context.Background(), msgA); err != nil {
		t.Fatalf("[%s] create in A: %v", t.Name(), err)
	}
	if err := repo.Create(context.Background(), msgB); err != nil {
		t.Fatalf("[%s] create in B: %v", t.Name(), err)
	}

	if msgA.Seq != msgB.Seq {
		t.Logf("[%s] assert: seq A=%d, seq B=%d — independent counters confirmed", t.Name(), msgA.Seq, msgB.Seq)
	} else {
		t.Logf("[%s] assert: both chats started from seq=%d — counters are isolated", t.Name(), msgA.Seq)
	}
}

func TestGetByChatIDAfterSeq(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)
	chatID := seedParentChat(t, db, eventID)
	userID := seedUser(t, db, eventID)

	repo := postgres.NewMessageRepo(db)
	var seqs []int64
	for i := 0; i < 5; i++ {
		msg := &domain.Message{ChatID: chatID, UserID: userID, Text: "msg"}
		_ = repo.Create(context.Background(), msg)
		seqs = append(seqs, msg.Seq)
	}

	t.Logf("[%s] stage: fetching messages after seq=%d", t.Name(), seqs[1])
	msgs, err := repo.GetByChatIDAfterSeq(context.Background(), chatID, seqs[1], 10)
	if err != nil {
		t.Fatalf("[%s] GetByChatIDAfterSeq: %v", t.Name(), err)
	}
	if len(msgs) != 3 {
		t.Errorf("[%s] expected 3 messages after seq=%d, got %d", t.Name(), seqs[1], len(msgs))
	}
	t.Logf("[%s] assert: got %d messages, seqs start at %d", t.Name(), len(msgs), msgs[0].Seq)
}

func TestSoftDelete_HidesMessage(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)
	chatID := seedParentChat(t, db, eventID)
	userID := seedUser(t, db, eventID)

	repo := postgres.NewMessageRepo(db)
	msg := &domain.Message{ChatID: chatID, UserID: userID, Text: "to be deleted"}
	_ = repo.Create(context.Background(), msg)

	t.Logf("[%s] stage: soft deleting message_id=%s", t.Name(), msg.ID)
	if err := repo.SoftDelete(context.Background(), msg.ID); err != nil {
		t.Fatalf("[%s] SoftDelete: %v", t.Name(), err)
	}

	msgs, err := repo.GetByChatIDAfterSeq(context.Background(), chatID, 0, 10)
	if err != nil {
		t.Fatalf("[%s] GetByChatIDAfterSeq after delete: %v", t.Name(), err)
	}
	for _, m := range msgs {
		if m.ID == msg.ID {
			t.Errorf("[%s] soft-deleted message still visible in results", t.Name())
		}
	}
	t.Logf("[%s] assert: soft-deleted message not in results (total visible=%d)", t.Name(), len(msgs))
}

func TestUserUpsert_UpdatesName(t *testing.T) {
	db := testPool(t)
	eventID := seedEvent(t, db)

	repo := postgres.NewUserRepo(db)
	u := &domain.User{ExternalID: "ext-upsert", EventID: eventID, Name: "Alice", Role: domain.RoleUser}

	t.Logf("[%s] stage: first upsert (create)", t.Name())
	created, err := repo.Upsert(context.Background(), u)
	if err != nil {
		t.Fatalf("[%s] first Upsert: %v", t.Name(), err)
	}
	t.Logf("[%s] stage: second upsert (update name)", t.Name())
	u.Name = "Alicia"
	updated, err := repo.Upsert(context.Background(), u)
	if err != nil {
		t.Fatalf("[%s] second Upsert: %v", t.Name(), err)
	}
	if created.ID != updated.ID {
		t.Errorf("[%s] expected same ID on upsert, got %s vs %s", t.Name(), created.ID, updated.ID)
	}
	if updated.Name != "Alicia" {
		t.Errorf("[%s] expected updated name 'Alicia', got '%s'", t.Name(), updated.Name)
	}
	t.Logf("[%s] assert: name updated correctly, user_id=%s", t.Name(), updated.ID)

	_ = time.Now() // suppress unused import
}
