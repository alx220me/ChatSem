package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

// eventOriginGetter is the minimal EventRepository interface needed by CORS middleware.
type eventOriginGetter interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
}

// CORS returns a middleware that sets CORS headers based on per-event allowed_origin.
// Wildcard "*" is never used because credentials=true requires an explicit origin.
func CORS(eventRepo eventOriginGetter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Try to parse event_id from the request body is not feasible in middleware,
			// so we accept the origin if any event has it as allowed_origin.
			// For preflight the event_id can be in a custom header X-Event-ID.
			eventIDStr := r.Header.Get("X-Event-ID")
			if eventIDStr != "" {
				eventID, err := uuid.Parse(eventIDStr)
				if err == nil {
					event, err := eventRepo.GetByID(r.Context(), eventID)
					if err == nil {
						if event.AllowedOrigin == origin {
							slog.Debug("[CORSMiddleware] origin allowed", "origin", origin)
							w.Header().Set("Access-Control-Allow-Origin", origin)
							w.Header().Set("Access-Control-Allow-Credentials", "true")
							w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Event-ID")
							w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
							w.Header().Set("Vary", "Origin")
						} else {
							slog.Warn("[CORSMiddleware] origin rejected", "origin", origin, "allowed", event.AllowedOrigin)
						}
					}
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
