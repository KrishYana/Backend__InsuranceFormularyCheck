package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Insurer holds the schema definition for an insurance company.
type Insurer struct {
	ent.Schema
}

// Fields of the Insurer.
func (Insurer) Fields() []ent.Field {
	return []ent.Field{
		field.String("insurer_name").
			NotEmpty(),
		field.String("parent_company").
			Optional(),
		field.String("hios_issuer_id").
			Optional().
			Unique(),
		field.String("fhir_endpoint_url").
			Optional(),
		field.String("website_url").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Insurer.
func (Insurer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("plans", Plan.Type),
	}
}

// Indexes of the Insurer.
func (Insurer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("insurer_name"),
	}
}
