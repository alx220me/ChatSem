package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"chatsem/services/chat/internal/middleware"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const onlineTTL = 60 * time.Second

// OnlineHandler handles presence (heartbeat + online count) endpoints.
type OnlineHandler struct {
	rdb *redis.Client
}

// NewOnlineHandler creates an OnlineHandler.
func NewOnlineHandler(rdb *redis.Client) *OnlineHandler {
	return &OnlineHandler{rdb: rdb}
}

func onlineKey(chatID uuid.UUID) string {
	return fmt.Sprintf("chat:%s:online", chatID)
}

// Heartbeat handles POST /api/chat/{chatID}/heartbeat.
// Registers the user as online for 60 seconds using a Redis sorted set.
func (h *OnlineHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	now := float64(time.Now().Unix())
	key := onlineKey(chatID)

	minScore := fmt.Sprintf("%d", time.Now().Add(-onlineTTL).Unix())
	pipe := h.rdb.Pipeline()
	pipe.ZAdd(r.Context(), key, redis.Z{Score: now, Member: claims.UserID.String()})
	pipe.ZRemRangeByScore(r.Context(), key, "-inf", minScore)
	pipe.Expire(r.Context(), key, onlineTTL+10*time.Second)
	zCountCmd := pipe.ZCount(r.Context(), key, minScore, "+inf")
	if _, err := pipe.Exec(r.Context()); err != nil {
		slog.Warn("[OnlineHandler.Heartbeat] redis error", "err", err)
	}

	count := zCountCmd.Val()
	slog.Debug("[OnlineHandler.Heartbeat] registered", "chat_id", chatID, "user_id", claims.UserID, "online", count)
	response.JSON(w, http.StatusOK, map[string]int64{"count": count})
}

// Leave handles DELETE /api/chat/{chatID}/heartbeat.
// Immediately removes the user from the online set.
func (h *OnlineHandler) Leave(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	claims, ok := middleware.ClaimsFromCtx(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}

	if err := h.rdb.ZRem(r.Context(), onlineKey(chatID), claims.UserID.String()).Err(); err != nil {
		slog.Warn("[OnlineHandler.Leave] redis error", "err", err)
	}

	slog.Debug("[OnlineHandler.Leave] removed", "chat_id", chatID, "user_id", claims.UserID)
	w.WriteHeader(http.StatusNoContent)
}

// OnlineCount handles GET /api/chat/{chatID}/online.
// Returns the number of users active in the last 60 seconds.
func (h *OnlineHandler) OnlineCount(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	min := fmt.Sprintf("%d", time.Now().Add(-onlineTTL).Unix())
	count, err := h.rdb.ZCount(r.Context(), onlineKey(chatID), min, "+inf").Result()
	if err != nil {
		slog.Warn("[OnlineHandler.OnlineCount] redis error", "err", err)
		count = 0
	}

	slog.Debug("[OnlineHandler.OnlineCount] count", "chat_id", chatID, "count", count)
	response.JSON(w, http.StatusOK, map[string]int64{"count": count})
}
