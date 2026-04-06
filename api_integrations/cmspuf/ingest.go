package cmspuf

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/plan"
)

const batchSize = 1000

// Ingestor downloads and ingests CMS Part D PUF data.
type Ingestor struct {
	db         *ent.Client
	downloader *Downloader
}

// NewIngestor creates a new CMS PUF ingestor.
// pufURL should point to the SPUF ZIP file on data.cms.gov.
func NewIngestor(db *ent.Client, pufURL string) *Ingestor {
	return &Ingestor{
		db:         db,
		downloader: NewDownloader(pufURL),
	}
}

// Run downloads the PUF, parses all files, and upserts data into the database.
func (ing *Ingestor) Run(ctx context.Context) error {
	// Download and extract
	files, err := ing.downloader.Download(ctx)
	if err != nil {
		return fmt.Errorf("download PUF: %w", err)
	}

	// Step 1: Parse and upsert plans
	if err := ing.ingestPlans(ctx, files.PlanInfo); err != nil {
		return fmt.Errorf("ingest plans: %w", err)
	}

	// Step 2: Build formulary_id → plan_id lookup
	formularyToPlan, err := ing.buildFormularyPlanMap(ctx)
	if err != nil {
		return fmt.Errorf("build formulary map: %w", err)
	}

	// Step 3: Parse and upsert formulary entries
	if err := ing.ingestFormulary(ctx, files.FormularyDrugs, formularyToPlan); err != nil {
		return fmt.Errorf("ingest formulary: %w", err)
	}

	log.Println("CMS PUF ingestion complete.")
	return nil
}

func (ing *Ingestor) ingestPlans(ctx context.Context, data []byte) error {
	log.Println("Parsing Plan Information file...")

	plans, err := ParsePlans(data)
	if err != nil {
		return err
	}

	log.Printf("Found %d plans. Upserting...", len(plans))

	var created, updated, errors int
	for _, p := range plans {
		existing, err := ing.db.Plan.Query().
			Where(
				plan.ContractID(p.ContractID),
				plan.PlanID(p.PlanID),
				plan.SegmentID(p.SegmentID),
			).
			Only(ctx)

		if ent.IsNotFound(err) {
			_, err = ing.db.Plan.Create().
				SetContractID(p.ContractID).
				SetPlanID(p.PlanID).
				SetSegmentID(p.SegmentID).
				SetContractName(p.ContractName).
				SetPlanName(p.PlanName).
				SetFormularyID(p.FormularyID).
				SetPlanType(p.PlanType).
				SetSnpType(p.SNPType).
				Save(ctx)
			if err != nil {
				errors++
				continue
			}
			created++
		} else if err != nil {
			errors++
			continue
		} else {
			_, err = existing.Update().
				SetContractName(p.ContractName).
				SetPlanName(p.PlanName).
				SetFormularyID(p.FormularyID).
				SetPlanType(p.PlanType).
				SetSnpType(p.SNPType).
				Save(ctx)
			if err != nil {
				errors++
				continue
			}
			updated++
		}
	}

	log.Printf("Plans: %d created, %d updated, %d errors", created, updated, errors)
	return nil
}

// buildFormularyPlanMap creates a lookup from formulary_id to plan entity IDs.
// Multiple plans can share the same formulary_id.
func (ing *Ingestor) buildFormularyPlanMap(ctx context.Context) (map[string][]int, error) {
	plans, err := ing.db.Plan.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]int)
	for _, p := range plans {
		result[p.FormularyID] = append(result[p.FormularyID], p.ID)
	}

	log.Printf("Built formulary-to-plan map: %d formulary IDs → %d plans", len(result), len(plans))
	return result, nil
}

func (ing *Ingestor) ingestFormulary(ctx context.Context, data []byte, formularyToPlan map[string][]int) error {
	log.Println("Parsing Basic Drugs Formulary file...")

	sourceDate := time.Now()
	var totalProcessed, mapped, noDrug, noPlan, errors int

	err := ParseFormulary(data, batchSize, func(batch []FormularyRow) error {
		for _, row := range batch {
			totalProcessed++

			// Find plans for this formulary
			planIDs, ok := formularyToPlan[row.FormularyID]
			if !ok {
				noPlan++
				continue
			}

			// Find drug by rxcui
			if row.RxCUI == "" {
				noDrug++
				continue
			}

			drugEnt, err := ing.db.Drug.Query().
				Where(drug.Rxcui(row.RxCUI)).
				Only(ctx)
			if err != nil {
				if ent.IsNotFound(err) {
					noDrug++
				} else {
					errors++
				}
				continue
			}

			// Create formulary entry for each plan that uses this formulary
			tierName := TierName[row.TierLevelValue]

			for _, planID := range planIDs {
				builder := ing.db.FormularyEntry.Create().
					SetPlanID(planID).
					SetDrugID(drugEnt.ID).
					SetTierLevel(row.TierLevelValue).
					SetTierName(tierName).
					SetPriorAuthRequired(row.PriorAuthYN).
					SetStepTherapy(row.StepTherapyYN).
					SetQuantityLimit(row.QuantityLimitYN).
					SetSourceType("CMS_PUF").
					SetSourceDate(sourceDate).
					SetIsCurrent(true)

				if row.QuantityLimitAmt > 0 {
					builder = builder.SetQuantityLimitAmount(row.QuantityLimitAmt)
				}
				if row.QuantityLimitDays > 0 {
					builder = builder.SetQuantityLimitDays(row.QuantityLimitDays)
				}

				if _, err := builder.Save(ctx); err != nil {
					errors++
					continue
				}
				mapped++
			}
		}

		if totalProcessed%50000 == 0 {
			log.Printf("Progress: %d processed (%d mapped, %d no drug, %d no plan, %d errors)",
				totalProcessed, mapped, noDrug, noPlan, errors)
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("Formulary done. %d processed, %d mapped, %d no drug, %d no plan, %d errors",
		totalProcessed, mapped, noDrug, noPlan, errors)
	return nil
}
