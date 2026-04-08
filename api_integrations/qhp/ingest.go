package qhp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/plan"
)

const concurrentCrawls = 10

// Ingestor discovers QHP issuers and ingests their formulary data.
type Ingestor struct {
	db      *ent.Client
	crawler *Crawler
	mrpufURL string
}

// NewIngestor creates a new QHP ingestor.
func NewIngestor(db *ent.Client, mrpufURL string) *Ingestor {
	return &Ingestor{
		db:       db,
		crawler:  NewCrawler(),
		mrpufURL: mrpufURL,
	}
}

// Run discovers issuers, crawls their JSON files, and upserts data.
func (ing *Ingestor) Run(ctx context.Context) error {
	issuers, err := DiscoverIssuers(ctx, ing.mrpufURL)
	if err != nil {
		return fmt.Errorf("discover issuers: %w", err)
	}

	log.Printf("Crawling %d issuers (%d concurrent)...", len(issuers), concurrentCrawls)

	// Crawl issuers concurrently
	results := ing.crawlAll(ctx, issuers)

	// Process results
	var success, failed int
	for _, result := range results {
		if result.Err != nil {
			failed++
			if failed <= 20 {
				log.Printf("FAIL [%s] %s: %v", result.Issuer.State, result.Issuer.IssuerID, result.Err)
			}
			continue
		}

		if err := ing.processResult(ctx, result); err != nil {
			failed++
			log.Printf("FAIL [%s] %s: process: %v", result.Issuer.State, result.Issuer.IssuerID, err)
			continue
		}

		success++
	}

	log.Printf("QHP ingestion complete. %d success, %d failed out of %d issuers.", success, failed, len(issuers))
	return nil
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
				log.Printf("Crawl progress: %d/%d issuers", i+1, len(issuers))
			}
		}(i, issuer)
	}

	wg.Wait()
	return results
}

func (ing *Ingestor) processResult(ctx context.Context, result *CrawlResult) error {
	// Build a map of plan_id → plan entity for this issuer
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
		existing, err := ing.db.Plan.Query().
			Where(
				plan.ContractID(contractID),
				plan.PlanID(planID),
				plan.SegmentID(segmentID),
			).
			Only(ctx)

		if ent.IsNotFound(err) {
			planEnt, err := ing.db.Plan.Create().
				SetContractID(contractID).
				SetPlanID(planID).
				SetSegmentID(segmentID).
				SetContractName(result.Issuer.IssuerID).
				SetPlanName(p.MarketingName).
				SetFormularyID(planID).
				SetPlanType("QHP").
				Save(ctx)
			if err != nil {
				return fmt.Errorf("create plan %s: %w", planID, err)
			}
			planMap[p.PlanID] = planEnt
		} else if err != nil {
			return fmt.Errorf("query plan %s: %w", planID, err)
		} else {
			planMap[p.PlanID] = existing
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
		log.Printf("  [%s] %s: %d drugs mapped, %d no match (from %d plans, %d drugs)",
			result.Issuer.State, result.Issuer.IssuerID, mapped, noDrug, len(result.Plans), len(result.Drugs))
	}

	return nil
}
