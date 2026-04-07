package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

var npiRegex = regexp.MustCompile(`^\d{10}$`)

// Physician holds the schema for authenticated app users (physicians).
type Physician struct {
	ent.Schema
}

// Fields of the Physician.
func (Physician) Fields() []ent.Field {
	return []ent.Field{
		field.String("supabase_user_id").
			Unique().
			NotEmpty(),
		field.String("email").
			NotEmpty(),
		field.String("display_name").
			NotEmpty(),
		field.String("npi").
			Optional().
			MaxLen(10).
			Match(npiRegex),
		field.String("specialty").
			Optional(),
		field.String("primary_state").
			Optional().
			MaxLen(2),
		field.Bool("is_npi_verified").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Physician.
func (Physician) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("saved_lookups", SavedLookup.Type),
		edge.To("search_history", SearchHistory.Type),
	}
}

// Indexes of the Physician.
func (Physician) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email"),
	}
}
