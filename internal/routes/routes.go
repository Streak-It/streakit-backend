// Package routes maps URLs to handlers and attaches per-route middleware.
package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/handler"
	"github.com/niteshs1/streakit-api/internal/middleware"
)

type Handlers struct {
	Health *handler.HealthHandler
	Auth   *handler.AuthHandler
	Team   *handler.TeamHandler
	Venue  *handler.VenueHandler
	Artist *handler.ArtistHandler
	Event  *handler.EventHandler
	Audit  *handler.AuditHandler
}

type Options struct {
	// Authenticator backs the access-cookie middleware.
	Authenticator middleware.Authenticator
	// EnableSwagger mounts /swagger (SWAGGER_ENABLED, default on).
	EnableSwagger bool
}

func Register(router *gin.Engine, h Handlers, opts Options) {
	// Probes at the root for orchestrators (k8s / docker healthcheck).
	router.GET("/health", h.Health.Live)
	router.GET("/ready", h.Health.Ready)

	if opts.EnableSwagger {
		router.GET("/swagger", func(c *gin.Context) { c.Redirect(302, "/swagger/index.html") })
		router.GET("/swagger/index.html", handler.SwaggerUI)
		router.GET("/swagger/doc.json", handler.SwaggerDoc)
	}

	api := router.Group("/api/v1", middleware.RequireJSON())
	api.GET("/health", h.Health.Live)
	api.GET("/ready", h.Health.Ready)

	authed := middleware.RequireAuth(opts.Authenticator)
	superAdmin := middleware.RequireRole(domain.RoleSuperAdmin)
	staff := middleware.RequireRole(domain.RoleSuperAdmin, domain.RoleAdmin)

	// ---- Auth (login/refresh/logout are public) ----
	a := api.Group("/auth")
	a.POST("/login", h.Auth.Login)
	a.POST("/refresh", h.Auth.Refresh)
	a.POST("/logout", h.Auth.Logout)
	a.GET("/me", authed, h.Auth.Me)
	a.POST("/password", authed, h.Auth.ChangePassword)

	// Everything below requires an authenticated staff member.
	s := api.Group("", authed, staff)

	// ---- Team: read for all staff, write for super admins ----
	team := s.Group("/team/users")
	team.GET("", h.Team.List)
	team.GET("/:id", h.Team.Get)
	team.POST("", superAdmin, h.Team.Create)
	team.PATCH("/:id", superAdmin, h.Team.Update)
	team.POST("/:id/block", superAdmin, h.Team.Block)
	team.POST("/:id/unblock", superAdmin, h.Team.Unblock)
	team.POST("/:id/role", superAdmin, h.Team.ChangeRole)

	// ---- Venues ----
	v := s.Group("/venues")
	v.POST("", h.Venue.Create)
	v.GET("", h.Venue.List)
	v.GET("/:id", h.Venue.Get)
	v.PATCH("/:id", h.Venue.Update)
	v.POST("/:id/retire", h.Venue.Retire)
	v.POST("/:id/restore", h.Venue.Restore)

	// ---- Artists ----
	ar := s.Group("/artists")
	ar.POST("", h.Artist.Create)
	ar.GET("", h.Artist.List)
	ar.GET("/:id", h.Artist.Get)
	ar.PATCH("/:id", h.Artist.Update)
	ar.POST("/:id/hide", h.Artist.Hide)
	ar.POST("/:id/unhide", h.Artist.Unhide)

	// ---- Events ----
	e := s.Group("/events")
	e.POST("", h.Event.Create)
	e.GET("", h.Event.List)
	e.GET("/:id", h.Event.Get)
	e.PATCH("/:id", h.Event.Update)
	e.PUT("/:id/lineup", h.Event.ReplaceLineup)
	e.POST("/:id/publish", h.Event.Publish)
	e.POST("/:id/unpublish", h.Event.Unpublish)
	e.POST("/:id/cancel", h.Event.Cancel)
	e.POST("/:id/postpone", h.Event.Postpone)
	e.POST("/:id/restore", h.Event.Restore)

	// ---- Audit ----
	s.GET("/audit", h.Audit.List)
}
