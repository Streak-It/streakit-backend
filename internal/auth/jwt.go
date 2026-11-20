package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessCookieName  = "access_token"
	RefreshCookieName = "refresh_token"
	// RefreshCookiePath scopes the refresh cookie to auth endpoints only.
	RefreshCookiePath = "/api/v1/auth"
	MinSecretBytes    = 32
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// Claims are the access token claims. Role is informational only: the
// middleware always reloads role and status from the database.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies HS256 access tokens.
type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
	now       func() time.Time
}

func NewTokenManager(secret string, accessTTL time.Duration) (*TokenManager, error) {
	if len(secret) < MinSecretBytes {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes", MinSecretBytes)
	}
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, now: time.Now}, nil
}

// WithClock overrides the clock (tests).
func (m *TokenManager) WithClock(now func() time.Time) *TokenManager {
	c := *m
	c.now = now
	return &c
}

func (m *TokenManager) AccessTTL() time.Duration { return m.accessTTL }

// IssueAccess signs a new access token for the user.
func (m *TokenManager) IssueAccess(userID uuid.UUID, role string) (string, time.Time, error) {
	now := m.now().UTC()
	exp := now.Add(m.accessTTL)
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return token, exp, nil
}

// VerifyAccess validates signature, algorithm (HS256 only) and expiry and
// returns the subject user ID. Errors are ErrTokenExpired or ErrTokenInvalid.
func (m *TokenManager) VerifyAccess(raw string) (uuid.UUID, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, ErrTokenExpired
		}
		return uuid.Nil, ErrTokenInvalid
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrTokenInvalid
	}
	return id, nil
}
