package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/patch"
	"github.com/niteshs1/streakit-api/internal/repository"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

// ---- Venues ----

type CreateVenueRequest struct {
	Name     string `json:"name" binding:"required,max=200" example:"Blue Frog"`
	City     string `json:"city" binding:"required,max=100" example:"Mumbai"`
	Address  string `json:"address" binding:"max=500" example:"Mathuradas Mills, Lower Parel"`
	Info     string `json:"info" binding:"max=10000" example:"Entry from Gate 2"`
	Timezone string `json:"timezone" binding:"omitempty,max=64" example:"Asia/Kolkata"`
}

func (r CreateVenueRequest) ToInput() service.CreateVenueInput {
	return service.CreateVenueInput{Name: r.Name, City: r.City, Address: r.Address, Info: r.Info, Timezone: r.Timezone}
}

type UpdateVenueRequest struct {
	// Version must match the stored version (optimistic locking).
	Version  int     `json:"version" binding:"required,min=1" example:"1"`
	Name     *string `json:"name" binding:"omitempty,max=200"`
	City     *string `json:"city" binding:"omitempty,max=100"`
	Address  *string `json:"address" binding:"omitempty,max=500"`
	Info     *string `json:"info" binding:"omitempty,max=10000"`
	Timezone *string `json:"timezone" binding:"omitempty,max=64"`
}

func (r UpdateVenueRequest) ToInput() service.UpdateVenueInput {
	return service.UpdateVenueInput{Version: r.Version, Name: r.Name, City: r.City, Address: r.Address, Info: r.Info, Timezone: r.Timezone}
}

type VenueListQuery struct {
	PageQuery
	City   string `form:"city" binding:"omitempty,max=100"`
	Name   string `form:"name" binding:"omitempty,max=200"`
	Status string `form:"status" binding:"omitempty,oneof=active retired" enums:"active,retired"`
}

func (q VenueListQuery) ToFilter() repository.VenueFilter {
	return repository.VenueFilter{
		PageRequest: q.ToDomain(),
		City:        q.City,
		Name:        q.Name,
		Status:      optEnum[domain.VenueStatus](q.Status),
	}
}

type VenueResponse struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	City      string             `json:"city"`
	Address   string             `json:"address"`
	Info      string             `json:"info"`
	Timezone  string             `json:"timezone" example:"Asia/Kolkata"`
	Status    domain.VenueStatus `json:"status" enums:"active,retired"`
	Version   int                `json:"version" example:"1"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func ToVenue(v *domain.Venue) VenueResponse {
	return VenueResponse{
		ID: v.ID, Name: v.Name, City: v.City, Address: v.Address, Info: v.Info,
		Timezone: v.Timezone, Status: v.Status, Version: v.Version,
		CreatedAt: v.CreatedAt.UTC(), UpdatedAt: v.UpdatedAt.UTC(),
	}
}

func ToVenuePage(p domain.PageResult[domain.Venue]) response.Page[VenueResponse] {
	return mapPage(p, ToVenue)
}

// ---- Artists ----

type CreateArtistRequest struct {
	Name string `json:"name" binding:"required,max=200" example:"Prateek Kuhad"`
}

type UpdateArtistRequest struct {
	Name *string `json:"name" binding:"omitempty,max=200" example:"Prateek Kuhad"`
}

type ArtistListQuery struct {
	PageQuery
	Name   string `form:"name" binding:"omitempty,max=200"`
	Status string `form:"status" binding:"omitempty,oneof=active hidden" enums:"active,hidden"`
}

func (q ArtistListQuery) ToFilter() repository.ArtistFilter {
	return repository.ArtistFilter{
		PageRequest: q.ToDomain(),
		Name:        q.Name,
		Status:      optEnum[domain.ArtistStatus](q.Status),
	}
}

type ArtistResponse struct {
	ID        uuid.UUID           `json:"id"`
	Name      string              `json:"name"`
	Slug      string              `json:"slug" example:"prateek-kuhad"`
	Status    domain.ArtistStatus `json:"status" enums:"active,hidden"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

func ToArtist(a *domain.Artist) ArtistResponse {
	return ArtistResponse{
		ID: a.ID, Name: a.Name, Slug: a.Slug, Status: a.Status,
		CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC(),
	}
}

func ToArtistPage(p domain.PageResult[domain.Artist]) response.Page[ArtistResponse] {
	return mapPage(p, ToArtist)
}

// ---- Events ----

type CreateEventRequest struct {
	VenueID          string     `json:"venue_id" binding:"required,uuid" format:"uuid"`
	Title            string     `json:"title" binding:"max=200" example:"Cold/Mess Tour"`
	Description      string     `json:"description" binding:"max=20000"`
	DoorsOpenAt      *time.Time `json:"doors_open_at" example:"2026-12-20T18:30:00Z"`
	ScheduledStartAt *time.Time `json:"scheduled_start_at" example:"2026-12-20T19:30:00Z"`
	ExpectedEndAt    *time.Time `json:"expected_end_at" example:"2026-12-20T22:30:00Z"`
	TicketURL        string     `json:"ticket_url" binding:"omitempty,max=2048,url" example:"https://tickets.example.com/e/123"`
}

func (r CreateEventRequest) ToInput() service.CreateEventInput {
	id, _ := uuid.Parse(r.VenueID) // validated by binding
	return service.CreateEventInput{
		VenueID: id, Title: r.Title, Description: r.Description,
		DoorsOpenAt: r.DoorsOpenAt, ScheduledStartAt: r.ScheduledStartAt, ExpectedEndAt: r.ExpectedEndAt,
		TicketURL: r.TicketURL,
	}
}

// UpdateEventRequest: omitted keys are unchanged. Time fields accept null
// to clear them.
type UpdateEventRequest struct {
	Version          int                    `json:"version" binding:"required,min=1" example:"1"`
	VenueID          *string                `json:"venue_id" binding:"omitempty,uuid" format:"uuid"`
	Title            *string                `json:"title" binding:"omitempty,max=200"`
	Description      *string                `json:"description" binding:"omitempty,max=20000"`
	TicketURL        *string                `json:"ticket_url" binding:"omitempty,max=2048"`
	DoorsOpenAt      patch.Field[time.Time] `json:"doors_open_at" swaggertype:"string" format:"date-time" extensions:"x-nullable"`
	ScheduledStartAt patch.Field[time.Time] `json:"scheduled_start_at" swaggertype:"string" format:"date-time" extensions:"x-nullable"`
	ExpectedEndAt    patch.Field[time.Time] `json:"expected_end_at" swaggertype:"string" format:"date-time" extensions:"x-nullable"`
}

func (r UpdateEventRequest) ToInput() service.UpdateEventInput {
	return service.UpdateEventInput{
		Version: r.Version, VenueID: optUUIDPtr(r.VenueID), Title: r.Title, Description: r.Description,
		TicketURL: r.TicketURL, DoorsOpenAt: r.DoorsOpenAt, ScheduledStartAt: r.ScheduledStartAt, ExpectedEndAt: r.ExpectedEndAt,
	}
}

func optUUIDPtr(s *string) *uuid.UUID {
	if s == nil {
		return nil
	}
	return optUUID(*s)
}

type LineupItemRequest struct {
	ArtistID  string `json:"artist_id" binding:"required,uuid" format:"uuid"`
	Billing   string `json:"billing" binding:"required,oneof=headliner support" enums:"headliner,support" example:"headliner"`
	SortOrder int    `json:"sort_order" binding:"min=0" example:"0"`
}

// ReplaceLineupRequest replaces the whole lineup; [] clears it.
type ReplaceLineupRequest struct {
	Lineup []LineupItemRequest `json:"lineup" binding:"required,max=100,dive"`
}

func (r ReplaceLineupRequest) ToInput() []repository.LineupInput {
	out := make([]repository.LineupInput, len(r.Lineup))
	for i, l := range r.Lineup {
		id, _ := uuid.Parse(l.ArtistID) // validated by binding
		out[i] = repository.LineupInput{ArtistID: id, Billing: domain.Billing(l.Billing), SortOrder: l.SortOrder}
	}
	return out
}

type PostponeEventRequest struct {
	DoorsOpenAt      *time.Time `json:"doors_open_at" example:"2027-01-10T18:30:00Z"`
	ScheduledStartAt *time.Time `json:"scheduled_start_at" binding:"required" example:"2027-01-10T19:30:00Z"`
	ExpectedEndAt    *time.Time `json:"expected_end_at" example:"2027-01-10T22:30:00Z"`
}

func (r PostponeEventRequest) ToInput() service.PostponeInput {
	in := service.PostponeInput{DoorsOpenAt: r.DoorsOpenAt, ExpectedEndAt: r.ExpectedEndAt}
	if r.ScheduledStartAt != nil {
		in.ScheduledStartAt = *r.ScheduledStartAt
	}
	return in
}

type EventListQuery struct {
	PageQuery
	PublishState string `form:"publish_state" binding:"omitempty,oneof=draft published" enums:"draft,published"`
	Status       string `form:"status" binding:"omitempty,oneof=scheduled cancelled postponed" enums:"scheduled,cancelled,postponed"`
	VenueID      string `form:"venue_id" binding:"omitempty,uuid" format:"uuid"`
}

func (q EventListQuery) ToFilter() repository.EventFilter {
	return repository.EventFilter{
		PageRequest:  q.ToDomain(),
		PublishState: optEnum[domain.PublishState](q.PublishState),
		Status:       optEnum[domain.EventStatus](q.Status),
		VenueID:      optUUID(q.VenueID),
	}
}

type EventResponse struct {
	ID               uuid.UUID           `json:"id"`
	VenueID          uuid.UUID           `json:"venue_id"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	DoorsOpenAt      *time.Time          `json:"doors_open_at"`
	ScheduledStartAt *time.Time          `json:"scheduled_start_at"`
	ExpectedEndAt    *time.Time          `json:"expected_end_at"`
	TicketURL        string              `json:"ticket_url"`
	PublishState     domain.PublishState `json:"publish_state" enums:"draft,published"`
	Status           domain.EventStatus  `json:"status" enums:"scheduled,cancelled,postponed"`
	Version          int                 `json:"version" example:"1"`
	CreatedBy        uuid.UUID           `json:"created_by"`
	UpdatedBy        uuid.UUID           `json:"updated_by"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

type LineupEntryResponse struct {
	ArtistID   uuid.UUID           `json:"artist_id"`
	ArtistName string              `json:"artist_name"`
	ArtistSlug string              `json:"artist_slug"`
	ArtistStat domain.ArtistStatus `json:"artist_status" enums:"active,hidden"`
	Billing    domain.Billing      `json:"billing" enums:"headliner,support"`
	SortOrder  int                 `json:"sort_order"`
}

// EventDetailResponse is returned by single-event endpoints.
type EventDetailResponse struct {
	EventResponse
	Lineup []LineupEntryResponse `json:"lineup"`
}

func ToEvent(e *domain.Event) EventResponse {
	return EventResponse{
		ID: e.ID, VenueID: e.VenueID, Title: e.Title, Description: e.Description,
		DoorsOpenAt: utcPtr(e.DoorsOpenAt), ScheduledStartAt: utcPtr(e.ScheduledStartAt), ExpectedEndAt: utcPtr(e.ExpectedEndAt),
		TicketURL: e.TicketURL, PublishState: e.PublishState, Status: e.Status, Version: e.Version,
		CreatedBy: e.CreatedBy, UpdatedBy: e.UpdatedBy,
		CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(),
	}
}

func ToEventDetail(e *domain.Event) EventDetailResponse {
	lineup := make([]LineupEntryResponse, len(e.Lineup))
	for i, l := range e.Lineup {
		lineup[i] = LineupEntryResponse{ArtistID: l.ArtistID, Billing: l.Billing, SortOrder: l.SortOrder}
		if l.Artist != nil {
			lineup[i].ArtistName = l.Artist.Name
			lineup[i].ArtistSlug = l.Artist.Slug
			lineup[i].ArtistStat = l.Artist.Status
		}
	}
	return EventDetailResponse{EventResponse: ToEvent(e), Lineup: lineup}
}

func ToEventPage(p domain.PageResult[domain.Event]) response.Page[EventResponse] {
	return mapPage(p, ToEvent)
}

// ---- Audit ----

type AuditListQuery struct {
	PageQuery
	EntityType string     `form:"entity_type" binding:"omitempty,oneof=user venue artist event" enums:"user,venue,artist,event"`
	EntityID   string     `form:"entity_id" binding:"omitempty,uuid" format:"uuid"`
	ActorID    string     `form:"actor_id" binding:"omitempty,uuid" format:"uuid"`
	From       *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00" format:"date-time"`
	To         *time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00" format:"date-time"`
}

func (q AuditListQuery) ToFilter() repository.AuditFilter {
	return repository.AuditFilter{
		PageRequest: q.ToDomain(),
		EntityType:  q.EntityType,
		EntityID:    optUUID(q.EntityID),
		ActorID:     optUUID(q.ActorID),
		From:        q.From,
		To:          q.To,
	}
}

type AuditEntryResponse struct {
	ID         uuid.UUID      `json:"id"`
	ActorID    *uuid.UUID     `json:"actor_id"`
	Action     string         `json:"action" example:"event.publish"`
	EntityType string         `json:"entity_type" example:"event"`
	EntityID   uuid.UUID      `json:"entity_id"`
	Before     map[string]any `json:"before"`
	After      map[string]any `json:"after"`
	RequestID  string         `json:"request_id"`
	CreatedAt  time.Time      `json:"created_at"`
}

func ToAuditEntry(e *domain.AuditEntry) AuditEntryResponse {
	return AuditEntryResponse{
		ID: e.ID, ActorID: e.ActorID, Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After, RequestID: e.RequestID, CreatedAt: e.CreatedAt.UTC(),
	}
}

func ToAuditPage(p domain.PageResult[domain.AuditEntry]) response.Page[AuditEntryResponse] {
	return mapPage(p, ToAuditEntry)
}
