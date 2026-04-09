package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kyanaman/formularycheck/api_integrations/articles"
	"github.com/kyanaman/formularycheck/ent"
	entarticle "github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/internal/summarizer"
)

// premiumSources are peer-reviewed journals and government agencies.
// Articles from these sources are retained for 90 days.
// All other sources are retained for 7 days.
var premiumSources = map[string]bool{
	"FDA":            true,
	"FDA Safety":     true,
	"PubMed":         true,
	"N Engl J Med":   true,
	"JAMA":           true,
	"Lancet":         true,
	"BMJ":            true,
	"Ann Intern Med": true,
}

// candidate holds a fetched-but-not-yet-stored article.
type candidate struct {
	Title      string
	Text       string // description or abstract
	SourceName string
	SourceURL  string
	PubDate    time.Time
}

// RunArticleIngestion runs the full article pipeline: collect candidates from
// RSS and PubMed, curate the top articles via LLM, summarize each, and store
// to the database. Finally, it applies tiered retention cleanup.
func RunArticleIngestion(ctx context.Context, db *ent.Client, sum *summarizer.Client) error {
	// Step 1: Collect candidate articles from all sources
	candidates := collectCandidates(ctx, db, sum)
	log.Printf("scheduler: collected %d candidate articles", len(candidates))

	if len(candidates) == 0 {
		log.Println("scheduler: no new article candidates")
		return nil
	}

	// Step 2: Curate and ingest
	if sum != nil && sum.IsConfigured() {
		curateAndIngest(ctx, db, sum, candidates)
	} else {
		// No LLM configured — take the first 5 candidates
		if len(candidates) > 5 {
			candidates = candidates[:5]
		}
		for _, c := range candidates {
			ingestCandidate(ctx, db, nil, c)
		}
		log.Printf("scheduler: ingested %d articles (no curation — OPENAI_API_KEY not set)", len(candidates))
	}

	// Step 3: Tiered retention cleanup
	applyRetentionPolicy(ctx, db)

	log.Println("scheduler: article ingestion complete")
	return nil
}

// StartArticleScheduler launches a background goroutine that runs article
// ingestion at the given interval. It respects context cancellation for
// graceful shutdown.
func StartArticleScheduler(ctx context.Context, db *ent.Client, sum *summarizer.Client, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Println("scheduler: starting article ingestion...")
				ingCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				if err := RunArticleIngestion(ingCtx, db, sum); err != nil {
					log.Printf("scheduler: article ingestion failed: %v", err)
				}
				cancel()
			case <-ctx.Done():
				log.Println("scheduler: shutting down article scheduler")
				return
			}
		}
	}()
}

// collectCandidates gathers article candidates from all configured sources
// (RSS feeds + PubMed) without storing them.
func collectCandidates(ctx context.Context, db *ent.Client, sum *summarizer.Client) []candidate {
	var all []candidate

	// RSS feeds
	rssIngestor := articles.NewRSSIngestor(db, articles.DefaultFeeds(), nil) // nil summarizer = collect only
	rssCandidates := rssIngestor.CollectCandidates(ctx)
	all = append(all, rawToCandidates(rssCandidates)...)

	// PubMed
	ncbiKey := os.Getenv("NCBI_API_KEY")
	pubmedIngestor := articles.NewPubMedIngestor(db, ncbiKey, nil)
	pmCandidates := pubmedIngestor.CollectCandidates(ctx)
	all = append(all, rawToCandidates(pmCandidates)...)

	return all
}

// rawToCandidates converts articles.RawCandidate slices to local candidate structs.
func rawToCandidates(raw []articles.RawCandidate) []candidate {
	out := make([]candidate, len(raw))
	for i, r := range raw {
		out[i] = candidate{
			Title:      r.Title,
			Text:       r.Text,
			SourceName: r.SourceName,
			SourceURL:  r.SourceURL,
			PubDate:    r.PubDate,
		}
	}
	return out
}

// curateAndIngest uses the LLM to select the top 3-5 articles from candidates,
// then summarizes and stores each selected article.
func curateAndIngest(ctx context.Context, db *ent.Client, sum *summarizer.Client, candidates []candidate) {
	// Build summarizer candidates
	sumCandidates := make([]summarizer.Candidate, len(candidates))
	for i, c := range candidates {
		sumCandidates[i] = summarizer.Candidate{
			Index:  i,
			Title:  c.Title,
			Source: c.SourceName,
			Text:   c.Text,
		}
	}

	selected, err := sum.Curate(ctx, sumCandidates)
	if err != nil {
		log.Printf("scheduler: curation failed, falling back to first 5: %v", err)
		selected = []int{}
		for i := 0; i < len(candidates) && i < 5; i++ {
			selected = append(selected, i)
		}
	}

	log.Printf("scheduler: curated %d articles from %d candidates", len(selected), len(candidates))

	// Summarize and ingest only the selected articles
	var ingested int
	for _, idx := range selected {
		if idx < 0 || idx >= len(candidates) {
			continue
		}
		if ingestCandidate(ctx, db, sum, candidates[idx]) {
			ingested++
		}
	}
	log.Printf("scheduler: ingested %d curated articles", ingested)
}

// ingestCandidate deduplicates, optionally summarizes via LLM, and stores a
// single article candidate to the database.
func ingestCandidate(ctx context.Context, db *ent.Client, sum *summarizer.Client, c candidate) bool {
	// Final dedup check
	exists, _ := db.Article.Query().
		Where(entarticle.SourceURL(c.SourceURL)).
		Exist(ctx)
	if exists {
		return false
	}

	builder := db.Article.Create().
		SetTitle(c.Title).
		SetSourceName(c.SourceName).
		SetSourceURL(c.SourceURL).
		SetPublishedAt(c.PubDate).
		SetIsActive(true)

	if c.Text != "" {
		builder = builder.SetSummary(c.Text)
	}

	// Summarize with LLM if configured
	if sum != nil && sum.IsConfigured() {
		text := c.Text
		if text == "" {
			text = c.Title
		}
		result, err := sum.Summarize(ctx, c.Title, text)
		if err != nil {
			log.Printf("scheduler: summarize failed for '%s': %v", truncate(c.Title, 50), err)
		} else {
			if result.Summary != "" {
				builder = builder.SetSummary(result.Summary)
			}
			if len(result.DrugClasses) > 0 {
				builder = builder.SetDrugClasses(result.DrugClasses)
			}
		}
	}

	if _, err := builder.Save(ctx); err != nil {
		log.Printf("scheduler: save failed for '%s': %v", truncate(c.Title, 50), err)
		return false
	}
	return true
}

// applyRetentionPolicy deactivates old articles using tiered retention:
// - Standard sources: 7-day retention
// - Premium sources (FDA, PubMed, major journals): 90-day retention
func applyRetentionPolicy(ctx context.Context, db *ent.Client) {
	now := time.Now()

	// Standard sources: 7-day retention
	weekAgo := now.AddDate(0, 0, -7)
	standardCount, err := db.Article.Update().
		Where(
			entarticle.PublishedAtLT(weekAgo),
			entarticle.IsActive(true),
			entarticle.SourceNameNotIn(premiumSourceList()...),
		).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		log.Printf("scheduler: standard retention cleanup failed: %v", err)
	} else if standardCount > 0 {
		log.Printf("scheduler: deactivated %d standard articles older than 7 days", standardCount)
	}

	// Premium sources: 90-day retention
	ninetyDaysAgo := now.AddDate(0, 0, -90)
	premiumCount, err := db.Article.Update().
		Where(
			entarticle.PublishedAtLT(ninetyDaysAgo),
			entarticle.IsActive(true),
			entarticle.SourceNameIn(premiumSourceList()...),
		).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		log.Printf("scheduler: premium retention cleanup failed: %v", err)
	} else if premiumCount > 0 {
		log.Printf("scheduler: deactivated %d premium articles older than 90 days", premiumCount)
	}
}

// premiumSourceList returns the premium source names as a slice.
func premiumSourceList() []string {
	list := make([]string, 0, len(premiumSources))
	for s := range premiumSources {
		list = append(list, s)
	}
	return list
}

// truncate shortens a string to n characters with an ellipsis suffix.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return fmt.Sprintf("%s...", s[:n])
}
