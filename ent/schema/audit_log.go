package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AuditLog is an append-only record of staff and system actions.
type AuditLog struct {
	ent.Schema
}

func (AuditLog) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		// Nil for system actions (e.g. bootstrap).
		field.UUID("actor_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.String("action").
			NotEmpty().
			MaxLen(64).
			Immutable(),
		field.String("entity_type").
			NotEmpty().
			MaxLen(32).
			Immutable(),
		field.UUID("entity_id", uuid.UUID{}).
			Immutable(),
		// Changed fields only; never contains password_hash.
		field.JSON("before", map[string]any{}).
			Optional().
			Immutable(),
		field.JSON("after", map[string]any{}).
			Optional().
			Immutable(),
		field.String("request_id").
			Optional().
			MaxLen(128).
			Immutable(),
	}
}

func (AuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entity_type", "entity_id"),
		index.Fields("actor_id"),
		index.Fields("created_at"),
	}
}
