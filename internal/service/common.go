// Package service implements business rules. Services depend only on
// repository interfaces and domain types, never on Ent or HTTP.
package service

import (
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/domain"
)

// Clock returns the current time; injectable for tests.
type Clock func() time.Time

func systemClock() time.Time { return time.Now().UTC() }

func clockOrDefault(c Clock) Clock {
	if c == nil {
		return systemClock
	}
	return c
}

// Limiter is a rate limiter keyed by an arbitrary string.
type Limiter interface {
	Allow(key string) bool
}

func requireSuperAdmin(a domain.Actor) error {
	if !a.IsSuperAdmin() {
		return apperror.Forbidden("Only a super admin can perform this action")
	}
	return nil
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return apperror.Field("email", "must be a valid email address")
	}
	return nil
}

// requiredText trims s and returns a field error if empty.
func requiredText(field, s string) (string, *apperror.FieldError) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", &apperror.FieldError{Field: field, Message: "required"}
	}
	return s, nil
}

// fieldErrors collects field errors and returns a validation error if any.
type fieldErrors []apperror.FieldError

func (f *fieldErrors) add(fe *apperror.FieldError) {
	if fe != nil {
		*f = append(*f, *fe)
	}
}

func (f *fieldErrors) addf(field, msg string, args ...any) {
	*f = append(*f, apperror.FieldError{Field: field, Message: fmt.Sprintf(msg, args...)})
}

func (f fieldErrors) err() error {
	if len(f) == 0 {
		return nil
	}
	return apperror.Validation("Validation failed", f...)
}

// normalizePage applies paging defaults and checks the sort key against an
// allow-list (which also keeps raw input out of ORDER BY).
func normalizePage(p domain.PageRequest, allowed []string, defaultSort string) (domain.PageRequest, error) {
	p = p.Normalize()
	if p.Sort == "" {
		p.Sort = defaultSort
	}
	field, _ := p.SortField()
	if !slices.Contains(allowed, field) {
		return p, apperror.Field("sort", "must be one of: "+strings.Join(allowed, ", ")+" (prefix with - for descending)")
	}
	return p, nil
}

func ptr[T any](v T) *T { return &v }

// toPage converts a repository page into a domain page (identity helper
// that keeps service signatures uniform).
func toPage[T any](r domain.PageResult[T]) domain.PageResult[T] {
	if r.Items == nil {
		r.Items = []T{}
	}
	return r
}
