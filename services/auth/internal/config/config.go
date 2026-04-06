package config

import (
	"log/slog"
	"os"
	"time"
)

// Config holds all runtime configuration for the auth service.
type Config struct {
	Addr        string        // HTTP listen address, e.g. ":8081"
	DatabaseURL string        // PostgreSQL connection string
	RedisAddr   string        // Redis address (for session storage)
	JWTSecret   string        // HMAC secret for JWT signing and validation
	JWTMaxTTL   time.Duration // Maximum token lifetime
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	ttl, err := time.ParseDuration(getenv("JWT_MAX_TTL", "4h"))
	if err != nil {
		ttl = 4 * time.Hour
	}
	cfg := &Config{
		Addr:        getenv("AUTH_ADDR", ":8081"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://chatsem:chatsem@localhost:5432/chatsem"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   getenv("JWT_SECRET", "change-me-in-production"),
		JWTMaxTTL:   ttl,
	}
	slog.Info("config loaded", "addr", cfg.Addr, "jwt_max_ttl", cfg.JWTMaxTTL)
	return cfg
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
