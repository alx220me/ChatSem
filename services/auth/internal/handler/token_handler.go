package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"chatsem/services/auth/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/response"

	"github.com/google/uuid"
)

// TokenExchanger is the interface for the auth service used by the handler.
type TokenExchanger interface {
	ExchangeToken(ctx context.Context, req service.TokenRequest) (string, error)
}

// TokenHandler handles token exchange requests.
type TokenHandler struct {
	svc TokenExchanger
}

// NewTokenHandler creates a TokenHandler.
func NewTokenHandler(svc TokenExchanger) *TokenHandler {
	return &TokenHandler{svc: svc}
}

type exchangeRequest struct {
	ExternalUserID string    `json:"external_user_id"`
	EventID        uuid.UUID `json:"event_id"`
	Name           string    `json:"name"`
	Role           string    `json:"role"`
}

// ExchangeToken handles POST /api/auth/token.
func (h *TokenHandler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	apiSecret := strings.TrimPrefix(authHeader, "Bearer ")
	if apiSecret == "" || apiSecret == authHeader {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
		return
	}

	var req exchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	slog.Debug("[TokenHandler.ExchangeToken] request", "event_id", req.EventID, "role", req.Role)

	token, err := h.svc.ExchangeToken(r.Context(), service.TokenRequest{
		ExternalUserID: req.ExternalUserID,
		EventID:        req.EventID,
		Name:           req.Name,
		Role:           req.Role,
		APISecret:      apiSecret,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			response.Error(w, http.StatusNotFound, "not_found", "event not found")
		case errors.Is(err, domain.ErrInvalidSecret):
			response.Error(w, http.StatusUnauthorized, "invalid_secret", "invalid api secret")
		case errors.Is(err, domain.ErrInvalidRole):
			response.Error(w, http.StatusBadRequest, "invalid_role", "invalid role")
		default:
			slog.Error("[TokenHandler.ExchangeToken] internal error", "err", err)
			response.Error(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	slog.Info("[TokenHandler.ExchangeToken] token issued")
	response.JSON(w, http.StatusOK, map[string]string{"token": token})
}
