package articles

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/kyanaman/formularycheck/ent"
)

const fdaRSSURL = "https://www.fda.gov/about-fda/contact-fda/stay-informed/rss-feeds/drug-safety-communications/rss.xml"

// FDAIngestor fetches FDA Drug Safety Communications via RSS.
type FDAIngestor struct {
	db *ent.Client
}

// NewFDAIngestor creates a new FDA article ingestor.
func NewFDAIngestor(db *ent.Client) *FDAIngestor {
	return &FDAIngestor{db: db}
}

type rssResponse struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// Run fetches and ingests FDA Drug Safety Communications.
func (f *FDAIngestor) Run(ctx context.Context) error {
	slog.Info("fetching FDA Drug Safety Communications")

	resp, err := http.Get(fdaRSSURL)
	if err != nil {
		return fmt.Errorf("fetch FDA RSS: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read FDA RSS body: %w", err)
	}

	var rss rssResponse
	if err := xml.Unmarshal(body, &rss); err != nil {
		return fmt.Errorf("parse FDA RSS: %w", err)
	}

	var created, skipped int
	for _, item := range rss.Channel.Items {
		pubDate, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			pubDate = time.Now()
		}

		// Check for existing article by source URL
		exists, _ := f.db.Article.Query().
			Where(/* article.SourceURL(item.Link) -- generated predicate */).
			Exist(ctx)
		if exists {
			skipped++
			continue
		}

		_, err = f.db.Article.Create().
			SetTitle(item.Title).
			SetSummary(item.Description).
			SetSourceName("FDA").
			SetSourceURL(item.Link).
			SetPublishedAt(pubDate).
			SetIsActive(true).
			Save(ctx)
		if err != nil {
			slog.Error("failed to create FDA article", "error", err)
			continue
		}
		created++
	}

	slog.Info("FDA articles ingested", "created", created, "skipped", skipped)
	return nil
}
