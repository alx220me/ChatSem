package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"chatsem/services/chat/internal/middleware"
	"chatsem/services/chat/internal/service"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChatHandler handles HTTP requests for chat hierarchy.
type ChatHandler struct {
	svc *service.ChatService
}

// NewChatHandler creates a ChatHandler backed by the given service.
func NewChatHandler(svc *service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

type joinRequest struct {
	EventID  string `json:"event_id"`
	RoomID   string `json:"room_id"`
	RoomName string `json:"room_name"` // optional human-readable name, stored in external_room_name
}

// JoinRoom handles POST /api/chat/join.
// Returns 201 if the child chat was created, 200 if it already existed.
func (h *ChatHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.EventID == "" || req.RoomID == "" {
		response.Error(w, http.StatusBadRequest, "bad_request", "event_id and room_id are required")
		return
	}

	eventID, err := uuid.Parse(req.EventID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	if eventID != claims.EventID {
		slog.Warn("[ChatHandler.JoinRoom] event_id mismatch", "token_event_id", claims.EventID, "req_event_id", eventID, "user_id", claims.UserID)
		response.Error(w, http.StatusForbidden, "forbidden", "token event_id does not match requested event")
		return
	}

	slog.Debug("[ChatHandler.JoinRoom] request", "event_id", eventID, "room_id", req.RoomID, "room_name", req.RoomName, "user_id", claims.UserID)

	result, err := h.svc.GetOrCreateChildChat(r.Context(), eventID, req.RoomID, req.RoomName)
	if err != nil {
		slog.Warn("[ChatHandler.JoinRoom] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to join room")
		return
	}

	slog.Info("[ChatHandler.JoinRoom] joined", "chat_id", result.Chat.ID, "new", result.IsNew)
	status := http.StatusOK
	if result.IsNew {
		status = http.StatusCreated
	}
	response.JSON(w, status, result.Chat)
}

// GetChat handles GET /api/chat/chats/{chatID}.
func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	slog.Debug("[ChatHandler.GetChat] request", "chat_id", chatID)
	chat, err := h.svc.GetChat(r.Context(), chatID)
	if err != nil {
		slog.Warn("[ChatHandler.GetChat] error", "err", err)
		response.Error(w, http.StatusNotFound, "not_found", "chat not found")
		return
	}
	response.JSON(w, http.StatusOK, chat)
}

// ListChats handles GET /api/chat/events/{eventID}/chats.
// Returns parent chat and all children (public endpoint, no auth required).
func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	slog.Debug("[ChatHandler.ListChats] request", "event_id", eventID)

	parent, err := h.svc.GetParentChat(r.Context(), eventID)
	if err != nil {
		slog.Warn("[ChatHandler.ListChats] parent not found", "event_id", eventID, "err", err)
		response.Error(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	children, err := h.svc.GetChildren(r.Context(), eventID)
	if err != nil {
		slog.Warn("[ChatHandler.ListChats] list children error", "event_id", eventID, "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list chats")
		return
	}

	type listResponse struct {
		Parent   interface{} `json:"parent"`
		Children interface{} `json:"children"`
	}
	response.JSON(w, http.StatusOK, listResponse{Parent: parent, Children: children})
}
