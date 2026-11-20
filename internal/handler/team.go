package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type TeamHandler struct {
	svc service.TeamService
}

func NewTeamHandler(svc service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// Create
//
//	@Summary	Create a team user (super admin)
//	@Tags		team
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		body	body		dto.CreateUserRequest	true	"New user"
//	@Success	201		{object}	response.Success{data=dto.UserResponse}
//	@Failure	403		{object}	response.Failure
//	@Failure	409		{object}	response.Failure	"email already in use"
//	@Failure	422		{object}	response.Failure
//	@Router		/team/users [post]
func (h *TeamHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.Create(c.Request.Context(), middleware.CurrentActor(c), req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, "User created", dto.ToUser(u))
}

// List
//
//	@Summary	List team users
//	@Tags		team
//	@Produce	json
//	@Security	CookieAuth
//	@Param		query	query		dto.UserListQuery	false	"Filters"
//	@Success	200		{object}	response.Success{data=response.Page[dto.UserResponse]}
//	@Failure	422		{object}	response.Failure
//	@Router		/team/users [get]
func (h *TeamHandler) List(c *gin.Context) {
	var q dto.UserListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), q.ToFilter())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToUserPage(page))
}

// Get
//
//	@Summary	Get a team user
//	@Tags		team
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"User ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.UserResponse}
//	@Failure	404	{object}	response.Failure
//	@Router		/team/users/{id} [get]
func (h *TeamHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToUser(u))
}

// Update
//
//	@Summary	Update name / email (super admin)
//	@Tags		team
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id		path		string					true	"User ID"	format(uuid)
//	@Param		body	body		dto.UpdateUserRequest	true	"Fields to change"
//	@Success	200		{object}	response.Success{data=dto.UserResponse}
//	@Failure	403		{object}	response.Failure
//	@Failure	404		{object}	response.Failure
//	@Failure	409		{object}	response.Failure
//	@Router		/team/users/{id} [patch]
func (h *TeamHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.Update(c.Request.Context(), middleware.CurrentActor(c), id, req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "User updated", dto.ToUser(u))
}

// Block
//
//	@Summary		Block a user (super admin)
//	@Description	Revokes all of the user's sessions. You cannot block yourself or the last active super admin.
//	@Tags			team
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"User ID"	format(uuid)
//	@Success		200	{object}	response.Success{data=dto.UserResponse}
//	@Failure		403	{object}	response.Failure
//	@Failure		409	{object}	response.Failure
//	@Router			/team/users/{id}/block [post]
func (h *TeamHandler) Block(c *gin.Context) {
	h.userAction(c, h.svc.Block, "User blocked")
}

// Unblock
//
//	@Summary	Unblock a user (super admin)
//	@Tags		team
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"User ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.UserResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/team/users/{id}/unblock [post]
func (h *TeamHandler) Unblock(c *gin.Context) {
	h.userAction(c, h.svc.Unblock, "User unblocked")
}

// ChangeRole
//
//	@Summary		Change a user's role (super admin)
//	@Description	Revokes all of the user's sessions. You cannot change your own role or demote the last active super admin.
//	@Tags			team
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"User ID"	format(uuid)
//	@Param			body	body		dto.ChangeRoleRequest	true	"New role"
//	@Success		200		{object}	response.Success{data=dto.UserResponse}
//	@Failure		403		{object}	response.Failure
//	@Failure		409		{object}	response.Failure
//	@Router			/team/users/{id}/role [post]
func (h *TeamHandler) ChangeRole(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.ChangeRoleRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.ChangeRole(c.Request.Context(), middleware.CurrentActor(c), id, domain.Role(req.Role))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Role changed", dto.ToUser(u))
}

func (h *TeamHandler) userAction(c *gin.Context, fn actionFunc[domain.User], msg string) {
	runAction(c, fn, msg, dto.ToUser)
}
