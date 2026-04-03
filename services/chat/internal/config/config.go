package config

import (
	"log/slog"
	"os"
)

// Config holds all runtime configuration for the chat service.
type Config struct {
	Addr        string // HTTP listen address, e.g. ":8080"
	DatabaseURL string // PostgreSQL DSN
	RedisAddr   string // Redis address, e.g. "localhost:6379"
	JWTSecret   string // HMAC secret for JWT validation
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		Addr:        getenv("CHAT_ADDR", ":8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://localhost:5432/chatsem?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   getenv("JWT_SECRET", "change-me-in-production"),
	}
	slog.Info("config loaded", "addr", cfg.Addr)
	return cfg
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
