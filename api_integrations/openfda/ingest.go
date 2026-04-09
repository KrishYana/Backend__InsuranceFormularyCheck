package openfda

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/drugndcmap"
	"github.com/kyanaman/formularycheck/internal/synctracker"
)

const sourceName = "openfda"

// Ingestor downloads NDC data from openFDA and maps it to drugs in the database.
type Ingestor struct {
	db         *ent.Client
	downloader *BulkDownloader
	tracker    *synctracker.Tracker
}

// NewIngestor creates a new openFDA ingestor.
func NewIngestor(db *ent.Client) *Ingestor {
	return &Ingestor{
		db:         db,
		downloader: NewBulkDownloader(),
		tracker:    synctracker.New(db),
	}
}

// Run downloads the full NDC dataset and upserts NDC-to-drug mappings.
// On subsequent runs, it first checks the bulk metadata endpoint via HEAD
// request to see if the data has changed since the last sync.
func (ing *Ingestor) Run(ctx context.Context) error {
	// Check if data has changed since last sync
	lastSync, err := ing.tracker.GetLastSync(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("get last sync: %w", err)
	}

	lastModified, err := ing.downloader.CheckLastModified(ctx)
	if err != nil {
		slog.Warn("openFDA could not check last-modified, proceeding with full download", "error", err)
	} else if lastSync != nil && lastSync.LastEtag != "" {
		// Compare stored last-modified with current
		if lastModified == lastSync.LastEtag {
			slog.Info("openFDA bulk data unchanged since last sync, skipping download", "last_modified", lastModified)
			return nil
		}
		slog.Info("openFDA data changed, downloading", "previous", lastSync.LastEtag, "current", lastModified)
	}

	slog.Info("starting openFDA NDC bulk download")

	records, err := ing.downloader.FetchAllNDCRecords(ctx)
	if err != nil {
		return fmt.Errorf("bulk download: %w", err)
	}

	slog.Info("downloaded NDC records, mapping to drugs", "count", len(records))

	var mapped, updated, noRxCUI, noDrug, errors int

	for i, record := range records {
		result, err := ing.processRecord(ctx, record)
		if err != nil {
			switch {
			case isNoRxCUI(err):
				noRxCUI++
			case isNoDrug(err):
				noDrug++
			default:
				errors++
				if errors <= 10 {
					slog.Warn("openFDA record processing error", "error", err)
				}
			}
			continue
		}

		switch result {
		case processResultCreated:
			mapped++
		case processResultUpdated:
			updated++
		}

		if (i+1)%5000 == 0 {
			slog.Info("openFDA progress", "processed", i+1, "total", len(records), "mapped", mapped, "updated", updated, "no_rxcui", noRxCUI, "no_drug", noDrug, "errors", errors)
		}
	}

	slog.Info("openFDA ingestion done", "mapped", mapped, "updated", updated, "no_rxcui", noRxCUI, "no_drug", noDrug, "errors", errors, "total", len(records))

	// Record sync metadata
	etag := lastModified
	if err := ing.tracker.RecordSync(ctx, sourceName, mapped+updated, etag, bulkDownloadURL); err != nil {
		slog.Warn("openFDA failed to record sync metadata", "error", err)
	}

	return nil
}

type processResultType int

const (
	processResultCreated processResultType = iota
	processResultUpdated
)

func (ing *Ingestor) processRecord(ctx context.Context, record NDCRecord) (processResultType, error) {
	// Need at least one rxcui to map
	if len(record.OpenFDA.RxCUI) == 0 {
		return 0, errNoRxCUI
	}

	// Find matching drug by rxcui
	rxcui := record.OpenFDA.RxCUI[0]
	drugEnt, err := ing.db.Drug.Query().
		Where(drug.Rxcui(rxcui)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, errNoDrug
		}
		return 0, fmt.Errorf("query drug for rxcui %s: %w", rxcui, err)
	}

	// Determine manufacturer
	manufacturer := ""
	if len(record.OpenFDA.ManufacturerName) > 0 {
		manufacturer = record.OpenFDA.ManufacturerName[0]
	}

	// Parse dates
	startDate := parseOpenFDADate(record.MarketingStartDate)

	var anyCreated, anyUpdated bool

	// Process each package NDC
	for _, pkg := range record.Packaging {
		ndc := normalizeNDC(pkg.PackageNDC)
		if ndc == "" {
			continue
		}

		endDate := parseOpenFDADate(pkg.MarketingEndDate)
		status := "ACTIVE"
		if endDate != nil {
			status = "DISCONTINUED"
		}

		// Check if NDC already exists
		existing, err := ing.db.DrugNdcMap.Query().
			Where(drugndcmap.Ndc(ndc)).
			Only(ctx)

		if ent.IsNotFound(err) {
			// Create new mapping
			builder := ing.db.DrugNdcMap.Create().
				SetNdc(ndc).
				SetNdcStatus(status).
				SetManufacturer(manufacturer).
				SetPackageDescription(pkg.Description).
				SetDrug(drugEnt)

			if startDate != nil {
				builder = builder.SetStartDate(*startDate)
			}
			if endDate != nil {
				builder = builder.SetEndDate(*endDate)
			}

			if _, err := builder.Save(ctx); err != nil {
				if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
					continue
				}
				return 0, fmt.Errorf("create ndc mapping %s: %w", ndc, err)
			}
			anyCreated = true
		} else if err != nil {
			return 0, fmt.Errorf("query ndc %s: %w", ndc, err)
		} else {
			// Update existing mapping if data has changed
			needsUpdate := false
			updater := existing.Update()

			if existing.NdcStatus != status {
				updater = updater.SetNdcStatus(status)
				needsUpdate = true
			}
			if manufacturer != "" && existing.Manufacturer != manufacturer {
				updater = updater.SetManufacturer(manufacturer)
				needsUpdate = true
			}
			if pkg.Description != "" && existing.PackageDescription != pkg.Description {
				updater = updater.SetPackageDescription(pkg.Description)
				needsUpdate = true
			}
			if endDate != nil && (existing.EndDate == nil || !existing.EndDate.Equal(*endDate)) {
				updater = updater.SetEndDate(*endDate)
				needsUpdate = true
			}

			if needsUpdate {
				if _, err := updater.Save(ctx); err != nil {
					return 0, fmt.Errorf("update ndc mapping %s: %w", ndc, err)
				}
				anyUpdated = true
			}
		}
	}

	if anyCreated {
		return processResultCreated, nil
	}
	if anyUpdated {
		return processResultUpdated, nil
	}
	return processResultCreated, nil
}

// normalizeNDC converts a dashed NDC (e.g., "0069-0730-01") to an 11-digit format.
func normalizeNDC(ndc string) string {
	parts := strings.Split(ndc, "-")
	if len(parts) != 3 {
		return ""
	}

	// Pad to 5-4-2 format (11 digits total)
	labeler := fmt.Sprintf("%05s", parts[0])
	product := fmt.Sprintf("%04s", parts[1])
	pkg := fmt.Sprintf("%02s", parts[2])

	return labeler + product + pkg
}

func parseOpenFDADate(s string) *time.Time {
	if s == "" {
		return nil
	}
	// openFDA dates are YYYYMMDD
	t, err := time.Parse("20060102", s)
	if err != nil {
		return nil
	}
	return &t
}

var (
	errNoRxCUI = fmt.Errorf("no rxcui in openfda fields")
	errNoDrug  = fmt.Errorf("no matching drug in database")
)

func isNoRxCUI(err error) bool { return err == errNoRxCUI }
func isNoDrug(err error) bool  { return err == errNoDrug }
