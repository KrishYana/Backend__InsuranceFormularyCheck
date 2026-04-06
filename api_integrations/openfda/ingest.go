package openfda

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/drugndcmap"
)

// Ingestor downloads NDC data from openFDA and maps it to drugs in the database.
type Ingestor struct {
	db         *ent.Client
	downloader *BulkDownloader
}

// NewIngestor creates a new openFDA ingestor.
func NewIngestor(db *ent.Client) *Ingestor {
	return &Ingestor{
		db:         db,
		downloader: NewBulkDownloader(),
	}
}

// Run downloads the full NDC dataset and upserts NDC-to-drug mappings.
func (ing *Ingestor) Run(ctx context.Context) error {
	log.Println("Starting openFDA NDC bulk download...")

	records, err := ing.downloader.FetchAllNDCRecords(ctx)
	if err != nil {
		return fmt.Errorf("bulk download: %w", err)
	}

	log.Printf("Downloaded %d NDC records. Mapping to drugs...", len(records))

	var mapped, noRxCUI, noDrug, errors int

	for i, record := range records {
		if err := ing.processRecord(ctx, record); err != nil {
			switch {
			case isNoRxCUI(err):
				noRxCUI++
			case isNoDrug(err):
				noDrug++
			default:
				errors++
				if errors <= 10 {
					log.Printf("WARN: %v", err)
				}
			}
			continue
		}

		mapped++
		if (i+1)%5000 == 0 {
			log.Printf("Progress: %d/%d processed (%d mapped, %d no rxcui, %d no drug match, %d errors)",
				i+1, len(records), mapped, noRxCUI, noDrug, errors)
		}
	}

	log.Printf("Done. %d mapped, %d no rxcui, %d no drug match, %d errors out of %d total.",
		mapped, noRxCUI, noDrug, errors, len(records))
	return nil
}

func (ing *Ingestor) processRecord(ctx context.Context, record NDCRecord) error {
	// Need at least one rxcui to map
	if len(record.OpenFDA.RxCUI) == 0 {
		return errNoRxCUI
	}

	// Find matching drug by rxcui
	rxcui := record.OpenFDA.RxCUI[0]
	drugEnt, err := ing.db.Drug.Query().
		Where(drug.Rxcui(rxcui)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errNoDrug
		}
		return fmt.Errorf("query drug for rxcui %s: %w", rxcui, err)
	}

	// Determine manufacturer
	manufacturer := ""
	if len(record.OpenFDA.ManufacturerName) > 0 {
		manufacturer = record.OpenFDA.ManufacturerName[0]
	}

	// Parse dates
	startDate := parseOpenFDADate(record.MarketingStartDate)

	// Process each package NDC
	for _, pkg := range record.Packaging {
		ndc := normalizeNDC(pkg.PackageNDC)
		if ndc == "" {
			continue
		}

		// Upsert: check if NDC already exists
		exists, err := ing.db.DrugNdcMap.Query().
			Where(drugndcmap.Ndc(ndc)).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check ndc %s: %w", ndc, err)
		}

		if exists {
			continue
		}

		endDate := parseOpenFDADate(pkg.MarketingEndDate)
		status := "ACTIVE"
		if endDate != nil {
			status = "DISCONTINUED"
		}

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
			return fmt.Errorf("create ndc mapping %s: %w", ndc, err)
		}
	}

	return nil
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
