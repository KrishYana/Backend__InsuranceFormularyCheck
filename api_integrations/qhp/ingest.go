package qhp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/insurer"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/internal/synctracker"
)

const (
	concurrentCrawls = 10
	sourceName       = "qhp"
	// issuerStaleAfter defines how long before an issuer is re-crawled.
	// QHP data typically updates quarterly, so 7 days is a reasonable interval.
	issuerStaleAfter = 7 * 24 * time.Hour
)

// Ingestor discovers QHP issuers and ingests their formulary data.
type Ingestor struct {
	db       *ent.Client
	crawler  *Crawler
	mrpufURL string
	tracker  *synctracker.Tracker
}

// NewIngestor creates a new QHP ingestor.
func NewIngestor(db *ent.Client, mrpufURL string) *Ingestor {
	return &Ingestor{
		db:       db,
		crawler:  NewCrawler(),
		mrpufURL: mrpufURL,
		tracker:  synctracker.New(db),
	}
}

// Run discovers issuers, crawls their JSON files, and upserts data.
// On subsequent runs, only re-crawls issuers that haven't been synced recently.
// Per-issuer sync times are tracked in SyncMetadata with source_name "qhp:<issuer_id>".
func (ing *Ingestor) Run(ctx context.Context) error {
	issuers, err := DiscoverIssuers(ctx, ing.mrpufURL)
	if err != nil {
		return fmt.Errorf("discover issuers: %w", err)
	}

	// Filter to only stale issuers (those not synced within issuerStaleAfter)
	staleIssuers, skippedCount := ing.filterStaleIssuers(ctx, issuers)

	if skippedCount > 0 {
		slog.Info("QHP issuers filtered", "skipped", skippedCount, "stale", len(staleIssuers))
	}

	if len(staleIssuers) == 0 {
		slog.Info("QHP: all issuers are up to date, nothing to crawl")
		return nil
	}

	slog.Info("crawling QHP issuers", "count", len(staleIssuers), "concurrency", concurrentCrawls)

	// Crawl stale issuers concurrently
	results := ing.crawlAll(ctx, staleIssuers)

	// Process results
	var success, failed, crawlFailed, noPlans, noDrugs int
	for _, result := range results {
		if result.Err != nil {
			crawlFailed++
			if crawlFailed <= 10 {
				slog.Error("QHP crawl failed", "state", result.Issuer.State, "issuer_id", result.Issuer.IssuerID, "error", result.Err)
			}
			failed++
			continue
		}

		if len(result.Plans) == 0 {
			noPlans++
			continue
		}

		slog.Info("QHP processing issuer", "state", result.Issuer.State, "issuer_id", result.Issuer.IssuerID, "plans", len(result.Plans), "drugs", len(result.Drugs))

		if err := ing.processResult(ctx, result); err != nil {
			failed++
			slog.Error("QHP processing failed", "state", result.Issuer.State, "issuer_id", result.Issuer.IssuerID, "error", err)
			continue
		}

		if len(result.Drugs) == 0 {
			noDrugs++
		}

		// Record per-issuer sync time
		issuerSource := "qhp:" + result.Issuer.IssuerID
		drugCount := len(result.Drugs)
		if err := ing.tracker.RecordSync(ctx, issuerSource, drugCount, "", result.Issuer.URL); err != nil {
			slog.Warn("QHP failed to record sync for issuer", "issuer_id", result.Issuer.IssuerID, "error", err)
		}

		success++
	}

	// Record overall QHP sync
	if err := ing.tracker.RecordSync(ctx, sourceName, success, "", ing.mrpufURL); err != nil {
		slog.Warn("QHP failed to record overall sync metadata", "error", err)
	}

	slog.Info("QHP ingestion complete",
		"success", success,
		"failed", failed,
		"crawl_errors", crawlFailed,
		"no_plans", noPlans,
		"no_drugs", noDrugs,
		"total_issuers", len(staleIssuers),
	)
	return nil
}

// filterStaleIssuers returns only issuers whose last sync is older than issuerStaleAfter.
func (ing *Ingestor) filterStaleIssuers(ctx context.Context, issuers []Issuer) (stale []Issuer, skipped int) {
	staleThreshold := time.Now().Add(-issuerStaleAfter)

	for _, issuer := range issuers {
		issuerSource := "qhp:" + issuer.IssuerID
		lastSync, err := ing.tracker.GetLastSync(ctx, issuerSource)
		if err != nil {
			// On error, include it to be safe
			stale = append(stale, issuer)
			continue
		}

		if lastSync != nil && lastSync.LastSyncAt.After(staleThreshold) {
			skipped++
			continue
		}

		stale = append(stale, issuer)
	}

	return stale, skipped
}

func (ing *Ingestor) crawlAll(ctx context.Context, issuers []Issuer) []*CrawlResult {
	results := make([]*CrawlResult, len(issuers))
	sem := make(chan struct{}, concurrentCrawls)
	var wg sync.WaitGroup

	for i, issuer := range issuers {
		wg.Add(1)
		go func(i int, issuer Issuer) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[i] = ing.crawler.CrawlIssuer(ctx, issuer)

			if (i+1)%50 == 0 {
				slog.Info("QHP crawl progress", "completed", i+1, "total", len(issuers))
			}
		}(i, issuer)
	}

	wg.Wait()
	return results
}

func (ing *Ingestor) processResult(ctx context.Context, result *CrawlResult) error {
	// Find or create an Insurer for this issuer
	var insurerEnt *ent.Insurer
	existing, err := ing.db.Insurer.Query().
		Where(insurer.HiosIssuerID(result.Issuer.IssuerID)).
		Only(ctx)
	if err == nil {
		insurerEnt = existing
	} else if ent.IsNotFound(err) {
		// Derive a human-readable name from the first plan's marketing name
		insurerName := result.Issuer.IssuerID
		if len(result.Plans) > 0 && result.Plans[0].MarketingName != "" {
			// Marketing names often start with the insurer name, e.g. "Blue Cross Blue Shield Gold PPO"
			insurerName = result.Plans[0].MarketingName
			// Trim plan-specific suffixes to get the insurer name
			for _, suffix := range []string{" Gold", " Silver", " Bronze", " Platinum", " Catastrophic", " PPO", " HMO", " EPO", " POS"} {
				if idx := strings.Index(insurerName, suffix); idx > 0 {
					insurerName = insurerName[:idx]
					break
				}
			}
		}
		ins, createErr := ing.db.Insurer.Create().
			SetInsurerName(insurerName).
			SetHiosIssuerID(result.Issuer.IssuerID).
			Save(ctx)
		if createErr != nil {
			slog.Warn("QHP failed to create insurer", "issuer_id", result.Issuer.IssuerID, "error", createErr)
		} else {
			insurerEnt = ins
		}
	}

	// Build a map of plan_id -> plan entity for this issuer
	planMap := make(map[string]*ent.Plan)

	for _, p := range result.Plans {
		if p.PlanID == "" {
			continue
		}

		// QHP plan IDs are 14-char HIOS IDs; extract components
		contractID := result.Issuer.IssuerID
		planID := p.PlanID
		segmentID := "000"

		// Upsert plan
		existingPlan, err := ing.db.Plan.Query().
			Where(
				plan.ContractID(contractID),
				plan.PlanID(planID),
				plan.SegmentID(segmentID),
			).
			Only(ctx)

		if ent.IsNotFound(err) {
			builder := ing.db.Plan.Create().
				SetContractID(contractID).
				SetPlanID(planID).
				SetSegmentID(segmentID).
				SetContractName(result.Issuer.IssuerID).
				SetPlanName(p.MarketingName).
				SetFormularyID(planID).
				SetPlanType("QHP")
			if result.Issuer.State != "" {
				builder = builder.SetStateCode(result.Issuer.State)
			}
			if insurerEnt != nil {
				builder = builder.SetInsurerID(insurerEnt.ID)
			}
			planEnt, err := builder.Save(ctx)
			if err != nil {
				return fmt.Errorf("create plan %s: %w", planID, err)
			}
			planMap[p.PlanID] = planEnt
		} else if err != nil {
			return fmt.Errorf("query plan %s: %w", planID, err)
		} else {
			// Update existing plan to link insurer and state if missing
			needsUpdate := false
			updater := existingPlan.Update()
			if insurerEnt != nil {
				updater = updater.SetInsurerID(insurerEnt.ID)
				needsUpdate = true
			}
			if result.Issuer.State != "" {
				updater = updater.SetStateCode(result.Issuer.State)
				needsUpdate = true
			}
			if needsUpdate {
				if _, err := updater.Save(ctx); err != nil {
					slog.Warn("QHP failed to update plan", "plan_id", planID, "error", err)
				}
			}
			planMap[p.PlanID] = existingPlan
		}
	}

	// Process drugs
	sourceDate := time.Now()
	var mapped, noDrug int

	for _, d := range result.Drugs {
		if d.RxNormID == "" {
			continue
		}

		// Find drug by rxcui
		drugEnt, err := ing.db.Drug.Query().
			Where(drug.Rxcui(d.RxNormID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				noDrug++
			}
			continue
		}

		for _, dp := range d.Plans {
			planEnt, ok := planMap[dp.PlanID]
			if !ok {
				continue
			}

			// Check if entry already exists for this plan+drug+source
			exists, err := ing.db.FormularyEntry.Query().
				Where(
					formularyentry.HasPlanWith(plan.ID(planEnt.ID)),
					formularyentry.HasDrugWith(drug.ID(drugEnt.ID)),
					formularyentry.SourceType("QHP"),
				).
				Exist(ctx)
			if err != nil {
				continue
			}
			if exists {
				continue // skip duplicate
			}

			tierLevel := TierMapping[strings.ToUpper(dp.DrugTier)]
			tierName := dp.DrugTier

			builder := ing.db.FormularyEntry.Create().
				SetPlanID(planEnt.ID).
				SetDrugID(drugEnt.ID).
				SetTierLevel(tierLevel).
				SetTierName(tierName).
				SetPriorAuthRequired(dp.PriorAuthorization).
				SetStepTherapy(dp.StepTherapy).
				SetQuantityLimit(dp.QuantityLimit).
				SetSourceType("QHP").
				SetSourceDate(sourceDate).
				SetIsCurrent(true)

			if _, err := builder.Save(ctx); err != nil {
				continue
			}
			mapped++
		}
	}

	if len(result.Drugs) > 0 {
		slog.Info("QHP drugs mapped", "state", result.Issuer.State, "issuer_id", result.Issuer.IssuerID, "mapped", mapped, "no_match", noDrug, "plans", len(result.Plans), "drugs", len(result.Drugs))
	}

	return nil
}
