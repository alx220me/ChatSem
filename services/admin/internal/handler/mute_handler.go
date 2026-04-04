package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"chatsem/services/admin/internal/middleware"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MuteHandler handles HTTP requests for user mutes.
type MuteHandler struct {
	svc *service.MuteService
}

// NewMuteHandler creates a MuteHandler backed by the given service.
func NewMuteHandler(svc *service.MuteService) *MuteHandler {
	return &MuteHandler{svc: svc}
}

type createMuteRequest struct {
	ChatID    string  `json:"chat_id"`
	UserID    string  `json:"user_id"`
	Reason    string  `json:"reason,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"` // RFC3339
}

// CreateMute handles POST /api/admin/mutes.
func (h *MuteHandler) CreateMute(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	var req createMuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid user_id")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid expires_at (RFC3339 required)")
			return
		}
		if !t.After(time.Now()) {
			response.Error(w, http.StatusBadRequest, "bad_request", "expires_at must be in the future")
			return
		}
		expiresAt = &t
	}

	slog.Debug("[MuteHandler.Create] request", "chat_id", chatID, "user_id", userID, "by", claims.UserID)
	mute, err := h.svc.CreateMute(r.Context(), chatID, userID, claims.UserID, req.Reason, expiresAt)
	if err != nil {
		slog.Warn("[MuteHandler.Create] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to create mute")
		return
	}
	response.JSON(w, http.StatusCreated, mute)
}

// DeleteMute handles DELETE /api/admin/mutes/{muteID}.
func (h *MuteHandler) DeleteMute(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "muteID")
	muteID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid mute_id")
		return
	}

	slog.Debug("[MuteHandler.Delete] request", "mute_id", muteID)
	if err := h.svc.UnmuteUser(r.Context(), muteID); err != nil {
		slog.Warn("[MuteHandler.Delete] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to delete mute")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMutes handles GET /api/admin/chats/{chatID}/mutes.
func (h *MuteHandler) ListMutes(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	slog.Debug("[MuteHandler.List] request", "chat_id", chatID)
	mutes, err := h.svc.ListMutes(r.Context(), chatID)
	if err != nil {
		slog.Warn("[MuteHandler.List] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list mutes")
		return
	}
	response.JSON(w, http.StatusOK, mutes)
}
