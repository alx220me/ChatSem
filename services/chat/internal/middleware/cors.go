package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"chatsem/services/chat/internal/ports"

	"github.com/google/uuid"
)

const corsCacheTTL = 60 * time.Second

// corsCache caches allowed_origin lookups to avoid a DB hit on every request.
// Entries expire after corsCacheTTL; a miss triggers a single DB fetch.
type corsCache struct {
	mu      sync.RWMutex
	byID    map[uuid.UUID]cacheEntry // event_id → allowed_origin
	byOrigin map[string]cacheEntry   // origin   → allowed_origin
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

func newCORSCache() *corsCache {
	return &corsCache{
		byID:     make(map[uuid.UUID]cacheEntry),
		byOrigin: make(map[string]cacheEntry),
	}
}

func (c *corsCache) getByID(id uuid.UUID) (string, bool) {
	c.mu.RLock()
	e, ok := c.byID[id]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		return e.value, true
	}
	return "", false
}

func (c *corsCache) getByOrigin(origin string) (string, bool) {
	c.mu.RLock()
	e, ok := c.byOrigin[origin]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		return e.value, true
	}
	return "", false
}

func (c *corsCache) set(id uuid.UUID, origin string) {
	exp := time.Now().Add(corsCacheTTL)
	c.mu.Lock()
	c.byID[id] = cacheEntry{value: origin, expiresAt: exp}
	c.byOrigin[origin] = cacheEntry{value: origin, expiresAt: exp}
	c.mu.Unlock()
}

// CORS returns a middleware that enforces per-event allowed origins.
// Wildcard "*" is never sent — incompatible with credentials=true.
//
// When JWT claims are present (authenticated routes) the event is resolved via claims.EventID.
// When there are no claims (public routes) a reverse-lookup by Origin is used.
// Results are cached in-memory for 60 s to avoid a DB hit on every request.
func CORS(eventRepo ports.EventRepository) func(http.Handler) http.Handler {
	cache := newCORSCache()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed := resolveAllowedOrigin(r, eventRepo, cache, origin)

			if allowed == "" || origin != allowed {
				slog.Warn("[CORSMiddleware] origin rejected", "origin", origin, "allowed", allowed)
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}

			slog.Debug("[CORSMiddleware] origin allowed", "origin", origin)
			setCORSHeaders(w, allowed)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveAllowedOrigin returns the allowed origin string for the request.
// Uses JWT claims when available, otherwise falls back to reverse-lookup by origin.
func resolveAllowedOrigin(r *http.Request, repo ports.EventRepository, cache *corsCache, origin string) string {
	if claims, ok := ClaimsFromCtx(r.Context()); ok {
		if v, hit := cache.getByID(claims.EventID); hit {
			return v
		}
		event, err := repo.GetByID(r.Context(), claims.EventID)
		if err != nil {
			slog.Warn("[CORSMiddleware] could not load event", "event_id", claims.EventID, "err", err)
			return ""
		}
		cache.set(event.ID, event.AllowedOrigin)
		return event.AllowedOrigin
	}

	// No claims — reverse-lookup by origin.
	if v, hit := cache.getByOrigin(origin); hit {
		return v
	}
	event, err := repo.GetByAllowedOrigin(r.Context(), origin)
	if err != nil {
		slog.Debug("[CORSMiddleware] origin not found in DB", "origin", origin)
		return ""
	}
	cache.set(event.ID, event.AllowedOrigin)
	return event.AllowedOrigin
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "7200") // 2h — Chrome max; browser caches preflight
	w.Header().Set("Vary", "Origin")
}
