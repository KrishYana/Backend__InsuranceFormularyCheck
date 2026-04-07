package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DrugAlternative holds the schema for drug-to-drug alternative relationships.
type DrugAlternative struct {
	ent.Schema
}

// Fields of the DrugAlternative.
func (DrugAlternative) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("relationship_type").
			Values("GENERIC_EQUIVALENT", "THERAPEUTIC_ALTERNATIVE", "BIOSIMILAR"),
		field.String("source").
			Optional(),
		field.String("notes").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the DrugAlternative.
func (DrugAlternative) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("drug", Drug.Type).
			Unique().
			Required(),
		edge.To("alternative_drug", Drug.Type).
			Unique().
			Required(),
	}
}

// Indexes of the DrugAlternative.
func (DrugAlternative) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("relationship_type"),
	}
}
