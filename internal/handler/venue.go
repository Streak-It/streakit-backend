package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type VenueHandler struct {
	svc service.VenueService
}

func NewVenueHandler(svc service.VenueService) *VenueHandler {
	return &VenueHandler{svc: svc}
}

// Create
//
//	@Summary	Create a venue
//	@Tags		venues
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		body	body		dto.CreateVenueRequest	true	"Venue"
//	@Success	201		{object}	response.Success{data=dto.VenueResponse}
//	@Failure	422		{object}	response.Failure
//	@Router		/venues [post]
func (h *VenueHandler) Create(c *gin.Context) {
	var req dto.CreateVenueRequest
	if !bindJSON(c, &req) {
		return
	}
	v, err := h.svc.Create(c.Request.Context(), middleware.CurrentActor(c), req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, "Venue created", dto.ToVenue(v))
}

// List
//
//	@Summary		List venues
//	@Description	Sort by name (default), city, created_at or updated_at; prefix with - for descending.
//	@Tags			venues
//	@Produce		json
//	@Security		CookieAuth
//	@Param			query	query		dto.VenueListQuery	false	"Filters"
//	@Success		200		{object}	response.Success{data=response.Page[dto.VenueResponse]}
//	@Router			/venues [get]
func (h *VenueHandler) List(c *gin.Context) {
	var q dto.VenueListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), q.ToFilter())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToVenuePage(page))
}

// Get
//
//	@Summary	Get a venue
//	@Tags		venues
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Venue ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.VenueResponse}
//	@Failure	404	{object}	response.Failure
//	@Router		/venues/{id} [get]
func (h *VenueHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	v, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToVenue(v))
}

// Update
//
//	@Summary		Update a venue
//	@Description	Requires the current `version`; a stale version returns 409 conflict.
//	@Tags			venues
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Venue ID"	format(uuid)
//	@Param			body	body		dto.UpdateVenueRequest	true	"Fields to change"
//	@Success		200		{object}	response.Success{data=dto.VenueResponse}
//	@Failure		404		{object}	response.Failure
//	@Failure		409		{object}	response.Failure	"version mismatch"
//	@Failure		422		{object}	response.Failure
//	@Router			/venues/{id} [patch]
func (h *VenueHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateVenueRequest
	if !bindJSON(c, &req) {
		return
	}
	v, err := h.svc.Update(c.Request.Context(), middleware.CurrentActor(c), id, req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Venue updated", dto.ToVenue(v))
}

// Retire
//
//	@Summary	Retire a venue
//	@Tags		venues
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Venue ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.VenueResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/venues/{id}/retire [post]
func (h *VenueHandler) Retire(c *gin.Context) {
	runAction(c, h.svc.Retire, "Venue retired", dto.ToVenue)
}

// Restore
//
//	@Summary	Restore a retired venue
//	@Tags		venues
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Venue ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.VenueResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/venues/{id}/restore [post]
func (h *VenueHandler) Restore(c *gin.Context) {
	runAction(c, h.svc.Restore, "Venue restored", dto.ToVenue)
}
