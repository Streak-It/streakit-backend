package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db Pinger
}

func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live reports that the process is up. It must not depend on external
// services, otherwise an orchestrator will restart healthy pods when the
// database blips.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service is running",
		"data":    gin.H{"status": "healthy"},
	})
}

// Ready reports whether the service can serve traffic (dependencies reachable).
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "readiness check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "Service is not ready",
			"data":    gin.H{"status": "unavailable", "database": "down"},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service is ready",
		"data":    gin.H{"status": "ready", "database": "up"},
	})
}
