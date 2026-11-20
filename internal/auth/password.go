// Package auth contains credential primitives: password hashing, access
// JWTs and opaque refresh tokens. It has no knowledge of HTTP or storage.
package auth

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordChars = 12
	// MaxPasswordBytes is bcrypt's input limit; longer input would be
	// silently truncated, so we reject it instead.
	MaxPasswordBytes = 72
)

var (
	ErrPasswordTooShort = fmt.Errorf("must be at least %d characters", MinPasswordChars)
	ErrPasswordTooLong  = fmt.Errorf("must be at most %d bytes", MaxPasswordBytes)
)

// ValidatePassword enforces the password policy.
func ValidatePassword(pw string) error {
	if len(pw) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	if utf8.RuneCountInString(pw) < MinPasswordChars {
		return ErrPasswordTooShort
	}
	return nil
}

// PasswordHasher hashes and verifies passwords with bcrypt.
type PasswordHasher struct {
	cost int
	// dummyHash is compared against when the user does not exist, so a
	// miss costs the same time as a wrong password.
	dummyHash []byte
}

// NewPasswordHasher creates a hasher. cost <= 0 uses bcrypt.DefaultCost.
func NewPasswordHasher(cost int) (*PasswordHasher, error) {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("streakit-dummy-password-for-timing"), cost)
	if err != nil {
		return nil, fmt.Errorf("generate dummy hash: %w", err)
	}
	return &PasswordHasher{cost: cost, dummyHash: dummy}, nil
}

func (h *PasswordHasher) Hash(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// Compare reports whether pw matches hash.
func (h *PasswordHasher) Compare(hash, pw string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}

// CompareDummy burns the same CPU as a real comparison and always fails.
func (h *PasswordHasher) CompareDummy(pw string) {
	_ = bcrypt.CompareHashAndPassword(h.dummyHash, []byte(pw))
}

// IsPolicyError reports whether err came from ValidatePassword.
func IsPolicyError(err error) bool {
	return errors.Is(err, ErrPasswordTooShort) || errors.Is(err, ErrPasswordTooLong)
}
