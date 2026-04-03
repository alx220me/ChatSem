package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chatsem/services/chat/internal/middleware"
	"chatsem/shared/pkg/jwt"

	"github.com/google/uuid"
)

const testSecret = "test-secret-key"

func makeToken(t *testing.T, secret string, ttl time.Duration) string {
	t.Helper()
	claims := &jwt.Claims{
		UserID:  uuid.New(),
		EventID: uuid.New(),
		Role:    "user",
	}
	tok, err := jwt.CreateToken(claims, secret, ttl)
	if err != nil {
		t.Fatalf("[%s] CreateToken: %v", t.Name(), err)
	}
	return tok
}

func TestAuth_ValidToken(t *testing.T) {
	tok := makeToken(t, testSecret, time.Hour)

	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromCtx(r.Context())
		if !ok {
			t.Errorf("[%s] expected claims in context", t.Name())
			return
		}
		t.Logf("[%s] assert: claims user_id=%s present in context", t.Name(), claims.UserID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("[%s] expected 200, got %d", t.Name(), rr.Code)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("[%s] handler should not be reached", t.Name())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("[%s] expected 401, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: missing token → 401", t.Name())
}

func TestAuth_ExpiredToken(t *testing.T) {
	tok := makeToken(t, testSecret, -time.Minute) // already expired

	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("[%s] handler should not be reached with expired token", t.Name())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("[%s] expected 401, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: expired token → 401", t.Name())
}
