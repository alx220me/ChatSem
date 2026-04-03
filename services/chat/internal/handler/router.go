package handler

import (
	"net/http"

	"chatsem/services/chat/internal/middleware"
	"chatsem/services/chat/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

// NewRouter creates the chi router for the chat service with standard middleware.
func NewRouter(
	jwtSecret string,
	chatSvc *service.ChatService,
	msgSvc *service.MessageService,
	eventRepo domain.EventRepository,
	rdb *redis.Client,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.IPRateLimit(rdb))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "chat"})
	})

	chatH := NewChatHandler(chatSvc)
	msgH := NewMessageHandler(msgSvc)

	// Public endpoint — no auth required.
	r.Get("/api/chat/events/{eventID}/chats", chatH.ListChats)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Use(middleware.CORS(eventRepo))
		r.Use(middleware.BanCheck(rdb))

		// Chat endpoints
		r.Post("/api/chat/join", chatH.JoinRoom)
		r.Get("/api/chat/chats/{chatID}", chatH.GetChat)

		// Message endpoints
		r.Get("/api/chat/{chatID}/messages", msgH.List)
		r.Delete("/api/chat/messages/{msgID}", msgH.Delete)

		// Message send with rate limit
		r.Group(func(r chi.Router) {
			r.Use(middleware.MessageRateLimit(rdb))
			r.Post("/api/chat/{chatID}/messages", msgH.Send)
		})
	})

	return r
}
