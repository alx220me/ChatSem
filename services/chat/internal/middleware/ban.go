package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"chatsem/shared/pkg/response"

	"github.com/redis/go-redis/v9"
)

// BanCheck returns a middleware that rejects requests from banned users.
// Ban status is checked via Redis key "ban:{eventID}:{userID}".
// Fail-open: if Redis is unavailable, the request is allowed through.
func BanCheck(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			slog.Debug("[BanCheck] checking", "event_id", claims.EventID, "user_id", claims.UserID)
			if rdb != nil {
				key := fmt.Sprintf("ban:%s:%s", claims.EventID, claims.UserID)
				exists, err := rdb.Exists(r.Context(), key).Result()
				if err != nil {
					slog.Warn("[BanCheck] redis error, failing open", "err", err, "user_id", claims.UserID)
				} else if exists > 0 {
					slog.Warn("[BanCheck] user banned", "event_id", claims.EventID, "user_id", claims.UserID)
					response.Error(w, http.StatusForbidden, "banned", "user is banned from this event")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
