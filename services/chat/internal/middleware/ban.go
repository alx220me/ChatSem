package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"chatsem/services/chat/internal/ports"
	"chatsem/shared/pkg/response"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// BanCheck returns a middleware that rejects requests from banned users.
// Fast path: Redis key "ban:{eventID}:{userID}".
// Fallback: DB query via banRepo when the Redis key is absent (cache miss or Redis unavailable).
func BanCheck(rdb *redis.Client, banRepo ports.BanRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if isBanned(r.Context(), claims.EventID, claims.UserID, rdb, banRepo) {
				slog.Warn("[BanCheck] user banned", "event_id", claims.EventID, "user_id", claims.UserID)
				response.Error(w, http.StatusForbidden, "banned", "user is banned from this event")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isBanned checks Redis first; falls back to DB if Redis key is absent or Redis errors.
func isBanned(ctx context.Context, eventID, userID uuid.UUID, rdb *redis.Client, banRepo ports.BanRepository) bool {
	key := fmt.Sprintf("ban:%s:%s", eventID, userID)

	if rdb != nil {
		slog.Debug("[BanCheck] redis check", "key", key)
		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil {
			slog.Warn("[BanCheck] redis error, falling back to DB", "err", err)
		} else if exists > 0 {
			return true // fast path: banned
		} else {
			// Key not in Redis — fall through to DB check below.
		}
	}

	// DB fallback: handles cache misses, Redis unavailability, and stale state.
	if banRepo != nil {
		banned, err := banRepo.IsUserBanned(ctx, userID, eventID)
		if err != nil {
			slog.Warn("[BanCheck] DB fallback error, failing open", "err", err)
			return false
		}
		if banned {
			slog.Debug("[BanCheck] banned via DB fallback", "event_id", eventID, "user_id", userID)
			// Optionally repopulate Redis so the next request uses the fast path.
			if rdb != nil {
				if setErr := rdb.Set(ctx, key, "1", 0).Err(); setErr != nil {
					slog.Warn("[BanCheck] failed to repopulate Redis ban key", "err", setErr)
				}
			}
			return true
		}
	}

	return false
}
