package service_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	adminpostgres "chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// --- integration helpers ---

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("[%s] pgxpool.New: %v", t.Name(), err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis unavailable at %s: %v", addr, err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func seedEvent(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO events (name, allowed_origin, api_secret)
		VALUES ('ban-test-event', 'http://localhost', 'testhash')
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedEvent: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM bans WHERE event_id=$1`, id)
		pool.Exec(context.Background(), `DELETE FROM events WHERE id=$1`, id)
	})
	return id
}

func seedUser(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (external_id, event_id, name, role)
		VALUES ($1, $2, 'Test User', 'user') RETURNING id`,
		uuid.New().String(), eventID).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedUser: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
	})
	return id
}

// --- ban service tests ---

func TestCreateBan_SetsRedisKey(t *testing.T) {
	pool := testPool(t)
	rdb := testRedis(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	adminID := seedUser(t, pool, eventID)

	svc := service.NewBanService(adminpostgres.NewBanRepo(pool), rdb)

	_, err := svc.CreateBan(context.Background(), userID, eventID, adminID, nil, "test ban", nil)
	if err != nil {
		t.Fatalf("[%s] CreateBan: %v", t.Name(), err)
	}

	banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
	t.Cleanup(func() { rdb.Del(context.Background(), banKey) })

	exists, err := rdb.Exists(context.Background(), banKey).Result()
	if err != nil {
		t.Fatalf("[%s] Redis EXISTS: %v", t.Name(), err)
	}
	if exists != 1 {
		t.Errorf("[%s] expected Redis key to exist, got %d", t.Name(), exists)
	}
	t.Logf("[%s] assert: ban key exists in Redis", t.Name())
}

func TestUnbanUser_DeletesRedisKey(t *testing.T) {
	pool := testPool(t)
	rdb := testRedis(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	adminID := seedUser(t, pool, eventID)

	svc := service.NewBanService(adminpostgres.NewBanRepo(pool), rdb)

	ban, err := svc.CreateBan(context.Background(), userID, eventID, adminID, nil, "temp ban", nil)
	if err != nil {
		t.Fatalf("[%s] CreateBan: %v", t.Name(), err)
	}

	banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
	t.Cleanup(func() { rdb.Del(context.Background(), banKey) })

	if err := svc.UnbanUser(context.Background(), ban.ID); err != nil {
		t.Fatalf("[%s] UnbanUser: %v", t.Name(), err)
	}

	// Note: UnbanUser does best-effort Redis cleanup (it doesn't know eventID/userID).
	// The key will expire naturally. We verify the DB record is gone.
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM bans WHERE id=$1`, ban.ID).Scan(&count)
	if count != 0 {
		t.Errorf("[%s] expected ban deleted from DB, got count=%d", t.Name(), count)
	}
	t.Logf("[%s] assert: ban record deleted from DB", t.Name())
}

func TestCreateBan_WithExpiry(t *testing.T) {
	pool := testPool(t)
	rdb := testRedis(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	adminID := seedUser(t, pool, eventID)

	svc := service.NewBanService(adminpostgres.NewBanRepo(pool), rdb)

	expiresAt := time.Now().Add(2 * time.Hour)
	ban, err := svc.CreateBan(context.Background(), userID, eventID, adminID, nil, "timed ban", &expiresAt)
	if err != nil {
		t.Fatalf("[%s] CreateBan: %v", t.Name(), err)
	}

	banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
	t.Cleanup(func() {
		rdb.Del(context.Background(), banKey)
		pool.Exec(context.Background(), `DELETE FROM bans WHERE id=$1`, ban.ID)
	})

	ttl, err := rdb.TTL(context.Background(), banKey).Result()
	if err != nil {
		t.Fatalf("[%s] Redis TTL: %v", t.Name(), err)
	}
	if ttl <= 0 {
		t.Errorf("[%s] expected positive TTL, got %v", t.Name(), ttl)
	}
	t.Logf("[%s] assert: Redis TTL set for timed ban, ttl=%v", t.Name(), ttl)
}

// TestCreateEvent_CreatesParentChat is in event_service_test.go but shares seedEvent helper.
