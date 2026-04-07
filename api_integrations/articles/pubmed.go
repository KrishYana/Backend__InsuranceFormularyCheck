package articles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/kyanaman/formularycheck/ent"
)

const eSearchURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi"
const eSummaryURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi"

// PubMedIngestor fetches recent drug formulary articles from PubMed.
type PubMedIngestor struct {
	db     *ent.Client
	apiKey string
}

// NewPubMedIngestor creates a new PubMed article ingestor.
func NewPubMedIngestor(db *ent.Client, apiKey string) *PubMedIngestor {
	return &PubMedIngestor{db: db, apiKey: apiKey}
}

// Run fetches recent PubMed articles about formulary and drug coverage.
func (p *PubMedIngestor) Run(ctx context.Context) error {
	log.Println("Fetching PubMed articles...")

	queries := []string{
		"drug formulary coverage policy",
		"prior authorization medication",
		"new drug approval FDA",
	}

	var totalCreated int
	for _, query := range queries {
		ids, err := p.search(ctx, query)
		if err != nil {
			log.Printf("PubMed search failed for '%s': %v", query, err)
			continue
		}

		created, err := p.fetchAndStore(ctx, ids)
		if err != nil {
			log.Printf("PubMed fetch failed: %v", err)
			continue
		}
		totalCreated += created
	}

	log.Printf("PubMed articles: %d created total", totalCreated)
	return nil
}

func (p *PubMedIngestor) search(ctx context.Context, query string) ([]string, error) {
	params := url.Values{
		"db":         {"pubmed"},
		"term":       {query},
		"retmax":     {"10"},
		"retmode":    {"json"},
		"sort":       {"date"},
		"datetype":   {"pdat"},
		"reldate":    {"30"}, // last 30 days
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

func (p *PubMedIngestor) fetchAndStore(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	// Fetch summaries for all IDs
	params := url.Values{
		"db":      {"pubmed"},
		"id":      ids,
		"retmode": {"json"},
	}
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	resp, err := http.Get(eSummaryURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	var created int
	for _, id := range ids {
		raw, ok := result.Result[id]
		if !ok {
			continue
		}

		var summary struct {
			Title      string `json:"title"`
			PubDate    string `json:"pubdate"`
			Source     string `json:"source"`
			SortTitle  string `json:"sorttitle"`
		}
		if err := json.Unmarshal(raw, &summary); err != nil {
			continue
		}

		pubDate, err := time.Parse("2006 Jan 2", summary.PubDate)
		if err != nil {
			pubDate = time.Now()
		}

		sourceURL := fmt.Sprintf("https://pubmed.ncbi.nlm.nih.gov/%s/", id)

		_, err = p.db.Article.Create().
			SetTitle(summary.Title).
			SetSourceName("PubMed").
			SetSourceURL(sourceURL).
			SetPublishedAt(pubDate).
			SetIsActive(true).
			Save(ctx)
		if err != nil {
			continue
		}
		created++
	}

	return created, nil
}
