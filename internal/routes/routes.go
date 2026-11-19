package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/handler"
)

type Handlers struct {
	Health *handler.HealthHandler
}

func Register(router *gin.Engine, h Handlers) {
	// Probes at the root for orchestrators (k8s / docker healthcheck).
	router.GET("/health", h.Health.Live)
	router.GET("/ready", h.Health.Ready)

	api := router.Group("/api/v1")
	{
		api.GET("/health", h.Health.Live)
	}
}
