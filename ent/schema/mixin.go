package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
)

// BaseMixin gives every table a UUID primary key and UTC timestamps.
type BaseMixin struct {
	mixin.Schema
}

func nowUTC() time.Time { return time.Now().UTC() }

func (BaseMixin) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.Time("created_at").
			Default(nowUTC).
			Immutable().
			SchemaType(timestamptz),
		field.Time("updated_at").
			Default(nowUTC).
			UpdateDefault(nowUTC).
			SchemaType(timestamptz),
	}
}

// timestamptz forces Postgres "timestamp with time zone" for time fields.
var timestamptz = map[string]string{"postgres": "timestamptz"}
