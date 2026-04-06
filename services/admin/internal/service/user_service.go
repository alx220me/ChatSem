package service

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/services/admin/internal/ports"
	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// UserService implements business logic for user management.
type UserService struct {
	users ports.UserRepository
}

// NewUserService creates a UserService backed by the given repository.
func NewUserService(users ports.UserRepository) *UserService {
	return &UserService{users: users}
}

// ListUsers returns users for an event with pagination.
func (s *UserService) ListUsers(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]*domain.User, error) {
	slog.Debug("[UserService.ListUsers] start", "event_id", eventID, "limit", limit, "offset", offset)
	users, err := s.users.ListByEventID(ctx, eventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("UserService.ListUsers: %w", err)
	}
	return users, nil
}

// UpdateRole sets a new role for the given user.
func (s *UserService) UpdateRole(ctx context.Context, userID uuid.UUID, role domain.UserRole) error {
	slog.Debug("[UserService.UpdateRole] start", "user_id", userID, "role", role)
	if err := s.users.UpdateRole(ctx, userID, role); err != nil {
		return fmt.Errorf("UserService.UpdateRole: %w", err)
	}
	slog.Info("[UserService.UpdateRole] updated", "user_id", userID, "role", role)
	return nil
}
