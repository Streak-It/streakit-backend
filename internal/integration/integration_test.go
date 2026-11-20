package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
	"github.com/niteshs1/streakit-api/internal/repository/entrepo"
	"github.com/niteshs1/streakit-api/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const secret = "integration-secret-0123456789abcdef0123"

type stack struct {
	client  *ent.Client
	tx      *entrepo.TxManager
	users   *entrepo.UserRepo
	refresh *entrepo.RefreshTokenRepo
	venues  *entrepo.VenueRepo
	artists *entrepo.ArtistRepo
	events  *entrepo.EventRepo
	audit   *entrepo.AuditRepo
	hasher  *auth.PasswordHasher
}

// newStack creates an isolated schema, migrates it, and drops it on cleanup.
func newStack(t *testing.T) *stack {
	t.Helper()
	base := os.Getenv("STREAKIT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("STREAKIT_TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("it_%s", uuid.NewString()[:8])
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hasher, _ := auth.NewPasswordHasher(bcrypt.MinCost)
	return &stack{
		client:  client,
		tx:      entrepo.NewTxManager(client),
		users:   entrepo.NewUserRepo(client),
		refresh: entrepo.NewRefreshTokenRepo(client),
		venues:  entrepo.NewVenueRepo(client),
		artists: entrepo.NewArtistRepo(client),
		events:  entrepo.NewEventRepo(client),
		audit:   entrepo.NewAuditRepo(client),
		hasher:  hasher,
	}
}

func (s *stack) eventService() service.EventService {
	return service.NewEventService(service.EventDeps{Tx: s.tx, Events: s.events, Venues: s.venues, Artists: s.artists, Audit: s.audit})
}

func (s *stack) authService(t *testing.T) service.AuthService {
	tokens, err := auth.NewTokenManager(secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return service.NewAuthService(service.AuthDeps{
		Tx: s.tx, Users: s.users, RefreshTokens: s.refresh, Audit: s.audit,
		Hasher: s.hasher, Tokens: tokens, RefreshMinter: auth.NewRefreshTokens(secret), RefreshTTL: time.Hour,
	})
}

func (s *stack) user(t *testing.T, email string, role domain.Role) *domain.User {
	t.Helper()
	hash, _ := s.hasher.Hash("integration-password")
	u, err := s.users.Create(context.Background(), &domain.User{Name: "U", Email: email, PasswordHash: hash, Role: role, Status: domain.UserActive})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (s *stack) venue(t *testing.T) *domain.Venue {
	t.Helper()
	v, err := s.venues.Create(context.Background(), &domain.Venue{Name: "V", City: "Pune", Timezone: "Asia/Kolkata", Status: domain.VenueActive})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if !apperror.HasCode(err, code) {
		t.Fatalf("want %q, got %v", code, err)
	}
}

func TestEventAndAuditCommitOrRollBackTogether(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	actor := s.user(t, "o@x.io", domain.RoleSuperAdmin)
	v := s.venue(t)

	// Force the audit insert to fail inside the same transaction: the
	// event insert must roll back with it.
	boom := errors.New("audit write failed")
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.events.Create(ctx, &domain.Event{
			VenueID: v.ID, PublishState: domain.Draft, Status: domain.EventScheduled, CreatedBy: actor.ID, UpdatedBy: actor.ID,
		}); err != nil {
			return err
		}
		if err := s.audit.Create(ctx, &domain.AuditEntry{Action: "event.create", EntityType: "event", EntityID: uuid.New()}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if n := s.client.Event.Query().CountX(ctx); n != 0 {
		t.Fatalf("event survived rollback: %d", n)
	}
	if n := s.client.AuditLog.Query().CountX(ctx); n != 0 {
		t.Fatalf("audit survived rollback: %d", n)
	}

	// Happy path through the service commits both.
	ev, err := s.eventService().Create(ctx, domain.Actor{ID: actor.ID, Role: actor.Role}, service.CreateEventInput{VenueID: v.ID, Title: "Show"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.audit.List(ctx, repository.AuditFilter{
		PageRequest: domain.PageRequest{Page: 1, PageSize: 10, Sort: "-created_at"},
		EntityType:  domain.EntityEvent, EntityID: &ev.ID,
	})
	if err != nil || page.Total != 1 || page.Items[0].Action != service.ActionEventCreate {
		t.Fatalf("audit for event: %v %+v", err, page)
	}
}

func TestRefreshRotationAndReuseRevokesAll(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	u := s.user(t, "r@x.io", domain.RoleAdmin)
	svc := s.authService(t)

	_, s1, err := svc.Login(ctx, service.LoginInput{Email: "r@x.io", Password: "integration-password"})
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := svc.Login(ctx, service.LoginInput{Email: "r@x.io", Password: "integration-password"})
	if err != nil {
		t.Fatal(err)
	}
	_, s2, err := svc.Refresh(ctx, s1.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = svc.Refresh(ctx, s1.RefreshToken) // reuse
	wantCode(t, err, apperror.CodeInvalidCredentials)

	for name, tok := range map[string]string{"rotated": s2.RefreshToken, "other device": other.RefreshToken} {
		if _, _, err := svc.Refresh(ctx, tok); !apperror.HasCode(err, apperror.CodeInvalidCredentials) {
			t.Fatalf("%s session survived reuse detection: %v", name, err)
		}
	}
	active := s.client.RefreshToken.Query().CountX(ctx)
	revoked := 0
	for _, rt := range s.client.RefreshToken.Query().AllX(ctx) {
		if rt.RevokedAt != nil && rt.UserID == u.ID {
			revoked++
		}
	}
	if revoked != active {
		t.Fatalf("want all %d tokens revoked, got %d", active, revoked)
	}
}

func TestUniqueConstraints(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	u := s.user(t, "dup@x.io", domain.RoleAdmin)

	_, err := s.users.Create(ctx, &domain.User{Name: "U", Email: "dup@x.io", PasswordHash: "h", Role: domain.RoleAdmin, Status: domain.UserActive})
	wantCode(t, err, apperror.CodeConflict)

	if _, err := s.artists.Create(ctx, &domain.Artist{Name: "A", Slug: "a", Status: domain.ArtistActive}); err != nil {
		t.Fatal(err)
	}
	_, err = s.artists.Create(ctx, &domain.Artist{Name: "A", Slug: "a", Status: domain.ArtistActive})
	wantCode(t, err, apperror.CodeConflict)

	if err := s.refresh.Create(ctx, u.ID, "hash-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	wantCode(t, s.refresh.Create(ctx, u.ID, "hash-1", time.Now().Add(time.Hour)), apperror.CodeConflict)

	v := s.venue(t)
	ev, err := s.events.Create(ctx, &domain.Event{VenueID: v.ID, PublishState: domain.Draft, Status: domain.EventScheduled, CreatedBy: u.ID, UpdatedBy: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	art, _ := s.artists.Create(ctx, &domain.Artist{Name: "B", Slug: "b", Status: domain.ArtistActive})
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.events.ReplaceLineup(ctx, ev.ID, []repository.LineupInput{
			{ArtistID: art.ID, Billing: domain.Headliner},
			{ArtistID: art.ID, Billing: domain.Support},
		})
		return err
	})
	wantCode(t, err, apperror.CodeConflict)
}

func TestVenueOptimisticLock(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	v := s.venue(t)

	upd, err := s.venues.Update(ctx, v.ID, 1, repository.VenueUpdate{Name: ptr("V2")})
	if err != nil || upd.Version != 2 {
		t.Fatalf("update: %v %+v", err, upd)
	}
	_, err = s.venues.Update(ctx, v.ID, 1, repository.VenueUpdate{Name: ptr("V3")})
	wantCode(t, err, apperror.CodeConflict)
	_, err = s.venues.Update(ctx, uuid.New(), 1, repository.VenueUpdate{Name: ptr("V3")})
	wantCode(t, err, apperror.CodeNotFound)
}

func TestConcurrentBlocksKeepOneSuperAdmin(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	a := s.user(t, "a@x.io", domain.RoleSuperAdmin)
	b := s.user(t, "b@x.io", domain.RoleSuperAdmin)
	team := service.NewTeamService(service.TeamDeps{Tx: s.tx, Users: s.users, RefreshTokens: s.refresh, Audit: s.audit, Hasher: s.hasher})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, pair := range [][2]*domain.User{{a, b}, {b, a}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = team.Block(ctx, domain.Actor{ID: pair[0].ID, Role: pair[0].Role}, pair[1].ID)
		}()
	}
	wg.Wait()

	ids, err := s.users.LockActiveSuperAdminIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("want exactly one active super admin, got %d (errs: %v)", len(ids), errs)
	}
}

func ptr[T any](v T) *T { return &v }
