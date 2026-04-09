package articles

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kyanaman/formularycheck/ent"
	entarticle "github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/internal/summarizer"
)

// RawCandidate is a fetched-but-not-yet-stored article candidate.
type RawCandidate struct {
	Title      string
	Text       string // description or abstract
	SourceName string
	SourceURL  string
	PubDate    time.Time
}

// Feed defines an RSS source to ingest.
type Feed struct {
	Name string // Display name (e.g., "NEJM", "STAT News")
	URL  string // RSS feed URL
}

// DefaultFeeds returns the built-in list of medical journal/news RSS feeds.
func DefaultFeeds() []Feed {
	return []Feed{
		{Name: "FDA Safety", URL: "https://www.fda.gov/about-fda/contact-fda/stay-informed/rss-feeds/drug-safety-communications/rss.xml"},
		{Name: "STAT News", URL: "https://www.statnews.com/feed/"},
		{Name: "FiercePharma", URL: "https://www.fiercepharma.com/rss/xml"},
		{Name: "Healio Pharmacy", URL: "https://www.healio.com/rss/pharmacy"},
	}
}

// RSSIngestor fetches articles from multiple RSS feeds.
type RSSIngestor struct {
	db         *ent.Client
	feeds      []Feed
	summarizer *summarizer.Client
	http       *http.Client
}

// NewRSSIngestor creates a new RSS ingestor for the given feeds.
func NewRSSIngestor(db *ent.Client, feeds []Feed, sum *summarizer.Client) *RSSIngestor {
	return &RSSIngestor{
		db:         db,
		feeds:      feeds,
		summarizer: sum,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

// CollectCandidates fetches articles from all feeds without storing them.
// Used by the curation pipeline to gather candidates for GPT selection.
func (r *RSSIngestor) CollectCandidates(ctx context.Context) []RawCandidate {
	var all []RawCandidate

	for _, feed := range r.feeds {
		candidates, err := r.fetchFeedCandidates(ctx, feed)
		if err != nil {
			log.Printf("RSS [%s]: failed: %v", feed.Name, err)
			continue
		}
		all = append(all, candidates...)
		log.Printf("RSS [%s]: %d candidates", feed.Name, len(candidates))
	}

	return all
}

func (r *RSSIngestor) fetchFeedCandidates(ctx context.Context, feed Feed) ([]RawCandidate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "PlanScanRx/1.0 (medical-news-aggregator)")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	items, err := parseRSS(body)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var candidates []RawCandidate
	for _, item := range items {
		if item.Link == "" || item.Title == "" {
			continue
		}

		// Skip if already in DB
		exists, _ := r.db.Article.Query().
			Where(entarticle.SourceURL(item.Link)).
			Exist(ctx)
		if exists {
			continue
		}

		candidates = append(candidates, RawCandidate{
			Title:      item.Title,
			Text:       cleanHTML(item.Description),
			SourceName: feed.Name,
			SourceURL:  item.Link,
			PubDate:    parseDate(item.PubDate),
		})
	}

	return candidates, nil
}

// Run fetches and ingests articles from all configured RSS feeds.
func (r *RSSIngestor) Run(ctx context.Context) error {
	var totalCreated, totalSkipped int

	for _, feed := range r.feeds {
		created, skipped, err := r.processFeed(ctx, feed)
		if err != nil {
			log.Printf("RSS [%s]: failed: %v", feed.Name, err)
			continue
		}
		totalCreated += created
		totalSkipped += skipped
		log.Printf("RSS [%s]: %d created, %d skipped", feed.Name, created, skipped)
	}

	log.Printf("RSS total: %d created, %d skipped across %d feeds", totalCreated, totalSkipped, len(r.feeds))
	return nil
}

func (r *RSSIngestor) processFeed(ctx context.Context, feed Feed) (created, skipped int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "PlanScanRx/1.0 (medical-news-aggregator)")

	resp, err := r.http.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("read body: %w", err)
	}

	items, err := parseRSS(body)
	if err != nil {
		return 0, 0, fmt.Errorf("parse: %w", err)
	}

	for _, item := range items {
		if item.Link == "" || item.Title == "" {
			continue
		}

		// Deduplicate by source URL
		exists, _ := r.db.Article.Query().
			Where(entarticle.SourceURL(item.Link)).
			Exist(ctx)
		if exists {
			skipped++
			continue
		}

		pubDate := parseDate(item.PubDate)

		builder := r.db.Article.Create().
			SetTitle(item.Title).
			SetSourceName(feed.Name).
			SetSourceURL(item.Link).
			SetPublishedAt(pubDate).
			SetIsActive(true)

		// Use RSS description as initial summary
		desc := cleanHTML(item.Description)
		if desc != "" {
			builder = builder.SetSummary(desc)
		}

		// Summarize with LLM if configured (enriches summary + extracts drug classes)
		if r.summarizer != nil && r.summarizer.IsConfigured() {
			text := desc
			if text == "" {
				text = item.Title
			}
			result, err := r.summarizer.Summarize(ctx, item.Title, text)
			if err != nil {
				log.Printf("RSS [%s]: summarize failed for '%s': %v", feed.Name, truncate(item.Title, 50), err)
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
			log.Printf("RSS [%s]: save failed: %v", feed.Name, err)
			continue
		}
		created++
	}

	return created, skipped, nil
}

// --- RSS XML parsing ---

type rssDoc struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssEntry `xml:"item"`
}

type rssEntry struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// parseRSS handles both RSS 2.0 and Atom-like feeds.
func parseRSS(data []byte) ([]rssEntry, error) {
	var doc rssDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.Channel.Items, nil
}

// --- helpers ---

func parseDate(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Now()
}

// cleanHTML strips basic HTML tags from RSS descriptions.
func cleanHTML(s string) string {
	// Simple tag stripper — sufficient for RSS descriptions
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
