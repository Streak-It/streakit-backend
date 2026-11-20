package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/patch"
	"github.com/niteshs1/streakit-api/internal/ratelimit"
	"github.com/niteshs1/streakit-api/internal/repository"
	"github.com/niteshs1/streakit-api/internal/repository/memrepo"
	"github.com/niteshs1/streakit-api/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	testSecret   = "test-secret-0123456789abcdef0123456789"
	testPassword = "correct-horse-battery"
)

type env struct {
	repos   *memrepo.Repos
	hasher  *auth.PasswordHasher
	tokens  *auth.TokenManager
	minter  *auth.RefreshTokens
	auth    service.AuthService
	team    service.TeamService
	venues  service.VenueService
	artists service.ArtistService
	events  service.EventService
}

func newEnv(t *testing.T) *env {
	t.Helper()
	hasher, err := auth.NewPasswordHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r := memrepo.New()
	e := &env{repos: r, hasher: hasher, tokens: tokens, minter: auth.NewRefreshTokens(testSecret)}
	e.auth = service.NewAuthService(service.AuthDeps{
		Tx: r.Tx, Users: r.Users, RefreshTokens: r.RefreshTokens, Audit: r.Audit,
		Hasher: hasher, Tokens: tokens, RefreshMinter: e.minter, RefreshTTL: time.Hour,
	})
	e.team = service.NewTeamService(service.TeamDeps{Tx: r.Tx, Users: r.Users, RefreshTokens: r.RefreshTokens, Audit: r.Audit, Hasher: hasher})
	e.venues = service.NewVenueService(service.VenueDeps{Tx: r.Tx, Venues: r.Venues, Audit: r.Audit})
	e.artists = service.NewArtistService(service.ArtistDeps{Tx: r.Tx, Artists: r.Artists, Audit: r.Audit})
	e.events = service.NewEventService(service.EventDeps{Tx: r.Tx, Events: r.Events, Venues: r.Venues, Artists: r.Artists, Audit: r.Audit})
	return e
}

func (e *env) user(t *testing.T, email string, role domain.Role, status domain.UserStatus) *domain.User {
	t.Helper()
	hash, err := e.hasher.Hash(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.repos.Users.Create(context.Background(), &domain.User{
		Name: "Test", Email: email, PasswordHash: hash, Role: role, Status: status,
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func actorOf(u *domain.User) domain.Actor { return domain.Actor{ID: u.ID, Role: u.Role} }

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if !apperror.HasCode(err, code) {
		t.Fatalf("want error code %q, got %v", code, err)
	}
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	wantCode(t, err, apperror.CodeValidation)
	for _, f := range apperror.From(err).Fields {
		if f.Field == field {
			return
		}
	}
	t.Fatalf("want field error on %q, got %+v", field, apperror.From(err).Fields)
}

// ---- Auth ----

func TestLogin_WrongPasswordAndUnknownEmailReturnSameError(t *testing.T) {
	e := newEnv(t)
	e.user(t, "a@x.io", domain.RoleAdmin, domain.UserActive)
	ctx := context.Background()

	_, _, errWrong := e.auth.Login(ctx, service.LoginInput{Email: "a@x.io", Password: "nope-nope-nope"})
	_, _, errUnknown := e.auth.Login(ctx, service.LoginInput{Email: "ghost@x.io", Password: "nope-nope-nope"})

	wantCode(t, errWrong, apperror.CodeInvalidCredentials)
	wantCode(t, errUnknown, apperror.CodeInvalidCredentials)
	if apperror.From(errWrong).Message != apperror.From(errUnknown).Message {
		t.Fatal("messages differ; enables account enumeration")
	}
	if n := len(e.repos.Store.AuditEntries()); n != 0 {
		t.Fatalf("failed logins must not be audited, got %d entries", n)
	}
}

func TestLogin_BlockedUserIsInvalidCredentials(t *testing.T) {
	e := newEnv(t)
	e.user(t, "b@x.io", domain.RoleAdmin, domain.UserBlocked)
	_, _, err := e.auth.Login(context.Background(), service.LoginInput{Email: "b@x.io", Password: testPassword})
	wantCode(t, err, apperror.CodeInvalidCredentials)
}

func TestLogin_SuccessNormalisesEmailAndAudits(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "c@x.io", domain.RoleAdmin, domain.UserActive)

	got, session, err := e.auth.Login(context.Background(), service.LoginInput{Email: "  C@X.IO ", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || got.LastLoginAt == nil {
		t.Fatalf("unexpected user %+v", got)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	entries := e.repos.Store.AuditEntries()
	if len(entries) != 1 || entries[0].Action != service.ActionAuthLogin {
		t.Fatalf("want one auth.login audit entry, got %+v", entries)
	}
}

func TestLogin_RateLimitedPerEmail(t *testing.T) {
	e := newEnv(t)
	r := e.repos
	svc := service.NewAuthService(service.AuthDeps{
		Tx: r.Tx, Users: r.Users, RefreshTokens: r.RefreshTokens, Audit: r.Audit,
		Hasher: e.hasher, Tokens: e.tokens, RefreshMinter: e.minter, RefreshTTL: time.Hour,
		EmailLimiter: ratelimit.New(2, time.Minute),
	})
	in := service.LoginInput{Email: "d@x.io", Password: "whatever-whatever"}
	for range 2 {
		_, _, err := svc.Login(context.Background(), in)
		wantCode(t, err, apperror.CodeInvalidCredentials)
	}
	_, _, err := svc.Login(context.Background(), in)
	wantCode(t, err, apperror.CodeRateLimited)
}

func TestRefresh_RotatesAndDetectsReuse(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "r@x.io", domain.RoleAdmin, domain.UserActive)
	ctx := context.Background()

	_, s1, err := e.auth.Login(ctx, service.LoginInput{Email: "r@x.io", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	_, s2, err := e.auth.Refresh(ctx, s1.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if s2.RefreshToken == s1.RefreshToken {
		t.Fatal("refresh token not rotated")
	}

	// Replaying the old token is reuse: rejected and every session revoked.
	_, _, err = e.auth.Refresh(ctx, s1.RefreshToken)
	wantCode(t, err, apperror.CodeInvalidCredentials)
	if n := e.repos.Store.ActiveTokenCount(u.ID); n != 0 {
		t.Fatalf("want all sessions revoked, %d still active", n)
	}
	_, _, err = e.auth.Refresh(ctx, s2.RefreshToken)
	wantCode(t, err, apperror.CodeInvalidCredentials)
}

func TestRefresh_UnknownButMintedTokenRevokesOwner(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "m@x.io", domain.RoleAdmin, domain.UserActive)
	ctx := context.Background()
	if _, _, err := e.auth.Login(ctx, service.LoginInput{Email: "m@x.io", Password: testPassword}); err != nil {
		t.Fatal(err)
	}

	orphan, _, _ := e.minter.Generate(u.ID) // valid MAC, no DB row
	_, _, err := e.auth.Refresh(ctx, orphan)
	wantCode(t, err, apperror.CodeInvalidCredentials)
	if n := e.repos.Store.ActiveTokenCount(u.ID); n != 0 {
		t.Fatalf("want sessions revoked, %d active", n)
	}

	// A forged token (bad MAC) must not let an attacker log someone out.
	_, s, _ := e.auth.Login(ctx, service.LoginInput{Email: "m@x.io", Password: testPassword})
	_, _, err = e.auth.Refresh(ctx, "AAAA"+s.RefreshToken[4:])
	wantCode(t, err, apperror.CodeInvalidCredentials)
	if n := e.repos.Store.ActiveTokenCount(u.ID); n != 1 {
		t.Fatalf("forged token revoked sessions: %d active", n)
	}
}

func TestAuthenticate_ExpiredAndBlocked(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "t@x.io", domain.RoleAdmin, domain.UserActive)
	ctx := context.Background()

	past := e.tokens.WithClock(func() time.Time { return time.Now().Add(-time.Hour) })
	expired, _, _ := past.IssueAccess(u.ID, string(u.Role))
	_, err := e.auth.Authenticate(ctx, expired)
	wantCode(t, err, apperror.CodeTokenExpired)

	valid, _, _ := e.tokens.IssueAccess(u.ID, string(u.Role))
	if _, err := e.auth.Authenticate(ctx, valid); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repos.Users.Update(ctx, u.ID, repository.UserUpdate{Status: ptr(domain.UserBlocked)}); err != nil {
		t.Fatal(err)
	}
	_, err = e.auth.Authenticate(ctx, valid)
	wantCode(t, err, apperror.CodeUnauthorized)
}

func TestAuthenticate_RoleComesFromDBNotToken(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "role@x.io", domain.RoleAdmin, domain.UserActive)
	forged, _, _ := e.tokens.IssueAccess(u.ID, string(domain.RoleSuperAdmin))
	got, err := e.auth.Authenticate(context.Background(), forged)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != domain.RoleAdmin {
		t.Fatalf("role taken from token: %s", got.Role)
	}
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t)
	u := e.user(t, "p@x.io", domain.RoleAdmin, domain.UserActive)
	ctx := context.Background()
	if _, _, err := e.auth.Login(ctx, service.LoginInput{Email: "p@x.io", Password: testPassword}); err != nil {
		t.Fatal(err)
	}

	err := e.auth.ChangePassword(ctx, actorOf(u), service.ChangePasswordInput{CurrentPassword: "wrong-wrong-wrong", NewPassword: "new-password-123"})
	wantField(t, err, "current_password")

	err = e.auth.ChangePassword(ctx, actorOf(u), service.ChangePasswordInput{CurrentPassword: testPassword, NewPassword: "short"})
	wantField(t, err, "new_password")

	if err := e.auth.ChangePassword(ctx, actorOf(u), service.ChangePasswordInput{CurrentPassword: testPassword, NewPassword: "new-password-123"}); err != nil {
		t.Fatal(err)
	}
	if n := e.repos.Store.ActiveTokenCount(u.ID); n != 0 {
		t.Fatalf("want refresh tokens revoked, %d active", n)
	}
	if _, _, err := e.auth.Login(ctx, service.LoginInput{Email: "p@x.io", Password: "new-password-123"}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestBootstrap(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	if err := e.auth.Bootstrap(ctx, service.BootstrapInput{}); err == nil {
		t.Fatal("want failure when no super admin and env incomplete")
	}
	err := e.auth.Bootstrap(ctx, service.BootstrapInput{Email: "o@x.io", Password: "short", Name: "O"})
	if err == nil {
		t.Fatal("want password policy failure")
	}
	if err := e.auth.Bootstrap(ctx, service.BootstrapInput{Email: "O@X.io", Password: testPassword, Name: "Owner"}); err != nil {
		t.Fatal(err)
	}
	u, err := e.repos.Users.GetByEmail(ctx, "o@x.io")
	if err != nil || u.Role != domain.RoleSuperAdmin {
		t.Fatalf("super admin not created: %v %+v", err, u)
	}
	// Second run is a no-op even with empty env.
	if err := e.auth.Bootstrap(ctx, service.BootstrapInput{}); err != nil {
		t.Fatal(err)
	}
	if a := e.repos.Store.AuditEntries(); len(a) != 1 || a[0].ActorID != nil {
		t.Fatalf("want one system audit entry, got %+v", a)
	}
}

// ---- Team ----

func TestTeam_SelfBlockAndSelfRoleDenied(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "o@x.io", domain.RoleSuperAdmin, domain.UserActive)
	e.user(t, "o2@x.io", domain.RoleSuperAdmin, domain.UserActive)

	_, err := e.team.Block(context.Background(), actorOf(owner), owner.ID)
	wantCode(t, err, apperror.CodeForbidden)
	_, err = e.team.ChangeRole(context.Background(), actorOf(owner), owner.ID, domain.RoleAdmin)
	wantCode(t, err, apperror.CodeForbidden)
}

func TestTeam_LastSuperAdminGuard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.user(t, "a@x.io", domain.RoleSuperAdmin, domain.UserActive)
	b := e.user(t, "b@x.io", domain.RoleSuperAdmin, domain.UserActive)

	// Two super admins: blocking one is fine.
	if _, err := e.team.Block(ctx, actorOf(a), b.ID); err != nil {
		t.Fatal(err)
	}
	// a is now the last active super admin. In production this is reached
	// when two super admins act on each other concurrently; here b's
	// (now stale) actor stands in for the second request.
	_, err := e.team.ChangeRole(ctx, actorOf(b), a.ID, domain.RoleAdmin)
	wantCode(t, err, apperror.CodeConflict)
	_, err = e.team.Block(ctx, actorOf(b), a.ID)
	wantCode(t, err, apperror.CodeConflict)
}

func TestTeam_BlockRevokesSessions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	owner := e.user(t, "o@x.io", domain.RoleSuperAdmin, domain.UserActive)
	admin := e.user(t, "a@x.io", domain.RoleAdmin, domain.UserActive)
	if _, _, err := e.auth.Login(ctx, service.LoginInput{Email: "a@x.io", Password: testPassword}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.team.Block(ctx, actorOf(owner), admin.ID); err != nil {
		t.Fatal(err)
	}
	if n := e.repos.Store.ActiveTokenCount(admin.ID); n != 0 {
		t.Fatalf("want sessions revoked, %d active", n)
	}
	_, err := e.team.Block(ctx, actorOf(owner), admin.ID)
	wantCode(t, err, apperror.CodeConflict)
}

func TestTeam_AdminCannotWrite(t *testing.T) {
	e := newEnv(t)
	admin := e.user(t, "a@x.io", domain.RoleAdmin, domain.UserActive)
	_, err := e.team.Create(context.Background(), actorOf(admin), service.CreateUserInput{Name: "N", Email: "n@x.io", Password: testPassword})
	wantCode(t, err, apperror.CodeForbidden)
}

func TestTeam_CreateValidatesAndAuditsWithoutHash(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "o@x.io", domain.RoleSuperAdmin, domain.UserActive)
	ctx := context.Background()

	_, err := e.team.Create(ctx, actorOf(owner), service.CreateUserInput{Name: "", Email: "bad", Password: "short"})
	wantField(t, err, "name")
	wantField(t, err, "email")
	wantField(t, err, "password")

	u, err := e.team.Create(ctx, actorOf(owner), service.CreateUserInput{Name: "N", Email: " N@X.io", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "n@x.io" || u.Role != domain.RoleAdmin {
		t.Fatalf("unexpected %+v", u)
	}
	for _, a := range e.repos.Store.AuditEntries() {
		if _, ok := a.After["password_hash"]; ok {
			t.Fatal("password_hash leaked into audit")
		}
	}
	_, err = e.team.Create(ctx, actorOf(owner), service.CreateUserInput{Name: "N", Email: "n@x.io", Password: testPassword})
	wantCode(t, err, apperror.CodeConflict)
}

// ---- Venues ----

func TestVenue_VersionConflict(t *testing.T) {
	e := newEnv(t)
	actor := domain.Actor{ID: uuid.New(), Role: domain.RoleAdmin}
	ctx := context.Background()
	v, err := e.venues.Create(ctx, actor, service.CreateVenueInput{Name: "V", City: "Pune"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Timezone != service.DefaultVenueTimezone {
		t.Fatalf("default timezone not applied: %q", v.Timezone)
	}
	if _, err := e.venues.Update(ctx, actor, v.ID, service.UpdateVenueInput{Version: 1, Name: ptr("V2")}); err != nil {
		t.Fatal(err)
	}
	_, err = e.venues.Update(ctx, actor, v.ID, service.UpdateVenueInput{Version: 1, Name: ptr("V3")})
	wantCode(t, err, apperror.CodeConflict)
}

func TestVenue_RetireStateMachine(t *testing.T) {
	e := newEnv(t)
	actor := domain.Actor{ID: uuid.New(), Role: domain.RoleAdmin}
	ctx := context.Background()
	v, _ := e.venues.Create(ctx, actor, service.CreateVenueInput{Name: "V", City: "Pune"})
	if _, err := e.venues.Retire(ctx, actor, v.ID); err != nil {
		t.Fatal(err)
	}
	_, err := e.venues.Retire(ctx, actor, v.ID)
	wantCode(t, err, apperror.CodeConflict)
	if _, err := e.venues.Restore(ctx, actor, v.ID); err != nil {
		t.Fatal(err)
	}
}

// ---- Artists ----

func TestArtist_SlugCollisionsAndRename(t *testing.T) {
	e := newEnv(t)
	actor := domain.Actor{ID: uuid.New(), Role: domain.RoleAdmin}
	ctx := context.Background()

	a1, _ := e.artists.Create(ctx, actor, service.CreateArtistInput{Name: "Café Tacvba"})
	a2, _ := e.artists.Create(ctx, actor, service.CreateArtistInput{Name: "cafe tacvba!"})
	a3, _ := e.artists.Create(ctx, actor, service.CreateArtistInput{Name: "CAFE  TACVBA"})
	if a1.Slug != "cafe-tacvba" || a2.Slug != "cafe-tacvba-2" || a3.Slug != "cafe-tacvba-3" {
		t.Fatalf("slugs: %s %s %s", a1.Slug, a2.Slug, a3.Slug)
	}

	renamed, err := e.artists.Update(ctx, actor, a2.ID, service.UpdateArtistInput{Name: ptr("Zoé")})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Slug != "zoe" {
		t.Fatalf("rename slug: %s", renamed.Slug)
	}
	// Cosmetic rename keeps the existing suffixed slug.
	kept, _ := e.artists.Update(ctx, actor, a3.ID, service.UpdateArtistInput{Name: ptr("Cafe Tacvba")})
	if kept.Slug != "cafe-tacvba-3" {
		t.Fatalf("cosmetic rename changed slug to %s", kept.Slug)
	}
}

// ---- Events ----

type eventFixture struct {
	actor  domain.Actor
	venue  *domain.Venue
	artist *domain.Artist
	event  *domain.Event
}

func newEventFixture(t *testing.T, e *env) eventFixture {
	t.Helper()
	ctx := context.Background()
	actor := domain.Actor{ID: uuid.New(), Role: domain.RoleAdmin}
	v, err := e.venues.Create(ctx, actor, service.CreateVenueInput{Name: "V", City: "Goa"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.artists.Create(ctx, actor, service.CreateArtistInput{Name: "Headliner"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2027, 1, 1, 20, 0, 0, 0, time.UTC)
	ev, err := e.events.Create(ctx, actor, service.CreateEventInput{VenueID: v.ID, Title: "Show", ScheduledStartAt: &start})
	if err != nil {
		t.Fatal(err)
	}
	return eventFixture{actor: actor, venue: v, artist: a, event: ev}
}

func TestEvent_TimeOrderRejected(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	start := time.Date(2027, 1, 1, 20, 0, 0, 0, time.UTC)
	doors := start.Add(time.Hour)
	end := start.Add(-time.Hour)

	_, err := e.events.Create(context.Background(), f.actor, service.CreateEventInput{
		VenueID: f.venue.ID, DoorsOpenAt: &doors, ScheduledStartAt: &start, ExpectedEndAt: &end,
	})
	wantField(t, err, "doors_open_at")
	wantField(t, err, "expected_end_at")

	_, err = e.events.Update(context.Background(), f.actor, f.event.ID, service.UpdateEventInput{
		Version: f.event.Version, DoorsOpenAt: patch.Of(doors),
	})
	wantField(t, err, "doors_open_at")
}

func TestEvent_TicketURLMustBeHTTPS(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	for _, u := range []string{"http://t.io/x", "https://user:pw@t.io/x", "ftp://t.io"} {
		_, err := e.events.Create(context.Background(), f.actor, service.CreateEventInput{VenueID: f.venue.ID, TicketURL: u})
		wantField(t, err, "ticket_url")
	}
}

func TestEvent_PublishRequiresHeadliner(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	ctx := context.Background()

	_, err := e.events.Publish(ctx, f.actor, f.event.ID)
	wantField(t, err, "lineup")

	if _, err := e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{
		{ArtistID: f.artist.ID, Billing: domain.Support},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = e.events.Publish(ctx, f.actor, f.event.ID)
	wantField(t, err, "lineup")

	if _, err := e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{
		{ArtistID: f.artist.ID, Billing: domain.Headliner},
	}); err != nil {
		t.Fatal(err)
	}
	pub, err := e.events.Publish(ctx, f.actor, f.event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pub.PublishState != domain.Published {
		t.Fatalf("not published: %+v", pub)
	}
}

func TestEvent_PublishRejectsRetiredVenueAndMissingStart(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	ctx := context.Background()
	if _, err := e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{{ArtistID: f.artist.ID, Billing: domain.Headliner}}); err != nil {
		t.Fatal(err)
	}
	cur, _ := e.events.Get(ctx, f.event.ID)
	if _, err := e.events.Update(ctx, f.actor, f.event.ID, service.UpdateEventInput{Version: cur.Version, ScheduledStartAt: patch.Null[time.Time]()}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.venues.Retire(ctx, f.actor, f.venue.ID); err != nil {
		t.Fatal(err)
	}
	_, err := e.events.Publish(ctx, f.actor, f.event.ID)
	wantField(t, err, "venue_id")
	wantField(t, err, "scheduled_start_at")
}

func TestEvent_VersionConflict(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	ctx := context.Background()
	if _, err := e.events.Update(ctx, f.actor, f.event.ID, service.UpdateEventInput{Version: 1, Title: ptr("A")}); err != nil {
		t.Fatal(err)
	}
	_, err := e.events.Update(ctx, f.actor, f.event.ID, service.UpdateEventInput{Version: 1, Title: ptr("B")})
	wantCode(t, err, apperror.CodeConflict)
}

func TestEvent_StateMachine(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	ctx := context.Background()
	id := f.event.ID

	_, err := e.events.Cancel(ctx, f.actor, id)
	wantCode(t, err, apperror.CodeConflict) // draft cannot be cancelled

	if _, err := e.events.ReplaceLineup(ctx, f.actor, id, []repository.LineupInput{{ArtistID: f.artist.ID, Billing: domain.Headliner}}); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		name string
		do   func() (*domain.Event, error)
		ps   domain.PublishState
		st   domain.EventStatus
	}{
		{"publish", func() (*domain.Event, error) { return e.events.Publish(ctx, f.actor, id) }, domain.Published, domain.EventScheduled},
		{"cancel", func() (*domain.Event, error) { return e.events.Cancel(ctx, f.actor, id) }, domain.Published, domain.EventCancelled},
		{"restore", func() (*domain.Event, error) { return e.events.Restore(ctx, f.actor, id) }, domain.Published, domain.EventScheduled},
		{"postpone", func() (*domain.Event, error) {
			return e.events.Postpone(ctx, f.actor, id, service.PostponeInput{ScheduledStartAt: time.Date(2027, 2, 1, 20, 0, 0, 0, time.UTC)})
		}, domain.Published, domain.EventScheduled},
		{"unpublish", func() (*domain.Event, error) { return e.events.Unpublish(ctx, f.actor, id) }, domain.Draft, domain.EventScheduled},
	}
	for _, s := range steps {
		got, err := s.do()
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got.PublishState != s.ps || got.Status != s.st {
			t.Fatalf("%s: got %s/%s", s.name, got.PublishState, got.Status)
		}
	}
	_, err = e.events.Restore(ctx, f.actor, id)
	wantCode(t, err, apperror.CodeConflict)
}

func TestEvent_LineupRules(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	ctx := context.Background()
	hidden, _ := e.artists.Create(ctx, f.actor, service.CreateArtistInput{Name: "Hidden"})
	if _, err := e.artists.Hide(ctx, f.actor, hidden.ID); err != nil {
		t.Fatal(err)
	}

	_, err := e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{
		{ArtistID: f.artist.ID, Billing: domain.Headliner},
		{ArtistID: hidden.ID, Billing: domain.Support},
	})
	wantField(t, err, "lineup[1].artist_id")

	_, err = e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{
		{ArtistID: f.artist.ID, Billing: domain.Headliner},
		{ArtistID: f.artist.ID, Billing: domain.Support},
	})
	wantField(t, err, "lineup[1].artist_id")

	// Published events must keep a headliner.
	if _, err := e.events.ReplaceLineup(ctx, f.actor, f.event.ID, []repository.LineupInput{{ArtistID: f.artist.ID, Billing: domain.Headliner}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.events.Publish(ctx, f.actor, f.event.ID); err != nil {
		t.Fatal(err)
	}
	_, err = e.events.ReplaceLineup(ctx, f.actor, f.event.ID, nil)
	wantField(t, err, "lineup")
}

func TestEvent_AuditOnlyChangedFields(t *testing.T) {
	e := newEnv(t)
	f := newEventFixture(t, e)
	if _, err := e.events.Update(context.Background(), f.actor, f.event.ID, service.UpdateEventInput{Version: 1, Title: ptr("New title")}); err != nil {
		t.Fatal(err)
	}
	entries := e.repos.Store.AuditEntries()
	last := entries[len(entries)-1]
	if last.Action != service.ActionEventUpdate || len(last.After) != 1 || last.After["title"] != "New title" || last.Before["title"] != "Show" {
		t.Fatalf("unexpected audit diff: before=%v after=%v", last.Before, last.After)
	}
}

func ptr[T any](v T) *T { return &v }
