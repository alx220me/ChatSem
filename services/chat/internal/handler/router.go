package handler

import (
	"net/http"

	"chatsem/services/chat/internal/middleware"
	"chatsem/services/chat/internal/ports"
	"chatsem/services/chat/internal/service"
	"chatsem/shared/pkg/longpoll"
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
	eventRepo ports.EventRepository,
	msgRepo ports.MessageRepository,
	banRepo ports.BanRepository,
	broker longpoll.Broker,
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
	pollH := NewPollHandler(broker, msgRepo, rdb)

	// Public endpoint — no auth required.
	r.Get("/api/chat/events/{eventID}/chats", chatH.ListChats)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Use(middleware.CORS(eventRepo))
		r.Use(middleware.BanCheck(rdb, banRepo))

		// Chat endpoints
		r.Post("/api/chat/join", chatH.JoinRoom)
		r.Get("/api/chat/chats/{chatID}", chatH.GetChat)

		// chatID-scoped endpoints: enforce that the chat belongs to the token's event.
		onlineH := NewOnlineHandler(rdb)
		r.Group(func(r chi.Router) {
			r.Use(middleware.EventOwnership(chatSvc))

			// Message endpoints
			r.Get("/api/chat/{chatID}/messages", msgH.List)

			// Message send with rate limit
			r.Group(func(r chi.Router) {
				r.Use(middleware.MessageRateLimit(rdb))
				r.Post("/api/chat/{chatID}/messages", msgH.Send)
			})

			// Long poll with per-IP rate limit
			r.Group(func(r chi.Router) {
				r.Use(middleware.PollIPRateLimit(rdb))
				r.Get("/api/chat/{chatID}/poll", pollH.Poll)
			})

			// Online presence
			r.Post("/api/chat/{chatID}/heartbeat", onlineH.Heartbeat)
			r.Delete("/api/chat/{chatID}/heartbeat", onlineH.Leave)
			r.Get("/api/chat/{chatID}/online", onlineH.OnlineCount)
		})

		// Message-level endpoints (by msgID, not chatID) — no ownership middleware needed here
		// since ownership is enforced at send/join level; moderator actions are role-checked.
		r.Delete("/api/chat/messages/{msgID}", msgH.Delete)
		r.Patch("/api/chat/messages/{msgID}", msgH.Edit)
	})

	return r
}
