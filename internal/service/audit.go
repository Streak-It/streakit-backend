package service

import (
	"context"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
	"github.com/niteshs1/streakit-api/internal/reqctx"
)

// Audit actions.
const (
	ActionAuthLogin          = "auth.login"
	ActionAuthPasswordChange = "auth.password_change"
	ActionUserBootstrap      = "user.bootstrap"
	ActionUserCreate         = "user.create"
	ActionUserUpdate         = "user.update"
	ActionUserBlock          = "user.block"
	ActionUserUnblock        = "user.unblock"
	ActionUserRoleChange     = "user.role_change"
	ActionVenueCreate        = "venue.create"
	ActionVenueUpdate        = "venue.update"
	ActionVenueRetire        = "venue.retire"
	ActionVenueRestore       = "venue.restore"
	ActionArtistCreate       = "artist.create"
	ActionArtistUpdate       = "artist.update"
	ActionArtistHide         = "artist.hide"
	ActionArtistUnhide       = "artist.unhide"
	ActionEventCreate        = "event.create"
	ActionEventUpdate        = "event.update"
	ActionEventLineup        = "event.lineup_replace"
	ActionEventPublish       = "event.publish"
	ActionEventUnpublish     = "event.unpublish"
	ActionEventCancel        = "event.cancel"
	ActionEventPostpone      = "event.postpone"
	ActionEventRestore       = "event.restore"
)

// auditor writes audit rows. Call it with the transactional ctx so the
// entry commits or rolls back together with the change it describes.
type auditor struct {
	repo repository.AuditRepository
}

func (a auditor) record(ctx context.Context, actorID *uuid.UUID, action, entityType string, entityID uuid.UUID, before, after map[string]any) error {
	before, after = diff(before, after)
	return a.repo.Create(ctx, &domain.AuditEntry{
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Before:     before,
		After:      after,
		RequestID:  reqctx.RequestID(ctx),
	})
}

// diff keeps only keys whose values differ. With a nil before (create),
// after is returned whole; with a nil after, before is returned whole.
func diff(before, after map[string]any) (map[string]any, map[string]any) {
	if before == nil || after == nil {
		return before, after
	}
	b, a := map[string]any{}, map[string]any{}
	for k, av := range after {
		if bv, ok := before[k]; !ok || !reflect.DeepEqual(bv, av) {
			b[k], a[k] = before[k], av
		}
	}
	for k, bv := range before {
		if _, ok := after[k]; !ok {
			b[k] = bv
		}
	}
	return b, a
}

// Snapshots: the audited fields of each entity. password_hash is never included.

func userSnapshot(u *domain.User) map[string]any {
	return map[string]any{
		"name":   u.Name,
		"email":  u.Email,
		"role":   string(u.Role),
		"status": string(u.Status),
	}
}

func venueSnapshot(v *domain.Venue) map[string]any {
	return map[string]any{
		"name":     v.Name,
		"city":     v.City,
		"address":  v.Address,
		"info":     v.Info,
		"timezone": v.Timezone,
		"status":   string(v.Status),
	}
}

func artistSnapshot(a *domain.Artist) map[string]any {
	return map[string]any{
		"name":   a.Name,
		"slug":   a.Slug,
		"status": string(a.Status),
	}
}

func eventSnapshot(e *domain.Event) map[string]any {
	return map[string]any{
		"venue_id":           e.VenueID.String(),
		"title":              e.Title,
		"description":        e.Description,
		"doors_open_at":      timeValue(e.DoorsOpenAt),
		"scheduled_start_at": timeValue(e.ScheduledStartAt),
		"expected_end_at":    timeValue(e.ExpectedEndAt),
		"ticket_url":         e.TicketURL,
		"publish_state":      string(e.PublishState),
		"status":             string(e.Status),
	}
}

func lineupSnapshot(entries []domain.LineupEntry) map[string]any {
	items := make([]any, len(entries))
	for i, e := range entries {
		items[i] = map[string]any{
			"artist_id":  e.ArtistID.String(),
			"billing":    string(e.Billing),
			"sort_order": e.SortOrder,
		}
	}
	return map[string]any{"lineup": items}
}

// timeValue normalises times so DeepEqual compares instants, not
// locations or monotonic readings.
func timeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// ---- AuditService ----

type AuditService interface {
	List(ctx context.Context, f repository.AuditFilter) (domain.PageResult[domain.AuditEntry], error)
}

type auditService struct {
	repo repository.AuditRepository
}

func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

var auditSorts = []string{"created_at"}

func (s *auditService) List(ctx context.Context, f repository.AuditFilter) (domain.PageResult[domain.AuditEntry], error) {
	page, err := normalizePage(f.PageRequest, auditSorts, "-created_at")
	if err != nil {
		return domain.PageResult[domain.AuditEntry]{}, err
	}
	f.PageRequest = page
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return domain.PageResult[domain.AuditEntry]{}, apperror.Field("from", "must be before to")
	}
	res, err := s.repo.List(ctx, f)
	return toPage(res), err
}
