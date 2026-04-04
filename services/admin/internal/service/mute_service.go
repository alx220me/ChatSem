package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// MuteService implements business logic for user mutes.
type MuteService struct {
	mutes domain.MuteRepository
}

// NewMuteService creates a MuteService backed by the given repository.
func NewMuteService(mutes domain.MuteRepository) *MuteService {
	return &MuteService{mutes: mutes}
}

// CreateMute mutes a user in a chat. If the user is already muted, returns the existing mute (idempotent).
func (s *MuteService) CreateMute(ctx context.Context, chatID, userID, mutedBy uuid.UUID, reason string, expiresAt *time.Time) (*domain.Mute, error) {
	slog.Debug("[MuteService.CreateMute] muting", "chat_id", chatID, "user_id", userID)

	existing, err := s.mutes.GetActive(ctx, chatID, userID)
	if err == nil {
		slog.Warn("[MuteService.CreateMute] already muted, returning existing", "mute_id", existing.ID)
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("MuteService.CreateMute: check active: %w", err)
	}

	m := &domain.Mute{
		ChatID:    chatID,
		UserID:    userID,
		MutedBy:   mutedBy,
		Reason:    reason,
		ExpiresAt: expiresAt,
	}
	if err := s.mutes.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("MuteService.CreateMute: %w", err)
	}
	slog.Info("[MuteService.CreateMute] muted", "mute_id", m.ID)
	return m, nil
}

// UnmuteUser expires (soft-deletes) an active mute by mute ID.
func (s *MuteService) UnmuteUser(ctx context.Context, muteID uuid.UUID) error {
	slog.Debug("[MuteService.UnmuteUser] unmuting", "mute_id", muteID)
	if err := s.mutes.Expire(ctx, muteID); err != nil {
		return fmt.Errorf("MuteService.UnmuteUser: %w", err)
	}
	slog.Info("[MuteService.UnmuteUser] unmuted", "mute_id", muteID)
	return nil
}

// ListMutes returns all mutes for a chat.
func (s *MuteService) ListMutes(ctx context.Context, chatID uuid.UUID) ([]*domain.Mute, error) {
	slog.Debug("[MuteService.ListMutes] start", "chat_id", chatID)
	mutes, err := s.mutes.ListByChatID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("MuteService.ListMutes: %w", err)
	}
	return mutes, nil
}
