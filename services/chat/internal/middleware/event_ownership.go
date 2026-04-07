package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"chatsem/shared/domain"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChatGetter is the minimal interface required by EventOwnership to look up a chat.
type ChatGetter interface {
	GetChat(ctx context.Context, chatID uuid.UUID) (*domain.Chat, error)
}

// EventOwnership returns a middleware that verifies the chatID URL param belongs to
// the event in the JWT claims. Prevents cross-event access using a valid token.
//
// Must be placed after Auth middleware (requires claims in context).
func EventOwnership(chats ChatGetter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawID := chi.URLParam(r, "chatID")
			chatID, err := uuid.Parse(rawID)
			if err != nil {
				// Let the handler deal with malformed chatID.
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
				return
			}

			chat, err := chats.GetChat(r.Context(), chatID)
			if err != nil {
				slog.Warn("[EventOwnership] chat not found", "chat_id", chatID, "err", err)
				response.Error(w, http.StatusNotFound, "not_found", "chat not found")
				return
			}

			if chat.EventID != claims.EventID {
				slog.Warn("[EventOwnership] event_id mismatch",
					"chat_id", chatID,
					"chat_event_id", chat.EventID,
					"token_event_id", claims.EventID,
					"user_id", claims.UserID,
				)
				response.Error(w, http.StatusForbidden, "forbidden", "token event_id does not match chat event")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
