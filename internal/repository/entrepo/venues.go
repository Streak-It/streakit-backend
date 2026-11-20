package entrepo

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/venue"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const venueEntity = "Venue"

type VenueRepo struct{ base }

func NewVenueRepo(c *ent.Client) *VenueRepo { return &VenueRepo{base{c}} }

var _ repository.VenueRepository = (*VenueRepo)(nil)

func toVenue(v *ent.Venue) *domain.Venue {
	return &domain.Venue{
		ID:        v.ID,
		Name:      v.Name,
		City:      v.City,
		Address:   v.Address,
		Info:      v.Info,
		Timezone:  v.Timezone,
		Status:    domain.VenueStatus(v.Status),
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

func (r *VenueRepo) Create(ctx context.Context, v *domain.Venue) (*domain.Venue, error) {
	row, err := r.db(ctx).Venue.Create().
		SetName(v.Name).
		SetCity(v.City).
		SetAddress(v.Address).
		SetInfo(v.Info).
		SetTimezone(v.Timezone).
		SetStatus(venue.Status(v.Status)).
		Save(ctx)
	if err != nil {
		return nil, mapErr(err, venueEntity, "")
	}
	return toVenue(row), nil
}

func (r *VenueRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Venue, error) {
	row, err := r.db(ctx).Venue.Get(ctx, id)
	if err != nil {
		return nil, mapErr(err, venueEntity, "")
	}
	return toVenue(row), nil
}

func (r *VenueRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.Venue, error) {
	row, err := r.db(ctx).Venue.Query().Where(venue.ID(id)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, mapErr(err, venueEntity, "")
	}
	return toVenue(row), nil
}

func (r *VenueRepo) List(ctx context.Context, f repository.VenueFilter) (domain.PageResult[domain.Venue], error) {
	q := r.db(ctx).Venue.Query()
	if f.City != "" {
		q = q.Where(venue.CityEqualFold(f.City))
	}
	if f.Name != "" {
		q = q.Where(venue.NameContainsFold(f.Name))
	}
	if f.Status != nil {
		q = q.Where(venue.StatusEQ(venue.Status(*f.Status)))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return domain.PageResult[domain.Venue]{}, mapErr(err, venueEntity, "")
	}

	field, desc := f.SortField()
	rows, err := q.Order(order[venue.OrderOption](field, desc)...).
		Offset(f.Offset()).Limit(f.PageSize).All(ctx)
	if err != nil {
		return domain.PageResult[domain.Venue]{}, mapErr(err, venueEntity, "")
	}

	items := make([]domain.Venue, len(rows))
	for i, row := range rows {
		items[i] = *toVenue(row)
	}
	return domain.PageResult[domain.Venue]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *VenueRepo) Update(ctx context.Context, id uuid.UUID, expectedVersion int, u repository.VenueUpdate) (*domain.Venue, error) {
	m := r.db(ctx).Venue.UpdateOneID(id).
		Where(venue.Version(expectedVersion)).
		AddVersion(1)
	if u.Name != nil {
		m.SetName(*u.Name)
	}
	if u.City != nil {
		m.SetCity(*u.City)
	}
	if u.Address != nil {
		m.SetAddress(*u.Address)
	}
	if u.Info != nil {
		m.SetInfo(*u.Info)
	}
	if u.Timezone != nil {
		m.SetTimezone(*u.Timezone)
	}
	if u.Status != nil {
		m.SetStatus(venue.Status(*u.Status))
	}

	row, err := m.Save(ctx)
	if ent.IsNotFound(err) {
		// Either the row is gone or the version predicate failed.
		if exists, exErr := r.db(ctx).Venue.Query().Where(venue.ID(id)).Exist(ctx); exErr == nil && exists {
			return nil, apperror.Conflict("Venue was modified by someone else; reload and retry")
		}
	}
	if err != nil {
		return nil, mapErr(err, venueEntity, "")
	}
	return toVenue(row), nil
}
