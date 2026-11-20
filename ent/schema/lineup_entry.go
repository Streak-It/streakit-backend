package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// LineupEntry links an artist to an event with a billing position.
type LineupEntry struct {
	ent.Schema
}

func (LineupEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (LineupEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("event_id", uuid.UUID{}),
		field.UUID("artist_id", uuid.UUID{}),
		field.Enum("billing").
			Values("headliner", "support"),
		field.Int("sort_order").
			Default(0).
			NonNegative(),
	}
}

func (LineupEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("event", Event.Type).
			Ref("lineup").
			Field("event_id").
			Unique().
			Required(),
		edge.From("artist", Artist.Type).
			Ref("lineup_entries").
			Field("artist_id").
			Unique().
			Required(),
	}
}

func (LineupEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_id", "artist_id").Unique(),
		index.Fields("artist_id"),
	}
}
