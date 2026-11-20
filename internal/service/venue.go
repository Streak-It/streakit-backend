package service

import (
	"context"
	"strings"
	"time"
	_ "time/tzdata" // embed IANA zones so validation works in minimal images

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const DefaultVenueTimezone = "Asia/Kolkata"

type CreateVenueInput struct {
	Name     string
	City     string
	Address  string
	Info     string
	Timezone string
}

type UpdateVenueInput struct {
	Version  int
	Name     *string
	City     *string
	Address  *string
	Info     *string
	Timezone *string
}

type VenueService interface {
	Create(ctx context.Context, actor domain.Actor, in CreateVenueInput) (*domain.Venue, error)
	List(ctx context.Context, f repository.VenueFilter) (domain.PageResult[domain.Venue], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Venue, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateVenueInput) (*domain.Venue, error)
	Retire(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Venue, error)
	Restore(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Venue, error)
}

type VenueDeps struct {
	Tx     repository.TxManager
	Venues repository.VenueRepository
	Audit  repository.AuditRepository
}

type venueService struct {
	VenueDeps
	audit auditor
}

func NewVenueService(d VenueDeps) VenueService {
	return &venueService{VenueDeps: d, audit: auditor{d.Audit}}
}

var venueSorts = []string{"created_at", "updated_at", "name", "city"}

func validateTimezone(tz string) *apperror.FieldError {
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return &apperror.FieldError{Field: "timezone", Message: "must be a valid IANA timezone, e.g. Asia/Kolkata"}
	}
	return nil
}

func (s *venueService) Create(ctx context.Context, actor domain.Actor, in CreateVenueInput) (*domain.Venue, error) {
	var errs fieldErrors
	name, fe := requiredText("name", in.Name)
	errs.add(fe)
	city, fe := requiredText("city", in.City)
	errs.add(fe)
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = DefaultVenueTimezone
	}
	errs.add(validateTimezone(tz))
	if err := errs.err(); err != nil {
		return nil, err
	}

	var created *domain.Venue
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.Venues.Create(ctx, &domain.Venue{
			Name:     name,
			City:     city,
			Address:  strings.TrimSpace(in.Address),
			Info:     strings.TrimSpace(in.Info),
			Timezone: tz,
			Status:   domain.VenueActive,
		})
		if err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionVenueCreate, domain.EntityVenue, created.ID, nil, venueSnapshot(created))
	})
	return created, err
}

func (s *venueService) List(ctx context.Context, f repository.VenueFilter) (domain.PageResult[domain.Venue], error) {
	page, err := normalizePage(f.PageRequest, venueSorts, "name")
	if err != nil {
		return domain.PageResult[domain.Venue]{}, err
	}
	f.PageRequest = page
	res, err := s.Venues.List(ctx, f)
	return toPage(res), err
}

func (s *venueService) Get(ctx context.Context, id uuid.UUID) (*domain.Venue, error) {
	return s.Venues.GetByID(ctx, id)
}

func (s *venueService) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateVenueInput) (*domain.Venue, error) {
	var (
		errs fieldErrors
		upd  repository.VenueUpdate
	)
	if in.Version < 1 {
		errs.addf("version", "required")
	}
	if in.Name != nil {
		v, fe := requiredText("name", *in.Name)
		errs.add(fe)
		upd.Name = &v
	}
	if in.City != nil {
		v, fe := requiredText("city", *in.City)
		errs.add(fe)
		upd.City = &v
	}
	if in.Address != nil {
		upd.Address = ptr(strings.TrimSpace(*in.Address))
	}
	if in.Info != nil {
		upd.Info = ptr(strings.TrimSpace(*in.Info))
	}
	if in.Timezone != nil {
		v := strings.TrimSpace(*in.Timezone)
		errs.add(validateTimezone(v))
		upd.Timezone = &v
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	var updated *domain.Venue
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Venues.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if cur.Version != in.Version {
			return versionConflict("Venue")
		}
		if updated, err = s.Venues.Update(ctx, id, in.Version, upd); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionVenueUpdate, domain.EntityVenue, id, venueSnapshot(cur), venueSnapshot(updated))
	})
	return updated, err
}

func (s *venueService) Retire(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Venue, error) {
	return s.setStatus(ctx, actor, id, domain.VenueActive, domain.VenueRetired, ActionVenueRetire)
}

func (s *venueService) Restore(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Venue, error) {
	return s.setStatus(ctx, actor, id, domain.VenueRetired, domain.VenueActive, ActionVenueRestore)
}

func (s *venueService) setStatus(ctx context.Context, actor domain.Actor, id uuid.UUID, from, to domain.VenueStatus, action string) (*domain.Venue, error) {
	var updated *domain.Venue
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Venues.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status != from {
			return apperror.Conflict("Venue is " + string(cur.Status) + "; cannot change to " + string(to))
		}
		if updated, err = s.Venues.Update(ctx, id, cur.Version, repository.VenueUpdate{Status: &to}); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, action, domain.EntityVenue, id, venueSnapshot(cur), venueSnapshot(updated))
	})
	return updated, err
}

func versionConflict(entity string) error {
	return apperror.Conflict(entity + " was modified by someone else; reload and retry")
}
