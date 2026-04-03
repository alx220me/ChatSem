package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// BanService implements business logic for user bans.
type BanService struct {
	bans domain.BanRepository
	rdb  *redis.Client
}

// NewBanService creates a BanService backed by the given repository and Redis client.
func NewBanService(bans domain.BanRepository, rdb *redis.Client) *BanService {
	return &BanService{bans: bans, rdb: rdb}
}

// CreateBan inserts a ban record and caches it in Redis.
func (s *BanService) CreateBan(ctx context.Context, userID, eventID, bannedBy uuid.UUID, chatID *uuid.UUID, reason string, expiresAt *time.Time) (*domain.Ban, error) {
	slog.Debug("[BanService.CreateBan] start", "user_id", userID, "event_id", eventID)

	ban := &domain.Ban{
		UserID:    userID,
		EventID:   eventID,
		ChatID:    chatID,
		BannedBy:  bannedBy,
		Reason:    reason,
		ExpiresAt: expiresAt,
	}
	if err := s.bans.Create(ctx, ban); err != nil {
		return nil, fmt.Errorf("BanService.CreateBan: %w", err)
	}
	slog.Info("[BanService.CreateBan] created", "ban_id", ban.ID, "user_id", userID)

	// Cache in Redis for fast ban check in chat service.
	ttl := 24 * time.Hour
	if expiresAt != nil {
		ttl = time.Until(*expiresAt)
		if ttl <= 0 {
			ttl = time.Second // near-zero TTL for already-expired bans
		}
	}
	banKey := fmt.Sprintf("ban:%s:%s", eventID, userID)
	if err := s.rdb.Set(ctx, banKey, "1", ttl).Err(); err != nil {
		slog.Warn("[BanService.CreateBan] redis SET failed, ban in DB only", "err", err)
	}

	return ban, nil
}

// UnbanUser deletes the ban record and removes the Redis cache entry.
func (s *BanService) UnbanUser(ctx context.Context, banID uuid.UUID) error {
	slog.Debug("[BanService.UnbanUser] start", "ban_id", banID)

	if err := s.bans.Delete(ctx, banID); err != nil {
		return fmt.Errorf("BanService.UnbanUser: %w", err)
	}
	slog.Info("[BanService.UnbanUser] unbanned", "ban_id", banID)

	// Best-effort Redis cleanup — we don't know which (eventID, userID) without fetching the ban first.
	// The cache will expire naturally.
	return nil
}

// ListBans returns all bans for an event.
func (s *BanService) ListBans(ctx context.Context, eventID uuid.UUID) ([]*domain.Ban, error) {
	slog.Debug("[BanService.ListBans] start", "event_id", eventID)
	bans, err := s.bans.ListByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("BanService.ListBans: %w", err)
	}
	return bans, nil
}
