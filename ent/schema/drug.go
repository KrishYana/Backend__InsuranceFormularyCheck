package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Drug holds the schema definition for the Drug entity.
type Drug struct {
	ent.Schema
}

// Fields of the Drug.
func (Drug) Fields() []ent.Field {
	return []ent.Field{
		field.String("rxcui").
			Unique().
			NotEmpty(),
		field.String("drug_name").
			NotEmpty(),
		field.String("generic_name").
			Optional(),
		field.Strings("brand_names").
			Optional(),
		field.String("dose_form").
			Optional(),
		field.String("strength").
			Optional(),
		field.String("route").
			Optional(),
		field.String("drug_class").
			Optional(),
		field.Bool("is_specialty").
			Default(false),
		field.Bool("is_controlled").
			Default(false),
		field.String("dea_schedule").
			Optional(),
		field.Time("last_rxnorm_sync").
			Default(time.Now),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Drug.
func (Drug) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("ndc_mappings", DrugNdcMap.Type),
		edge.To("formulary_entries", FormularyEntry.Type),
	}
}

// Indexes of the Drug.
func (Drug) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rxcui"),
		index.Fields("drug_name"),
	}
}
