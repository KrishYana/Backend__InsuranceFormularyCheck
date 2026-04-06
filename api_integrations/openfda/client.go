package openfda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const baseURL = "https://api.fda.gov"

// Client is an HTTP client for the openFDA API.
type Client struct {
	http    *http.Client
	limiter *rate.Limiter
	apiKey  string
}

// NewClient creates a new openFDA API client.
// apiKey is optional — without it, rate limits are lower (240/min, 1K/day).
func NewClient(apiKey string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(4), 1), // 4 req/sec to stay well under 240/min
		apiKey:  apiKey,
	}
}

// SearchNDC queries the NDC directory endpoint.
func (c *Client) SearchNDC(ctx context.Context, query string, limit, skip int) (*NDCResponse, error) {
	path := fmt.Sprintf("/drug/ndc.json?search=%s&limit=%d&skip=%d", query, limit, skip)
	if c.apiKey != "" {
		path = fmt.Sprintf("/drug/ndc.json?api_key=%s&search=%s&limit=%d&skip=%d", c.apiKey, query, limit, skip)
	}

	var resp NDCResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// LookupByNDC fetches a single product by its NDC code.
func (c *Client) LookupByNDC(ctx context.Context, ndc string) (*NDCRecord, error) {
	resp, err := c.SearchNDC(ctx, fmt.Sprintf("product_ndc:\"%s\"", ndc), 1, 0)
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("no results for NDC %s", ndc)
	}
	return &resp.Results[0], nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
