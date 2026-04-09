package qhp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	maxRetries     = 3
	requestTimeout = 60 * time.Second
	// hostRateLimit is the minimum interval between requests to the same host.
	// This prevents 429 Too Many Requests errors when multiple issuers share
	// the same CDN or hosting provider.
	hostRateLimit = 500 * time.Millisecond
)

// Crawler fetches JSON files from individual QHP issuers.
type Crawler struct {
	http        *http.Client
	hostLimiter sync.Map // map[string]time.Time — last request time per hostname
}

// NewCrawler creates a new QHP crawler.
func NewCrawler() *Crawler {
	return &Crawler{
		http: &http.Client{Timeout: requestTimeout},
	}
}

// waitForHost enforces per-host rate limiting. If a request was made to the
// same host within hostRateLimit, it sleeps until the interval has passed.
func (c *Crawler) waitForHost(rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return
	}
	host := parsed.Host

	now := time.Now()
	if lastVal, ok := c.hostLimiter.Load(host); ok {
		last := lastVal.(time.Time)
		elapsed := now.Sub(last)
		if elapsed < hostRateLimit {
			time.Sleep(hostRateLimit - elapsed)
		}
	}
	c.hostLimiter.Store(host, time.Now())
}

// CrawlResult holds the parsed data from a single issuer.
type CrawlResult struct {
	Issuer Issuer
	Plans  []PlanJSON
	Drugs  []DrugJSON
	Err    error
}

// CrawlIssuer fetches and parses all JSON files for a single issuer.
func (c *Crawler) CrawlIssuer(ctx context.Context, issuer Issuer) *CrawlResult {
	result := &CrawlResult{Issuer: issuer}

	// Fetch index.json
	var index IndexJSON
	if err := c.fetchJSON(ctx, issuer.URL, &index); err != nil {
		result.Err = fmt.Errorf("index.json: %w", err)
		return result
	}

	// Fetch plans
	for _, url := range index.PlanURLs {
		var plans []PlanJSON
		if err := c.fetchJSON(ctx, url, &plans); err != nil {
			result.Err = fmt.Errorf("plans.json (%s): %w", url, err)
			return result
		}
		result.Plans = append(result.Plans, plans...)
	}

	// Fetch drugs from all formulary URLs
	for _, url := range index.FormularyURLs {
		var drugs []DrugJSON
		if err := c.fetchJSON(ctx, url, &drugs); err != nil {
			// Log but continue — some drug files may fail while others succeed
			continue
		}
		result.Drugs = append(result.Drugs, drugs...)
	}

	return result
}

func (c *Crawler) fetchJSON(ctx context.Context, url string, target any) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}

		if err := c.doFetch(ctx, url, target); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func (c *Crawler) doFetch(ctx context.Context, url string, target any) error {
	// Enforce per-host rate limiting to avoid 429 errors
	c.waitForHost(url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	// Use streaming decoder for large files
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	return nil
}
