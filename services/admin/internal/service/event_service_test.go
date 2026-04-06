package service_test

import (
	"context"
	"testing"

	adminpostgres "chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/service"
)

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
