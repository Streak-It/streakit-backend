package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/config"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/handler"
	"github.com/niteshs1/streakit-api/internal/repository/memrepo"
	"github.com/niteshs1/streakit-api/internal/routes"
	"github.com/niteshs1/streakit-api/internal/server"
	"github.com/niteshs1/streakit-api/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	secret   = "test-secret-0123456789abcdef0123456789"
	password = "correct-horse-battery"
)

type app struct {
	t      *testing.T
	router http.Handler
	repos  *memrepo.Repos
	hasher *auth.PasswordHasher
	tokens *auth.TokenManager
}

type pingOK struct{}

func (pingOK) Ping(context.Context) error { return nil }

func newApp(t *testing.T) *app {
	t.Helper()
	hasher, _ := auth.NewPasswordHasher(bcrypt.MinCost)
	tokens, _ := auth.NewTokenManager(secret, 15*time.Minute)
	r := memrepo.New()

	authSvc := service.NewAuthService(service.AuthDeps{
		Tx: r.Tx, Users: r.Users, RefreshTokens: r.RefreshTokens, Audit: r.Audit,
		Hasher: hasher, Tokens: tokens, RefreshMinter: auth.NewRefreshTokens(secret), RefreshTTL: time.Hour,
	})
	cookies := handler.CookieConfig{Secure: true}
	h := routes.Handlers{
		Health: handler.NewHealthHandler(pingOK{}),
		Auth:   handler.NewAuthHandler(authSvc, cookies),
		Team:   handler.NewTeamHandler(service.NewTeamService(service.TeamDeps{Tx: r.Tx, Users: r.Users, RefreshTokens: r.RefreshTokens, Audit: r.Audit, Hasher: hasher})),
		Venue:  handler.NewVenueHandler(service.NewVenueService(service.VenueDeps{Tx: r.Tx, Venues: r.Venues, Audit: r.Audit})),
		Artist: handler.NewArtistHandler(service.NewArtistService(service.ArtistDeps{Tx: r.Tx, Artists: r.Artists, Audit: r.Audit})),
		Event:  handler.NewEventHandler(service.NewEventService(service.EventDeps{Tx: r.Tx, Events: r.Events, Venues: r.Venues, Artists: r.Artists, Audit: r.Audit})),
		Audit:  handler.NewAuditHandler(service.NewAuditService(r.Audit)),
	}
	router, err := server.NewRouter(config.AppConfig{Env: "test"}, slog.New(slog.NewTextHandler(io.Discard, nil)), h, routes.Options{Authenticator: authSvc, EnableSwagger: true})
	if err != nil {
		t.Fatal(err)
	}
	return &app{t: t, router: router, repos: r, hasher: hasher, tokens: tokens}
}

func (a *app) user(email string, role domain.Role) *domain.User {
	hash, _ := a.hasher.Hash(password)
	u, err := a.repos.Users.Create(context.Background(), &domain.User{Name: "U", Email: email, PasswordHash: hash, Role: role, Status: domain.UserActive})
	if err != nil {
		a.t.Fatal(err)
	}
	return u
}

func (a *app) accessCookie(u *domain.User) *http.Cookie {
	tok, _, _ := a.tokens.IssueAccess(u.ID, string(u.Role))
	return &http.Cookie{Name: auth.AccessCookieName, Value: tok}
}

func (a *app) do(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code   string `json:"code"`
		Fields []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"error"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("bad JSON %q: %v", w.Body.String(), err)
	}
	return e
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, code string) envelope {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d; body %s", w.Code, status, w.Body.String())
	}
	e := decode(t, w)
	if code == "" {
		if !e.Success {
			t.Fatalf("want success, got %s", w.Body.String())
		}
	} else if e.Error == nil || e.Error.Code != code {
		t.Fatalf("want error code %q, got %s", code, w.Body.String())
	}
	return e
}

func cookieByName(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLogin_SetsHardenedCookiesAndNoTokensInBody(t *testing.T) {
	a := newApp(t)
	a.user("owner@x.io", domain.RoleSuperAdmin)

	w := a.do("POST", "/api/v1/auth/login", `{"email":"owner@x.io","password":"`+password+`"}`)
	e := expect(t, w, http.StatusOK, "")

	access, refresh := cookieByName(w, auth.AccessCookieName), cookieByName(w, auth.RefreshCookieName)
	if access == nil || refresh == nil {
		t.Fatal("missing session cookies")
	}
	for _, c := range []*http.Cookie{access, refresh} {
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 {
			t.Fatalf("cookie %s flags wrong: %+v", c.Name, c)
		}
	}
	if access.Path != "/" || refresh.Path != auth.RefreshCookiePath {
		t.Fatalf("cookie paths: %q %q", access.Path, refresh.Path)
	}
	body := string(e.Data)
	if strings.Contains(body, access.Value) || strings.Contains(body, refresh.Value) || strings.Contains(body, "token") {
		t.Fatalf("token leaked into body: %s", body)
	}
}

func TestLogin_InvalidCredentialsAndValidation(t *testing.T) {
	a := newApp(t)
	a.user("owner@x.io", domain.RoleSuperAdmin)

	expect(t, a.do("POST", "/api/v1/auth/login", `{"email":"owner@x.io","password":"wrong-wrong-wrong"}`), http.StatusUnauthorized, "invalid_credentials")

	e := expect(t, a.do("POST", "/api/v1/auth/login", `{"email":"not-an-email"}`), http.StatusUnprocessableEntity, "validation_error")
	fields := map[string]string{}
	for _, f := range e.Error.Fields {
		fields[f.Field] = f.Message
	}
	if fields["email"] == "" || fields["password"] != "required" {
		t.Fatalf("unexpected fields %+v", e.Error.Fields)
	}
}

func TestAuth_MissingCookie(t *testing.T) {
	a := newApp(t)
	expect(t, a.do("GET", "/api/v1/auth/me", ""), http.StatusUnauthorized, "unauthorized")
}

func TestAuth_BearerHeaderIsIgnored(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+a.accessCookie(u).Value)
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	expect(t, w, http.StatusUnauthorized, "unauthorized")
}

func TestAuth_TokenExpired(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	old, _, _ := a.tokens.WithClock(func() time.Time { return time.Now().Add(-time.Hour) }).IssueAccess(u.ID, string(u.Role))
	w := a.do("GET", "/api/v1/auth/me", "", &http.Cookie{Name: auth.AccessCookieName, Value: old})
	expect(t, w, http.StatusUnauthorized, "token_expired")
}

func TestAuth_MeReturnsProfile(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	e := expect(t, a.do("GET", "/api/v1/auth/me", "", a.accessCookie(u)), http.StatusOK, "")
	if !strings.Contains(string(e.Data), u.ID.String()) || strings.Contains(string(e.Data), "password") {
		t.Fatalf("unexpected profile %s", e.Data)
	}
}

func TestAdminCannotUseSuperAdminRoutes(t *testing.T) {
	a := newApp(t)
	admin := a.user("admin@x.io", domain.RoleAdmin)
	c := a.accessCookie(admin)

	expect(t, a.do("POST", "/api/v1/team/users", `{"name":"N","email":"n@x.io","password":"`+password+`"}`, c), http.StatusForbidden, "forbidden")
	expect(t, a.do("POST", "/api/v1/team/users/"+admin.ID.String()+"/block", "", c), http.StatusForbidden, "forbidden")
	// Reads are allowed for admins.
	expect(t, a.do("GET", "/api/v1/team/users", "", c), http.StatusOK, "")
	expect(t, a.do("GET", "/api/v1/audit", "", c), http.StatusOK, "")
}

func TestChangePassword_ClearsCookies(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	c := a.accessCookie(u)

	expect(t, a.do("POST", "/api/v1/auth/password", `{"current_password":"wrong-wrong-wrong","new_password":"another-password-1"}`, c),
		http.StatusUnprocessableEntity, "validation_error")

	w := a.do("POST", "/api/v1/auth/password", `{"current_password":"`+password+`","new_password":"another-password-1"}`, c)
	expect(t, w, http.StatusOK, "")
	for _, name := range []string{auth.AccessCookieName, auth.RefreshCookieName} {
		ck := cookieByName(w, name)
		if ck == nil || ck.MaxAge >= 0 || ck.Value != "" {
			t.Fatalf("cookie %s not cleared: %+v", name, ck)
		}
	}
	expect(t, a.do("POST", "/api/v1/auth/login", `{"email":"owner@x.io","password":"another-password-1"}`), http.StatusOK, "")
}

func TestRefreshAndLogoutFlow(t *testing.T) {
	a := newApp(t)
	a.user("owner@x.io", domain.RoleSuperAdmin)
	login := a.do("POST", "/api/v1/auth/login", `{"email":"owner@x.io","password":"`+password+`"}`)
	refresh := cookieByName(login, auth.RefreshCookieName)

	w := a.do("POST", "/api/v1/auth/refresh", "", refresh)
	expect(t, w, http.StatusOK, "")
	rotated := cookieByName(w, auth.RefreshCookieName)
	if rotated == nil || rotated.Value == refresh.Value {
		t.Fatal("refresh cookie not rotated")
	}

	// Reusing the old cookie fails and clears cookies.
	reuse := a.do("POST", "/api/v1/auth/refresh", "", refresh)
	expect(t, reuse, http.StatusUnauthorized, "invalid_credentials")
	if ck := cookieByName(reuse, auth.AccessCookieName); ck == nil || ck.MaxAge >= 0 {
		t.Fatal("cookies not cleared after failed refresh")
	}

	out := a.do("POST", "/api/v1/auth/logout", "", rotated)
	expect(t, out, http.StatusOK, "")
}

func TestRequireJSONOnMutations(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	req := httptest.NewRequest("POST", "/api/v1/venues", strings.NewReader("name=x&city=y"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(a.accessCookie(u))
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	expect(t, w, http.StatusUnsupportedMediaType, "unsupported_media_type")
}

func TestValidationAndListShape(t *testing.T) {
	a := newApp(t)
	u := a.user("owner@x.io", domain.RoleSuperAdmin)
	c := a.accessCookie(u)

	e := expect(t, a.do("POST", "/api/v1/venues", `{"name":"V","city":"Pune","extra":true}`, c), http.StatusUnprocessableEntity, "validation_error")
	if e.Error.Fields[0].Field != "extra" {
		t.Fatalf("unknown field not reported: %+v", e.Error.Fields)
	}
	expect(t, a.do("GET", "/api/v1/venues?page_size=101", "", c), http.StatusUnprocessableEntity, "validation_error")
	expect(t, a.do("GET", "/api/v1/venues/not-a-uuid", "", c), http.StatusUnprocessableEntity, "validation_error")

	expect(t, a.do("POST", "/api/v1/venues", `{"name":"V","city":"Pune"}`, c), http.StatusCreated, "")
	list := expect(t, a.do("GET", "/api/v1/venues", "", c), http.StatusOK, "")
	var page struct {
		Items    []map[string]any `json:"items"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(list.Data, &page); err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.PageSize != domain.DefaultPageSize || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("unexpected page %+v", page)
	}
}

func TestNotFoundEnvelope(t *testing.T) {
	a := newApp(t)
	expect(t, a.do("GET", "/api/v1/nope", ""), http.StatusNotFound, "not_found")
}

func TestSwaggerServed(t *testing.T) {
	a := newApp(t)
	if w := a.do("GET", "/swagger/index.html", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "swagger-ui") {
		t.Fatalf("swagger UI: %d", w.Code)
	}
	if w := a.do("GET", "/swagger/doc.json", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "CookieAuth") {
		t.Fatalf("swagger doc: %d", w.Code)
	}
}
