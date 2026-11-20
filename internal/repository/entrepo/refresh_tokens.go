package entrepo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/refreshtoken"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const refreshEntity = "Refresh token"

type RefreshTokenRepo struct{ base }

func NewRefreshTokenRepo(c *ent.Client) *RefreshTokenRepo { return &RefreshTokenRepo{base{c}} }

var _ repository.RefreshTokenRepository = (*RefreshTokenRepo)(nil)

func (r *RefreshTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db(ctx).RefreshToken.Create().
		SetUserID(userID).
		SetTokenHash(tokenHash).
		SetExpiresAt(expiresAt.UTC()).
		Save(ctx)
	return mapErr(err, refreshEntity, "")
}

func (r *RefreshTokenRepo) LockByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	row, err := r.db(ctx).RefreshToken.Query().
		Where(refreshtoken.TokenHash(tokenHash)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, mapErr(err, refreshEntity, "")
	}
	return &domain.RefreshToken{
		ID:        row.ID,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db(ctx).RefreshToken.Update().
		Where(refreshtoken.ID(id), refreshtoken.RevokedAtIsNil()).
		SetRevokedAt(at.UTC()).
		Save(ctx)
	return mapErr(err, refreshEntity, "")
}

func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, tokenHash string, at time.Time) error {
	_, err := r.db(ctx).RefreshToken.Update().
		Where(refreshtoken.TokenHash(tokenHash), refreshtoken.RevokedAtIsNil()).
		SetRevokedAt(at.UTC()).
		Save(ctx)
	return mapErr(err, refreshEntity, "")
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := r.db(ctx).RefreshToken.Update().
		Where(refreshtoken.UserID(userID), refreshtoken.RevokedAtIsNil()).
		SetRevokedAt(at.UTC()).
		Save(ctx)
	return mapErr(err, refreshEntity, "")
}
