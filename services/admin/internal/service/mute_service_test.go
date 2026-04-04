package service_test

import (
	"context"
	"errors"
	"testing"

	adminpostgres "chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// testPool / seedEvent / seedUser are defined in ban_service_test.go (same package).

func TestCreateMute_GetActive(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	muterID := seedUser(t, pool, eventID)

	// Need a chat for the mute.
	var chatID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatID)
	if err != nil {
		t.Fatalf("[%s] seedChat: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM mutes WHERE chat_id=$1`, chatID)
		pool.Exec(context.Background(), `DELETE FROM chats WHERE id=$1`, chatID)
	})

	svc := service.NewMuteService(adminpostgres.NewMuteRepo(pool))
	t.Logf("[%s] setup: muting user %s in chat %s", t.Name(), userID, chatID)

	mute, err := svc.CreateMute(context.Background(), chatID, userID, muterID, "test mute", nil)
	if err != nil {
		t.Fatalf("[%s] CreateMute: %v", t.Name(), err)
	}
	if mute.ID == uuid.Nil {
		t.Errorf("[%s] expected valid mute ID", t.Name())
	}

	// GetActive should find it.
	muteRepo := adminpostgres.NewMuteRepo(pool)
	found, err := muteRepo.GetActive(context.Background(), chatID, userID)
	if err != nil {
		t.Fatalf("[%s] GetActive: %v", t.Name(), err)
	}
	if found.ID != mute.ID {
		t.Errorf("[%s] GetActive ID=%s, want %s", t.Name(), found.ID, mute.ID)
	}
	t.Logf("[%s] assert: GetActive returns created mute", t.Name())
}

func TestCreateMute_Idempotent(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	muterID := seedUser(t, pool, eventID)

	var chatID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatID)
	if err != nil {
		t.Fatalf("[%s] seedChat: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM mutes WHERE chat_id=$1`, chatID)
		pool.Exec(context.Background(), `DELETE FROM chats WHERE id=$1`, chatID)
	})

	svc := service.NewMuteService(adminpostgres.NewMuteRepo(pool))

	mute1, err := svc.CreateMute(context.Background(), chatID, userID, muterID, "first", nil)
	if err != nil {
		t.Fatalf("[%s] first CreateMute: %v", t.Name(), err)
	}

	mute2, err := svc.CreateMute(context.Background(), chatID, userID, muterID, "second", nil)
	if err != nil {
		t.Fatalf("[%s] second CreateMute: %v", t.Name(), err)
	}

	if mute1.ID != mute2.ID {
		t.Errorf("[%s] expected same mute ID on idempotent call, got %s and %s", t.Name(), mute1.ID, mute2.ID)
	}

	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM mutes WHERE chat_id=$1 AND user_id=$2`, chatID, userID).Scan(&count)
	if count != 1 {
		t.Errorf("[%s] expected 1 mute record, got %d", t.Name(), count)
	}
	t.Logf("[%s] assert: double CreateMute → single record", t.Name())
}

func TestUnmuteUser_GetActiveNotFound(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool)
	userID := seedUser(t, pool, eventID)
	muterID := seedUser(t, pool, eventID)

	var chatID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO chats (event_id, type) VALUES ($1, 'parent') RETURNING id`, eventID).Scan(&chatID)
	if err != nil {
		t.Fatalf("[%s] seedChat: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM mutes WHERE chat_id=$1`, chatID)
		pool.Exec(context.Background(), `DELETE FROM chats WHERE id=$1`, chatID)
	})

	svc := service.NewMuteService(adminpostgres.NewMuteRepo(pool))
	muteRepo := adminpostgres.NewMuteRepo(pool)

	mute, err := svc.CreateMute(context.Background(), chatID, userID, muterID, "temp", nil)
	if err != nil {
		t.Fatalf("[%s] CreateMute: %v", t.Name(), err)
	}

	if err := svc.UnmuteUser(context.Background(), mute.ID); err != nil {
		t.Fatalf("[%s] UnmuteUser: %v", t.Name(), err)
	}

	_, err = muteRepo.GetActive(context.Background(), chatID, userID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("[%s] after Expire: expected ErrNotFound, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: after UnmuteUser → GetActive returns ErrNotFound", t.Name())
}
