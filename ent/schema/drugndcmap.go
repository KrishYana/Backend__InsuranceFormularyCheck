package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DrugNdcMap holds the schema definition for the NDC-to-Drug mapping entity.
type DrugNdcMap struct {
	ent.Schema
}

// Fields of the DrugNdcMap.
func (DrugNdcMap) Fields() []ent.Field {
	return []ent.Field{
		field.String("ndc").
			Unique().
			NotEmpty(),
		field.String("ndc_status").
			NotEmpty(),
		field.String("manufacturer").
			Optional(),
		field.String("package_description").
			Optional(),
		field.Time("start_date").
			Optional(),
		field.Time("end_date").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the DrugNdcMap.
func (DrugNdcMap) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("drug", Drug.Type).
			Ref("ndc_mappings").
			Unique().
			Required(),
	}
}

// Indexes of the DrugNdcMap.
func (DrugNdcMap) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ndc"),
	}
}
