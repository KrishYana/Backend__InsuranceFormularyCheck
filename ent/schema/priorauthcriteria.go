package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PriorAuthCriteria holds the schema for PA requirement details.
type PriorAuthCriteria struct {
	ent.Schema
}

// Fields of the PriorAuthCriteria.
func (PriorAuthCriteria) Fields() []ent.Field {
	return []ent.Field{
		field.String("criteria_type").
			NotEmpty(),
		field.String("criteria_description").
			Optional(),
		field.Strings("required_diagnoses").
			Optional(),
		field.Int("age_min").
			Optional().
			Nillable(),
		field.Int("age_max").
			Optional().
			Nillable(),
		field.String("gender_restriction").
			Optional(),
		field.Strings("step_therapy_drugs").
			Optional(),
		field.String("step_therapy_description").
			Optional(),
		field.Int("max_quantity").
			Optional().
			Nillable(),
		field.Int("quantity_period_days").
			Optional().
			Nillable(),
		field.String("source_document_url").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the PriorAuthCriteria.
func (PriorAuthCriteria) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("formulary_entry", FormularyEntry.Type).
			Ref("prior_auth_criteria").
			Unique().
			Required(),
	}
}

// Indexes of the PriorAuthCriteria.
func (PriorAuthCriteria) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("criteria_type"),
	}
}
