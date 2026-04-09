package articles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	entarticle "github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/internal/summarizer"
)

const eSearchURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi"
const eSummaryURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi"

// PubMedIngestor fetches recent drug formulary articles from PubMed.
type PubMedIngestor struct {
	db         *ent.Client
	apiKey     string
	summarizer *summarizer.Client
}

// NewPubMedIngestor creates a new PubMed article ingestor.
func NewPubMedIngestor(db *ent.Client, apiKey string, sum *summarizer.Client) *PubMedIngestor {
	return &PubMedIngestor{db: db, apiKey: apiKey, summarizer: sum}
}

// CollectCandidates fetches PubMed articles without storing them.
func (p *PubMedIngestor) CollectCandidates(ctx context.Context) []RawCandidate {
	var all []RawCandidate

	queries := []string{
		"drug formulary coverage policy",
		"prior authorization medication",
		"new drug approval FDA",
		"clinical practice guideline pharmacotherapy",
		"drug safety update",
	}

	for _, query := range queries {
		ids, err := p.search(ctx, query)
		if err != nil {
			log.Printf("PubMed search failed for '%s': %v", query, err)
			continue
		}

		candidates, err := p.fetchCandidates(ctx, ids)
		if err != nil {
			log.Printf("PubMed fetch candidates failed: %v", err)
			continue
		}
		all = append(all, candidates...)
	}

	log.Printf("PubMed: %d candidates collected", len(all))
	return all
}

func (p *PubMedIngestor) fetchCandidates(ctx context.Context, ids []string) ([]RawCandidate, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	params := url.Values{
		"db":      {"pubmed"},
		"id":      {strings.Join(ids, ",")},
		"retmode": {"json"},
	}
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	resp, err := http.Get(eSummaryURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var candidates []RawCandidate
	for _, id := range ids {
		raw, ok := result.Result[id]
		if !ok {
			continue
		}

		var summary struct {
			Title   string `json:"title"`
			PubDate string `json:"pubdate"`
			Source  string `json:"source"`
		}
		if err := json.Unmarshal(raw, &summary); err != nil {
			continue
		}

		sourceURL := fmt.Sprintf("https://pubmed.ncbi.nlm.nih.gov/%s/", id)

		// Skip if already in DB
		exists, _ := p.db.Article.Query().
			Where(entarticle.SourceURL(sourceURL)).
			Exist(ctx)
		if exists {
			continue
		}

		pubDate, parseErr := time.Parse("2006 Jan 2", summary.PubDate)
		if parseErr != nil {
			pubDate, parseErr = time.Parse("2006 Jan", summary.PubDate)
			if parseErr != nil {
				pubDate = time.Now()
			}
		}

		sourceName := "PubMed"
		if summary.Source != "" {
			sourceName = summary.Source
		}

		candidates = append(candidates, RawCandidate{
			Title:      summary.Title,
			Text:       summary.Title, // PubMed ESummary doesn't include abstract
			SourceName: sourceName,
			SourceURL:  sourceURL,
			PubDate:    pubDate,
		})
	}

	return candidates, nil
}

// Run fetches recent PubMed articles about formulary and drug coverage.
func (p *PubMedIngestor) Run(ctx context.Context) error {
	log.Println("Fetching PubMed articles...")

	queries := []string{
		"drug formulary coverage policy",
		"prior authorization medication",
		"new drug approval FDA",
		"clinical practice guideline pharmacotherapy",
		"drug safety update",
	}

	var totalCreated, totalSkipped int
	for _, query := range queries {
		ids, err := p.search(ctx, query)
		if err != nil {
			log.Printf("PubMed search failed for '%s': %v", query, err)
			continue
		}

		created, skipped, err := p.fetchAndStore(ctx, ids)
		if err != nil {
			log.Printf("PubMed fetch failed: %v", err)
			continue
		}
		totalCreated += created
		totalSkipped += skipped
	}

	log.Printf("PubMed articles: %d created, %d skipped", totalCreated, totalSkipped)
	return nil
}

func (p *PubMedIngestor) search(ctx context.Context, query string) ([]string, error) {
	params := url.Values{
		"db":       {"pubmed"},
		"term":     {query},
		"retmax":   {"10"},
		"retmode":  {"json"},
		"sort":     {"date"},
		"datetype": {"pdat"},
		"reldate":  {"30"},
	}
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	resp, err := http.Get(eSearchURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.ESearchResult.IDList, nil
}

func (p *PubMedIngestor) fetchAndStore(ctx context.Context, ids []string) (created, skipped int, err error) {
	if len(ids) == 0 {
		return 0, 0, nil
	}

	params := url.Values{
		"db":      {"pubmed"},
		"id":      {strings.Join(ids, ",")},
		"retmode": {"json"},
	}
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	resp, err := http.Get(eSummaryURL + "?" + params.Encode())
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, err
	}

	for _, id := range ids {
		raw, ok := result.Result[id]
		if !ok {
			continue
		}

		var summary struct {
			Title   string `json:"title"`
			PubDate string `json:"pubdate"`
			Source  string `json:"source"`
		}
		if err := json.Unmarshal(raw, &summary); err != nil {
			continue
		}

		sourceURL := fmt.Sprintf("https://pubmed.ncbi.nlm.nih.gov/%s/", id)

		// Deduplicate by source URL
		exists, _ := p.db.Article.Query().
			Where(entarticle.SourceURL(sourceURL)).
			Exist(ctx)
		if exists {
			skipped++
			continue
		}

		pubDate, parseErr := time.Parse("2006 Jan 2", summary.PubDate)
		if parseErr != nil {
			pubDate, parseErr = time.Parse("2006 Jan", summary.PubDate)
			if parseErr != nil {
				pubDate = time.Now()
			}
		}

		sourceName := "PubMed"
		if summary.Source != "" {
			sourceName = summary.Source
		}

		builder := p.db.Article.Create().
			SetTitle(summary.Title).
			SetSourceName(sourceName).
			SetSourceURL(sourceURL).
			SetPublishedAt(pubDate).
			SetIsActive(true)

		// Summarize with LLM if configured
		if p.summarizer != nil && p.summarizer.IsConfigured() {
			res, sumErr := p.summarizer.Summarize(ctx, summary.Title, summary.Title)
			if sumErr != nil {
				log.Printf("PubMed: summarize failed for PMID %s: %v", id, sumErr)
			} else {
				if res.Summary != "" {
					builder = builder.SetSummary(res.Summary)
				}
				if len(res.DrugClasses) > 0 {
					builder = builder.SetDrugClasses(res.DrugClasses)
				}
			}
		}

		if _, saveErr := builder.Save(ctx); saveErr != nil {
			log.Printf("PubMed: save failed for PMID %s: %v", id, saveErr)
			continue
		}
		created++
	}

	return created, skipped, nil
}
