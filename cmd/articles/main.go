package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/kyanaman/formularycheck/api_integrations/articles"
	"github.com/kyanaman/formularycheck/ent"
	entarticle "github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/internal/summarizer"

	_ "github.com/lib/pq"
)

// premiumSources are peer-reviewed journals and government agencies.
// Articles from these sources are retained for 90 days.
// All other sources are retained for 7 days.
var premiumSources = map[string]bool{
	"FDA":            true,
	"FDA Safety":     true,
	"PubMed":         true,
	// Journal names that come through as sourceName from PubMed ESummary
	"N Engl J Med":   true,
	"JAMA":           true,
	"Lancet":         true,
	"BMJ":            true,
	"Ann Intern Med": true,
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://formulary:formulary@localhost:5432/formularycheck?sslmode=disable"
	}

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Deactivate articles from paywalled sources that were previously ingested
	deactivatePaywalled(ctx, client)

	// Initialize summarizer
	openaiKey := os.Getenv("OPENAI_API_KEY")
	openaiModel := os.Getenv("OPENAI_MODEL")
	sum := summarizer.New(openaiKey, openaiModel)
	if sum.IsConfigured() {
		log.Printf("Summarizer configured (model: %s)", openaiModel)
	} else {
		log.Println("OPENAI_API_KEY not set — articles will be ingested without AI curation/summaries")
	}

	// Step 1: Collect candidate articles from all sources (no summarization yet)
	candidates := collectCandidates(ctx, client, sum)
	log.Printf("Collected %d candidate articles", len(candidates))

	if len(candidates) == 0 {
		log.Println("No new candidates. Skipping curation.")
	} else if sum.IsConfigured() {
		// Step 2: Use GPT to curate the top 3-5 most important articles
		curateAndIngest(ctx, client, sum, candidates)
	} else {
		// No LLM — just take the first 5 candidates
		if len(candidates) > 5 {
			candidates = candidates[:5]
		}
		for _, c := range candidates {
			ingestCandidate(ctx, client, nil, c)
		}
		log.Printf("Ingested %d articles (no curation — OPENAI_API_KEY not set)", len(candidates))
	}

	// Step 3: Tiered retention cleanup
	applyRetentionPolicy(ctx, client)

	log.Println("Article ingestion complete.")
}

// candidate holds a fetched-but-not-yet-stored article.
type candidate struct {
	Title      string
	Text       string // description or abstract
	SourceName string
	SourceURL  string
	PubDate    time.Time
}

func collectCandidates(ctx context.Context, db *ent.Client, sum *summarizer.Client) []candidate {
	var all []candidate

	// RSS feeds (fetch without summarizing)
	rssIngestor := articles.NewRSSIngestor(db, articles.DefaultFeeds(), nil) // nil summarizer = collect only
	rssCandidates := rssIngestor.CollectCandidates(ctx)
	all = append(all, toCandidates(rssCandidates)...)

	// PubMed
	ncbiKey := os.Getenv("NCBI_API_KEY")
	pubmedIngestor := articles.NewPubMedIngestor(db, ncbiKey, nil)
	pmCandidates := pubmedIngestor.CollectCandidates(ctx)
	all = append(all, toCandidates(pmCandidates)...)

	return all
}

func toCandidates(raw []articles.RawCandidate) []candidate {
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
		log.Printf("Curation failed: %v — falling back to first 5", err)
		selected = []int{}
		for i := 0; i < len(candidates) && i < 5; i++ {
			selected = append(selected, i)
		}
	}

	log.Printf("Curated %d articles from %d candidates", len(selected), len(candidates))

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
	log.Printf("Ingested %d curated articles", ingested)
}

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

	// Summarize with LLM
	if sum != nil && sum.IsConfigured() {
		text := c.Text
		if text == "" {
			text = c.Title
		}
		result, err := sum.Summarize(ctx, c.Title, text)
		if err != nil {
			log.Printf("Summarize failed for '%s': %v", truncate(c.Title, 50), err)
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
		log.Printf("Save failed for '%s': %v", truncate(c.Title, 50), err)
		return false
	}
	return true
}

func applyRetentionPolicy(ctx context.Context, db *ent.Client) {
	now := time.Now()

	// Standard sources: 7-day retention
	weekAgo := now.AddDate(0, 0, -7)
	standardCount, err := db.Article.Update().
		Where(
			entarticle.PublishedAtLT(weekAgo),
			entarticle.IsActive(true),
			// Exclude premium sources by matching standard ones
			entarticle.SourceNameNotIn(premiumSourceList()...),
		).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		log.Printf("Standard retention cleanup failed: %v", err)
	} else if standardCount > 0 {
		log.Printf("Deactivated %d standard articles older than 7 days", standardCount)
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
		log.Printf("Premium retention cleanup failed: %v", err)
	} else if premiumCount > 0 {
		log.Printf("Deactivated %d premium articles older than 90 days", premiumCount)
	}
}

// deactivatePaywalled sets is_active=false for articles from paywalled sources
// that were ingested before those sources were removed from the feed list.
func deactivatePaywalled(ctx context.Context, db *ent.Client) {
	paywallSources := []string{"STAT News", "FiercePharma"}
	count, err := db.Article.Update().
		Where(
			entarticle.SourceNameIn(paywallSources...),
			entarticle.IsActive(true),
		).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		log.Printf("Failed to deactivate paywalled articles: %v", err)
		return
	}
	if count > 0 {
		log.Printf("Deactivated %d articles from paywalled sources (STAT News, FiercePharma)", count)
	}
}

func premiumSourceList() []string {
	list := make([]string, 0, len(premiumSources))
	for s := range premiumSources {
		list = append(list, s)
	}
	return list
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
