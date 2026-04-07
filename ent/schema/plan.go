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
		field.String("state_code").
			Optional().
			MaxLen(2),
		field.String("market_type").
			Optional(),
		field.String("metal_level").
			Optional(),
		field.Int("plan_year").
			Optional().
			Nillable(),
		field.Bool("is_active").
			Default(true),
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
		edge.From("insurer", Insurer.Type).
			Ref("plans").
			Unique(),
		edge.To("formulary_entries", FormularyEntry.Type),
		edge.To("saved_lookups_for_plan", SavedLookup.Type),
		edge.To("search_history_plans", SearchHistory.Type),
	}
}

// Indexes of the Plan.
func (Plan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contract_id", "plan_id", "segment_id").
			Unique(),
		index.Fields("formulary_id"),
		index.Fields("state_code"),
		index.Fields("is_active"),
	}
}
