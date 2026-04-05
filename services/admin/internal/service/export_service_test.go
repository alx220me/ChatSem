package service_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	adminpostgres "chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedParentChatForExport inserts a parent chat for an event and initialises its seq counter.
func seedParentChatForExport(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	var chatID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, parent_id, external_room_id, type)
		VALUES ($1, NULL, NULL, 'parent')
		RETURNING id`,
		eventID).Scan(&chatID)
	if err != nil {
		t.Fatalf("[%s] seedParentChatForExport: %v", t.Name(), err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO chat_seqs (chat_id, last_seq) VALUES ($1, 0)`, chatID)
	if err != nil {
		t.Fatalf("[%s] seedParentChatForExport: init seq: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM messages WHERE chat_id=$1`, chatID)
		pool.Exec(context.Background(), `DELETE FROM chat_seqs WHERE chat_id=$1`, chatID)
		pool.Exec(context.Background(), `DELETE FROM chats WHERE id=$1`, chatID)
	})
	return chatID
}

// seedMessageAt inserts a message with the specified created_at timestamp.
func seedMessageAt(t *testing.T, pool *pgxpool.Pool, chatID, userID uuid.UUID, text string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var msgID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		WITH next_seq AS (
			UPDATE chat_seqs SET last_seq = last_seq + 1 WHERE chat_id = $1 RETURNING last_seq
		)
		INSERT INTO messages (id, chat_id, user_id, text, seq, created_at)
		SELECT gen_random_uuid(), $1, $2, $3, next_seq.last_seq, $4
		FROM next_seq
		RETURNING id`,
		chatID, userID, text, createdAt).Scan(&msgID)
	if err != nil {
		t.Fatalf("[%s] seedMessageAt: %v", t.Name(), err)
	}
	return msgID
}

// seedDeletedMessage inserts a soft-deleted message.
func seedDeletedMessage(t *testing.T, pool *pgxpool.Pool, chatID, userID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		WITH next_seq AS (
			UPDATE chat_seqs SET last_seq = last_seq + 1 WHERE chat_id = $1 RETURNING last_seq
		)
		INSERT INTO messages (id, chat_id, user_id, text, seq, created_at, deleted_at)
		SELECT gen_random_uuid(), $1, $2, 'deleted', next_seq.last_seq, NOW(), NOW()
		FROM next_seq`,
		chatID, userID)
	if err != nil {
		t.Fatalf("[%s] seedDeletedMessage: %v", t.Name(), err)
	}
}

// exportSvc returns a ready ExportService backed by the admin message repo.
func exportSvc(pool *pgxpool.Pool) *service.ExportService {
	return service.NewExportService(adminpostgres.NewMessageRepo(pool))
}

// --- tests ---

func TestExportCSV_AllMessages(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	chatID := seedParentChatForExport(t, pool, eventID)

	now := time.Now()
	for i := 0; i < 10; i++ {
		seedMessageAt(t, pool, chatID, userID, "msg", now.Add(time.Duration(i)*time.Second))
	}

	var buf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &buf, chatID, "csv", nil, nil); err != nil {
		t.Fatalf("[%s] ExportMessages: %v", t.Name(), err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("[%s] csv.ReadAll: %v", t.Name(), err)
	}
	// 1 header + 10 data rows
	if len(records) != 11 {
		t.Errorf("[%s] expected 11 CSV records (1 header + 10 rows), got %d", t.Name(), len(records))
	}
	if records[0][0] != "id" {
		t.Errorf("[%s] expected header 'id', got %q", t.Name(), records[0][0])
	}
	t.Logf("[%s] exported %d rows", t.Name(), len(records)-1)
}

func TestExportJSON_AllMessages(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	chatID := seedParentChatForExport(t, pool, eventID)

	now := time.Now()
	for i := 0; i < 10; i++ {
		seedMessageAt(t, pool, chatID, userID, "msg", now.Add(time.Duration(i)*time.Second))
	}

	var buf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &buf, chatID, "json", nil, nil); err != nil {
		t.Fatalf("[%s] ExportMessages: %v", t.Name(), err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 10 {
		t.Errorf("[%s] expected 10 NDJSON lines, got %d", t.Name(), len(lines))
	}
	var msg domain.Message
	if err := json.Unmarshal([]byte(lines[0]), &msg); err != nil {
		t.Errorf("[%s] failed to parse first NDJSON line: %v", t.Name(), err)
	}
	t.Logf("[%s] exported %d rows", t.Name(), len(lines))
}

func TestExport_DateFilter(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	chatID := seedParentChatForExport(t, pool, eventID)

	base := time.Now().UTC().Truncate(time.Second)
	seedMessageAt(t, pool, chatID, userID, "old", base.Add(-2*time.Hour))
	seedMessageAt(t, pool, chatID, userID, "in-range", base)
	seedMessageAt(t, pool, chatID, userID, "in-range-2", base.Add(time.Hour))
	seedMessageAt(t, pool, chatID, userID, "future", base.Add(3*time.Hour))

	from := base.Add(-30 * time.Minute)
	to := base.Add(90 * time.Minute)

	var buf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &buf, chatID, "csv", &from, &to); err != nil {
		t.Fatalf("[%s] ExportMessages: %v", t.Name(), err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("[%s] csv.ReadAll: %v", t.Name(), err)
	}
	// header + 2 in-range rows
	if len(records) != 3 {
		t.Errorf("[%s] expected 3 records (1 header + 2 rows), got %d", t.Name(), len(records))
	}
	t.Logf("[%s] exported %d rows", t.Name(), len(records)-1)
}

func TestExport_BatchingWorks(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	chatID := seedParentChatForExport(t, pool, eventID)

	now := time.Now()
	for i := 0; i < 250; i++ {
		seedMessageAt(t, pool, chatID, userID, "msg", now.Add(time.Duration(i)*time.Millisecond))
	}

	var buf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &buf, chatID, "csv", nil, nil); err != nil {
		t.Fatalf("[%s] ExportMessages: %v", t.Name(), err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("[%s] csv.ReadAll: %v", t.Name(), err)
	}
	// header + 250 rows
	if len(records) != 251 {
		t.Errorf("[%s] expected 251 records (1 header + 250 rows), got %d", t.Name(), len(records))
	}
	t.Logf("[%s] exported %d rows", t.Name(), len(records)-1)
}

func TestExport_DeletedMessages(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	chatID := seedParentChatForExport(t, pool, eventID)

	now := time.Now()
	seedMessageAt(t, pool, chatID, userID, "visible", now)
	seedDeletedMessage(t, pool, chatID, userID)

	var buf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &buf, chatID, "csv", nil, nil); err != nil {
		t.Fatalf("[%s] ExportMessages: %v", t.Name(), err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("[%s] csv.ReadAll: %v", t.Name(), err)
	}
	// header + 1 visible row (deleted excluded)
	if len(records) != 2 {
		t.Errorf("[%s] expected 2 records (1 header + 1 row), got %d", t.Name(), len(records))
	}
	t.Logf("[%s] exported %d rows", t.Name(), len(records)-1)
}

func TestExport_EmptyChat(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	chatID := seedParentChatForExport(t, pool, eventID)

	var csvBuf bytes.Buffer
	svc := exportSvc(pool)
	if err := svc.ExportMessages(context.Background(), &csvBuf, chatID, "csv", nil, nil); err != nil {
		t.Fatalf("[%s] CSV ExportMessages: %v", t.Name(), err)
	}
	r := csv.NewReader(strings.NewReader(csvBuf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("[%s] csv.ReadAll: %v", t.Name(), err)
	}
	if len(records) != 1 {
		t.Errorf("[%s] expected 1 record (header only), got %d", t.Name(), len(records))
	}

	var jsonBuf bytes.Buffer
	if err := svc.ExportMessages(context.Background(), &jsonBuf, chatID, "json", nil, nil); err != nil {
		t.Fatalf("[%s] JSON ExportMessages: %v", t.Name(), err)
	}
	if strings.TrimSpace(jsonBuf.String()) != "" {
		t.Errorf("[%s] expected empty NDJSON output, got %q", t.Name(), jsonBuf.String())
	}
	t.Logf("[%s] exported 0 rows", t.Name())
}
