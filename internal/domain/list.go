package domain

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// PageRequest is a normalised pagination + sort request.
type PageRequest struct {
	Page     int
	PageSize int
	// Sort is a field name, optionally prefixed with "-" for descending.
	Sort string
}

// Normalize applies defaults and bounds.
func (p PageRequest) Normalize() PageRequest {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	return p
}

func (p PageRequest) Offset() int { return (p.Page - 1) * p.PageSize }

// SortField splits Sort into field and direction.
func (p PageRequest) SortField() (field string, desc bool) {
	if len(p.Sort) > 0 && p.Sort[0] == '-' {
		return p.Sort[1:], true
	}
	return p.Sort, false
}

// PageResult is a page of items plus the total match count.
type PageResult[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int
}
