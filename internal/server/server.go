package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/routes"
)

func New(cfg config.AppConfig) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routes.Register(router)

	return router
}

func Address(cfg config.AppConfig) string {
	return fmt.Sprintf(":%s", cfg.Port)
}
