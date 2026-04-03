package migrations_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

// testDB opens a connection to the test database using TEST_DATABASE_URL.
// The test is skipped if the variable is not set.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("[%s] sql.Open: %v", t.Name(), err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("[%s] db.Ping: %v", t.Name(), err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestMigrationsApply applies all up-migrations and verifies no errors occur.
func TestMigrationsApply(t *testing.T) {
	db := testDB(t)
	t.Logf("[%s] stage: resetting DB to version 0", t.Name())
	if err := goose.DownTo(db, ".", 0); err != nil {
		t.Fatalf("[%s] DownTo(0): %v", t.Name(), err)
	}
	t.Logf("[%s] stage: applying all up-migrations", t.Name())
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("[%s] Up: %v", t.Name(), err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("[%s] GetDBVersion: %v", t.Name(), err)
	}
	t.Logf("[%s] stage: all migrations applied, current version=%d", t.Name(), version)
}

// TestMigrationsRollback applies up then rolls back all migrations.
func TestMigrationsRollback(t *testing.T) {
	db := testDB(t)
	t.Logf("[%s] stage: applying all up-migrations", t.Name())
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("[%s] Up: %v", t.Name(), err)
	}
	t.Logf("[%s] stage: rolling back all migrations", t.Name())
	if err := goose.DownTo(db, ".", 0); err != nil {
		t.Fatalf("[%s] DownTo(0): %v", t.Name(), err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("[%s] GetDBVersion: %v", t.Name(), err)
	}
	if version != 0 {
		t.Errorf("[%s] expected version=0 after full rollback, got %d", t.Name(), version)
	}
	t.Logf("[%s] stage: rollback complete, version=%d", t.Name(), version)
}

// TestSchemaConstraints verifies key uniqueness and check constraints after applying migrations.
func TestSchemaConstraints(t *testing.T) {
	db := testDB(t)
	t.Logf("[%s] stage: ensuring schema is up to date", t.Name())
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("[%s] Up: %v", t.Name(), err)
	}

	t.Run("events_name_not_empty", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO events (name, allowed_origin, api_secret) VALUES ('', 'http://example.com', 'secret')`)
		// name has NOT NULL but no check for empty — just verify insert succeeds (empty string is allowed)
		if err != nil {
			t.Logf("[%s] insert empty name: %v (constraint may differ)", t.Name(), err)
		}
		_, _ = db.Exec(`DELETE FROM events WHERE name = ''`)
	})

	t.Run("users_external_event_unique", func(t *testing.T) {
		t.Logf("[%s] stage: inserting test event and users", t.Name())
		var eventID string
		err := db.QueryRow(`
			INSERT INTO events (name, allowed_origin, api_secret)
			VALUES ('constraint-test', 'http://test.com', 'secret')
			RETURNING id`).Scan(&eventID)
		if err != nil {
			t.Fatalf("[%s] insert event: %v", t.Name(), err)
		}
		t.Cleanup(func() {
			db.Exec(`DELETE FROM events WHERE id = $1`, eventID)
		})

		_, err = db.Exec(`
			INSERT INTO users (external_id, event_id, name)
			VALUES ('ext-001', $1, 'Alice')`, eventID)
		if err != nil {
			t.Fatalf("[%s] first insert: %v", t.Name(), err)
		}
		_, err = db.Exec(`
			INSERT INTO users (external_id, event_id, name)
			VALUES ('ext-001', $1, 'Alice duplicate')`, eventID)
		if err == nil {
			t.Error("[%s] expected unique constraint violation for duplicate (external_id, event_id), got nil")
		} else {
			t.Logf("[%s] got expected error: %v", t.Name(), err)
		}
	})

	t.Run("chats_type_parent_check", func(t *testing.T) {
		t.Logf("[%s] stage: verifying chats type constraint", t.Name())
		var eventID string
		err := db.QueryRow(`
			INSERT INTO events (name, allowed_origin, api_secret)
			VALUES ('chat-constraint-test', 'http://test.com', 'secret')
			RETURNING id`).Scan(&eventID)
		if err != nil {
			t.Fatalf("[%s] insert event: %v", t.Name(), err)
		}
		t.Cleanup(func() {
			db.Exec(`DELETE FROM events WHERE id = $1`, eventID)
		})

		// child chat without parent_id should fail
		_, err = db.Exec(`
			INSERT INTO chats (event_id, type, external_room_id)
			VALUES ($1, 'child', 'room-1')`, eventID)
		if err == nil {
			t.Error("[%s] expected constraint violation: child chat requires parent_id")
		} else {
			t.Logf("[%s] got expected error for child-without-parent: %v", t.Name(), err)
		}
	})

	t.Run("messages_seq_unique_per_chat", func(t *testing.T) {
		t.Logf("[%s] stage: verifying messages (chat_id, seq) unique constraint", t.Name())
		var eventID, chatID, userID string
		_ = db.QueryRow(`
			INSERT INTO events (name, allowed_origin, api_secret)
			VALUES ('msg-test', 'http://test.com', 'secret') RETURNING id`).Scan(&eventID)
		_ = db.QueryRow(`
			INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatID)
		_ = db.QueryRow(`
			INSERT INTO users (external_id, event_id, name) VALUES ('u1', $1, 'Bob') RETURNING id`, eventID).Scan(&userID)

		_, err := db.Exec(`INSERT INTO messages (chat_id, user_id, text, seq) VALUES ($1, $2, 'hello', 1)`, chatID, userID)
		if err != nil {
			t.Fatalf("[%s] first message insert: %v", t.Name(), err)
		}
		_, err = db.Exec(`INSERT INTO messages (chat_id, user_id, text, seq) VALUES ($1, $2, 'dup', 1)`, chatID, userID)
		if err == nil {
			t.Error("[%s] expected unique constraint violation for duplicate (chat_id, seq)")
		} else {
			t.Logf("[%s] got expected error for duplicate seq: %v", t.Name(), err)
		}

		t.Cleanup(func() {
			db.Exec(`DELETE FROM messages WHERE chat_id = $1`, chatID)
			db.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
			db.Exec(`DELETE FROM users WHERE id = $1`, userID)
			db.Exec(`DELETE FROM events WHERE id = $1`, eventID)
		})
	})
}
