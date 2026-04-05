package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"chatsem/shared/domain"
	"chatsem/shared/pkg/jwt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TokenRequest holds the inputs for a token exchange.
type TokenRequest struct {
	ExternalUserID string
	EventID        uuid.UUID
	Name           string
	Role           string
	APISecret      string // raw secret from Authorization header (Bearer <secret>)
}

// AuthService handles SSO token exchange.
type AuthService struct {
	eventRepo domain.EventRepository
	userRepo  domain.UserRepository
	jwtSecret string
	jwtTTL    time.Duration
}

// NewAuthService creates an AuthService.
func NewAuthService(
	eventRepo domain.EventRepository,
	userRepo domain.UserRepository,
	jwtSecret string,
	jwtTTL time.Duration,
) *AuthService {
	return &AuthService{
		eventRepo: eventRepo,
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}
}

// ExchangeToken validates the request and issues a signed JWT.
func (s *AuthService) ExchangeToken(ctx context.Context, req TokenRequest) (string, error) {
	slog.Debug("[AuthService.ExchangeToken] request",
		"external_user_id", req.ExternalUserID,
		"event_id", req.EventID,
		"role", req.Role,
	)

	if req.Role != string(domain.RoleUser) && req.Role != string(domain.RoleModerator) && req.Role != string(domain.RoleAdmin) {
		return "", domain.ErrInvalidRole
	}

	event, err := s.eventRepo.GetByID(ctx, req.EventID)
	if err != nil {
		return "", fmt.Errorf("AuthService.ExchangeToken: get event: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(event.APISecret), []byte(req.APISecret)); err != nil {
		slog.Warn("[AuthService.ExchangeToken] invalid secret attempt", "event_id", req.EventID)
		return "", domain.ErrInvalidSecret
	}

	user, err := s.userRepo.Upsert(ctx, &domain.User{
		ExternalID: req.ExternalUserID,
		EventID:    req.EventID,
		Name:       req.Name,
		Role:       domain.UserRole(req.Role),
	})
	if err != nil {
		return "", fmt.Errorf("AuthService.ExchangeToken: upsert user: %w", err)
	}

	token, err := jwt.CreateToken(&jwt.Claims{
		UserID:     user.ID,
		ExternalID: user.ExternalID,
		EventID:    user.EventID,
		Name:       user.Name,
		Role:       string(user.Role),
	}, s.jwtSecret, s.jwtTTL)
	if err != nil {
		return "", fmt.Errorf("AuthService.ExchangeToken: create token: %w", err)
	}

	slog.Info("[AuthService.ExchangeToken] token issued", "user_id", user.ID, "event_id", req.EventID)
	return token, nil
}
