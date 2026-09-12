package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
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

	log.Info("worker started", "env", cfg.Env)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("worker shutting down")
			return
		case <-ticker.C:
			log.Debug("worker heartbeat")
		}
	}
}
