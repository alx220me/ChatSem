package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"chatsem/shared/pkg/response"

	"github.com/redis/go-redis/v9"
)

const (
	ipRateLimit         = 100
	ipRateWindow        = 60 * time.Second
	pollRateLimit       = 60
	pollRateWindow      = 60 * time.Second
	msgRateLimit        = 2
	msgRateWindow       = 10 * time.Second
	moderatorMultiplier = 3
)

// allowScript is an atomic Lua sliding-window rate limiter.
//
// KEYS[1] — Redis key (sorted set)
// ARGV[1] — current time in milliseconds
// ARGV[2] — window start cutoff in milliseconds (now - window)
// ARGV[3] — request limit
// ARGV[4] — TTL in milliseconds (= window size)
//
// Returns the number of requests in the current window after adding the current one.
var allowScript = redis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local cutoff = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])

redis.call('ZREMRANGEBYSCORE', key, 0, cutoff)
redis.call('ZADD', key, now, now)
local count = redis.call('ZCARD', key)
redis.call('PEXPIRE', key, ttl_ms)
return count
`)

// allow uses an atomic Lua sliding-window counter in Redis.
// Returns true if the request is within the limit.
// Fail-open: if Redis is unavailable, returns true.
func allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().UnixMilli()
	cutoff := now - window.Milliseconds()
	ttlMs := window.Milliseconds()

	count, err := allowScript.Run(ctx, rdb, []string{key},
		now, cutoff, limit, ttlMs,
	).Int64()
	if err != nil {
		return true, err // fail-open
	}
	return count <= int64(limit), nil
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
