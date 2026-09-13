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
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/imanjofaris/cloud-photo-delivery/internal/devices"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/gallery"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/internal/qr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/uploads"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
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
	billingRepo := billing.NewRepository(pool.Pool)
	billingProvider := provider.NewManual(cfg.BillingWebhookSecret)
	billingSvc := billing.NewService(billingRepo, billingProvider)
	billingHandler := billing.NewHandler(billingSvc, billingProvider, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})
	entitlements := billing.NewEntitlements(billingRepo)
	eventSvc := events.NewService(eventRepo, entitlements, auth.HashPassword)
	eventHandler := events.NewHandler(eventSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})
	qrSvc := qr.NewService(eventSvc, cfg.PublicBaseURL)
	qrHandler := qr.NewHandler(qrSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	photoRepo := photos.NewRepository(pool.Pool)
	uploadRepo := uploads.NewRepository(pool.Pool)
	var store r2.ObjectStore = r2.UnavailableStore{}
	if err := cfg.ValidateStorage(); err == nil {
		s, err := r2.New(context.Background(), r2.Options{
			Endpoint:  cfg.R2Endpoint,
			AccessKey: cfg.R2AccessKey,
			SecretKey: cfg.R2SecretKey,
			Bucket:    cfg.R2Bucket,
			Region:    cfg.R2Region,
		})
		if err != nil {
			log.Error("object store init failed", "error", err)
			os.Exit(1)
		}
		store = s
	}
	brandingSvc := users.NewBrandingService(userRepo, store, cfg.SignedURLTTL)
	brandingHandler := users.NewBrandingHandler(brandingSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})
	uploadSvc := uploads.NewService(photoRepo, uploadRepo, store, uploads.NewPostgresQueue(pool.Pool), entitlements)
	uploadHandler := uploads.NewHandler(uploadSvc, func(req *http.Request) (uploads.Actor, bool) {
		if device, ok := devices.FromContext(req.Context()); ok {
			return uploads.Actor{
				UserID:        device.UserID,
				DeviceID:      device.ID,
				AssignedEvent: device.AssignedEventID,
			}, true
		}
		id, ok := auth.UserID(req.Context())
		if !ok {
			return uploads.Actor{}, false
		}
		return uploads.Actor{UserID: id}, true
	})

	deviceRepo := devices.NewRepository(pool.Pool)
	deviceSvc := devices.NewService(deviceRepo)
	deviceHandler := devices.NewHandler(deviceSvc, func(req *http.Request) (string, bool) {
		id, ok := auth.UserID(req.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	galleryRepo := gallery.NewRepository(pool.Pool)
	galleryURLs := photos.NewSignedURLGenerator(store, cfg.SignedURLTTL)
	galleryTokens := gallery.NewUnlockTokens(cfg.JWTSecret, cfg.GalleryUnlockTTL)
	gallerySvc := gallery.NewService(galleryRepo, galleryURLs, galleryTokens, auth.VerifyPassword, brandingSvc)
	galleryHandler := gallery.NewHandler(gallerySvc)

	photoURLs := photos.NewSignedURLGenerator(store, cfg.SignedURLTTL)
	photoSvc := photos.NewService(photoRepo, photoURLs, store, photos.NewPostgresQueue(pool.Pool))
	photoHandler := photos.NewHandler(photoSvc, func(req *http.Request) (photos.Actor, bool) {
		if device, ok := devices.FromContext(req.Context()); ok {
			return photos.Actor{
				UserID:        device.UserID,
				DeviceID:      device.ID,
				AssignedEvent: device.AssignedEventID,
			}, true
		}
		id, ok := auth.UserID(req.Context())
		if !ok {
			return photos.Actor{}, false
		}
		return photos.Actor{UserID: id}, true
	})

	authLimiter := httpx.NewRateLimiter(30, 10, 10*time.Minute)
	deviceUploadLimiter := httpx.NewRateLimiter(100, 100, 10*time.Minute)

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

		// Public gallery (no auth).
		r.Route("/public/events/{slug}", func(r chi.Router) {
			r.Get("/", galleryHandler.GetEvent)
			r.Post("/unlock", galleryHandler.Unlock)
			r.Get("/photos", galleryHandler.ListPhotos)
			r.Get("/photos/{photoID}", galleryHandler.GetPhoto)
			r.Get("/photos/{photoID}/url", galleryHandler.PhotoURL)
		})

		// Provider callback: unauthenticated but signature-verified.
		r.Post("/billing/webhook", billingHandler.Webhook)

		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Get("/account/me", userHandler.Me)
			r.Patch("/account/me", userHandler.UpdateMe)
			r.Get("/account/branding", brandingHandler.Get)
			r.Patch("/account/branding", brandingHandler.Update)
			r.Post("/account/branding/assets", brandingHandler.CreateAssetUpload)

			r.Route("/billing", func(r chi.Router) {
				r.Get("/plans", billingHandler.Plans)
				r.Get("/subscription", billingHandler.GetSubscription)
				r.Post("/subscribe", billingHandler.Subscribe)
				r.Post("/upgrade", billingHandler.Upgrade)
				r.Post("/downgrade", billingHandler.Downgrade)
				r.Post("/cancel", billingHandler.Cancel)
				r.Post("/resume", billingHandler.Resume)
				r.Get("/invoices", billingHandler.Invoices)
			})

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
					r.Get("/url", qrHandler.URL)
					r.Get("/qr.png", qrHandler.PNG)
					r.Get("/qr.svg", qrHandler.SVG)
				})
			})

			r.Route("/devices", func(r chi.Router) {
				r.Post("/", deviceHandler.Create)
				r.Get("/", deviceHandler.List)
				r.Route("/{deviceID}", func(r chi.Router) {
					r.Patch("/", deviceHandler.Update)
					r.Delete("/", deviceHandler.Revoke)
					r.Post("/rotate", deviceHandler.Rotate)
				})
			})
		})

		// Uploads accept an operator JWT or a device key scoped to the event.
		r.Group(func(r chi.Router) {
			r.Use(deviceSvc.RequireDeviceOrOperator(authSvc.RequireAuth))
			r.With(devices.RateLimit(deviceUploadLimiter)).Post("/events/{eventID}/uploads", uploadHandler.Initialize)
			r.Get("/events/{eventID}/photos", photoHandler.List)
			r.Route("/uploads/{photoID}", func(r chi.Router) {
				r.Get("/", uploadHandler.Status)
				r.Post("/url", uploadHandler.RePresign)
				r.Post("/parts", uploadHandler.Parts)
				r.Post("/multipart/complete", uploadHandler.CompleteMultipart)
				r.Post("/multipart/abort", uploadHandler.AbortMultipart)
				r.Post("/complete", uploadHandler.Complete)
			})
			r.Route("/photos/{photoID}", func(r chi.Router) {
				r.Get("/url", photoHandler.URL)
				r.Delete("/", photoHandler.Delete)
			})
		})
	})

	return r
}
