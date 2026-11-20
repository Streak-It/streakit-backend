// Package repository declares persistence interfaces used by services.
// The Ent-backed implementation lives in repository/entrepo.
//
// Error contract for every method:
//   - missing row          → apperror with code not_found
//   - unique/FK violation  → apperror with code conflict
//   - anything else        → wrapped error (mapped to internal_error)
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/domain"
)

// TxManager runs fn inside a database transaction. Repositories called with
// the ctx passed to fn automatically use that transaction. Nested calls
// reuse the outer transaction. Returning an error rolls back.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// ---- Users ----

type UserFilter struct {
	domain.PageRequest
	Role   *domain.Role
	Status *domain.UserStatus
	// Query matches name or email (case-insensitive substring).
	Query string
}

type UserUpdate struct {
	Name         *string
	Email        *string
	PasswordHash *string
	Role         *domain.Role
	Status       *domain.UserStatus
	LastLoginAt  *time.Time
}

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	// GetByEmail expects an already-normalised email.
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, f UserFilter) (domain.PageResult[domain.User], error)
	Update(ctx context.Context, id uuid.UUID, u UserUpdate) (*domain.User, error)
	// LockActiveSuperAdminIDs returns active super admin IDs, row-locking
	// them (SELECT ... FOR UPDATE) when called inside a transaction.
	LockActiveSuperAdminIDs(ctx context.Context) ([]uuid.UUID, error)
	// LockByID loads a user with a row lock (inside a transaction).
	LockByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// ---- Refresh tokens ----

type RefreshTokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	// LockByHash loads a token row with a row lock (inside a transaction).
	LockByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeByHash(ctx context.Context, tokenHash string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
}

// ---- Venues ----

type VenueFilter struct {
	domain.PageRequest
	City   string
	Name   string
	Status *domain.VenueStatus
}

type VenueUpdate struct {
	Name     *string
	City     *string
	Address  *string
	Info     *string
	Timezone *string
	Status   *domain.VenueStatus
}

type VenueRepository interface {
	Create(ctx context.Context, v *domain.Venue) (*domain.Venue, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Venue, error)
	LockByID(ctx context.Context, id uuid.UUID) (*domain.Venue, error)
	List(ctx context.Context, f VenueFilter) (domain.PageResult[domain.Venue], error)
	// Update applies u only if the stored version equals expectedVersion,
	// incrementing version. A mismatch returns a conflict error.
	Update(ctx context.Context, id uuid.UUID, expectedVersion int, u VenueUpdate) (*domain.Venue, error)
}

// ---- Artists ----

type ArtistFilter struct {
	domain.PageRequest
	Name   string
	Status *domain.ArtistStatus
}

type ArtistUpdate struct {
	Name   *string
	Slug   *string
	Status *domain.ArtistStatus
}

type ArtistRepository interface {
	Create(ctx context.Context, a *domain.Artist) (*domain.Artist, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Artist, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error)
	List(ctx context.Context, f ArtistFilter) (domain.PageResult[domain.Artist], error)
	Update(ctx context.Context, id uuid.UUID, u ArtistUpdate) (*domain.Artist, error)
	// SlugsWithBase returns existing slugs equal to base or starting with base+"-".
	SlugsWithBase(ctx context.Context, base string) ([]string, error)
}

// ---- Events ----

type EventFilter struct {
	domain.PageRequest
	PublishState *domain.PublishState
	Status       *domain.EventStatus
	VenueID      *uuid.UUID
}

// EventUpdate uses double pointers for nullable times:
// nil = unchanged, &nil = clear, &&t = set.
type EventUpdate struct {
	VenueID          *uuid.UUID
	Title            *string
	Description      *string
	DoorsOpenAt      **time.Time
	ScheduledStartAt **time.Time
	ExpectedEndAt    **time.Time
	TicketURL        *string
	PublishState     *domain.PublishState
	Status           *domain.EventStatus
	UpdatedBy        uuid.UUID
}

type LineupInput struct {
	ArtistID  uuid.UUID
	Billing   domain.Billing
	SortOrder int
}

type EventRepository interface {
	Create(ctx context.Context, e *domain.Event) (*domain.Event, error)
	// GetByID returns the event with its lineup (artists populated).
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	// LockByID loads the event (with lineup) holding a row lock.
	LockByID(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	List(ctx context.Context, f EventFilter) (domain.PageResult[domain.Event], error)
	// Update applies u only if the stored version equals expectedVersion,
	// incrementing version. A mismatch returns a conflict error.
	Update(ctx context.Context, id uuid.UUID, expectedVersion int, u EventUpdate) (*domain.Event, error)
	// ReplaceLineup deletes all lineup rows for the event and inserts entries.
	ReplaceLineup(ctx context.Context, eventID uuid.UUID, entries []LineupInput) ([]domain.LineupEntry, error)
}

// ---- Audit ----

type AuditFilter struct {
	domain.PageRequest
	EntityType string
	EntityID   *uuid.UUID
	ActorID    *uuid.UUID
	From       *time.Time
	To         *time.Time
}

type AuditRepository interface {
	Create(ctx context.Context, e *domain.AuditEntry) error
	List(ctx context.Context, f AuditFilter) (domain.PageResult[domain.AuditEntry], error)
}
