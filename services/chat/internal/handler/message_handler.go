package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"chatsem/services/chat/internal/middleware"
	"chatsem/services/chat/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MessageHandler handles HTTP requests for messages.
type MessageHandler struct {
	svc *service.MessageService
}

// NewMessageHandler creates a MessageHandler backed by the given service.
func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

type sendMessageRequest struct {
	Text      string  `json:"text"`
	ReplyToID *string `json:"reply_to_id,omitempty"`
}

// Send handles POST /api/chat/{chatID}/messages.
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	rawChatID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawChatID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Parse optional reply_to_id.
	var replyToID *uuid.UUID
	if req.ReplyToID != nil {
		parsed, parseErr := uuid.Parse(*req.ReplyToID)
		if parseErr != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid reply_to_id")
			return
		}
		replyToID = &parsed
	}

	slog.Debug("[MessageHandler.Send] request",
		"chat_id", chatID, "user_id", claims.UserID, "text_len", len(req.Text), "reply_to_id", replyToID)
	msg, err := h.svc.SendMessage(r.Context(), chatID, claims.UserID, claims.EventID, req.Text, replyToID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyMessage), errors.Is(err, domain.ErrMessageTooLong):
			response.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		case errors.Is(err, domain.ErrUserBanned), errors.Is(err, domain.ErrUserMuted), errors.Is(err, domain.ErrForbidden):
			response.Error(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			response.Error(w, http.StatusBadRequest, "bad_request", "reply message not found")
		case errors.Is(err, domain.ErrInvalidReply):
			response.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			slog.Warn("[MessageHandler.Send] error", "err", err)
			response.Error(w, http.StatusInternalServerError, "internal_error", "failed to send message")
		}
		return
	}

	slog.Info("[MessageHandler.Send] sent", "seq", msg.Seq)
	type sendResp struct {
		ID  uuid.UUID `json:"id"`
		Seq int64     `json:"seq"`
		TS  string    `json:"ts"`
	}
	response.JSON(w, http.StatusCreated, sendResp{ID: msg.ID, Seq: msg.Seq, TS: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
}

// List handles GET /api/chat/{chatID}/messages.
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	rawChatID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawChatID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	afterSeq := int64(0)
	if s := r.URL.Query().Get("after"); s != "" {
		afterSeq, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid after parameter")
			return
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err == nil && n > 0 {
			limit = n
		}
	}

	slog.Debug("[MessageHandler.List] request", "chat_id", chatID, "after_seq", afterSeq, "limit", limit)

	var msgs []*domain.Message
	var listErr error
	if afterSeq == 0 {
		// Initial load: return the most recent messages in chronological order.
		msgs, listErr = h.svc.ListMessages(r.Context(), chatID, limit)
	} else {
		// Incremental load: return messages after a known seq.
		msgs, listErr = h.svc.GetMessages(r.Context(), chatID, afterSeq, limit)
	}
	if listErr != nil {
		slog.Warn("[MessageHandler.List] error", "err", listErr)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to list messages")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{"messages": msgs})
}

// Delete handles DELETE /api/chat/messages/{msgID}.
func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	rawMsgID := chi.URLParam(r, "msgID")
	msgID, err := uuid.Parse(rawMsgID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid msg_id")
		return
	}

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	slog.Debug("[MessageHandler.Delete] request", "msg_id", msgID, "user_id", claims.UserID)
	if err := h.svc.SoftDelete(r.Context(), msgID, claims.UserID, claims.Role); err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			response.Error(w, http.StatusForbidden, "forbidden", "not allowed to delete this message")
		case errors.Is(err, domain.ErrNotFound):
			response.Error(w, http.StatusNotFound, "not_found", "message not found")
		default:
			slog.Warn("[MessageHandler.Delete] error", "err", err)
			response.Error(w, http.StatusInternalServerError, "internal_error", "failed to delete message")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
