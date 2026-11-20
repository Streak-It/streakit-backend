package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type AuthHandler struct {
	svc     service.AuthService
	cookies CookieConfig
}

func NewAuthHandler(svc service.AuthService, cookies CookieConfig) *AuthHandler {
	return &AuthHandler{svc: svc, cookies: cookies}
}

// Login
//
//	@Summary		Log in with email and password
//	@Description	Sets HttpOnly `access_token` and `refresh_token` cookies. Tokens are never returned in the body.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.LoginRequest	true	"Credentials"
//	@Success		200		{object}	response.Success{data=dto.ProfileResponse}
//	@Header			200		{string}	Set-Cookie			"access_token and refresh_token (HttpOnly)"
//	@Failure		401		{object}	response.Failure	"invalid_credentials"
//	@Failure		422		{object}	response.Failure	"validation_error"
//	@Failure		429		{object}	response.Failure	"rate_limited"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	u, session, err := h.svc.Login(c.Request.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.cookies.setSession(c, session)
	response.OK(c, "Logged in", dto.ToProfile(u))
}

// Refresh
//
//	@Summary		Rotate the session
//	@Description	Uses the `refresh_token` cookie; issues new access and refresh cookies. Reusing an old refresh token revokes all sessions of that user.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	response.Success{data=dto.ProfileResponse}
//	@Failure		401	{object}	response.Failure	"invalid_credentials"
//	@Router			/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(auth.RefreshCookieName)
	u, session, err := h.svc.Refresh(c.Request.Context(), raw)
	if err != nil {
		if apperror.HasCode(err, apperror.CodeInvalidCredentials) {
			h.cookies.clearSession(c)
		}
		response.Error(c, err)
		return
	}
	h.cookies.setSession(c, session)
	response.OK(c, "Session refreshed", dto.ToProfile(u))
}

// Logout
//
//	@Summary		Log out
//	@Description	Revokes the current refresh token and clears both cookies. Works even if the access token has expired.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	response.Success
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(auth.RefreshCookieName)
	err := h.svc.Logout(c.Request.Context(), raw)
	h.cookies.clearSession(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Logged out", nil)
}

// Me
//
//	@Summary	Current user
//	@Tags		auth
//	@Produce	json
//	@Security	CookieAuth
//	@Success	200	{object}	response.Success{data=dto.ProfileResponse}
//	@Failure	401	{object}	response.Failure	"unauthorized / token_expired"
//	@Router		/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	response.OK(c, "OK", dto.ToProfile(middleware.CurrentUser(c)))
}

// ChangePassword
//
//	@Summary		Change own password
//	@Description	Revokes every refresh token of the caller and clears the session cookies; log in again afterwards.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		dto.ChangePasswordRequest	true	"Passwords"
//	@Success		200		{object}	response.Success
//	@Failure		401		{object}	response.Failure
//	@Failure		422		{object}	response.Failure
//	@Router			/auth/password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	err := h.svc.ChangePassword(c.Request.Context(), middleware.CurrentActor(c), service.ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.cookies.clearSession(c)
	response.OK(c, "Password changed; please log in again", nil)
}
