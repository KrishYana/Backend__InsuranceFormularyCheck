package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Result holds the extracted summary and drug classes from an article.
type Result struct {
	Summary     string   `json:"summary"`
	DrugClasses []string `json:"drug_classes"`
	Category    string   `json:"category"`
}

// Client wraps the OpenAI chat completions API for article summarization.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// New creates a summarizer client. Reads OPENAI_API_KEY from env if apiKey is empty.
// Model defaults to "gpt-5-mini" if not specified.
func New(apiKey, model string) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = "gpt-4o-mini" // fallback until gpt-5-mini is GA; swap when available
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   model,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// IsConfigured returns true if the API key is set.
func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

const systemPrompt = `You are a medical news summarizer for a physician-facing mobile app called PlanScanRx.
Given an article title and text (or abstract), return a JSON object with:
- "summary": A 2-sentence summary written for a busy physician. Be specific about drug names, conditions, and clinical implications.
- "drug_classes": An array of pharmacological drug class names mentioned or relevant (e.g., ["SGLT2 inhibitors", "GLP-1 receptor agonists"]). Use standard pharmacological class names. Return empty array if none apply.
- "category": Exactly one of: "fda_approval", "safety_alert", "formulary_change", "clinical_guideline", "drug_research", "industry_news"

Return ONLY valid JSON. No markdown, no explanation.`

// Summarize sends an article to the LLM and returns structured summary data.
func (c *Client) Summarize(ctx context.Context, title, text string) (*Result, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("OPENAI_API_KEY not configured")
	}

	userContent := fmt.Sprintf("Title: %s\n\nText: %s", title, text)
	// Truncate to ~6000 chars to stay within context limits for smaller models
	if len(userContent) > 6000 {
		userContent = userContent[:6000]
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Temperature:  0.3,
		MaxTokens:    300,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	var result Result
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal summary JSON: %w (raw: %s)", err, content)
	}

	return &result, nil
}

// Candidate represents an article candidate for curation.
type Candidate struct {
	Index  int    // position in the original list
	Title  string
	Source string
	Text   string // description or abstract
}

// CurationResult holds the indices of selected articles.
type CurationResult struct {
	Selected []int `json:"selected"`
}

const curationPrompt = `You are an editorial curator for PlanScanRx, a physician-facing mobile app.
Given a numbered list of medical article candidates, select the 3 to 5 MOST clinically relevant and impactful articles.

Prioritize:
1. FDA drug approvals, safety alerts, or recalls
2. Major clinical guideline changes
3. Significant drug formulary or coverage policy changes
4. Breakthrough clinical trial results
5. Drug class-level research with practice-changing implications

Deprioritize:
- Industry business news (mergers, earnings)
- Duplicates or near-duplicate topics (pick the best one)
- Overly narrow or niche topics unlikely to affect most physicians

Return a JSON object with a single key "selected" containing an array of the candidate numbers (integers) you chose, ordered by importance. Example: {"selected": [3, 7, 1]}

Return ONLY valid JSON. No markdown, no explanation.`

// Curate selects the top N articles from a pool of candidates using the LLM.
func (c *Client) Curate(ctx context.Context, candidates []Candidate) ([]int, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("OPENAI_API_KEY not configured")
	}

	if len(candidates) <= 5 {
		// If we have 5 or fewer candidates, just return all of them
		indices := make([]int, len(candidates))
		for i, cand := range candidates {
			indices[i] = cand.Index
		}
		return indices, nil
	}

	// Build numbered list for the LLM
	var listing string
	for _, cand := range candidates {
		desc := cand.Text
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		listing += fmt.Sprintf("%d. [%s] %s\n   %s\n\n", cand.Index, cand.Source, cand.Title, desc)
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: curationPrompt},
			{Role: "user", Content: listing},
		},
		Temperature:    0.2,
		MaxTokens:      100,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	var result CurationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal curation JSON: %w (raw: %s)", err, content)
	}

	return result.Selected, nil
}

// --- OpenAI API types ---

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}
