package synctracker

import (
	"context"
	"fmt"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/syncmetadata"
)

// Tracker provides read/write access to sync metadata for data sources.
type Tracker struct {
	db *ent.Client
}

// New creates a new sync metadata tracker.
func New(db *ent.Client) *Tracker {
	return &Tracker{db: db}
}

// GetLastSync returns the last sync metadata for a source, or nil if never synced.
func (t *Tracker) GetLastSync(ctx context.Context, sourceName string) (*ent.SyncMetadata, error) {
	meta, err := t.db.SyncMetadata.Query().
		Where(syncmetadata.SourceName(sourceName)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query sync metadata for %s: %w", sourceName, err)
	}
	return meta, nil
}

// RecordSync creates or updates the sync metadata for a source.
func (t *Tracker) RecordSync(ctx context.Context, sourceName string, recordsProcessed int, etag, url string) error {
	existing, err := t.db.SyncMetadata.Query().
		Where(syncmetadata.SourceName(sourceName)).
		Only(ctx)

	if ent.IsNotFound(err) {
		builder := t.db.SyncMetadata.Create().
			SetSourceName(sourceName).
			SetLastSyncAt(time.Now()).
			SetRecordsProcessed(recordsProcessed)
		if etag != "" {
			builder = builder.SetLastEtag(etag)
		}
		if url != "" {
			builder = builder.SetLastURL(url)
		}
		_, err = builder.Save(ctx)
		return err
	}
	if err != nil {
		return fmt.Errorf("query sync metadata for %s: %w", sourceName, err)
	}

	updater := existing.Update().
		SetLastSyncAt(time.Now()).
		SetRecordsProcessed(recordsProcessed)
	if etag != "" {
		updater = updater.SetLastEtag(etag)
	}
	if url != "" {
		updater = updater.SetLastURL(url)
	}
	_, err = updater.Save(ctx)
	return err
}
