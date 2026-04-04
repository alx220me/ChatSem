package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"chatsem/services/admin/internal/middleware"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserHandler handles HTTP requests for user management.
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler creates a UserHandler backed by the given service.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// List handles GET /api/admin/events/{eventID}/users?limit=50&offset=0.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	slog.Debug("[UserHandler.List] request", "event_id", eventID, "limit", limit, "offset", offset)
	users, err := h.svc.ListUsers(r.Context(), eventID, limit, offset)
	if err != nil {
		slog.Warn("[UserHandler.List] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	response.JSON(w, http.StatusOK, users)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateRole handles PATCH /api/admin/users/{userID}/role.
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	rawID := chi.URLParam(r, "userID")
	userID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid user_id")
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// admin role cannot be assigned via API — only manually in DB.
	if req.Role != string(domain.RoleUser) && req.Role != string(domain.RoleModerator) {
		response.Error(w, http.StatusBadRequest, "invalid_role", "role must be 'user' or 'moderator'")
		return
	}

	slog.Debug("[UserHandler.UpdateRole] request", "user_id", userID, "new_role", req.Role, "by", claims.UserID)
	if err := h.svc.UpdateRole(r.Context(), userID, domain.UserRole(req.Role)); err != nil {
		slog.Warn("[UserHandler.UpdateRole] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to update role")
		return
	}
	slog.Info("[UserHandler.UpdateRole] role updated", "user_id", userID, "role", req.Role)
	response.JSON(w, http.StatusOK, map[string]string{"user_id": userID.String(), "role": req.Role})
}
