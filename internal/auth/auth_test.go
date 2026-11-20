package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const secret = "test-secret-0123456789abcdef0123456789"

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Fatal("short accepted")
	}
	if ValidatePassword(strings.Repeat("a", 73)) == nil {
		t.Fatal(">72 bytes accepted")
	}
	// 12 multi-byte characters is fine as long as it stays <= 72 bytes.
	if err := ValidatePassword(strings.Repeat("é", 12)); err != nil {
		t.Fatal(err)
	}
}

func TestTokenManager(t *testing.T) {
	m, err := NewTokenManager(secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok, _, err := m.IssueAccess(id, "admin")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.VerifyAccess(tok)
	if err != nil || got != id {
		t.Fatalf("verify: %v %v", got, err)
	}

	expired, _, _ := m.WithClock(func() time.Time { return time.Now().Add(-time.Hour) }).IssueAccess(id, "admin")
	if _, err := m.VerifyAccess(expired); err != ErrTokenExpired {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}

	other, _ := NewTokenManager(strings.Repeat("x", 32), time.Minute)
	foreign, _, _ := other.IssueAccess(id, "admin")
	if _, err := m.VerifyAccess(foreign); err != ErrTokenInvalid {
		t.Fatalf("wrong key accepted: %v", err)
	}
}

func TestTokenManager_RejectsOtherAlgorithms(t *testing.T) {
	m, _ := NewTokenManager(secret, time.Minute)
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := m.VerifyAccess(none); err != ErrTokenInvalid {
		t.Fatalf("alg=none accepted: %v", err)
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(secret))
	if _, err := m.VerifyAccess(hs512); err != ErrTokenInvalid {
		t.Fatalf("HS512 accepted: %v", err)
	}
}

func TestNewTokenManager_ShortSecret(t *testing.T) {
	if _, err := NewTokenManager("short", time.Minute); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestRefreshTokens(t *testing.T) {
	r := NewRefreshTokens(secret)
	id := uuid.New()
	raw, hash, err := r.Generate(id)
	if err != nil {
		t.Fatal(err)
	}
	if hash != Hash(raw) || hash == raw {
		t.Fatal("hash mismatch")
	}
	if owner, ok := r.Owner(raw); !ok || owner != id {
		t.Fatal("owner not recovered")
	}
	tampered := raw[:len(raw)-2] + "AA"
	if _, ok := r.Owner(tampered); ok {
		t.Fatal("tampered token accepted")
	}
	if _, ok := NewRefreshTokens("another-secret-another-secret-xx").Owner(raw); ok {
		t.Fatal("token from another key accepted")
	}
	raw2, _, _ := r.Generate(id)
	if raw2 == raw {
		t.Fatal("tokens not random")
	}
}
