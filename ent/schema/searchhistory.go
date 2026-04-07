package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SearchHistory holds the schema for a physician's search log entry.
type SearchHistory struct {
	ent.Schema
}

// Fields of the SearchHistory.
func (SearchHistory) Fields() []ent.Field {
	return []ent.Field{
		field.String("state_code").
			Optional().
			MaxLen(2),
		field.String("search_text").
			Optional(),
		field.Int("results_count").
			Optional().
			Nillable(),
		field.Time("searched_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the SearchHistory.
func (SearchHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("physician", Physician.Type).
			Ref("search_history").
			Unique().
			Required(),
		edge.From("plan", Plan.Type).
			Ref("search_history_plans").
			Unique(),
		edge.From("drug", Drug.Type).
			Ref("search_history_drugs").
			Unique(),
	}
}

// Indexes of the SearchHistory.
func (SearchHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("searched_at"),
	}
}
