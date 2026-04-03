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
func NewRouter(jwtSecret string, eventSvc *service.EventService, banSvc *service.BanService) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "admin"})
	})

	eventH := NewEventHandler(eventSvc)
	banH := NewBanHandler(banSvc)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))

		// Admin-only endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Post("/api/admin/events", eventH.CreateEvent)
			r.Post("/api/admin/events/{eventID}/chat", eventH.CreateParentChat)
		})

		// Admin and moderator endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin", "moderator"))
			r.Get("/api/admin/events", eventH.ListEvents)
			r.Post("/api/admin/bans", banH.CreateBan)
			r.Delete("/api/admin/bans/{banID}", banH.DeleteBan)
			r.Get("/api/admin/events/{eventID}/bans", banH.ListBans)
		})
	})

	return r
}
