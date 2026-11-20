// Package dto defines HTTP request/response shapes and their mapping to
// and from domain/service types. Validation tags are applied by gin.
package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/response"
)

// PageQuery is embedded in every list query.
type PageQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1" example:"1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100" example:"20"`
	Sort     string `form:"sort" binding:"omitempty,max=64" example:"-created_at"`
}

func (q PageQuery) ToDomain() domain.PageRequest {
	return domain.PageRequest{Page: q.Page, PageSize: q.PageSize, Sort: q.Sort}
}

// mapPage converts a domain page to a response page with a mapper.
func mapPage[T, R any](p domain.PageResult[T], fn func(*T) R) response.Page[R] {
	items := make([]R, len(p.Items))
	for i := range p.Items {
		items[i] = fn(&p.Items[i])
	}
	return response.NewPage(items, p.Page, p.PageSize, p.Total)
}

func optEnum[T ~string](s string) *T {
	if s == "" {
		return nil
	}
	v := T(s)
	return &v
}

func optUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
