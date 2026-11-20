package domain

import (
	"time"

	"github.com/google/uuid"
)

type VenueStatus string

const (
	VenueActive  VenueStatus = "active"
	VenueRetired VenueStatus = "retired"
)

type Venue struct {
	ID        uuid.UUID
	Name      string
	City      string
	Address   string
	Info      string
	Timezone  string
	Status    VenueStatus
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ArtistStatus string

const (
	ArtistActive ArtistStatus = "active"
	ArtistHidden ArtistStatus = "hidden"
)

type Artist struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Status    ArtistStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PublishState string

const (
	Draft     PublishState = "draft"
	Published PublishState = "published"
)

type EventStatus string

const (
	EventScheduled EventStatus = "scheduled"
	EventCancelled EventStatus = "cancelled"
	// EventPostponed is reserved for "new date unknown"; unused in this slice.
	EventPostponed EventStatus = "postponed"
)

type Event struct {
	ID               uuid.UUID
	VenueID          uuid.UUID
	Title            string
	Description      string
	DoorsOpenAt      *time.Time
	ScheduledStartAt *time.Time
	ExpectedEndAt    *time.Time
	TicketURL        string
	PublishState     PublishState
	Status           EventStatus
	Version          int
	CreatedBy        uuid.UUID
	UpdatedBy        uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// Lineup is populated by detail reads, nil on list reads.
	Lineup []LineupEntry
}

type Billing string

const (
	Headliner Billing = "headliner"
	Support   Billing = "support"
)

func (b Billing) Valid() bool { return b == Headliner || b == Support }

type LineupEntry struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	ArtistID  uuid.UUID
	Billing   Billing
	SortOrder int
	// Artist is populated on reads for display.
	Artist *Artist
}

// HasHeadliner reports whether at least one entry is a headliner.
func HasHeadliner(entries []LineupEntry) bool {
	for _, e := range entries {
		if e.Billing == Headliner {
			return true
		}
	}
	return false
}
