package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chatsem/services/admin/internal/config"
	"chatsem/services/admin/internal/handler"
	"chatsem/services/admin/internal/repository/postgres"
	"chatsem/services/admin/internal/service"

	pgx "github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	buildVersion = "dev"
	buildTime    = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	})))

	cfg := config.Load()
	slog.Info("service starting", "service", "admin", "addr", cfg.Addr, "version", buildVersion, "built_at", buildTime)

	// Database pool
	pool, err := pgx.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database: connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Debug("database: connected")

	// Redis client for ban cache
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx2).Err(); err != nil {
		slog.Warn("redis: ping failed, ban cache unavailable", "addr", cfg.RedisAddr, "err", err)
	}
	cancel2()

	// Repositories and services
	eventRepo := postgres.NewEventRepo(pool)
	chatRepo := postgres.NewChatRepo(pool)
	banRepo := postgres.NewBanRepo(pool)
	eventSvc := service.NewEventService(eventRepo, chatRepo)
	banSvc := service.NewBanService(banRepo, rdb)

	r := handler.NewRouter(cfg.JWTSecret, eventSvc, banSvc)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  60 * time.Second, // export may take time
		WriteTimeout: 65 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
	slog.Info("service stopped", "service", "admin")
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
