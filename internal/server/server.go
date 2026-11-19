package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/routes"
)

func NewRouter(cfg config.AppConfig, logger *slog.Logger, h routes.Handlers) (*gin.Engine, error) {
	switch cfg.Env {
	case "production", "staging":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// By default gin trusts every proxy, letting clients spoof ClientIP()
	// via X-Forwarded-For. Only trust what is explicitly configured.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	// Order matters: request ID first so the logger can include it.
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(logger))
	router.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		logger.ErrorContext(c.Request.Context(), "panic recovered",
			"request_id", c.GetString(middleware.RequestIDKey),
			"error", err,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Internal server error",
		})
	}))

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Route not found"})
	})

	routes.Register(router, h)

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
