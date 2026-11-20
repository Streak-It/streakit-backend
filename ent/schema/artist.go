package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Artist is a performer that can appear in event lineups.
type Artist struct {
	ent.Schema
}

func (Artist) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (Artist) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(200),
		// Derived from name by the service; unique.
		field.String("slug").
			NotEmpty().
			MaxLen(220).
			Unique(),
		field.Enum("status").
			Values("active", "hidden").
			Default("active"),
	}
}

func (Artist) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("lineup_entries", LineupEntry.Type),
	}
}

func (Artist) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
	}
}
