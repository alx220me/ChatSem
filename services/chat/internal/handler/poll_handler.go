package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"chatsem/services/chat/internal/middleware"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PollHandler handles long-poll requests for new messages.
type PollHandler struct {
	broker   longpoll.Broker
	messages domain.MessageRepository
}

// NewPollHandler creates a PollHandler backed by the given broker and message repository.
func NewPollHandler(broker longpoll.Broker, messages domain.MessageRepository) *PollHandler {
	return &PollHandler{broker: broker, messages: messages}
}

// Poll handles GET /api/chat/{chatID}/poll?after={seq}.
func (h *PollHandler) Poll(w http.ResponseWriter, r *http.Request) {
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

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	slog.Debug("[PollHandler.Poll] waiting", "chat_id", chatID, "after_seq", afterSeq, "user_id", claims.UserID)

	ch := h.broker.Subscribe(chatID)
	defer h.broker.Unsubscribe(chatID, ch)

	// Settling loop: wait for a message or timeout.
	select {
	case <-ch:
		slog.Debug("[PollHandler.Poll] event received, settling", "chat_id", chatID)
		time.Sleep(longpoll.LongPollSettleDelay)
	case <-time.After(longpoll.LongPollTimeout):
		slog.Debug("[PollHandler.Poll] timeout, no messages", "chat_id", chatID)
		w.WriteHeader(http.StatusNoContent)
		return
	case <-r.Context().Done():
		return
	}

	// Use an independent context — the client may have disconnected, but we still want to complete the DB read.
	dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msgs, err := h.messages.GetByChatIDAfterSeq(dbCtx, chatID, afterSeq, 100)
	if err != nil {
		slog.Warn("[PollHandler.Poll] db error", "chat_id", chatID, "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to fetch messages")
		return
	}

	slog.Info("[PollHandler.Poll] returning messages", "chat_id", chatID, "count", len(msgs))
	response.JSON(w, http.StatusOK, map[string]interface{}{"messages": msgs})
}
