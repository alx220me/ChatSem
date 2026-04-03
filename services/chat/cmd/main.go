package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chatsem/services/chat/internal/broker"
	"chatsem/services/chat/internal/config"
	"chatsem/services/chat/internal/handler"
	"chatsem/services/chat/internal/repository/postgres"
	"chatsem/services/chat/internal/service"

	pgx "github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	buildVersion = "dev"
	buildTime    = "unknown"
)

func main() {
	// Structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	})))

	cfg := config.Load()
	slog.Info("service starting", "service", "chat", "addr", cfg.Addr, "version", buildVersion, "built_at", buildTime)

	// Redis client
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("redis: ping failed", "addr", cfg.RedisAddr, "err", err)
		os.Exit(1)
	}
	cancel()
	slog.Debug("redis: connected", "addr", cfg.RedisAddr)

	// Message broker
	b := broker.NewRedisBroker(rdb)
	slog.Debug("broker: initialized")
	_ = b // broker will be injected into poll handler in long-polling milestone

	// Database pool
	pool, err := pgx.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database: connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Debug("database: connected")

	// Repositories and services
	chatRepo := postgres.NewChatRepo(pool)
	eventRepo := postgres.NewEventRepo(pool)
	msgRepo := postgres.NewMessageRepo(pool)
	muteRepo := postgres.NewMuteRepo(pool)
	chatSvc := service.NewChatService(chatRepo)
	msgSvc := service.NewMessageService(msgRepo, muteRepo, b, rdb)

	// HTTP router
	r := handler.NewRouter(cfg.JWTSecret, chatSvc, msgSvc, eventRepo, msgRepo, b, rdb)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 35 * time.Second, // >= LongPollTimeout
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("http server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "err", err)
	}
	slog.Info("service stopped", "service", "chat")
}

func logLevel() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
