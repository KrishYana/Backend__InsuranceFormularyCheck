package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FormularyEntry holds the schema for drug coverage per plan.
type FormularyEntry struct {
	ent.Schema
}

// Fields of the FormularyEntry.
func (FormularyEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int("tier_level").
			Optional(),
		field.String("tier_name").
			Optional(),
		field.Bool("prior_auth_required").
			Default(false),
		field.Bool("step_therapy").
			Default(false),
		field.Bool("quantity_limit").
			Default(false),
		field.Float("quantity_limit_amount").
			Optional().
			Nillable(),
		field.Int("quantity_limit_days").
			Optional().
			Nillable(),
		field.Float("copay_amount").
			Optional().
			Nillable(),
		field.Float("coinsurance_pct").
			Optional().
			Nillable(),
		field.Float("copay_mail_order").
			Optional().
			Nillable(),
		field.Bool("is_covered").
			Default(true),
		field.Bool("specialty_drug").
			Default(false),
		field.String("quantity_limit_detail").
			Optional(),
		field.String("source_type").
			NotEmpty(),
		field.Time("source_date"),
		field.Bool("is_current").
			Default(true),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the FormularyEntry.
func (FormularyEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", Plan.Type).
			Ref("formulary_entries").
			Unique().
			Required(),
		edge.From("drug", Drug.Type).
			Ref("formulary_entries").
			Unique().
			Required(),
		edge.To("prior_auth_criteria", PriorAuthCriteria.Type),
	}
}

// Indexes of the FormularyEntry.
func (FormularyEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_current"),
		index.Fields("is_covered"),
		index.Fields("source_type"),
		// Prevent duplicate entries: one entry per plan+drug+source
		index.Fields("source_type").
			Edges("plan", "drug").
			Unique(),
	}
}
