package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SavedLookup holds the schema for a physician's bookmarked plan+drug lookup.
type SavedLookup struct {
	ent.Schema
}

// Fields of the SavedLookup.
func (SavedLookup) Fields() []ent.Field {
	return []ent.Field{
		field.String("nickname").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the SavedLookup.
func (SavedLookup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("physician", Physician.Type).
			Ref("saved_lookups").
			Unique().
			Required(),
		edge.From("plan", Plan.Type).
			Ref("saved_lookups_for_plan").
			Unique().
			Required(),
		edge.From("drug", Drug.Type).
			Ref("saved_lookups_for_drug").
			Unique().
			Required(),
	}
}

// Indexes of the SavedLookup.
func (SavedLookup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
	}
}
