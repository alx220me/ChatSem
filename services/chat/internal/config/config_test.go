package config_test

import (
	"os"
	"testing"

	"chatsem/services/chat/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Logf("[%s] stage: loading config with no env vars set", t.Name())
	// Ensure env vars are cleared
	os.Unsetenv("CHAT_ADDR")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("JWT_SECRET")

	cfg := config.Load()

	if cfg.Addr != ":8080" {
		t.Errorf("[%s] Addr: want :8080, got %s", t.Name(), cfg.Addr)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("[%s] RedisAddr: want localhost:6379, got %s", t.Name(), cfg.RedisAddr)
	}
	t.Logf("[%s] stage: defaults verified: addr=%s redis=%s", t.Name(), cfg.Addr, cfg.RedisAddr)
}

func TestLoad_FromEnv(t *testing.T) {
	t.Logf("[%s] stage: loading config from env vars", t.Name())
	os.Setenv("CHAT_ADDR", ":9999")
	os.Setenv("REDIS_ADDR", "redis:6380")
	os.Setenv("JWT_SECRET", "my-secret")
	t.Cleanup(func() {
		os.Unsetenv("CHAT_ADDR")
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("JWT_SECRET")
	})

	cfg := config.Load()

	if cfg.Addr != ":9999" {
		t.Errorf("[%s] Addr: want :9999, got %s", t.Name(), cfg.Addr)
	}
	if cfg.RedisAddr != "redis:6380" {
		t.Errorf("[%s] RedisAddr: want redis:6380, got %s", t.Name(), cfg.RedisAddr)
	}
	if cfg.JWTSecret != "my-secret" {
		t.Errorf("[%s] JWTSecret: want my-secret, got %s", t.Name(), cfg.JWTSecret)
	}
	t.Logf("[%s] stage: env vars read correctly", t.Name())
}
