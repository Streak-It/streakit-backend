package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

type CreateArtistInput struct {
	Name string
}

type UpdateArtistInput struct {
	Name *string
}

type ArtistService interface {
	Create(ctx context.Context, actor domain.Actor, in CreateArtistInput) (*domain.Artist, error)
	List(ctx context.Context, f repository.ArtistFilter) (domain.PageResult[domain.Artist], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Artist, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateArtistInput) (*domain.Artist, error)
	Hide(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Artist, error)
	Unhide(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Artist, error)
}

type ArtistDeps struct {
	Tx      repository.TxManager
	Artists repository.ArtistRepository
	Audit   repository.AuditRepository
}

type artistService struct {
	ArtistDeps
	audit auditor
}

func NewArtistService(d ArtistDeps) ArtistService {
	return &artistService{ArtistDeps: d, audit: auditor{d.Audit}}
}

var artistSorts = []string{"created_at", "updated_at", "name", "slug"}

// slugRetries bounds retries when a concurrent writer takes the same slug
// between our read and insert (the unique index reports it as a conflict).
const slugRetries = 3

func (s *artistService) Create(ctx context.Context, actor domain.Actor, in CreateArtistInput) (*domain.Artist, error) {
	name, fe := requiredText("name", in.Name)
	if fe != nil {
		return nil, apperror.Validation("Validation failed", *fe)
	}

	var created *domain.Artist
	err := retryOnConflict(func() error {
		return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
			slug, err := s.uniqueSlug(ctx, name)
			if err != nil {
				return err
			}
			created, err = s.Artists.Create(ctx, &domain.Artist{Name: name, Slug: slug, Status: domain.ArtistActive})
			if err != nil {
				return err
			}
			return s.audit.record(ctx, &actor.ID, ActionArtistCreate, domain.EntityArtist, created.ID, nil, artistSnapshot(created))
		})
	})
	return created, err
}

func (s *artistService) List(ctx context.Context, f repository.ArtistFilter) (domain.PageResult[domain.Artist], error) {
	page, err := normalizePage(f.PageRequest, artistSorts, "name")
	if err != nil {
		return domain.PageResult[domain.Artist]{}, err
	}
	f.PageRequest = page
	res, err := s.Artists.List(ctx, f)
	return toPage(res), err
}

func (s *artistService) Get(ctx context.Context, id uuid.UUID) (*domain.Artist, error) {
	return s.Artists.GetByID(ctx, id)
}

func (s *artistService) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateArtistInput) (*domain.Artist, error) {
	if in.Name == nil {
		return s.Artists.GetByID(ctx, id)
	}
	name, fe := requiredText("name", *in.Name)
	if fe != nil {
		return nil, apperror.Validation("Validation failed", *fe)
	}

	var updated *domain.Artist
	err := retryOnConflict(func() error {
		return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
			cur, err := s.Artists.GetByID(ctx, id)
			if err != nil {
				return err
			}
			if cur.Name == name {
				updated = cur
				return nil
			}
			upd := repository.ArtistUpdate{Name: &name}
			// Keep the current slug if it already derives from the new name
			// (e.g. only casing/punctuation changed).
			if !slugHasBase(cur.Slug, slugify(name)) {
				slug, err := s.uniqueSlug(ctx, name)
				if err != nil {
					return err
				}
				upd.Slug = &slug
			}
			if updated, err = s.Artists.Update(ctx, id, upd); err != nil {
				return err
			}
			return s.audit.record(ctx, &actor.ID, ActionArtistUpdate, domain.EntityArtist, id, artistSnapshot(cur), artistSnapshot(updated))
		})
	})
	return updated, err
}

func (s *artistService) Hide(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Artist, error) {
	return s.setStatus(ctx, actor, id, domain.ArtistActive, domain.ArtistHidden, ActionArtistHide)
}

func (s *artistService) Unhide(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Artist, error) {
	return s.setStatus(ctx, actor, id, domain.ArtistHidden, domain.ArtistActive, ActionArtistUnhide)
}

func (s *artistService) setStatus(ctx context.Context, actor domain.Actor, id uuid.UUID, from, to domain.ArtistStatus, action string) (*domain.Artist, error) {
	var updated *domain.Artist
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Artists.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status != from {
			return apperror.Conflict("Artist is " + string(cur.Status) + "; cannot change to " + string(to))
		}
		if updated, err = s.Artists.Update(ctx, id, repository.ArtistUpdate{Status: &to}); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, action, domain.EntityArtist, id, artistSnapshot(cur), artistSnapshot(updated))
	})
	return updated, err
}

func (s *artistService) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slugify(name)
	taken, err := s.Artists.SlugsWithBase(ctx, base)
	if err != nil {
		return "", err
	}
	return nextFreeSlug(base, taken), nil
}

func retryOnConflict(fn func() error) error {
	var err error
	for range slugRetries {
		if err = fn(); !apperror.HasCode(err, apperror.CodeConflict) {
			return err
		}
	}
	return err
}
