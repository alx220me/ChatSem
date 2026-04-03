package config

import (
	"log/slog"
	"os"
)

// Config holds all runtime configuration for the admin service.
type Config struct {
	Addr        string // HTTP listen address, e.g. ":8082"
	DatabaseURL string // PostgreSQL DSN
	JWTSecret   string // HMAC secret for JWT validation
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		Addr:        getenv("ADMIN_ADDR", ":8082"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://localhost:5432/chatsem?sslmode=disable"),
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
