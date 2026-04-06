package handler

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"chatsem/shared/pkg/jwt"
	"chatsem/shared/pkg/response"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles admin authentication (username + password → JWT).
type AuthHandler struct {
	username     string
	passwordHash []byte
	jwtSecret    string
	jwtTTL       time.Duration
}

// NewAuthHandler creates an AuthHandler and pre-hashes the password with bcrypt.
func NewAuthHandler(username, password, jwtSecret string, ttl time.Duration) (*AuthHandler, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	slog.Debug("[AuthHandler] password hash generated", "username", username)
	return &AuthHandler{
		username:     username,
		passwordHash: hash,
		jwtSecret:    jwtSecret,
		jwtTTL:       ttl,
	}, nil
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login handles POST /api/admin/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Debug("[AuthHandler] bad request body", "err", err)
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		slog.Debug("[AuthHandler] missing credentials")
		response.Error(w, http.StatusBadRequest, "bad_request", "username and password required")
		return
	}

	slog.Debug("[AuthHandler] login attempt", "username", req.Username)

	// Constant-time username comparison to prevent timing attacks.
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.username)) == 1
	passwordMatch := bcrypt.CompareHashAndPassword(h.passwordHash, []byte(req.Password)) == nil

	if !usernameMatch || !passwordMatch {
		slog.Warn("[AuthHandler] login failed", "username", req.Username)
		response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	claims := &jwt.Claims{
		UserID:     uuid.Nil,
		ExternalID: req.Username,
		EventID:    uuid.Nil,
		Name:       req.Username,
		Role:       "admin",
	}
	token, err := jwt.CreateToken(claims, h.jwtSecret, h.jwtTTL)
	if err != nil {
		slog.Error("[AuthHandler] failed to create token", "err", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "failed to create token")
		return
	}

	slog.Info("[AuthHandler] login success", "username", req.Username)
	response.JSON(w, http.StatusOK, loginResponse{Token: token})
}
