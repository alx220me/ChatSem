package handler

import (
	"net/http"

	"chatsem/services/auth/internal/middleware"
	"chatsem/services/auth/internal/ports"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router for the auth service with standard middleware.
func NewRouter(tokenHandler *TokenHandler, eventRepo ports.EventRepository) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(eventRepo))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "auth"})
	})

	r.Post("/api/auth/token", tokenHandler.ExchangeToken)

	return r
}
