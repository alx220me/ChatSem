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

// BanHandler handles HTTP requests for user bans.
type BanHandler struct {
	svc *service.BanService
}

// NewBanHandler creates a BanHandler backed by the given service.
func NewBanHandler(svc *service.BanService) *BanHandler {
	return &BanHandler{svc: svc}
}

type createBanRequest struct {
	UserID    string  `json:"user_id"`
	EventID   string  `json:"event_id"`
	ChatID    *string `json:"chat_id,omitempty"`
	Reason    string  `json:"reason,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"` // RFC3339
}

// CreateBan handles POST /api/admin/bans.
func (h *BanHandler) CreateBan(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	var req createBanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid user_id")
		return
	}
	eventID, err := uuid.Parse(req.EventID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	var chatID *uuid.UUID
	if req.ChatID != nil {
		id, err := uuid.Parse(*req.ChatID)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
			return
		}
		chatID = &id
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid expires_at (RFC3339 required)")
			return
		}
		expiresAt = &t
	}

	slog.Debug("[BanHandler.CreateBan] request", "user_id", userID, "event_id", eventID, "by", claims.UserID)
	ban, err := h.svc.CreateBan(r.Context(), userID, eventID, claims.UserID, chatID, req.Reason, expiresAt)
	if err != nil {
		slog.Warn("[BanHandler.CreateBan] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to create ban")
		return
	}
	response.JSON(w, http.StatusCreated, ban)
}

// DeleteBan handles DELETE /api/admin/bans/{banID}.
func (h *BanHandler) DeleteBan(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "banID")
	banID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid ban_id")
		return
	}

	slog.Debug("[BanHandler.DeleteBan] request", "ban_id", banID)
	if err := h.svc.UnbanUser(r.Context(), banID); err != nil {
		slog.Warn("[BanHandler.DeleteBan] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to delete ban")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListBans handles GET /api/admin/events/{eventID}/bans.
func (h *BanHandler) ListBans(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	slog.Debug("[BanHandler.ListBans] request", "event_id", eventID)
	bans, err := h.svc.ListBans(r.Context(), eventID)
	if err != nil {
		slog.Warn("[BanHandler.ListBans] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list bans")
		return
	}
	response.JSON(w, http.StatusOK, bans)
}
