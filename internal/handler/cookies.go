package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/service"
)

// CookieConfig controls session cookie attributes.
type CookieConfig struct {
	Secure bool
	Domain string
}

// setSession writes both cookies with the refresh token's lifetime. The
// access cookie deliberately outlives its 15m JWT so an expired token still
// reaches the API and yields token_expired (prompting a refresh) instead of
// silently disappearing and looking like a logout.
func (cfg CookieConfig) setSession(c *gin.Context, s *service.Session) {
	cfg.write(c, auth.AccessCookieName, s.AccessToken, "/", time.Until(s.RefreshExpiresAt))
	cfg.write(c, auth.RefreshCookieName, s.RefreshToken, auth.RefreshCookiePath, time.Until(s.RefreshExpiresAt))
}

func (cfg CookieConfig) clearSession(c *gin.Context) {
	cfg.write(c, auth.AccessCookieName, "", "/", -1)
	cfg.write(c, auth.RefreshCookieName, "", auth.RefreshCookiePath, -1)
}

func (cfg CookieConfig) write(c *gin.Context, name, value, path string, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	if ttl < 0 {
		maxAge = -1 // emits Max-Age=0: delete now
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
