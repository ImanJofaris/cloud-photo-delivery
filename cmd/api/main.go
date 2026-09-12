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
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
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

	// Domain wiring.
	userRepo := users.NewRepository(pool.Pool)
	userSvc := users.NewService(userRepo)
	authRepo := auth.NewRepository(pool.Pool)
	tokenSvc := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	mailer := &auth.LogMailer{Log: func(msg string, args ...any) { log.Info(msg, args...) }}
	authSvc := auth.NewService(userRepo, authRepo, tokenSvc, mailer, auth.Config{
		AccessTTL:        cfg.AccessTokenTTL,
		RefreshTTL:       cfg.RefreshTokenTTL,
		PasswordResetTTL: cfg.PasswordResetTTL,
		LockoutMaxFailed: cfg.LockoutMaxFailed,
		LockoutDuration:  cfg.LockoutDuration,
		PublicBaseURL:    cfg.PublicBaseURL,
	})
	authHandler := auth.NewHandler(authSvc)
	userHandler := users.NewHandler(userSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	eventRepo := events.NewRepository(pool.Pool)
	eventSvc := events.NewService(eventRepo, limits.NewDefault(), auth.HashPassword)
	eventHandler := events.NewHandler(eventSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	authLimiter := httpx.NewRateLimiter(30, 10, 10*time.Minute)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Middleware(httpx.ClientIP))
			r.Post("/auth/signup", authHandler.Signup)
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.Refresh)
			r.Post("/auth/logout", authHandler.Logout)
			r.Post("/auth/password/reset-request", authHandler.RequestPasswordReset)
			r.Post("/auth/password/reset-confirm", authHandler.ConfirmPasswordReset)
		})

		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Get("/account/me", userHandler.Me)
			r.Patch("/account/me", userHandler.UpdateMe)

			r.Route("/events", func(r chi.Router) {
				r.Post("/", eventHandler.Create)
				r.Get("/", eventHandler.List)

				r.Route("/{eventID}", func(r chi.Router) {
					r.Get("/", eventHandler.Get)
					r.Patch("/", eventHandler.Update)
					r.Post("/archive", eventHandler.Archive)
					r.Delete("/", eventHandler.Delete)

					r.Get("/settings", eventHandler.GetSettings)
					r.Patch("/settings", eventHandler.UpdateSettings)
					r.Get("/dashboard", eventHandler.Dashboard)
				})
			})
		})
	})

	return r
}
