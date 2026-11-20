package entrepo

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/event"
	"github.com/niteshs1/streakit-api/ent/lineupentry"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const eventEntity = "Event"

type EventRepo struct{ base }

func NewEventRepo(c *ent.Client) *EventRepo { return &EventRepo{base{c}} }

var _ repository.EventRepository = (*EventRepo)(nil)

func toEvent(e *ent.Event) *domain.Event {
	out := &domain.Event{
		ID:               e.ID,
		VenueID:          e.VenueID,
		Title:            e.Title,
		Description:      e.Description,
		DoorsOpenAt:      e.DoorsOpenAt,
		ScheduledStartAt: e.ScheduledStartAt,
		ExpectedEndAt:    e.ExpectedEndAt,
		TicketURL:        e.TicketURL,
		PublishState:     domain.PublishState(e.PublishState),
		Status:           domain.EventStatus(e.Status),
		Version:          e.Version,
		CreatedBy:        e.CreatedBy,
		UpdatedBy:        e.UpdatedBy,
		CreatedAt:        e.CreatedAt,
		UpdatedAt:        e.UpdatedAt,
	}
	if e.Edges.Lineup != nil {
		out.Lineup = toLineup(e.Edges.Lineup)
	}
	return out
}

func toLineup(rows []*ent.LineupEntry) []domain.LineupEntry {
	out := make([]domain.LineupEntry, len(rows))
	for i, l := range rows {
		out[i] = domain.LineupEntry{
			ID:        l.ID,
			EventID:   l.EventID,
			ArtistID:  l.ArtistID,
			Billing:   domain.Billing(l.Billing),
			SortOrder: l.SortOrder,
		}
		if l.Edges.Artist != nil {
			out[i].Artist = toArtist(l.Edges.Artist)
		}
	}
	return out
}

func withLineup(q *ent.LineupEntryQuery) {
	q.WithArtist().Order(lineupentry.BySortOrder(), lineupentry.ByID())
}

func (r *EventRepo) Create(ctx context.Context, e *domain.Event) (*domain.Event, error) {
	row, err := r.db(ctx).Event.Create().
		SetVenueID(e.VenueID).
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetNillableDoorsOpenAt(utcPtr(e.DoorsOpenAt)).
		SetNillableScheduledStartAt(utcPtr(e.ScheduledStartAt)).
		SetNillableExpectedEndAt(utcPtr(e.ExpectedEndAt)).
		SetTicketURL(e.TicketURL).
		SetPublishState(event.PublishState(e.PublishState)).
		SetStatus(event.Status(e.Status)).
		SetCreatedBy(e.CreatedBy).
		SetUpdatedBy(e.UpdatedBy).
		Save(ctx)
	if err != nil {
		return nil, mapErr(err, eventEntity, "")
	}
	out := toEvent(row)
	out.Lineup = []domain.LineupEntry{}
	return out, nil
}

func (r *EventRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	row, err := r.db(ctx).Event.Query().Where(event.ID(id)).WithLineup(withLineup).Only(ctx)
	if err != nil {
		return nil, mapErr(err, eventEntity, "")
	}
	return toEvent(row), nil
}

func (r *EventRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	row, err := r.db(ctx).Event.Query().Where(event.ID(id)).ForUpdate().WithLineup(withLineup).Only(ctx)
	if err != nil {
		return nil, mapErr(err, eventEntity, "")
	}
	return toEvent(row), nil
}

func (r *EventRepo) List(ctx context.Context, f repository.EventFilter) (domain.PageResult[domain.Event], error) {
	q := r.db(ctx).Event.Query()
	if f.PublishState != nil {
		q = q.Where(event.PublishStateEQ(event.PublishState(*f.PublishState)))
	}
	if f.Status != nil {
		q = q.Where(event.StatusEQ(event.Status(*f.Status)))
	}
	if f.VenueID != nil {
		q = q.Where(event.VenueID(*f.VenueID))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return domain.PageResult[domain.Event]{}, mapErr(err, eventEntity, "")
	}

	field, desc := f.SortField()
	rows, err := q.Order(order[event.OrderOption](field, desc)...).
		Offset(f.Offset()).Limit(f.PageSize).All(ctx)
	if err != nil {
		return domain.PageResult[domain.Event]{}, mapErr(err, eventEntity, "")
	}

	items := make([]domain.Event, len(rows))
	for i, row := range rows {
		items[i] = *toEvent(row)
	}
	return domain.PageResult[domain.Event]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *EventRepo) Update(ctx context.Context, id uuid.UUID, expectedVersion int, u repository.EventUpdate) (*domain.Event, error) {
	m := r.db(ctx).Event.UpdateOneID(id).
		Where(event.Version(expectedVersion)).
		AddVersion(1).
		SetUpdatedBy(u.UpdatedBy)
	if u.VenueID != nil {
		m.SetVenueID(*u.VenueID)
	}
	if u.Title != nil {
		m.SetTitle(*u.Title)
	}
	if u.Description != nil {
		m.SetDescription(*u.Description)
	}
	if u.TicketURL != nil {
		m.SetTicketURL(*u.TicketURL)
	}
	if u.PublishState != nil {
		m.SetPublishState(event.PublishState(*u.PublishState))
	}
	if u.Status != nil {
		m.SetStatus(event.Status(*u.Status))
	}
	if u.DoorsOpenAt != nil {
		if t := *u.DoorsOpenAt; t != nil {
			m.SetDoorsOpenAt(t.UTC())
		} else {
			m.ClearDoorsOpenAt()
		}
	}
	if u.ScheduledStartAt != nil {
		if t := *u.ScheduledStartAt; t != nil {
			m.SetScheduledStartAt(t.UTC())
		} else {
			m.ClearScheduledStartAt()
		}
	}
	if u.ExpectedEndAt != nil {
		if t := *u.ExpectedEndAt; t != nil {
			m.SetExpectedEndAt(t.UTC())
		} else {
			m.ClearExpectedEndAt()
		}
	}

	if _, err := m.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			if exists, exErr := r.db(ctx).Event.Query().Where(event.ID(id)).Exist(ctx); exErr == nil && exists {
				return nil, apperror.Conflict("Event was modified by someone else; reload and retry")
			}
		}
		return nil, mapErr(err, eventEntity, "")
	}
	return r.GetByID(ctx, id)
}

func (r *EventRepo) ReplaceLineup(ctx context.Context, eventID uuid.UUID, entries []repository.LineupInput) ([]domain.LineupEntry, error) {
	db := r.db(ctx)
	if _, err := db.LineupEntry.Delete().Where(lineupentry.EventID(eventID)).Exec(ctx); err != nil {
		return nil, mapErr(err, "Lineup", "")
	}
	if len(entries) > 0 {
		builders := make([]*ent.LineupEntryCreate, len(entries))
		for i, e := range entries {
			builders[i] = db.LineupEntry.Create().
				SetEventID(eventID).
				SetArtistID(e.ArtistID).
				SetBilling(lineupentry.Billing(e.Billing)).
				SetSortOrder(e.SortOrder)
		}
		if _, err := db.LineupEntry.CreateBulk(builders...).Save(ctx); err != nil {
			return nil, mapErr(err, "Lineup", "Lineup contains a duplicate artist")
		}
	}
	rows, err := db.LineupEntry.Query().Where(lineupentry.EventID(eventID)).WithArtist().
		Order(lineupentry.BySortOrder(), lineupentry.ByID()).All(ctx)
	if err != nil {
		return nil, mapErr(err, "Lineup", "")
	}
	return toLineup(rows), nil
}
