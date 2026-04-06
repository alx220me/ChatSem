package config

import (
	"log/slog"
	"os"
	"time"
)

// Config holds all runtime configuration for the admin service.
type Config struct {
	Addr          string        // HTTP listen address, e.g. ":8082"
	DatabaseURL   string        // PostgreSQL connection string
	JWTSecret     string        // HMAC secret for JWT validation
	JWTMaxTTL     time.Duration // JWT TTL for admin-issued tokens
	RedisAddr     string        // Redis address for ban cache
	AdminUsername string        // Admin login username
	AdminPassword string        // Admin login password (plaintext, hashed at startup)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	ttl, err := time.ParseDuration(getenv("JWT_MAX_TTL", "8h"))
	if err != nil {
		slog.Warn("config: invalid JWT_MAX_TTL, using 8h", "err", err)
		ttl = 8 * time.Hour
	}
	cfg := &Config{
		Addr:          getenv("ADMIN_ADDR", ":8082"),
		DatabaseURL:   getenv("DATABASE_URL", "postgres://chatsem:chatsem@localhost:5432/chatsem"),
		JWTSecret:     getenv("JWT_SECRET", "change-me-in-production"),
		JWTMaxTTL:     ttl,
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		AdminUsername: getenv("ADMIN_USERNAME", "admin"),
		AdminPassword: getenv("ADMIN_PASSWORD", "changeme"),
	}
	slog.Info("config loaded", "addr", cfg.Addr, "admin_username", cfg.AdminUsername)
	return cfg
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
