package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type ArtistHandler struct {
	svc service.ArtistService
}

func NewArtistHandler(svc service.ArtistService) *ArtistHandler {
	return &ArtistHandler{svc: svc}
}

// Create
//
//	@Summary		Create an artist
//	@Description	The slug is generated from the name; collisions get -2, -3, …
//	@Tags			artists
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		dto.CreateArtistRequest	true	"Artist"
//	@Success		201		{object}	response.Success{data=dto.ArtistResponse}
//	@Failure		422		{object}	response.Failure
//	@Router			/artists [post]
func (h *ArtistHandler) Create(c *gin.Context) {
	var req dto.CreateArtistRequest
	if !bindJSON(c, &req) {
		return
	}
	a, err := h.svc.Create(c.Request.Context(), middleware.CurrentActor(c), service.CreateArtistInput{Name: req.Name})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, "Artist created", dto.ToArtist(a))
}

// List
//
//	@Summary	List artists
//	@Tags		artists
//	@Produce	json
//	@Security	CookieAuth
//	@Param		query	query		dto.ArtistListQuery	false	"Filters"
//	@Success	200		{object}	response.Success{data=response.Page[dto.ArtistResponse]}
//	@Router		/artists [get]
func (h *ArtistHandler) List(c *gin.Context) {
	var q dto.ArtistListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), q.ToFilter())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToArtistPage(page))
}

// Get
//
//	@Summary	Get an artist
//	@Tags		artists
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Artist ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.ArtistResponse}
//	@Failure	404	{object}	response.Failure
//	@Router		/artists/{id} [get]
func (h *ArtistHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToArtist(a))
}

// Update
//
//	@Summary		Rename an artist
//	@Description	Renaming regenerates the slug.
//	@Tags			artists
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Artist ID"	format(uuid)
//	@Param			body	body		dto.UpdateArtistRequest	true	"Fields to change"
//	@Success		200		{object}	response.Success{data=dto.ArtistResponse}
//	@Failure		404		{object}	response.Failure
//	@Failure		422		{object}	response.Failure
//	@Router			/artists/{id} [patch]
func (h *ArtistHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateArtistRequest
	if !bindJSON(c, &req) {
		return
	}
	a, err := h.svc.Update(c.Request.Context(), middleware.CurrentActor(c), id, service.UpdateArtistInput{Name: req.Name})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Artist updated", dto.ToArtist(a))
}

// Hide
//
//	@Summary	Hide an artist
//	@Tags		artists
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Artist ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.ArtistResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/artists/{id}/hide [post]
func (h *ArtistHandler) Hide(c *gin.Context) {
	runAction(c, h.svc.Hide, "Artist hidden", dto.ToArtist)
}

// Unhide
//
//	@Summary	Unhide an artist
//	@Tags		artists
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Artist ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.ArtistResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/artists/{id}/unhide [post]
func (h *ArtistHandler) Unhide(c *gin.Context) {
	runAction(c, h.svc.Unhide, "Artist visible", dto.ToArtist)
}
