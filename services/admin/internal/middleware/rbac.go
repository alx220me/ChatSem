package middleware

import (
	"log/slog"
	"net/http"

	"chatsem/shared/pkg/response"
)

// RequireRole returns a middleware that allows only users with one of the given roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromCtx(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
				return
			}
			if !allowed[claims.Role] {
				slog.Warn("[RBACMiddleware] role not allowed", "user_id", claims.UserID, "role", claims.Role, "required", roles)
				response.Error(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
