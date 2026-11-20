package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User is a staff member (super admin or admin).
type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100),
		// Stored trimmed + lower-cased by the service layer.
		field.String("email").
			NotEmpty().
			MaxLen(255).
			Unique(),
		field.String("password_hash").
			NotEmpty().
			Sensitive(),
		field.Enum("role").
			Values("super_admin", "admin").
			Default("admin"),
		field.Enum("status").
			Values("active", "blocked").
			Default("active"),
		field.Time("last_login_at").
			Optional().
			Nillable().
			SchemaType(timestamptz),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("refresh_tokens", RefreshToken.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role", "status"),
	}
}
