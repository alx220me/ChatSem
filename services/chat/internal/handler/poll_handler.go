package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"chatsem/services/chat/internal/middleware"
	"chatsem/services/chat/internal/ports"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/longpoll"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// PollHandler handles long-poll requests for new messages.
type PollHandler struct {
	broker   longpoll.Broker
	messages ports.MessageRepository
	rdb      *redis.Client
}

// NewPollHandler creates a PollHandler backed by the given broker and message repository.
func NewPollHandler(broker longpoll.Broker, messages ports.MessageRepository, rdb *redis.Client) *PollHandler {
	return &PollHandler{broker: broker, messages: messages, rdb: rdb}
}

// lastSeqKey returns the Redis key for the latest message seq of a chat.
func lastSeqKey(chatID uuid.UUID) string {
	return fmt.Sprintf("chat:%s:last_seq", chatID)
}

// deleteLogKey returns the Redis sorted-set key for deletion events of a chat.
func deleteLogKey(chatID uuid.UUID) string {
	return fmt.Sprintf("chat:%s:delete_log", chatID)
}

// deleteSeqKey returns the Redis key for the current delete_seq counter.
func deleteSeqKey(chatID uuid.UUID) string {
	return fmt.Sprintf("chat:%s:delete_seq", chatID)
}

// fetchDeletesSince returns deleted message IDs with delete_seq > afterDeleteSeq,
// and the maximum delete_seq seen (0 if nothing returned).
func fetchDeletesSince(ctx context.Context, rdb *redis.Client, chatID uuid.UUID, afterDeleteSeq int64) (ids []string, maxSeq int64) {
	if rdb == nil {
		return nil, 0
	}
	results, err := rdb.ZRangeByScoreWithScores(ctx, deleteLogKey(chatID), &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", afterDeleteSeq),
		Max: "+inf",
	}).Result()
	if err != nil || len(results) == 0 {
		return nil, 0
	}
	ids = make([]string, len(results))
	for i, z := range results {
		ids[i] = z.Member.(string)
		if int64(z.Score) > maxSeq {
			maxSeq = int64(z.Score)
		}
	}
	return ids, maxSeq
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

	afterDeleteSeq := int64(0)
	if s := r.URL.Query().Get("after_delete_seq"); s != "" {
		afterDeleteSeq, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid after_delete_seq parameter")
			return
		}
	}

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	slog.Debug("[PollHandler.Poll] waiting", "chat_id", chatID, "after_seq", afterSeq, "after_delete_seq", afterDeleteSeq, "user_id", claims.UserID)

	// pollResult bundles the response payload.
	type pollResult struct {
		msgs           []*domain.Message
		deletedIDs     []string
		lastDeleteSeq  int64
	}

	buildResult := func(ctx context.Context) *pollResult {
		var msgs []*domain.Message
		latestSeq, seqErr := h.rdb.Get(ctx, lastSeqKey(chatID)).Int64()
		if seqErr == nil && latestSeq > afterSeq {
			var dbErr error
			msgs, dbErr = h.messages.GetByChatIDAfterSeq(ctx, chatID, afterSeq, 100)
			if dbErr != nil {
				slog.Warn("[PollHandler.Poll] db error", "chat_id", chatID, "err", dbErr)
			}
		}
		deletedIDs, lastDeleteSeq := fetchDeletesSince(ctx, h.rdb, chatID, afterDeleteSeq)
		return &pollResult{msgs: msgs, deletedIDs: deletedIDs, lastDeleteSeq: lastDeleteSeq}
	}

	// Fast path: if there is already something to deliver, return immediately.
	if h.rdb != nil {
		fastCtx, fastCancel := context.WithTimeout(context.Background(), 3*time.Second)
		res := buildResult(fastCtx)
		fastCancel()
		if len(res.msgs) > 0 || len(res.deletedIDs) > 0 {
			slog.Info("[PollHandler.Poll] fast-path: returning data", "chat_id", chatID, "msgs", len(res.msgs), "deletes", len(res.deletedIDs))
			response.JSON(w, http.StatusOK, map[string]interface{}{
				"messages":        res.msgs,
				"deleted_ids":     res.deletedIDs,
				"last_delete_seq": res.lastDeleteSeq,
			})
			return
		}
	}

	// Slow path: subscribe to broker and wait for new message/deletion or timeout.
	ch := h.broker.Subscribe(chatID)
	defer h.broker.Unsubscribe(chatID, ch)

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

	var msgs []*domain.Message
	var dbErr error
	msgs, dbErr = h.messages.GetByChatIDAfterSeq(dbCtx, chatID, afterSeq, 100)
	if dbErr != nil {
		slog.Warn("[PollHandler.Poll] db error", "chat_id", chatID, "err", dbErr)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to fetch messages")
		return
	}

	deletedIDs, lastDeleteSeq := fetchDeletesSince(dbCtx, h.rdb, chatID, afterDeleteSeq)

	if len(msgs) == 0 && len(deletedIDs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	slog.Info("[PollHandler.Poll] returning data", "chat_id", chatID, "msgs", len(msgs), "deletes", len(deletedIDs))
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"messages":        msgs,
		"deleted_ids":     deletedIDs,
		"last_delete_seq": lastDeleteSeq,
	})
}
