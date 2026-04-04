package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chatsem/services/auth/internal/config"
	"chatsem/services/auth/internal/handler"
	"chatsem/services/auth/internal/repository/postgres"
	"chatsem/services/auth/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
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
	slog.Info("service starting", "service", "auth", "addr", cfg.Addr, "version", buildVersion, "built_at", buildTime)

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	eventRepo := postgres.NewEventRepo(pool)
	userRepo := postgres.NewUserRepo(pool)

	authSvc := service.NewAuthService(eventRepo, userRepo, cfg.JWTSecret, cfg.JWTMaxTTL)
	tokenHandler := handler.NewTokenHandler(authSvc)

	r := handler.NewRouter(tokenHandler, eventRepo)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
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
	slog.Info("service stopped", "service", "auth")
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
