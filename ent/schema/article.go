package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Article holds the schema for medical news articles (Discover tab).
type Article struct {
	ent.Schema
}

// Fields of the Article.
func (Article) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			NotEmpty(),
		field.String("summary").
			Optional(),
		field.String("source_name").
			NotEmpty(),
		field.String("source_url").
			NotEmpty(),
		field.Time("published_at"),
		field.Strings("drug_classes").
			Optional(),
		field.String("image_url").
			Optional(),
		field.Bool("is_active").
			Default(true),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Indexes of the Article.
func (Article) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("published_at"),
		index.Fields("is_active"),
	}
}
