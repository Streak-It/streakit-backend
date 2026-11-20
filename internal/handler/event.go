package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type EventHandler struct {
	svc service.EventService
}

func NewEventHandler(svc service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

// Create
//
//	@Summary		Create a draft event
//	@Description	Times are RFC 3339 and stored in UTC. If set, doors ≤ start ≤ end. ticket_url must be https.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		dto.CreateEventRequest	true	"Event"
//	@Success		201		{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure		422		{object}	response.Failure
//	@Router			/events [post]
func (h *EventHandler) Create(c *gin.Context) {
	var req dto.CreateEventRequest
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.svc.Create(c.Request.Context(), middleware.CurrentActor(c), req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, "Event created", dto.ToEventDetail(e))
}

// List
//
//	@Summary		List events
//	@Description	Sort by created_at (default desc), updated_at, scheduled_start_at or title.
//	@Tags			events
//	@Produce		json
//	@Security		CookieAuth
//	@Param			query	query		dto.EventListQuery	false	"Filters"
//	@Success		200		{object}	response.Success{data=response.Page[dto.EventResponse]}
//	@Router			/events [get]
func (h *EventHandler) List(c *gin.Context) {
	var q dto.EventListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), q.ToFilter())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToEventPage(page))
}

// Get
//
//	@Summary	Get an event with its lineup
//	@Tags		events
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Event ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure	404	{object}	response.Failure
//	@Router		/events/{id} [get]
func (h *EventHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	e, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToEventDetail(e))
}

// Update
//
//	@Summary		Update event fields
//	@Description	Requires the current `version`. Omitted keys are unchanged; time fields accept null to clear.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Event ID"	format(uuid)
//	@Param			body	body		dto.UpdateEventRequest	true	"Fields to change"
//	@Success		200		{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure		404		{object}	response.Failure
//	@Failure		409		{object}	response.Failure	"version mismatch"
//	@Failure		422		{object}	response.Failure
//	@Router			/events/{id} [patch]
func (h *EventHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateEventRequest
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.svc.Update(c.Request.Context(), middleware.CurrentActor(c), id, req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Event updated", dto.ToEventDetail(e))
}

// ReplaceLineup
//
//	@Summary		Replace the lineup
//	@Description	Replaces all lineup rows. Hidden artists cannot be newly added. A published event must keep at least one headliner.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string						true	"Event ID"	format(uuid)
//	@Param			body	body		dto.ReplaceLineupRequest	true	"Lineup"
//	@Success		200		{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure		404		{object}	response.Failure
//	@Failure		422		{object}	response.Failure
//	@Router			/events/{id}/lineup [put]
func (h *EventHandler) ReplaceLineup(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.ReplaceLineupRequest
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.svc.ReplaceLineup(c.Request.Context(), middleware.CurrentActor(c), id, req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Lineup replaced", dto.ToEventDetail(e))
}

// Publish
//
//	@Summary		Publish a draft event
//	@Description	Requires: active venue, scheduled_start_at, valid time order, ≥1 headliner, no hidden artists.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Event ID"	format(uuid)
//	@Success		200	{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure		409	{object}	response.Failure	"illegal state transition"
//	@Failure		422	{object}	response.Failure	"publish requirements not met"
//	@Router			/events/{id}/publish [post]
func (h *EventHandler) Publish(c *gin.Context) {
	runAction(c, h.svc.Publish, "Event published", dto.ToEventDetail)
}

// Unpublish
//
//	@Summary	Unpublish a scheduled event (back to draft)
//	@Tags		events
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Event ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/events/{id}/unpublish [post]
func (h *EventHandler) Unpublish(c *gin.Context) {
	runAction(c, h.svc.Unpublish, "Event unpublished", dto.ToEventDetail)
}

// Cancel
//
//	@Summary	Cancel a published event
//	@Tags		events
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Event ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/events/{id}/cancel [post]
func (h *EventHandler) Cancel(c *gin.Context) {
	runAction(c, h.svc.Cancel, "Event cancelled", dto.ToEventDetail)
}

// Postpone
//
//	@Summary		Postpone a published event to new times
//	@Description	Status stays `scheduled` with the new times.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string						true	"Event ID"	format(uuid)
//	@Param			body	body		dto.PostponeEventRequest	true	"New times"
//	@Success		200		{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure		409		{object}	response.Failure
//	@Failure		422		{object}	response.Failure
//	@Router			/events/{id}/postpone [post]
func (h *EventHandler) Postpone(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.PostponeEventRequest
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.svc.Postpone(c.Request.Context(), middleware.CurrentActor(c), id, req.ToInput())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "Event postponed", dto.ToEventDetail(e))
}

// Restore
//
//	@Summary	Restore a cancelled event
//	@Tags		events
//	@Accept		json
//	@Produce	json
//	@Security	CookieAuth
//	@Param		id	path		string	true	"Event ID"	format(uuid)
//	@Success	200	{object}	response.Success{data=dto.EventDetailResponse}
//	@Failure	409	{object}	response.Failure
//	@Router		/events/{id}/restore [post]
func (h *EventHandler) Restore(c *gin.Context) {
	runAction(c, h.svc.Restore, "Event restored", dto.ToEventDetail)
}
