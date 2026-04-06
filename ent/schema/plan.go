package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Plan holds the schema definition for an insurance plan.
type Plan struct {
	ent.Schema
}

// Fields of the Plan.
func (Plan) Fields() []ent.Field {
	return []ent.Field{
		field.String("contract_id").
			NotEmpty(),
		field.String("plan_id").
			NotEmpty(),
		field.String("segment_id").
			NotEmpty(),
		field.String("contract_name").
			Optional(),
		field.String("plan_name").
			Optional(),
		field.String("formulary_id").
			NotEmpty(),
		field.String("plan_type").
			Optional(),
		field.String("snp_type").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Plan.
func (Plan) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("formulary_entries", FormularyEntry.Type),
	}
}

// Indexes of the Plan.
func (Plan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contract_id", "plan_id", "segment_id").
			Unique(),
		index.Fields("formulary_id"),
	}
}
