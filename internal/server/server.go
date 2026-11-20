// Package server wires configuration, repositories, services, handlers and
// middleware into an *http.Server. main stays a thin entrypoint.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/niteshs1/streakit-api/docs/swagger" // registers the OpenAPI doc
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/database"
	"github.com/niteshs1/streakit-api/internal/handler"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/ratelimit"
	"github.com/niteshs1/streakit-api/internal/repository/entrepo"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/routes"
	"github.com/niteshs1/streakit-api/internal/service"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// New migrates (if enabled), bootstraps the first super admin, wires all
// layers and returns a ready-to-serve HTTP server.
func New(ctx context.Context, cfg *config.Config, db *database.DB, logger *slog.Logger) (*http.Server, error) {
	if cfg.Database.AutoMigrate {
		if err := db.Ent.Schema.Create(ctx); err != nil {
			return nil, fmt.Errorf("database schema migration: %w", err)
		}
		logger.Info("database schema synchronized")
	} else {
		logger.Info("auto-migration disabled; skipping schema sync")
	}

	hasher, err := auth.NewPasswordHasher(cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	tokens, err := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL)
	if err != nil {
		return nil, err
	}

	// ---- Repositories ----
	tx := entrepo.NewTxManager(db.Ent)
	users := entrepo.NewUserRepo(db.Ent)
	refresh := entrepo.NewRefreshTokenRepo(db.Ent)
	venues := entrepo.NewVenueRepo(db.Ent)
	artists := entrepo.NewArtistRepo(db.Ent)
	events := entrepo.NewEventRepo(db.Ent)
	audit := entrepo.NewAuditRepo(db.Ent)

	// ---- Services ----
	authSvc := service.NewAuthService(service.AuthDeps{
		Tx: tx, Users: users, RefreshTokens: refresh, Audit: audit,
		Hasher: hasher, Tokens: tokens, RefreshMinter: auth.NewRefreshTokens(cfg.Auth.JWTSecret),
		RefreshTTL:   cfg.Auth.RefreshTTL,
		IPLimiter:    ratelimit.New(cfg.RateLimit.LoginPerIP, cfg.RateLimit.LoginWindow),
		EmailLimiter: ratelimit.New(cfg.RateLimit.LoginPerEmail, cfg.RateLimit.LoginWindow),
		Logger:       logger,
	})
	teamSvc := service.NewTeamService(service.TeamDeps{Tx: tx, Users: users, RefreshTokens: refresh, Audit: audit, Hasher: hasher})
	venueSvc := service.NewVenueService(service.VenueDeps{Tx: tx, Venues: venues, Audit: audit})
	artistSvc := service.NewArtistService(service.ArtistDeps{Tx: tx, Artists: artists, Audit: audit})
	eventSvc := service.NewEventService(service.EventDeps{Tx: tx, Events: events, Venues: venues, Artists: artists, Audit: audit})
	auditSvc := service.NewAuditService(audit)

	if err := authSvc.Bootstrap(ctx, service.BootstrapInput{
		Email:    cfg.Bootstrap.Email,
		Password: cfg.Bootstrap.Password,
		Name:     cfg.Bootstrap.Name,
	}); err != nil {
		return nil, fmt.Errorf("bootstrap super admin: %w", err)
	}

	// ---- HTTP ----
	cookies := handler.CookieConfig{Secure: cfg.Auth.CookieSecure, Domain: cfg.Auth.CookieDomain}
	router, err := NewRouter(cfg.App, logger, routes.Handlers{
		Health: handler.NewHealthHandler(db),
		Auth:   handler.NewAuthHandler(authSvc, cookies),
		Team:   handler.NewTeamHandler(teamSvc),
		Venue:  handler.NewVenueHandler(venueSvc),
		Artist: handler.NewArtistHandler(artistSvc),
		Event:  handler.NewEventHandler(eventSvc),
		Audit:  handler.NewAuditHandler(auditSvc),
	}, routes.Options{
		Authenticator: authSvc,
		EnableSwagger: cfg.App.SwaggerEnabled,
	})
	if err != nil {
		return nil, err
	}

	if cfg.App.SwaggerEnabled {
		logger.Info("swagger UI enabled", "path", "/swagger/index.html")
	}

	return NewHTTPServer(cfg.App, router), nil
}

// NewRouter builds the gin engine with global middleware and routes.
func NewRouter(cfg config.AppConfig, logger *slog.Logger, h routes.Handlers, opts routes.Options) (*gin.Engine, error) {
	switch cfg.Env {
	case "production", "staging":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
	handler.ConfigureValidation()

	router := gin.New()
	router.HandleMethodNotAllowed = true

	// By default gin trusts every proxy, letting clients spoof ClientIP()
	// via X-Forwarded-For. Only trust what is explicitly configured.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	// Order matters: request ID first so the logger and audit can use it.
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(logger))
	router.Use(response.Recovery(logger))
	router.Use(middleware.CORS(cfg.CORSOrigins))
	router.Use(middleware.BodyLimit(maxBodyBytes))

	router.NoRoute(response.NoRoute)
	router.NoMethod(response.NoMethod)

	routes.Register(router, h, opts)

	return router, nil
}

func NewHTTPServer(cfg config.AppConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}
}
