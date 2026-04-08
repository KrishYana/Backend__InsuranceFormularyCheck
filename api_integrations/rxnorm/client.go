package rxnorm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const baseURL = "https://rxnav.nlm.nih.gov"

// Client is an HTTP client for the RxNorm REST API.
type Client struct {
	http    *http.Client
	limiter *rate.Limiter
}

// NewClient creates a new RxNorm API client with rate limiting at 20 req/sec.
func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(20), 1),
	}
}

// get performs a rate-limited GET request and decodes the JSON response.
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

// GetAllConcepts fetches all drug concepts for the given term types (e.g., "SCD+SBD").
func (c *Client) GetAllConcepts(ctx context.Context, tty string) ([]MinConcept, error) {
	var resp AllConceptsResponse
	path := fmt.Sprintf("/REST/allconcepts.json?tty=%s", tty)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get all concepts: %w", err)
	}
	return resp.MinConceptGroup.MinConcept, nil
}

// GetProperties fetches full properties for a given RxCUI.
func (c *Client) GetProperties(ctx context.Context, rxcui string) (*ConceptProperties, error) {
	var resp PropertiesResponse
	path := fmt.Sprintf("/REST/rxcui/%s/properties.json", rxcui)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get properties for %s: %w", rxcui, err)
	}
	return resp.Properties, nil
}

// GetNDCs fetches all NDCs for a given RxCUI.
func (c *Client) GetNDCs(ctx context.Context, rxcui string) ([]string, error) {
	var resp NDCResponse
	path := fmt.Sprintf("/REST/rxcui/%s/ndcs.json", rxcui)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get NDCs for %s: %w", rxcui, err)
	}
	return resp.NDCGroup.NDCList.NDC, nil
}

// GetDrugClass fetches drug class information for a given RxCUI.
func (c *Client) GetDrugClass(ctx context.Context, rxcui string) ([]RxclassDrugInfo, error) {
	var resp ClassResponse
	path := fmt.Sprintf("/REST/rxclass/class/byRxcui.json?rxcui=%s", rxcui)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get drug class for %s: %w", rxcui, err)
	}
	return resp.RxclassDrugInfoList.RxclassDrugInfo, nil
}

// GetAllRelated fetches all related concepts (brand/generic links) for a given RxCUI.
func (c *Client) GetAllRelated(ctx context.Context, rxcui string) ([]ConceptGroup, error) {
	var resp AllRelatedResponse
	path := fmt.Sprintf("/REST/rxcui/%s/allrelated.json", rxcui)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get all related for %s: %w", rxcui, err)
	}
	return resp.AllRelatedGroup.ConceptGroup, nil
}

// GetNewConcepts fetches concepts added to RxNorm since the given date.
// Uses /REST/version/newConcepts.json?type=all endpoint.
// The date format expected by RxNorm is MMDDYYYY.
func (c *Client) GetNewConcepts(ctx context.Context, since time.Time) ([]MinConcept, error) {
	dateStr := since.Format("01022006") // MMDDYYYY
	var resp AllConceptsResponse
	path := fmt.Sprintf("/REST/version/newConcepts.json?type=all&startDate=%s", dateStr)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get new concepts since %s: %w", dateStr, err)
	}
	return resp.MinConceptGroup.MinConcept, nil
}
