package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chatsem/services/admin/internal/handler"
)

const (
	testUsername  = "admin"
	testPassword  = "s3cr3t"
	testJWTSecret = "test-secret-1234"
)

func newTestAuthHandler(t *testing.T) *handler.AuthHandler {
	t.Helper()
	h, err := handler.NewAuthHandler(testUsername, testPassword, testJWTSecret, time.Hour)
	if err != nil {
		t.Fatalf("[%s] NewAuthHandler: %v", t.Name(), err)
	}
	return h
}

func doLoginRequest(h *handler.AuthHandler, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	return rr
}

func TestAdminLogin_Success(t *testing.T) {
	h := newTestAuthHandler(t)
	rr := doLoginRequest(h, map[string]string{"username": testUsername, "password": testPassword})

	if rr.Code != http.StatusOK {
		t.Fatalf("[%s] expected 200, got %d: %s", t.Name(), rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("[%s] decode response: %v", t.Name(), err)
	}
	if resp["token"] == "" {
		t.Errorf("[%s] expected non-empty token in response", t.Name())
	}
	t.Logf("[%s] assert: login success, token present", t.Name())
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	h := newTestAuthHandler(t)
	rr := doLoginRequest(h, map[string]string{"username": testUsername, "password": "wrong"})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("[%s] expected 401, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: wrong password → 401", t.Name())
}

func TestAdminLogin_WrongUsername(t *testing.T) {
	h := newTestAuthHandler(t)
	rr := doLoginRequest(h, map[string]string{"username": "hacker", "password": testPassword})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("[%s] expected 401, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: wrong username → 401", t.Name())
}

func TestAdminLogin_MissingBody(t *testing.T) {
	h := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/auth/login", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("[%s] expected 400, got %d", t.Name(), rr.Code)
	}
	t.Logf("[%s] assert: empty credentials → 400", t.Name())
}
