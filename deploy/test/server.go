package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const (
	apiSecret      = "d430dcb3211440b0a0122784f4123fd5a1cbd19a10bee005897ddfd505ab86e9"
	eventID        = "2e77dd95-786d-4c03-92c0-a2ce794f7bac"
	authServiceURL = "http://localhost/api/auth/token"
	listenAddr     = ":8090"
)

// _, currentFile, _, _ = runtime.Caller(0) gives us this file's path at compile time.
var dir = func() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Dir(f)
}()

func main() {
	slog.Info("demo token server starting", "addr", listenAddr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /chat-token", handleChatToken)
	mux.HandleFunc("GET /", handleRoot)

	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(dir, "sem1220.html"))
}

func handleChatToken(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "demo-user"
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Demo User"
	}

	slog.Debug("calling auth service", "auth_url", authServiceURL, "user_id", userID, "name", name)

	reqBody, err := json.Marshal(map[string]string{
		"external_user_id": userID,
		"event_id":         eventID,
		"name":             name,
		"role":             "user",
	})
	if err != nil {
		slog.Error("failed to marshal auth request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, authServiceURL, bytes.NewReader(reqBody))
	if err != nil {
		slog.Error("failed to create auth request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("auth service unreachable", "err", err)
		http.Error(w, "auth service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read auth response", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("auth service returned error", "status", resp.StatusCode, "body", string(body))
		http.Error(w, "auth service error", http.StatusBadGateway)
		return
	}

	// Parse upstream response and re-emit just { "token": "..." }
	var upstream struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &upstream); err != nil || upstream.Token == "" {
		slog.Error("invalid auth response", "err", err, "body", string(body))
		http.Error(w, "invalid auth response", http.StatusBadGateway)
		return
	}

	slog.Debug("token obtained successfully", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": upstream.Token})
}
