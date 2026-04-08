package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// SyncMetadata tracks the last sync state for each data source.
type SyncMetadata struct {
	ent.Schema
}

// Fields of the SyncMetadata.
func (SyncMetadata) Fields() []ent.Field {
	return []ent.Field{
		field.String("source_name").
			Unique().
			NotEmpty().
			Comment("Identifier: rxnorm, openfda, cms_puf, qhp"),
		field.Time("last_sync_at"),
		field.String("last_etag").
			Optional().
			Comment("HTTP ETag for change detection"),
		field.String("last_url").
			Optional().
			Comment("URL used for last sync"),
		field.Int("records_processed").
			Default(0),
	}
}
