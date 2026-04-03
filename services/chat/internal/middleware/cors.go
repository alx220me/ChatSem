package middleware

import (
	"log/slog"
	"net/http"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// CORS returns a middleware that enforces per-event allowed origins.
// Wildcard "*" is never sent — incompatible with credentials=true.
func CORS(eventRepo domain.EventRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Resolve event from JWT claims (already validated by Auth middleware upstream).
			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				// No claims — skip CORS; Auth middleware will have rejected already.
				next.ServeHTTP(w, r)
				return
			}

			allowed := allowedOriginForEvent(r, eventRepo, claims.EventID)
			if allowed == "" || origin != allowed {
				slog.Warn("[CORSMiddleware] origin rejected", "origin", origin, "allowed", allowed)
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}

			slog.Debug("[CORSMiddleware] origin allowed", "origin", origin)
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func allowedOriginForEvent(r *http.Request, repo domain.EventRepository, eventID uuid.UUID) string {
	event, err := repo.GetByID(r.Context(), eventID)
	if err != nil {
		slog.Warn("[CORSMiddleware] could not load event", "event_id", eventID, "err", err)
		return ""
	}
	return event.AllowedOrigin
}
