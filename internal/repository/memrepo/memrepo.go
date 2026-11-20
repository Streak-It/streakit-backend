// Package memrepo provides in-memory repository implementations for unit
// and handler tests. It mirrors the error contract of entrepo (not_found,
// conflict on unique keys, conflict on version mismatch) but does not
// implement real transactions: WithinTx just runs fn.
//
// Not for production use.
package memrepo

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

// Store holds all tables behind one mutex.
type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	users   map[uuid.UUID]*domain.User
	tokens  map[string]*storedToken // by hash
	venues  map[uuid.UUID]*domain.Venue
	artists map[uuid.UUID]*domain.Artist
	events  map[uuid.UUID]*domain.Event
	lineups map[uuid.UUID][]domain.LineupEntry // by event id
	audit   []domain.AuditEntry
	// FailAudit makes AuditRepo.Create fail (to test error paths).
	FailAudit error
}

type storedToken struct {
	domain.RefreshToken
	hash string
}

func NewStore() *Store {
	return &Store{
		now:     func() time.Time { return time.Now().UTC() },
		users:   map[uuid.UUID]*domain.User{},
		tokens:  map[string]*storedToken{},
		venues:  map[uuid.UUID]*domain.Venue{},
		artists: map[uuid.UUID]*domain.Artist{},
		events:  map[uuid.UUID]*domain.Event{},
		lineups: map[uuid.UUID][]domain.LineupEntry{},
	}
}

// Repos bundles every repository backed by one store.
type Repos struct {
	Store         *Store
	Tx            repository.TxManager
	Users         *UserRepo
	RefreshTokens *RefreshTokenRepo
	Venues        *VenueRepo
	Artists       *ArtistRepo
	Events        *EventRepo
	Audit         *AuditRepo
}

func New() *Repos {
	s := NewStore()
	return &Repos{
		Store:         s,
		Tx:            TxManager{},
		Users:         &UserRepo{s},
		RefreshTokens: &RefreshTokenRepo{s},
		Venues:        &VenueRepo{s},
		Artists:       &ArtistRepo{s},
		Events:        &EventRepo{s},
		Audit:         &AuditRepo{s},
	}
}

// AuditEntries returns a copy of all audit rows (oldest first).
func (s *Store) AuditEntries() []domain.AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.audit)
}

// ActiveTokenCount counts unrevoked refresh tokens for a user.
func (s *Store) ActiveTokenCount(userID uuid.UUID) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, t := range s.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			n++
		}
	}
	return n
}

type TxManager struct{}

func (TxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func page[T any](items []T, p domain.PageRequest) domain.PageResult[T] {
	total := len(items)
	start := min(p.Offset(), total)
	end := min(start+p.PageSize, total)
	return domain.PageResult[T]{Items: slices.Clone(items[start:end]), Page: p.Page, PageSize: p.PageSize, Total: total}
}

func sortByCreated[T any](items []T, created func(T) time.Time, desc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		if desc {
			return created(items[i]).After(created(items[j]))
		}
		return created(items[i]).Before(created(items[j]))
	})
}

// ---- Users ----

type UserRepo struct{ s *Store }

func (r *UserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, x := range r.s.users {
		if x.Email == u.Email {
			return nil, apperror.Conflict("Email is already in use")
		}
	}
	c := *u
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), r.s.now(), r.s.now()
	r.s.users[c.ID] = &c
	out := c
	return &out, nil
}

func (r *UserRepo) get(id uuid.UUID) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return nil, apperror.NotFound("User")
	}
	c := *u
	return &c, nil
}

func (r *UserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error)  { return r.get(id) }
func (r *UserRepo) LockByID(_ context.Context, id uuid.UUID) (*domain.User, error) { return r.get(id) }

func (r *UserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.Email == email {
			c := *u
			return &c, nil
		}
	}
	return nil, apperror.NotFound("User")
}

func (r *UserRepo) List(_ context.Context, f repository.UserFilter) (domain.PageResult[domain.User], error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var items []domain.User
	for _, u := range r.s.users {
		if (f.Role == nil || u.Role == *f.Role) && (f.Status == nil || u.Status == *f.Status) &&
			(f.Query == "" || strings.Contains(strings.ToLower(u.Name+" "+u.Email), strings.ToLower(f.Query))) {
			items = append(items, *u)
		}
	}
	_, desc := f.SortField()
	sortByCreated(items, func(u domain.User) time.Time { return u.CreatedAt }, desc)
	return page(items, f.PageRequest), nil
}

func (r *UserRepo) Update(_ context.Context, id uuid.UUID, u repository.UserUpdate) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.users[id]
	if !ok {
		return nil, apperror.NotFound("User")
	}
	if u.Email != nil {
		for _, x := range r.s.users {
			if x.ID != id && x.Email == *u.Email {
				return nil, apperror.Conflict("Email is already in use")
			}
		}
		cur.Email = *u.Email
	}
	if u.Name != nil {
		cur.Name = *u.Name
	}
	if u.PasswordHash != nil {
		cur.PasswordHash = *u.PasswordHash
	}
	if u.Role != nil {
		cur.Role = *u.Role
	}
	if u.Status != nil {
		cur.Status = *u.Status
	}
	if u.LastLoginAt != nil {
		t := *u.LastLoginAt
		cur.LastLoginAt = &t
	}
	cur.UpdatedAt = r.s.now()
	c := *cur
	return &c, nil
}

func (r *UserRepo) LockActiveSuperAdminIDs(_ context.Context) ([]uuid.UUID, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var ids []uuid.UUID
	for _, u := range r.s.users {
		if u.Role == domain.RoleSuperAdmin && u.Status == domain.UserActive {
			ids = append(ids, u.ID)
		}
	}
	return ids, nil
}

// ---- Refresh tokens ----

type RefreshTokenRepo struct{ s *Store }

func (r *RefreshTokenRepo) Create(_ context.Context, userID uuid.UUID, hash string, exp time.Time) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.tokens[hash]; ok {
		return apperror.Conflict("Refresh token conflicts with an existing record")
	}
	r.s.tokens[hash] = &storedToken{
		RefreshToken: domain.RefreshToken{ID: uuid.New(), UserID: userID, ExpiresAt: exp, CreatedAt: r.s.now()},
		hash:         hash,
	}
	return nil
}

func (r *RefreshTokenRepo) LockByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.tokens[hash]
	if !ok {
		return nil, apperror.NotFound("Refresh token")
	}
	c := t.RefreshToken
	return &c, nil
}

func (r *RefreshTokenRepo) revokeWhere(at time.Time, match func(*storedToken) bool) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.tokens {
		if t.RevokedAt == nil && match(t) {
			a := at
			t.RevokedAt = &a
		}
	}
}

func (r *RefreshTokenRepo) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	r.revokeWhere(at, func(t *storedToken) bool { return t.ID == id })
	return nil
}

func (r *RefreshTokenRepo) RevokeByHash(_ context.Context, hash string, at time.Time) error {
	r.revokeWhere(at, func(t *storedToken) bool { return t.hash == hash })
	return nil
}

func (r *RefreshTokenRepo) RevokeAllForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	r.revokeWhere(at, func(t *storedToken) bool { return t.UserID == userID })
	return nil
}

// ---- Venues ----

type VenueRepo struct{ s *Store }

func (r *VenueRepo) Create(_ context.Context, v *domain.Venue) (*domain.Venue, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c := *v
	c.ID, c.Version, c.CreatedAt, c.UpdatedAt = uuid.New(), 1, r.s.now(), r.s.now()
	r.s.venues[c.ID] = &c
	out := c
	return &out, nil
}

func (r *VenueRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Venue, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	v, ok := r.s.venues[id]
	if !ok {
		return nil, apperror.NotFound("Venue")
	}
	c := *v
	return &c, nil
}

func (r *VenueRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.Venue, error) {
	return r.GetByID(ctx, id)
}

func (r *VenueRepo) List(_ context.Context, f repository.VenueFilter) (domain.PageResult[domain.Venue], error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var items []domain.Venue
	for _, v := range r.s.venues {
		if (f.City == "" || strings.EqualFold(v.City, f.City)) &&
			(f.Name == "" || strings.Contains(strings.ToLower(v.Name), strings.ToLower(f.Name))) &&
			(f.Status == nil || v.Status == *f.Status) {
			items = append(items, *v)
		}
	}
	_, desc := f.SortField()
	sortByCreated(items, func(v domain.Venue) time.Time { return v.CreatedAt }, desc)
	return page(items, f.PageRequest), nil
}

func (r *VenueRepo) Update(_ context.Context, id uuid.UUID, version int, u repository.VenueUpdate) (*domain.Venue, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	v, ok := r.s.venues[id]
	if !ok {
		return nil, apperror.NotFound("Venue")
	}
	if v.Version != version {
		return nil, apperror.Conflict("Venue was modified by someone else; reload and retry")
	}
	set(&v.Name, u.Name)
	set(&v.City, u.City)
	set(&v.Address, u.Address)
	set(&v.Info, u.Info)
	set(&v.Timezone, u.Timezone)
	set(&v.Status, u.Status)
	v.Version++
	v.UpdatedAt = r.s.now()
	c := *v
	return &c, nil
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// ---- Artists ----

type ArtistRepo struct{ s *Store }

func (r *ArtistRepo) Create(_ context.Context, a *domain.Artist) (*domain.Artist, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, x := range r.s.artists {
		if x.Slug == a.Slug {
			return nil, apperror.Conflict("Artist slug is already in use")
		}
	}
	c := *a
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), r.s.now(), r.s.now()
	r.s.artists[c.ID] = &c
	out := c
	return &out, nil
}

func (r *ArtistRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Artist, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	a, ok := r.s.artists[id]
	if !ok {
		return nil, apperror.NotFound("Artist")
	}
	c := *a
	return &c, nil
}

func (r *ArtistRepo) GetByIDs(_ context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Artist
	for _, id := range ids {
		if a, ok := r.s.artists[id]; ok {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (r *ArtistRepo) List(_ context.Context, f repository.ArtistFilter) (domain.PageResult[domain.Artist], error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var items []domain.Artist
	for _, a := range r.s.artists {
		if (f.Name == "" || strings.Contains(strings.ToLower(a.Name), strings.ToLower(f.Name))) &&
			(f.Status == nil || a.Status == *f.Status) {
			items = append(items, *a)
		}
	}
	_, desc := f.SortField()
	sortByCreated(items, func(a domain.Artist) time.Time { return a.CreatedAt }, desc)
	return page(items, f.PageRequest), nil
}

func (r *ArtistRepo) Update(_ context.Context, id uuid.UUID, u repository.ArtistUpdate) (*domain.Artist, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	a, ok := r.s.artists[id]
	if !ok {
		return nil, apperror.NotFound("Artist")
	}
	if u.Slug != nil {
		for _, x := range r.s.artists {
			if x.ID != id && x.Slug == *u.Slug {
				return nil, apperror.Conflict("Artist slug is already in use")
			}
		}
	}
	set(&a.Name, u.Name)
	set(&a.Slug, u.Slug)
	set(&a.Status, u.Status)
	a.UpdatedAt = r.s.now()
	c := *a
	return &c, nil
}

func (r *ArtistRepo) SlugsWithBase(_ context.Context, base string) ([]string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []string
	for _, a := range r.s.artists {
		if a.Slug == base || strings.HasPrefix(a.Slug, base+"-") {
			out = append(out, a.Slug)
		}
	}
	return out, nil
}

// ---- Events ----

type EventRepo struct{ s *Store }

// withLineup copies an event and attaches its lineup (caller holds lock).
func (r *EventRepo) withLineup(e *domain.Event) *domain.Event {
	c := *e
	lineup := slices.Clone(r.s.lineups[e.ID])
	for i := range lineup {
		if a, ok := r.s.artists[lineup[i].ArtistID]; ok {
			ac := *a
			lineup[i].Artist = &ac
		}
	}
	if lineup == nil {
		lineup = []domain.LineupEntry{}
	}
	c.Lineup = lineup
	return &c
}

func (r *EventRepo) Create(_ context.Context, e *domain.Event) (*domain.Event, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.venues[e.VenueID]; !ok {
		return nil, apperror.Conflict("Event conflicts with an existing record")
	}
	c := *e
	c.ID, c.Version, c.CreatedAt, c.UpdatedAt = uuid.New(), 1, r.s.now(), r.s.now()
	c.Lineup = nil
	r.s.events[c.ID] = &c
	return r.withLineup(&c), nil
}

func (r *EventRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Event, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.events[id]
	if !ok {
		return nil, apperror.NotFound("Event")
	}
	return r.withLineup(e), nil
}

func (r *EventRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return r.GetByID(ctx, id)
}

func (r *EventRepo) List(_ context.Context, f repository.EventFilter) (domain.PageResult[domain.Event], error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var items []domain.Event
	for _, e := range r.s.events {
		if (f.PublishState == nil || e.PublishState == *f.PublishState) &&
			(f.Status == nil || e.Status == *f.Status) &&
			(f.VenueID == nil || e.VenueID == *f.VenueID) {
			items = append(items, *e)
		}
	}
	_, desc := f.SortField()
	sortByCreated(items, func(e domain.Event) time.Time { return e.CreatedAt }, desc)
	return page(items, f.PageRequest), nil
}

func (r *EventRepo) Update(_ context.Context, id uuid.UUID, version int, u repository.EventUpdate) (*domain.Event, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.events[id]
	if !ok {
		return nil, apperror.NotFound("Event")
	}
	if e.Version != version {
		return nil, apperror.Conflict("Event was modified by someone else; reload and retry")
	}
	set(&e.VenueID, u.VenueID)
	set(&e.Title, u.Title)
	set(&e.Description, u.Description)
	set(&e.TicketURL, u.TicketURL)
	set(&e.PublishState, u.PublishState)
	set(&e.Status, u.Status)
	set(&e.DoorsOpenAt, u.DoorsOpenAt)
	set(&e.ScheduledStartAt, u.ScheduledStartAt)
	set(&e.ExpectedEndAt, u.ExpectedEndAt)
	e.UpdatedBy = u.UpdatedBy
	e.Version++
	e.UpdatedAt = r.s.now()
	return r.withLineup(e), nil
}

func (r *EventRepo) ReplaceLineup(_ context.Context, eventID uuid.UUID, entries []repository.LineupInput) ([]domain.LineupEntry, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	seen := map[uuid.UUID]bool{}
	out := make([]domain.LineupEntry, 0, len(entries))
	for _, in := range entries {
		if seen[in.ArtistID] {
			return nil, apperror.Conflict("Lineup contains a duplicate artist")
		}
		seen[in.ArtistID] = true
		out = append(out, domain.LineupEntry{ID: uuid.New(), EventID: eventID, ArtistID: in.ArtistID, Billing: in.Billing, SortOrder: in.SortOrder})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	r.s.lineups[eventID] = out
	return r.withLineup(&domain.Event{ID: eventID}).Lineup, nil
}

// ---- Audit ----

type AuditRepo struct{ s *Store }

func (r *AuditRepo) Create(_ context.Context, e *domain.AuditEntry) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.FailAudit != nil {
		return r.s.FailAudit
	}
	c := *e
	c.ID, c.CreatedAt = uuid.New(), r.s.now()
	r.s.audit = append(r.s.audit, c)
	return nil
}

func (r *AuditRepo) List(_ context.Context, f repository.AuditFilter) (domain.PageResult[domain.AuditEntry], error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var items []domain.AuditEntry
	for _, e := range r.s.audit {
		if (f.EntityType == "" || e.EntityType == f.EntityType) &&
			(f.EntityID == nil || e.EntityID == *f.EntityID) &&
			(f.ActorID == nil || (e.ActorID != nil && *e.ActorID == *f.ActorID)) &&
			(f.From == nil || !e.CreatedAt.Before(*f.From)) &&
			(f.To == nil || !e.CreatedAt.After(*f.To)) {
			items = append(items, e)
		}
	}
	_, desc := f.SortField()
	sortByCreated(items, func(e domain.AuditEntry) time.Time { return e.CreatedAt }, desc)
	return page(items, f.PageRequest), nil
}

var (
	_ repository.UserRepository         = (*UserRepo)(nil)
	_ repository.RefreshTokenRepository = (*RefreshTokenRepo)(nil)
	_ repository.VenueRepository        = (*VenueRepo)(nil)
	_ repository.ArtistRepository       = (*ArtistRepo)(nil)
	_ repository.EventRepository        = (*EventRepo)(nil)
	_ repository.AuditRepository        = (*AuditRepo)(nil)
)
