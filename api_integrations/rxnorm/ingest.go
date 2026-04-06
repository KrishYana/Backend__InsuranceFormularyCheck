package rxnorm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
)

// Ingestor fetches drug data from RxNorm and writes it to the database.
type Ingestor struct {
	client *Client
	db     *ent.Client
}

// NewIngestor creates a new RxNorm ingestor.
func NewIngestor(db *ent.Client) *Ingestor {
	return &Ingestor{
		client: NewClient(),
		db:     db,
	}
}

// Run fetches all SCD (generic) and SBD (branded) drug concepts from RxNorm
// and upserts them into the drugs table.
func (ing *Ingestor) Run(ctx context.Context) error {
	log.Println("Fetching all drug concepts from RxNorm (SCD+SBD)...")

	concepts, err := ing.client.GetAllConcepts(ctx, "SCD+SBD")
	if err != nil {
		return fmt.Errorf("fetch concepts: %w", err)
	}

	log.Printf("Found %d drug concepts. Beginning ingestion...", len(concepts))

	var ingested, skipped, errors int

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

	log.Printf("Done. %d ingested, %d skipped, %d errors out of %d total.", ingested, skipped, errors, len(concepts))
	return nil
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
// e.g., "atorvastatin 10 MG Oral Tablet" → ("Oral Tablet", "10 MG")
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
