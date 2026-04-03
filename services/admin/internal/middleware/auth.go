package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"chatsem/shared/pkg/jwt"
	"chatsem/shared/pkg/response"
)

type ctxKey int

const claimsKey ctxKey = 0

// Auth returns a middleware that validates the Bearer JWT and stores Claims in context.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				slog.Warn("[AdminAuthMiddleware] missing Authorization header", "path", r.URL.Path)
				response.Error(w, http.StatusUnauthorized, "unauthorized", "missing Bearer token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := jwt.ValidateToken(tokenStr, jwtSecret)
			if err != nil {
				slog.Warn("[AdminAuthMiddleware] invalid token", "err", err)
				response.Error(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}
			slog.Debug("[AdminAuthMiddleware] validated", "user_id", claims.UserID, "role", claims.Role)
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromCtx retrieves JWT claims stored by Auth middleware.
func ClaimsFromCtx(ctx context.Context) (*jwt.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*jwt.Claims)
	return c, ok
}
