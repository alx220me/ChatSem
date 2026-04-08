package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"chatsem/services/admin/internal/service"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EventHandler handles HTTP requests for event management.
type EventHandler struct {
	svc *service.EventService
}

// NewEventHandler creates an EventHandler backed by the given service.
func NewEventHandler(svc *service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

type createEventRequest struct {
	Name          string `json:"name"`
	AllowedOrigin string `json:"allowed_origin"`
}

// createEventResponse wraps the created event and includes the plaintext API secret
// (returned only once — the server stores only the bcrypt hash).
type createEventResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	AllowedOrigin string    `json:"allowedOrigin"`
	CreatedAt     time.Time `json:"createdAt"`
	// APISecret is the plaintext secret shown exactly once at creation time.
	// Store it securely — it cannot be recovered later.
	APISecret string `json:"api_secret"`
}

// CreateEvent handles POST /api/admin/events.
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.AllowedOrigin == "" {
		response.Error(w, http.StatusBadRequest, "bad_request", "name and allowed_origin are required")
		return
	}

	slog.Debug("[EventHandler.CreateEvent] request", "name", req.Name, "origin", req.AllowedOrigin)

	event, plainSecret, err := h.svc.CreateEvent(r.Context(), req.Name, req.AllowedOrigin)
	if err != nil {
		slog.Warn("[EventHandler.CreateEvent] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to create event")
		return
	}

	slog.Info("[EventHandler.CreateEvent] created", "event_id", event.ID)
	response.JSON(w, http.StatusCreated, createEventResponse{
		ID:            event.ID,
		Name:          event.Name,
		AllowedOrigin: event.AllowedOrigin,
		CreatedAt:     event.CreatedAt,
		APISecret:     plainSecret,
	})
}

// ListEvents handles GET /api/admin/events.
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	slog.Debug("[EventHandler.ListEvents] request")
	events, err := h.svc.ListEvents(r.Context())
	if err != nil {
		slog.Warn("[EventHandler.ListEvents] error", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list events")
		return
	}
	response.JSON(w, http.StatusOK, events)
}

// RotateSecret handles POST /api/admin/events/{eventID}/rotate-secret.
func (h *EventHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	slog.Debug("[EventHandler.RotateSecret] request", "event_id", eventID)

	plainSecret, err := h.svc.RotateAPISecret(r.Context(), eventID)
	if err != nil {
		slog.Warn("[EventHandler.RotateSecret] error", "event_id", eventID, "err", err)
		// Distinguish "not found" from generic errors
		if containsNotFound(err) {
			response.Error(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to rotate secret")
		return
	}

	slog.Info("[EventHandler.RotateSecret] rotated", "event_id", eventID)
	response.JSON(w, http.StatusOK, map[string]string{"api_secret": plainSecret})
}

// containsNotFound reports whether the error message contains "not found".
func containsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// CreateParentChat handles POST /api/admin/events/{eventID}/chat.
func (h *EventHandler) CreateParentChat(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid event_id")
		return
	}

	slog.Debug("[EventHandler.CreateParentChat] request", "event_id", eventID)
	event, err := h.svc.GetEvent(r.Context(), eventID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{"event_id": event.ID, "message": "parent chat already created on event creation"})
}
