// Package handler contains thin HTTP handlers: bind input, call a
// service, write the response envelope. No business rules live here.
package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/response"
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

type HealthStatus struct {
	Status   string `json:"status" example:"healthy"`
	Database string `json:"database,omitempty" example:"up"`
}

// Live reports that the process is up. It must not depend on external
// services, otherwise an orchestrator will restart healthy pods when the
// database blips.
//
//	@Summary	Liveness probe
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	response.Success{data=handler.HealthStatus}
//	@Router		/health [get]
func (h *HealthHandler) Live(c *gin.Context) {
	response.OK(c, "Service is running", HealthStatus{Status: "healthy"})
}

// Ready reports whether the service can serve traffic (dependencies reachable).
//
//	@Summary	Readiness probe
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	response.Success{data=handler.HealthStatus}
//	@Failure	503	{object}	response.Success{data=handler.HealthStatus}
//	@Router		/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "readiness check failed", "error", err)
		response.Unavailable(c, "Service is not ready", HealthStatus{Status: "unavailable", Database: "down"})
		return
	}
	response.OK(c, "Service is ready", HealthStatus{Status: "ready", Database: "up"})
}
