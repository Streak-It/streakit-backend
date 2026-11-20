package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

// Refresh token wire format (base64url, no padding):
//
//	user_id (16 bytes) || random (32 bytes) || hmac-sha256(key, user_id||random)[:16]
//
// The MAC lets the server recognise a token it issued even when the DB row
// is gone, so it can attribute reuse to a user without trusting forged
// input. Only sha256(token) is stored.
const (
	refreshRandomLen = 32
	refreshMACLen    = 16
	refreshRawLen    = 16 + refreshRandomLen + refreshMACLen
)

var refreshEncoding = base64.RawURLEncoding

// RefreshTokens creates and inspects opaque refresh tokens.
type RefreshTokens struct {
	macKey []byte
}

// NewRefreshTokens derives a dedicated MAC key from the JWT secret.
func NewRefreshTokens(secret string) *RefreshTokens {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("streakit/refresh-token/v1"))
	return &RefreshTokens{macKey: mac.Sum(nil)}
}

// Generate returns a new raw token for the cookie and its storage hash.
func (r *RefreshTokens) Generate(userID uuid.UUID) (raw, hash string, err error) {
	buf := make([]byte, refreshRawLen)
	copy(buf, userID[:])
	if _, err := rand.Read(buf[16 : 16+refreshRandomLen]); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	copy(buf[16+refreshRandomLen:], r.mac(buf[:16+refreshRandomLen]))
	raw = refreshEncoding.EncodeToString(buf)
	return raw, Hash(raw), nil
}

// Owner returns the user a token was issued to, if the token carries a
// valid MAC (i.e. it was minted by this server).
func (r *RefreshTokens) Owner(raw string) (uuid.UUID, bool) {
	buf, err := refreshEncoding.DecodeString(raw)
	if err != nil || len(buf) != refreshRawLen {
		return uuid.Nil, false
	}
	payload, sig := buf[:16+refreshRandomLen], buf[16+refreshRandomLen:]
	if !hmac.Equal(sig, r.mac(payload)) {
		return uuid.Nil, false
	}
	id, err := uuid.FromBytes(buf[:16])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func (r *RefreshTokens) mac(b []byte) []byte {
	m := hmac.New(sha256.New, r.macKey)
	m.Write(b)
	return m.Sum(nil)[:refreshMACLen]
}

// Hash returns the hex SHA-256 used as the DB lookup key.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
