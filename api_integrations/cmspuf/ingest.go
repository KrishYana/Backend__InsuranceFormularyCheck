package cmspuf

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/insurer"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/internal/synctracker"
)

const (
	batchSize  = 1000
	sourceName = "cms_puf"
)

// Ingestor downloads and ingests CMS Part D PUF data.
type Ingestor struct {
	db         *ent.Client
	downloader *Downloader
	tracker    *synctracker.Tracker
	pufURL     string
}

// NewIngestor creates a new CMS PUF ingestor.
// pufURL should point to the SPUF ZIP file on data.cms.gov.
func NewIngestor(db *ent.Client, pufURL string) *Ingestor {
	return &Ingestor{
		db:         db,
		downloader: NewDownloader(pufURL),
		tracker:    synctracker.New(db),
		pufURL:     pufURL,
	}
}

// Run downloads the PUF, parses all files, and upserts data into the database.
// On subsequent runs, it first sends a HEAD request to check if the file has
// changed since the last sync (via Last-Modified or Content-Length comparison).
func (ing *Ingestor) Run(ctx context.Context) error {
	// Check if data has changed since last sync
	lastSync, err := ing.tracker.GetLastSync(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("get last sync: %w", err)
	}

	fingerprint, err := ing.downloader.CheckFingerprint(ctx)
	if err != nil {
		slog.Warn("CMS PUF could not check file fingerprint, proceeding with full download", "error", err)
	} else if lastSync != nil && lastSync.LastEtag != "" {
		if fingerprint == lastSync.LastEtag {
			slog.Info("CMS PUF file unchanged since last sync, skipping download", "fingerprint", fingerprint)
			return nil
		}
		slog.Info("CMS PUF file changed, downloading", "previous", lastSync.LastEtag, "current", fingerprint)
	}

	// Download and extract
	files, err := ing.downloader.Download(ctx)
	if err != nil {
		return fmt.Errorf("download PUF: %w", err)
	}

	// Step 1: Parse and upsert plans
	if err := ing.ingestPlans(ctx, files.PlanInfo); err != nil {
		return fmt.Errorf("ingest plans: %w", err)
	}

	// Step 2: Build formulary_id -> plan_id lookup
	formularyToPlan, err := ing.buildFormularyPlanMap(ctx)
	if err != nil {
		return fmt.Errorf("build formulary map: %w", err)
	}

	// Step 3: Parse and upsert formulary entries
	totalMapped, err := ing.ingestFormulary(ctx, files.FormularyDrugs, formularyToPlan)
	if err != nil {
		return fmt.Errorf("ingest formulary: %w", err)
	}

	// Record sync metadata
	etag := fingerprint
	if err := ing.tracker.RecordSync(ctx, sourceName, totalMapped, etag, ing.pufURL); err != nil {
		slog.Warn("CMS PUF failed to record sync metadata", "error", err)
	}

	slog.Info("CMS PUF ingestion complete")
	return nil
}

func (ing *Ingestor) ingestPlans(ctx context.Context, data []byte) error {
	slog.Info("parsing Plan Information file")

	plans, err := ParsePlans(data)
	if err != nil {
		return err
	}

	slog.Info("found plans, upserting", "count", len(plans))

	// First pass: create an Insurer for each unique contract
	insurerMap := make(map[string]*ent.Insurer) // contractID → Insurer
	for _, p := range plans {
		if _, ok := insurerMap[p.ContractID]; ok {
			continue
		}

		// Find existing insurer by contract ID stored in hios_issuer_id
		existing, err := ing.db.Insurer.Query().
			Where(insurer.HiosIssuerID(p.ContractID)).
			Only(ctx)
		if err == nil {
			insurerMap[p.ContractID] = existing
			continue
		}
		if !ent.IsNotFound(err) {
			slog.Warn("CMS PUF error querying insurer", "contract_id", p.ContractID, "error", err)
			continue
		}

		// Create new insurer from contract info
		ins, err := ing.db.Insurer.Create().
			SetInsurerName(p.ContractName).
			SetHiosIssuerID(p.ContractID).
			Save(ctx)
		if err != nil {
			slog.Warn("CMS PUF failed to create insurer", "contract_id", p.ContractID, "error", err)
			continue
		}
		insurerMap[p.ContractID] = ins
	}
	slog.Info("insurers mapped", "unique_contracts", len(insurerMap))

	// Second pass: upsert plans linked to their insurer
	var created, updated, errors int
	for _, p := range plans {
		existing, err := ing.db.Plan.Query().
			Where(
				plan.ContractID(p.ContractID),
				plan.PlanID(p.PlanID),
				plan.SegmentID(p.SegmentID),
			).
			Only(ctx)

		insurerEnt := insurerMap[p.ContractID]

		if ent.IsNotFound(err) {
			builder := ing.db.Plan.Create().
				SetContractID(p.ContractID).
				SetPlanID(p.PlanID).
				SetSegmentID(p.SegmentID).
				SetContractName(p.ContractName).
				SetPlanName(p.PlanName).
				SetFormularyID(p.FormularyID).
				SetPlanType(p.PlanType).
				SetSnpType(p.SNPType)
			if insurerEnt != nil {
				builder = builder.SetInsurerID(insurerEnt.ID)
			}
			if _, err = builder.Save(ctx); err != nil {
				errors++
				continue
			}
			created++
		} else if err != nil {
			errors++
			continue
		} else {
			updater := existing.Update().
				SetContractName(p.ContractName).
				SetPlanName(p.PlanName).
				SetFormularyID(p.FormularyID).
				SetPlanType(p.PlanType).
				SetSnpType(p.SNPType)
			if insurerEnt != nil {
				updater = updater.SetInsurerID(insurerEnt.ID)
			}
			if _, err = updater.Save(ctx); err != nil {
				errors++
				continue
			}
			updated++
		}
	}

	slog.Info("plans upserted", "created", created, "updated", updated, "errors", errors)
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

	slog.Info("built formulary-to-plan map", "formulary_ids", len(result), "plans", len(plans))
	return result, nil
}

func (ing *Ingestor) ingestFormulary(ctx context.Context, data []byte, formularyToPlan map[string][]int) (int, error) {
	slog.Info("parsing Basic Drugs Formulary file")

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
				// Check if entry already exists for this plan+drug+source
				exists, err := ing.db.FormularyEntry.Query().
					Where(
						formularyentry.HasPlanWith(plan.ID(planID)),
						formularyentry.HasDrugWith(drug.ID(drugEnt.ID)),
						formularyentry.SourceType("CMS_PUF"),
					).
					Exist(ctx)
				if err != nil {
					errors++
					continue
				}
				if exists {
					continue // skip duplicate
				}

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
			slog.Info("CMS PUF formulary progress", "processed", totalProcessed, "mapped", mapped, "no_drug", noDrug, "no_plan", noPlan, "errors", errors)
		}

		return nil
	})

	if err != nil {
		return mapped, err
	}

	slog.Info("CMS PUF formulary done", "processed", totalProcessed, "mapped", mapped, "no_drug", noDrug, "no_plan", noPlan, "errors", errors)
	return mapped, nil
}
