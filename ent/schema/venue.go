package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Venue is a physical location that hosts events.
type Venue struct {
	ent.Schema
}

func (Venue) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (Venue) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(200),
		field.String("city").
			NotEmpty().
			MaxLen(100),
		field.String("address").
			Optional().
			MaxLen(500),
		field.Text("info").
			Optional(),
		// IANA timezone name, used for display only.
		field.String("timezone").
			NotEmpty().
			MaxLen(64).
			Default("Asia/Kolkata"),
		field.Enum("status").
			Values("active", "retired").
			Default("active"),
		// Optimistic-lock counter; incremented on every write.
		field.Int("version").
			Default(1).
			Positive(),
	}
}

func (Venue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("events", Event.Type),
	}
}

func (Venue) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("city"),
		index.Fields("status"),
	}
}
