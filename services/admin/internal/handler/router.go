package handler

import (
	"net/http"

	"chatsem/services/admin/internal/middleware"
	"chatsem/services/admin/internal/service"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router for the admin service with standard middleware.
func NewRouter(
	jwtSecret string,
	authH *AuthHandler,
	eventSvc *service.EventService,
	banSvc *service.BanService,
	muteSvc *service.MuteService,
	userSvc *service.UserService,
	exportSvc *service.ExportService,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "admin"})
	})

	// Public auth endpoint — no JWT required.
	r.Post("/api/admin/auth/login", authH.Login)

	eventH := NewEventHandler(eventSvc)
	banH := NewBanHandler(banSvc)
	muteH := NewMuteHandler(muteSvc)
	chatH := NewChatHandler(eventSvc)
	userH := NewUserHandler(userSvc)
	exportH := NewExportHandler(exportSvc, jwtSecret)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))

		// Admin-only endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Post("/api/admin/events", eventH.CreateEvent)
			r.Post("/api/admin/events/{eventID}/chat", eventH.CreateParentChat)
			r.Patch("/api/admin/chats/{chatID}/settings", chatH.UpdateSettings)
			r.Patch("/api/admin/users/{userID}/role", userH.UpdateRole)
		})

		// Admin and moderator endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin", "moderator"))
			r.Get("/api/admin/events", eventH.ListEvents)
			r.Get("/api/admin/events/{eventID}/chats", chatH.ListChats)
			r.Get("/api/admin/events/{eventID}/users", userH.List)
			r.Post("/api/admin/bans", banH.CreateBan)
			r.Delete("/api/admin/bans/{banID}", banH.DeleteBan)
			r.Get("/api/admin/events/{eventID}/bans", banH.ListBans)
			r.Post("/api/admin/mutes", muteH.CreateMute)
			r.Delete("/api/admin/mutes/{muteID}", muteH.DeleteMute)
			r.Get("/api/admin/chats/{chatID}/mutes", muteH.ListMutes)
		})
	})

	// Export endpoint handles its own auth (supports ?token= for browser downloads).
	r.Get("/api/admin/chats/{chatID}/export", exportH.Export)

	return r
}
