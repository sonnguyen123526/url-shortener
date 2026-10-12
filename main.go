package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/sonnguyen/url-shortener/internal/handler"
	"github.com/sonnguyen/url-shortener/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// All configuration comes from environment variables.
	// In Kubernetes these are injected from a ConfigMap and a Secret.
	var (
		port          = getenv("PORT", "8080")
		redisAddr     = getenv("REDIS_ADDR", "localhost:6379")
		redisPassword = os.Getenv("REDIS_PASSWORD")
		baseURL       = getenv("BASE_URL", "http://localhost:8080")
	)

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr, Password: redisPassword})
	defer rdb.Close()

	h := handler.New(store.NewRedis(rdb), baseURL)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", port, "redis", redisAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	// Kubernetes sends SIGTERM before killing a pod. Finish in-flight requests first.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
