package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"chatsem/shared/pkg/response"
)

func TestJSON_SetsContentTypeAndStatus(t *testing.T) {
	t.Logf("[%s] stage: writing JSON response", t.Name())
	w := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	response.JSON(w, http.StatusOK, payload)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("[%s] Content-Type: want application/json, got %s", t.Name(), ct)
	}
	if w.Code != http.StatusOK {
		t.Errorf("[%s] status: want 200, got %d", t.Name(), w.Code)
	}
	t.Logf("[%s] stage: body=%s", t.Name(), w.Body.String())
}

func TestError_Format(t *testing.T) {
	t.Logf("[%s] stage: writing error response", t.Name())
	w := httptest.NewRecorder()

	response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("[%s] status: want 401, got %d", t.Name(), w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("[%s] unmarshal: %v", t.Name(), err)
	}
	if body["error"] != "invalid token" {
		t.Errorf("[%s] error field: want 'invalid token', got %q", t.Name(), body["error"])
	}
	if body["code"] != "unauthorized" {
		t.Errorf("[%s] code field: want 'unauthorized', got %q", t.Name(), body["code"])
	}
	t.Logf("[%s] stage: response format validated", t.Name())
}
