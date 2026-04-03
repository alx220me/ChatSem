package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"chatsem/shared/pkg/response"

	"github.com/redis/go-redis/v9"
)

const (
	ipRateLimit         = 100
	ipRateWindow        = 60 * time.Second
	pollRateLimit       = 60
	pollRateWindow      = 60 * time.Second
	msgRateLimit        = 10
	msgRateWindow       = 10 * time.Second
	moderatorMultiplier = 3
)

// allow uses a sliding-window counter in Redis. Returns true if the request should be allowed.
// Fail-open: if Redis is unavailable, returns true.
func allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now()
	windowStart := float64(now.Add(-window).UnixMilli())
	nowScore := float64(now.UnixMilli())

	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatFloat(windowStart, 'f', 0, 64))
	pipe.ZAdd(ctx, key, redis.Z{Score: nowScore, Member: nowScore})
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err // fail-open
	}
	return countCmd.Val() <= int64(limit), nil
}

// IPRateLimit limits requests per client IP to 100 requests per 60 seconds.
func IPRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			key := fmt.Sprintf("rl:ip:%s", ip)
			ok, err := allow(r.Context(), rdb, key, ipRateLimit, ipRateWindow)
			if err != nil {
				slog.Warn("[IPRateLimit] redis error, failing open", "err", err, "ip", ip)
			}
			if !ok {
				slog.Warn("[IPRateLimit] exceeded", "ip", ip)
				w.Header().Set("Retry-After", "60")
				response.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PollIPRateLimit limits poll requests per client IP to 60 per 60 seconds.
func PollIPRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rdb == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := r.RemoteAddr
			key := fmt.Sprintf("rl:poll:%s", ip)
			ok, err := allow(r.Context(), rdb, key, pollRateLimit, pollRateWindow)
			if err != nil {
				slog.Warn("[PollIPRateLimit] redis error, failing open", "err", err, "ip", ip)
			}
			if !ok {
				slog.Warn("[PollIPRateLimit] exceeded", "ip", ip)
				w.Header().Set("Retry-After", "60")
				response.Error(w, http.StatusTooManyRequests, "rate_limited", "poll rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MessageRateLimit limits message sends per (eventID, chatID, userID):
// 10/10s for users, 30/10s for moderators, unlimited for admins.
func MessageRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				// No claims — let Auth middleware handle this.
				next.ServeHTTP(w, r)
				return
			}

			if claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			limit := msgRateLimit
			if claims.Role == "moderator" {
				limit = msgRateLimit * moderatorMultiplier
			}

			key := fmt.Sprintf("rl:msg:%s:%s", claims.EventID, claims.UserID)
			ok, err := allow(r.Context(), rdb, key, limit, msgRateWindow)
			if err != nil {
				slog.Warn("[MessageRateLimit] redis error, failing open", "err", err, "user_id", claims.UserID)
			}
			if !ok {
				slog.Warn("[MessageRateLimit] exceeded", "user_id", claims.UserID, "event_id", claims.EventID)
				w.Header().Set("Retry-After", "10")
				response.Error(w, http.StatusTooManyRequests, "rate_limited", "message rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
