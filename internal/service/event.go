package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/patch"
	"github.com/niteshs1/streakit-api/internal/repository"
)

type CreateEventInput struct {
	VenueID          uuid.UUID
	Title            string
	Description      string
	DoorsOpenAt      *time.Time
	ScheduledStartAt *time.Time
	ExpectedEndAt    *time.Time
	TicketURL        string
}

// UpdateEventInput: nil pointers / unset patch fields are left unchanged.
// Time fields use patch.Field so clients can clear them with null.
type UpdateEventInput struct {
	Version          int
	VenueID          *uuid.UUID
	Title            *string
	Description      *string
	TicketURL        *string
	DoorsOpenAt      patch.Field[time.Time]
	ScheduledStartAt patch.Field[time.Time]
	ExpectedEndAt    patch.Field[time.Time]
}

type PostponeInput struct {
	DoorsOpenAt      *time.Time
	ScheduledStartAt time.Time
	ExpectedEndAt    *time.Time
}

type EventService interface {
	Create(ctx context.Context, actor domain.Actor, in CreateEventInput) (*domain.Event, error)
	List(ctx context.Context, f repository.EventFilter) (domain.PageResult[domain.Event], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateEventInput) (*domain.Event, error)
	ReplaceLineup(ctx context.Context, actor domain.Actor, id uuid.UUID, entries []repository.LineupInput) (*domain.Event, error)
	Publish(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error)
	Unpublish(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error)
	Cancel(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error)
	Postpone(ctx context.Context, actor domain.Actor, id uuid.UUID, in PostponeInput) (*domain.Event, error)
	Restore(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error)
}

type EventDeps struct {
	Tx      repository.TxManager
	Events  repository.EventRepository
	Venues  repository.VenueRepository
	Artists repository.ArtistRepository
	Audit   repository.AuditRepository
}

type eventService struct {
	EventDeps
	audit auditor
}

func NewEventService(d EventDeps) EventService {
	return &eventService{EventDeps: d, audit: auditor{d.Audit}}
}

var eventSorts = []string{"created_at", "updated_at", "scheduled_start_at", "title"}

const MaxLineupEntries = 100

// ---- Rules ----

// validateTimes enforces doors ≤ start ≤ end, ignoring unset values.
func validateTimes(doors, start, end *time.Time) fieldErrors {
	var errs fieldErrors
	if doors != nil && start != nil && doors.After(*start) {
		errs.addf("doors_open_at", "must be at or before scheduled_start_at")
	}
	if start != nil && end != nil && start.After(*end) {
		errs.addf("expected_end_at", "must be at or after scheduled_start_at")
	}
	if doors != nil && end != nil && doors.After(*end) {
		errs.addf("expected_end_at", "must be at or after doors_open_at")
	}
	return errs
}

// validateTicketURL: optional; if set must be https with a host and no userinfo.
func validateTicketURL(raw string) *apperror.FieldError {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return &apperror.FieldError{Field: "ticket_url", Message: "must be an https URL without credentials"}
	}
	return nil
}

// requireActiveVenue loads a venue that is being newly attached.
func (s *eventService) requireActiveVenue(ctx context.Context, id uuid.UUID) *apperror.FieldError {
	v, err := s.Venues.GetByID(ctx, id)
	if err != nil {
		return &apperror.FieldError{Field: "venue_id", Message: "venue not found"}
	}
	if v.Status != domain.VenueActive {
		return &apperror.FieldError{Field: "venue_id", Message: "venue is retired"}
	}
	return nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// ---- CRUD ----

func (s *eventService) Create(ctx context.Context, actor domain.Actor, in CreateEventInput) (*domain.Event, error) {
	in.TicketURL = strings.TrimSpace(in.TicketURL)
	errs := validateTimes(in.DoorsOpenAt, in.ScheduledStartAt, in.ExpectedEndAt)
	errs.add(validateTicketURL(in.TicketURL))
	if in.VenueID == uuid.Nil {
		errs.addf("venue_id", "required")
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	var created *domain.Event
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if fe := s.requireActiveVenue(ctx, in.VenueID); fe != nil {
			return apperror.Validation("Validation failed", *fe)
		}
		var err error
		created, err = s.Events.Create(ctx, &domain.Event{
			VenueID:          in.VenueID,
			Title:            strings.TrimSpace(in.Title),
			Description:      strings.TrimSpace(in.Description),
			DoorsOpenAt:      utc(in.DoorsOpenAt),
			ScheduledStartAt: utc(in.ScheduledStartAt),
			ExpectedEndAt:    utc(in.ExpectedEndAt),
			TicketURL:        in.TicketURL,
			PublishState:     domain.Draft,
			Status:           domain.EventScheduled,
			CreatedBy:        actor.ID,
			UpdatedBy:        actor.ID,
		})
		if err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionEventCreate, domain.EntityEvent, created.ID, nil, eventSnapshot(created))
	})
	return created, err
}

func (s *eventService) List(ctx context.Context, f repository.EventFilter) (domain.PageResult[domain.Event], error) {
	page, err := normalizePage(f.PageRequest, eventSorts, "-created_at")
	if err != nil {
		return domain.PageResult[domain.Event]{}, err
	}
	f.PageRequest = page
	res, err := s.Events.List(ctx, f)
	return toPage(res), err
}

func (s *eventService) Get(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return s.Events.GetByID(ctx, id)
}

func (s *eventService) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateEventInput) (*domain.Event, error) {
	if in.Version < 1 {
		return nil, apperror.Field("version", "required")
	}

	var updated *domain.Event
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Events.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if cur.Version != in.Version {
			return versionConflict("Event")
		}

		upd := repository.EventUpdate{UpdatedBy: actor.ID}
		var errs fieldErrors

		// Merge so cross-field rules see the resulting state.
		doors, start, end := cur.DoorsOpenAt, cur.ScheduledStartAt, cur.ExpectedEndAt
		if in.DoorsOpenAt.Set {
			doors = utc(in.DoorsOpenAt.Ptr())
			upd.DoorsOpenAt = &doors
		}
		if in.ScheduledStartAt.Set {
			start = utc(in.ScheduledStartAt.Ptr())
			upd.ScheduledStartAt = &start
		}
		if in.ExpectedEndAt.Set {
			end = utc(in.ExpectedEndAt.Ptr())
			upd.ExpectedEndAt = &end
		}
		errs = append(errs, validateTimes(doors, start, end)...)
		if cur.PublishState == domain.Published && start == nil {
			errs.addf("scheduled_start_at", "required while the event is published")
		}

		if in.VenueID != nil && *in.VenueID != cur.VenueID {
			errs.add(s.requireActiveVenue(ctx, *in.VenueID))
			upd.VenueID = in.VenueID
		}
		if in.Title != nil {
			upd.Title = ptr(strings.TrimSpace(*in.Title))
		}
		if in.Description != nil {
			upd.Description = ptr(strings.TrimSpace(*in.Description))
		}
		if in.TicketURL != nil {
			v := strings.TrimSpace(*in.TicketURL)
			errs.add(validateTicketURL(v))
			upd.TicketURL = &v
		}
		if err := errs.err(); err != nil {
			return err
		}

		if updated, err = s.Events.Update(ctx, id, cur.Version, upd); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionEventUpdate, domain.EntityEvent, id, eventSnapshot(cur), eventSnapshot(updated))
	})
	return updated, err
}

func (s *eventService) ReplaceLineup(ctx context.Context, actor domain.Actor, id uuid.UUID, entries []repository.LineupInput) (*domain.Event, error) {
	var errs fieldErrors
	if len(entries) > MaxLineupEntries {
		errs.addf("lineup", "must have at most %d entries", MaxLineupEntries)
	}
	seen := make(map[uuid.UUID]bool, len(entries))
	ids := make([]uuid.UUID, 0, len(entries))
	for i, e := range entries {
		field := fmt.Sprintf("lineup[%d]", i)
		if e.ArtistID == uuid.Nil {
			errs.addf(field+".artist_id", "required")
			continue
		}
		if seen[e.ArtistID] {
			errs.addf(field+".artist_id", "duplicate artist in lineup")
		}
		seen[e.ArtistID] = true
		ids = append(ids, e.ArtistID)
		if !e.Billing.Valid() {
			errs.addf(field+".billing", "must be one of: headliner, support")
		}
		if e.SortOrder < 0 {
			errs.addf(field+".sort_order", "must be >= 0")
		}
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	var result *domain.Event
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Events.LockByID(ctx, id)
		if err != nil {
			return err
		}

		artists, err := s.Artists.GetByIDs(ctx, ids)
		if err != nil {
			return err
		}
		byID := make(map[uuid.UUID]domain.Artist, len(artists))
		for _, a := range artists {
			byID[a.ID] = a
		}
		existing := make(map[uuid.UUID]bool, len(cur.Lineup))
		for _, l := range cur.Lineup {
			existing[l.ArtistID] = true
		}

		var errs fieldErrors
		newLineup := make([]domain.LineupEntry, len(entries))
		for i, e := range entries {
			newLineup[i] = domain.LineupEntry{ArtistID: e.ArtistID, Billing: e.Billing, SortOrder: e.SortOrder}
			a, ok := byID[e.ArtistID]
			switch {
			case !ok:
				errs.addf(fmt.Sprintf("lineup[%d].artist_id", i), "artist not found")
			// Hidden artists may stay on a lineup but cannot be newly added.
			case a.Status != domain.ArtistActive && !existing[e.ArtistID]:
				errs.addf(fmt.Sprintf("lineup[%d].artist_id", i), "artist is hidden")
			}
		}
		if cur.PublishState == domain.Published && !domain.HasHeadliner(newLineup) {
			errs.addf("lineup", "a published event needs at least one headliner")
		}
		if err := errs.err(); err != nil {
			return err
		}

		saved, err := s.Events.ReplaceLineup(ctx, id, entries)
		if err != nil {
			return err
		}
		// Bump version so concurrent editors notice the lineup changed.
		if result, err = s.Events.Update(ctx, id, cur.Version, repository.EventUpdate{UpdatedBy: actor.ID}); err != nil {
			return err
		}
		result.Lineup = saved
		return s.audit.record(ctx, &actor.ID, ActionEventLineup, domain.EntityEvent, id, lineupSnapshot(cur.Lineup), lineupSnapshot(saved))
	})
	return result, err
}

// ---- State machine ----

// transition loads and locks the event, checks the from-state, validates,
// applies the change and writes the audit row — all in one transaction.
func (s *eventService) transition(
	ctx context.Context,
	actor domain.Actor,
	id uuid.UUID,
	action string,
	allowed func(e *domain.Event) error,
	apply func(ctx context.Context, e *domain.Event, upd *repository.EventUpdate) error,
) (*domain.Event, error) {
	var updated *domain.Event
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Events.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if err := allowed(cur); err != nil {
			return err
		}
		upd := repository.EventUpdate{UpdatedBy: actor.ID}
		if err := apply(ctx, cur, &upd); err != nil {
			return err
		}
		if updated, err = s.Events.Update(ctx, id, cur.Version, upd); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, action, domain.EntityEvent, id, eventSnapshot(cur), eventSnapshot(updated))
	})
	return updated, err
}

func requireState(ps domain.PublishState, st domain.EventStatus, verb string) func(*domain.Event) error {
	return func(e *domain.Event) error {
		if e.PublishState != ps || e.Status != st {
			return apperror.Conflict(fmt.Sprintf("Cannot %s an event that is %s and %s", verb, e.PublishState, e.Status))
		}
		return nil
	}
}

func (s *eventService) Publish(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error) {
	return s.transition(ctx, actor, id, ActionEventPublish,
		requireState(domain.Draft, domain.EventScheduled, "publish"),
		func(ctx context.Context, e *domain.Event, upd *repository.EventUpdate) error {
			if err := s.checkPublishable(ctx, e); err != nil {
				return err
			}
			upd.PublishState = ptr(domain.Published)
			return nil
		})
}

// checkPublishable: active venue, start time set, valid time order, at
// least one headliner, and no hidden artists in the lineup.
func (s *eventService) checkPublishable(ctx context.Context, e *domain.Event) error {
	var errs fieldErrors
	venue, err := s.Venues.GetByID(ctx, e.VenueID)
	if err != nil {
		return err
	}
	if venue.Status != domain.VenueActive {
		errs.addf("venue_id", "venue is retired")
	}
	if e.ScheduledStartAt == nil {
		errs.addf("scheduled_start_at", "required to publish")
	}
	errs = append(errs, validateTimes(e.DoorsOpenAt, e.ScheduledStartAt, e.ExpectedEndAt)...)
	if !domain.HasHeadliner(e.Lineup) {
		errs.addf("lineup", "at least one headliner is required to publish")
	}
	for i, l := range e.Lineup {
		if l.Artist != nil && l.Artist.Status != domain.ArtistActive {
			errs.addf(fmt.Sprintf("lineup[%d].artist_id", i), "artist is hidden")
		}
	}
	if len(errs) > 0 {
		return apperror.Validation("Event cannot be published", errs...)
	}
	return nil
}

func (s *eventService) Unpublish(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error) {
	return s.transition(ctx, actor, id, ActionEventUnpublish,
		requireState(domain.Published, domain.EventScheduled, "unpublish"),
		func(_ context.Context, _ *domain.Event, upd *repository.EventUpdate) error {
			upd.PublishState = ptr(domain.Draft)
			return nil
		})
}

func (s *eventService) Cancel(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error) {
	return s.transition(ctx, actor, id, ActionEventCancel,
		requireState(domain.Published, domain.EventScheduled, "cancel"),
		func(_ context.Context, _ *domain.Event, upd *repository.EventUpdate) error {
			upd.Status = ptr(domain.EventCancelled)
			return nil
		})
}

func (s *eventService) Postpone(ctx context.Context, actor domain.Actor, id uuid.UUID, in PostponeInput) (*domain.Event, error) {
	if in.ScheduledStartAt.IsZero() {
		return nil, apperror.Field("scheduled_start_at", "required")
	}
	start := in.ScheduledStartAt.UTC()
	doors, end := utc(in.DoorsOpenAt), utc(in.ExpectedEndAt)
	if err := validateTimes(doors, &start, end).err(); err != nil {
		return nil, err
	}

	return s.transition(ctx, actor, id, ActionEventPostpone,
		requireState(domain.Published, domain.EventScheduled, "postpone"),
		func(_ context.Context, _ *domain.Event, upd *repository.EventUpdate) error {
			startPtr := &start
			upd.ScheduledStartAt = &startPtr
			upd.DoorsOpenAt = &doors
			upd.ExpectedEndAt = &end
			// This slice always supplies new times, so status stays scheduled.
			upd.Status = ptr(domain.EventScheduled)
			return nil
		})
}

func (s *eventService) Restore(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Event, error) {
	return s.transition(ctx, actor, id, ActionEventRestore,
		requireState(domain.Published, domain.EventCancelled, "restore"),
		func(_ context.Context, _ *domain.Event, upd *repository.EventUpdate) error {
			upd.Status = ptr(domain.EventScheduled)
			return nil
		})
}
