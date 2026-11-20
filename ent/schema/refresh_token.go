package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// RefreshToken stores only the SHA-256 hash of an opaque refresh token.
type RefreshToken struct {
	ent.Schema
}

func (RefreshToken) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).
			Immutable(),
		field.String("token_hash").
			NotEmpty().
			Unique().
			Immutable().
			Sensitive(),
		field.Time("expires_at").
			Immutable().
			SchemaType(timestamptz),
		field.Time("revoked_at").
			Optional().
			Nillable().
			SchemaType(timestamptz),
	}
}

func (RefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("refresh_tokens").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (RefreshToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
