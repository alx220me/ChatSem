package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"chatsem/services/admin/internal/service"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChatHandler handles HTTP requests for chat management.
type ChatHandler struct {
	svc *service.EventService
}

// NewChatHandler creates a ChatHandler backed by the given event service.
func NewChatHandler(svc *service.EventService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// ListChats handles GET /api/admin/events/{eventID}/chats.
func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	slog.Debug("[ChatHandler.ListChats] request", "event_id", eventID)
	chats, err := h.svc.ListChats(r.Context(), eventID)
	if err != nil {
		slog.Warn("[ChatHandler.ListChats] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list chats")
		return
	}
	response.JSON(w, http.StatusOK, chats)
}

// UpdateSettings handles PATCH /api/admin/chats/{chatID}/settings.
func (h *ChatHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	var settings json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	slog.Debug("[ChatHandler.UpdateSettings] request", "chat_id", chatID)
	if err := h.svc.UpdateChatSettings(r.Context(), chatID, []byte(settings)); err != nil {
		slog.Warn("[ChatHandler.UpdateSettings] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to update settings")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"id": chatID.String()})
}
