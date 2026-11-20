package entrepo

import (
	"context"

	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/auditlog"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const auditEntity = "Audit entry"

type AuditRepo struct{ base }

func NewAuditRepo(c *ent.Client) *AuditRepo { return &AuditRepo{base{c}} }

var _ repository.AuditRepository = (*AuditRepo)(nil)

func (r *AuditRepo) Create(ctx context.Context, e *domain.AuditEntry) error {
	_, err := r.db(ctx).AuditLog.Create().
		SetNillableActorID(e.ActorID).
		SetAction(e.Action).
		SetEntityType(e.EntityType).
		SetEntityID(e.EntityID).
		SetBefore(e.Before).
		SetAfter(e.After).
		SetRequestID(e.RequestID).
		Save(ctx)
	return mapErr(err, auditEntity, "")
}

func (r *AuditRepo) List(ctx context.Context, f repository.AuditFilter) (domain.PageResult[domain.AuditEntry], error) {
	q := r.db(ctx).AuditLog.Query()
	if f.EntityType != "" {
		q = q.Where(auditlog.EntityType(f.EntityType))
	}
	if f.EntityID != nil {
		q = q.Where(auditlog.EntityID(*f.EntityID))
	}
	if f.ActorID != nil {
		q = q.Where(auditlog.ActorID(*f.ActorID))
	}
	if f.From != nil {
		q = q.Where(auditlog.CreatedAtGTE(*f.From))
	}
	if f.To != nil {
		q = q.Where(auditlog.CreatedAtLTE(*f.To))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return domain.PageResult[domain.AuditEntry]{}, mapErr(err, auditEntity, "")
	}

	field, desc := f.SortField()
	rows, err := q.Order(order[auditlog.OrderOption](field, desc)...).
		Offset(f.Offset()).Limit(f.PageSize).All(ctx)
	if err != nil {
		return domain.PageResult[domain.AuditEntry]{}, mapErr(err, auditEntity, "")
	}

	items := make([]domain.AuditEntry, len(rows))
	for i, row := range rows {
		items[i] = domain.AuditEntry{
			ID:         row.ID,
			ActorID:    row.ActorID,
			Action:     row.Action,
			EntityType: row.EntityType,
			EntityID:   row.EntityID,
			Before:     row.Before,
			After:      row.After,
			RequestID:  row.RequestID,
			CreatedAt:  row.CreatedAt,
		}
	}
	return domain.PageResult[domain.AuditEntry]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}
