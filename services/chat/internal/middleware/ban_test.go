package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chatsem/services/chat/internal/middleware"
	"chatsem/shared/pkg/jwt"

	"github.com/google/uuid"
)

// injectClaims wraps a handler and injects JWT claims into the context.
func injectClaims(claims *jwt.Claims, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Use the exported ClaimsFromCtx to confirm the setter works through Auth middleware.
		// Here we build a valid JWT and go through Auth to set claims properly.
		tok, err := jwt.CreateToken(claims, testSecret, time.Hour)
		if err != nil {
			http.Error(w, "token error", http.StatusInternalServerError)
			return
		}
		r.Header.Set("Authorization", "Bearer "+tok)
		// Re-run Auth middleware inline.
		authMW := middleware.Auth(testSecret)
		authMW(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestBanCheck_NotBanned_NilRedis(t *testing.T) {
	// With nil Redis the ban check should fail-open and let the request through.
	claims := &jwt.Claims{UserID: uuid.New(), EventID: uuid.New(), Role: "user"}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := injectClaims(claims, middleware.BanCheck(nil, nil)(inner))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("[%s] expected 200 (fail-open), got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: nil Redis → fail-open → 200", t.Name())
}

func TestBanCheck_NoClaims_PassThrough(t *testing.T) {
	// No Auth middleware → no claims → BanCheck should pass through.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.BanCheck(nil, nil)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("[%s] expected 200 with no claims, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: no claims → pass-through → 200", t.Name())
}

// testSecret is defined in auth_test.go — same package.
var _ = context.Background // suppress unused import if needed
