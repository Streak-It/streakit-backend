// Package domain holds the core business types returned by services.
// It has no dependency on Ent, HTTP or any framework.
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
)

func (r Role) Valid() bool { return r == RoleSuperAdmin || r == RoleAdmin }

type UserStatus string

const (
	UserActive  UserStatus = "active"
	UserBlocked UserStatus = "blocked"
)

type User struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string // never serialised or audited
	Role         Role
	Status       UserStatus
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) IsActive() bool { return u.Status == UserActive }

// Actor is the authenticated caller performing an action.
type Actor struct {
	ID   uuid.UUID
	Role Role
}

func (a Actor) IsSuperAdmin() bool { return a.Role == RoleSuperAdmin }

// RefreshToken is a stored (hashed) refresh session.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}
