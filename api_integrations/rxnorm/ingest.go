package rxnorm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/internal/synctracker"
)

const (
	sourceName = "rxnorm"
	// staleDuration defines how old a drug's last_rxnorm_sync can be before re-fetching.
	staleDuration = 30 * 24 * time.Hour // 30 days
)

// Ingestor fetches drug data from RxNorm and writes it to the database.
type Ingestor struct {
	client  *Client
	db      *ent.Client
	tracker *synctracker.Tracker
}

// NewIngestor creates a new RxNorm ingestor.
func NewIngestor(db *ent.Client) *Ingestor {
	return &Ingestor{
		client:  NewClient(),
		db:      db,
		tracker: synctracker.New(db),
	}
}

// Run performs an incremental sync of RxNorm drug concepts.
//
// On first run (no prior sync): fetches ALL SCD+SBD concepts (full load).
// On subsequent runs:
//  1. Fetches new concepts added since last sync via getNewConcepts API.
//  2. Re-fetches stale drugs whose last_rxnorm_sync is older than 30 days.
func (ing *Ingestor) Run(ctx context.Context) error {
	lastSync, err := ing.tracker.GetLastSync(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("get last sync: %w", err)
	}

	var totalIngested int

	if lastSync == nil {
		// First run: full load
		log.Println("RxNorm: no prior sync found. Performing full ingestion...")
		totalIngested, err = ing.fullLoad(ctx)
		if err != nil {
			return err
		}
	} else {
		// Incremental run
		log.Printf("RxNorm: last sync at %s. Running incremental update...", lastSync.LastSyncAt.Format(time.RFC3339))

		// Step 1: Fetch concepts added since last sync
		newCount, err := ing.ingestNewConcepts(ctx, lastSync.LastSyncAt)
		if err != nil {
			log.Printf("RxNorm: WARN: getNewConcepts failed, will still refresh stale: %v", err)
		} else {
			totalIngested += newCount
		}

		// Step 2: Refresh stale drug records
		staleCount, err := ing.refreshStaleDrugs(ctx)
		if err != nil {
			log.Printf("RxNorm: WARN: stale drug refresh had errors: %v", err)
		}
		totalIngested += staleCount
	}

	// Record this sync
	if err := ing.tracker.RecordSync(ctx, sourceName, totalIngested, "", ""); err != nil {
		log.Printf("RxNorm: WARN: failed to record sync metadata: %v", err)
	}

	log.Printf("RxNorm: sync complete. %d records processed.", totalIngested)
	return nil
}

// fullLoad fetches all SCD+SBD concepts from RxNorm (first-time ingestion).
func (ing *Ingestor) fullLoad(ctx context.Context) (int, error) {
	log.Println("Fetching all drug concepts from RxNorm (SCD+SBD)...")

	concepts, err := ing.client.GetAllConcepts(ctx, "SCD+SBD")
	if err != nil {
		return 0, fmt.Errorf("fetch concepts: %w", err)
	}

	log.Printf("Found %d drug concepts. Beginning ingestion...", len(concepts))

	var ingested, errors int

	for i, concept := range concepts {
		if err := ing.upsertDrug(ctx, concept); err != nil {
			log.Printf("WARN: skipping %s (%s): %v", concept.RxCUI, concept.Name, err)
			errors++
			continue
		}

		ingested++
		if (i+1)%500 == 0 {
			log.Printf("Progress: %d/%d processed (%d ingested, %d errors)", i+1, len(concepts), ingested, errors)
		}
	}

	log.Printf("Full load done. %d ingested, %d errors out of %d total.", ingested, errors, len(concepts))
	return ingested, nil
}

// ingestNewConcepts fetches concepts added to RxNorm since the given date.
// Uses RxNorm's /REST/newConcepts.json endpoint which returns concepts added
// since a specified date (format: MMDDYYYY).
func (ing *Ingestor) ingestNewConcepts(ctx context.Context, since time.Time) (int, error) {
	log.Printf("RxNorm: fetching new concepts since %s...", since.Format("2006-01-02"))

	concepts, err := ing.client.GetNewConcepts(ctx, since)
	if err != nil {
		return 0, fmt.Errorf("get new concepts: %w", err)
	}

	if len(concepts) == 0 {
		log.Println("RxNorm: no new concepts found since last sync.")
		return 0, nil
	}

	log.Printf("RxNorm: found %d new concepts. Ingesting...", len(concepts))

	var ingested, errors int
	for _, concept := range concepts {
		// Only process SCD and SBD types
		if concept.TTY != "SCD" && concept.TTY != "SBD" {
			continue
		}

		if err := ing.upsertDrug(ctx, concept); err != nil {
			log.Printf("WARN: skipping new concept %s (%s): %v", concept.RxCUI, concept.Name, err)
			errors++
			continue
		}
		ingested++
	}

	log.Printf("RxNorm: %d new concepts ingested, %d errors.", ingested, errors)
	return ingested, nil
}

// refreshStaleDrugs finds drugs whose last_rxnorm_sync is older than staleDuration
// and re-fetches their data from RxNorm.
func (ing *Ingestor) refreshStaleDrugs(ctx context.Context) (int, error) {
	staleThreshold := time.Now().Add(-staleDuration)

	staleDrugs, err := ing.db.Drug.Query().
		Where(drug.LastRxnormSyncLT(staleThreshold)).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query stale drugs: %w", err)
	}

	if len(staleDrugs) == 0 {
		log.Println("RxNorm: no stale drugs to refresh.")
		return 0, nil
	}

	log.Printf("RxNorm: refreshing %d stale drugs (last synced before %s)...", len(staleDrugs), staleThreshold.Format("2006-01-02"))

	var refreshed, errors int
	for i, d := range staleDrugs {
		concept := MinConcept{
			RxCUI: d.Rxcui,
			Name:  d.DrugName,
			TTY:   "SCD", // Default; upsertDrug will re-resolve
		}

		if err := ing.upsertDrug(ctx, concept); err != nil {
			errors++
			continue
		}
		refreshed++

		if (i+1)%200 == 0 {
			log.Printf("RxNorm: stale refresh progress: %d/%d", i+1, len(staleDrugs))
		}
	}

	log.Printf("RxNorm: %d stale drugs refreshed, %d errors.", refreshed, errors)
	return refreshed, nil
}

// upsertDrug fetches detailed info for a concept and creates or updates the drug record.
func (ing *Ingestor) upsertDrug(ctx context.Context, concept MinConcept) error {
	// Parse drug name components from the SCD/SBD name
	// Format: "ingredient strength dose_form" (e.g., "atorvastatin 10 MG Oral Tablet")
	drugName := concept.Name
	genericName, brandName := resolveNames(concept)

	// Fetch drug class
	drugClass := ""
	classInfos, err := ing.client.GetDrugClass(ctx, concept.RxCUI)
	if err == nil && len(classInfos) > 0 {
		drugClass = classInfos[0].RxclassMinConceptItem.ClassName
	}

	// Parse dose form and strength from the name
	doseForm, strength := parseDrugComponents(drugName)

	// Upsert: create if not exists, update if exists
	id, err := ing.db.Drug.Query().
		Where(drug.Rxcui(concept.RxCUI)).
		OnlyID(ctx)

	now := time.Now()

	if ent.IsNotFound(err) {
		// Create new drug
		brandNames := []string{}
		if brandName != "" {
			brandNames = []string{brandName}
		}

		_, err = ing.db.Drug.Create().
			SetRxcui(concept.RxCUI).
			SetDrugName(drugName).
			SetGenericName(genericName).
			SetBrandNames(brandNames).
			SetDoseForm(doseForm).
			SetStrength(strength).
			SetDrugClass(drugClass).
			SetLastRxnormSync(now).
			Save(ctx)
		return err
	} else if err != nil {
		return fmt.Errorf("query drug: %w", err)
	}

	// Update existing drug
	_, err = ing.db.Drug.UpdateOneID(id).
		SetDrugName(drugName).
		SetGenericName(genericName).
		SetDoseForm(doseForm).
		SetStrength(strength).
		SetDrugClass(drugClass).
		SetLastRxnormSync(now).
		Save(ctx)
	return err
}

// resolveNames determines generic and brand names based on term type.
func resolveNames(concept MinConcept) (generic, brand string) {
	switch concept.TTY {
	case "SCD": // Semantic Clinical Drug (generic)
		return concept.Name, ""
	case "SBD": // Semantic Branded Drug
		// SBD format: "ingredient strength dose_form [BrandName]"
		if idx := strings.Index(concept.Name, "["); idx != -1 {
			brand = strings.TrimRight(concept.Name[idx+1:], "]")
			generic = strings.TrimSpace(concept.Name[:idx])
		}
		return generic, brand
	default:
		return concept.Name, ""
	}
}

// parseDrugComponents extracts dose form and strength from a drug name.
// e.g., "atorvastatin 10 MG Oral Tablet" -> ("Oral Tablet", "10 MG")
func parseDrugComponents(name string) (doseForm, strength string) {
	// Common dose forms to look for
	forms := []string{
		"Oral Tablet", "Oral Capsule", "Oral Solution", "Oral Suspension",
		"Injectable Solution", "Injection", "Topical Cream", "Topical Ointment",
		"Ophthalmic Solution", "Nasal Spray", "Inhalation Powder",
		"Transdermal Patch", "Rectal Suppository", "Sublingual Tablet",
		"Chewable Tablet", "Extended Release Oral Tablet", "Delayed Release Oral Capsule",
	}

	for _, form := range forms {
		if strings.Contains(name, form) {
			doseForm = form
			break
		}
	}

	// Extract strength: look for patterns like "10 MG", "0.5 MG/ML"
	parts := strings.Fields(name)
	for i, part := range parts {
		if i+1 < len(parts) && isStrengthUnit(parts[i+1]) {
			strength = part + " " + parts[i+1]
			break
		}
	}

	return doseForm, strength
}

func isStrengthUnit(s string) bool {
	units := []string{"MG", "MG/ML", "MCG", "MG/HR", "UNIT", "UNIT/ML", "MEQ", "%"}
	upper := strings.ToUpper(s)
	for _, u := range units {
		if upper == u {
			return true
		}
	}
	return false
}
