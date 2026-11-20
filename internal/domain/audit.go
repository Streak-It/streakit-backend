package domain

import (
	"time"

	"github.com/google/uuid"
)

// Entity types used in the audit log.
const (
	EntityUser   = "user"
	EntityVenue  = "venue"
	EntityArtist = "artist"
	EntityEvent  = "event"
)

type AuditEntry struct {
	ID         uuid.UUID
	ActorID    *uuid.UUID // nil for system actions
	Action     string
	EntityType string
	EntityID   uuid.UUID
	Before     map[string]any
	After      map[string]any
	RequestID  string
	CreatedAt  time.Time
}
