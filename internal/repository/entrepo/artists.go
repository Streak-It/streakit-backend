package entrepo

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/artist"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const artistEntity = "Artist"
const slugConflict = "Artist slug is already in use"

type ArtistRepo struct{ base }

func NewArtistRepo(c *ent.Client) *ArtistRepo { return &ArtistRepo{base{c}} }

var _ repository.ArtistRepository = (*ArtistRepo)(nil)

func toArtist(a *ent.Artist) *domain.Artist {
	return &domain.Artist{
		ID:        a.ID,
		Name:      a.Name,
		Slug:      a.Slug,
		Status:    domain.ArtistStatus(a.Status),
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

func (r *ArtistRepo) Create(ctx context.Context, a *domain.Artist) (*domain.Artist, error) {
	row, err := r.db(ctx).Artist.Create().
		SetName(a.Name).
		SetSlug(a.Slug).
		SetStatus(artist.Status(a.Status)).
		Save(ctx)
	if err != nil {
		return nil, mapErr(err, artistEntity, slugConflict)
	}
	return toArtist(row), nil
}

func (r *ArtistRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artist, error) {
	row, err := r.db(ctx).Artist.Get(ctx, id)
	if err != nil {
		return nil, mapErr(err, artistEntity, "")
	}
	return toArtist(row), nil
}

func (r *ArtistRepo) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db(ctx).Artist.Query().Where(artist.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, mapErr(err, artistEntity, "")
	}
	out := make([]domain.Artist, len(rows))
	for i, row := range rows {
		out[i] = *toArtist(row)
	}
	return out, nil
}

func (r *ArtistRepo) List(ctx context.Context, f repository.ArtistFilter) (domain.PageResult[domain.Artist], error) {
	q := r.db(ctx).Artist.Query()
	if f.Name != "" {
		q = q.Where(artist.NameContainsFold(f.Name))
	}
	if f.Status != nil {
		q = q.Where(artist.StatusEQ(artist.Status(*f.Status)))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return domain.PageResult[domain.Artist]{}, mapErr(err, artistEntity, "")
	}

	field, desc := f.SortField()
	rows, err := q.Order(order[artist.OrderOption](field, desc)...).
		Offset(f.Offset()).Limit(f.PageSize).All(ctx)
	if err != nil {
		return domain.PageResult[domain.Artist]{}, mapErr(err, artistEntity, "")
	}

	items := make([]domain.Artist, len(rows))
	for i, row := range rows {
		items[i] = *toArtist(row)
	}
	return domain.PageResult[domain.Artist]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *ArtistRepo) Update(ctx context.Context, id uuid.UUID, u repository.ArtistUpdate) (*domain.Artist, error) {
	m := r.db(ctx).Artist.UpdateOneID(id)
	if u.Name != nil {
		m.SetName(*u.Name)
	}
	if u.Slug != nil {
		m.SetSlug(*u.Slug)
	}
	if u.Status != nil {
		m.SetStatus(artist.Status(*u.Status))
	}
	row, err := m.Save(ctx)
	if err != nil {
		return nil, mapErr(err, artistEntity, slugConflict)
	}
	return toArtist(row), nil
}

func (r *ArtistRepo) SlugsWithBase(ctx context.Context, base string) ([]string, error) {
	slugs, err := r.db(ctx).Artist.Query().
		Where(artist.Or(artist.Slug(base), artist.SlugHasPrefix(base+"-"))).
		Select(artist.FieldSlug).
		Strings(ctx)
	if err != nil {
		return nil, mapErr(err, artistEntity, "")
	}
	return slugs, nil
}
