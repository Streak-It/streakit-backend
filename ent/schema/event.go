package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Event is a scheduled show at a venue.
type Event struct {
	ent.Schema
}

func (Event) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (Event) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("venue_id", uuid.UUID{}),
		field.String("title").
			Optional().
			MaxLen(200),
		field.Text("description").
			Optional(),
		field.Time("doors_open_at").
			Optional().
			Nillable().
			SchemaType(timestamptz),
		field.Time("scheduled_start_at").
			Optional().
			Nillable().
			SchemaType(timestamptz),
		field.Time("expected_end_at").
			Optional().
			Nillable().
			SchemaType(timestamptz),
		field.String("ticket_url").
			Optional().
			MaxLen(2048),
		field.Enum("publish_state").
			Values("draft", "published").
			Default("draft"),
		field.Enum("status").
			Values("scheduled", "cancelled", "postponed").
			Default("scheduled"),
		// Optimistic-lock counter; incremented on every write.
		field.Int("version").
			Default(1).
			Positive(),
		field.UUID("created_by", uuid.UUID{}).
			Immutable(),
		field.UUID("updated_by", uuid.UUID{}),
	}
}

func (Event) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("venue", Venue.Type).
			Ref("events").
			Field("venue_id").
			Unique().
			Required(),
		edge.To("lineup", LineupEntry.Type),
	}
}

func (Event) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("venue_id"),
		index.Fields("publish_state", "status"),
		index.Fields("scheduled_start_at"),
	}
}
