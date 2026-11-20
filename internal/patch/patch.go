// Package patch provides a tri-state field for JSON PATCH bodies:
// absent (leave unchanged), null (clear), or a value (set).
package patch

import (
	"bytes"
	"encoding/json"
)

type Field[T any] struct {
	// Set is true when the key was present in the JSON body.
	Set bool
	// Null is true when the key was present with a JSON null.
	Null  bool
	Value T
}

// Of returns a set, non-null field.
func Of[T any](v T) Field[T] { return Field[T]{Set: true, Value: v} }

// Null returns a set, null field.
func Null[T any]() Field[T] { return Field[T]{Set: true, Null: true} }

// UnmarshalJSON is only called when the key is present.
func (f *Field[T]) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Null = true
		var zero T
		f.Value = zero
		return nil
	}
	f.Null = false
	return json.Unmarshal(data, &f.Value)
}

// Ptr returns nil when null, or a pointer to the value. Callers must check Set.
func (f Field[T]) Ptr() *T {
	if f.Null {
		return nil
	}
	v := f.Value
	return &v
}
