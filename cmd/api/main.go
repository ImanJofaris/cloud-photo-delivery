package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

func main() {
	cfgPath := flag.String("config", "", "path to .env file (optional)")
	_ = cfgPath
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		// Logger not yet built; use default.
		logging.New("dev", "info").Error("config load failed", "error", err)
		os.Exit(1)
	}

	log := logging.New(cfg.Env, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	router := NewRouter(cfg, log, pool)

	srv := httpx.NewServer(cfg.HTTPAddr, router)

	go func() {
		log.Info("api server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
}

func NewRouter(cfg config.Config, log *slog.Logger, pool *database.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(httpx.RequestID)
	r.Use(httpx.Recover)
	r.Use(httpx.Logging(log))
	r.Use(httpx.CORS([]string{"*"}))

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		httpx.Success(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.Fail(w, http.StatusServiceUnavailable, "NOT_READY", "Database is not available")
			return
		}
		httpx.Success(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	return r
}
