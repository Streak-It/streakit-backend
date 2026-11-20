package middleware

import (
	"context"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/response"
)

const userKey = "auth_user"

// Authenticator verifies an access token and loads the current user.
type Authenticator interface {
	Authenticate(ctx context.Context, accessToken string) (*domain.User, error)
}

// RequireAuth reads the access_token cookie (never the Authorization
// header), verifies it and loads the user from the DB so role and status
// are always current.
func RequireAuth(a Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.AccessCookieName)
		if err != nil || token == "" {
			response.Error(c, apperror.Unauthorized(""))
			return
		}
		u, err := a.Authenticate(c.Request.Context(), token)
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Set(userKey, u)
		c.Next()
	}
}

// RequireRole allows only the listed roles. Must run after RequireAuth.
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			response.Error(c, apperror.Unauthorized(""))
			return
		}
		if !slices.Contains(roles, u.Role) {
			response.Error(c, apperror.Forbidden(""))
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user, or nil.
func CurrentUser(c *gin.Context) *domain.User {
	u, _ := c.Get(userKey)
	user, _ := u.(*domain.User)
	return user
}

// CurrentActor returns the caller as a domain.Actor (zero value if none).
func CurrentActor(c *gin.Context) domain.Actor {
	if u := CurrentUser(c); u != nil {
		return domain.Actor{ID: u.ID, Role: u.Role}
	}
	return domain.Actor{}
}

// SetCurrentUser is used by tests to inject a caller.
func SetCurrentUser(c *gin.Context, u *domain.User) { c.Set(userKey, u) }
