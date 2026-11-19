package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/handler"
	"github.com/niteshs1/streakit-api/internal/middleware"
)

func Register(router *gin.Engine) {
	router.Use(middleware.RequestID())

	healthHandler := handler.NewHealthHandler()

	router.GET("/health", healthHandler.Check)

	api := router.Group("/api/v1")
	{
		api.GET("/health", healthHandler.Check)
	}
}
