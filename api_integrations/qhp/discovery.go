package qhp

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// DiscoverIssuers downloads the MR-PUF CSV and returns the list of issuers.
func DiscoverIssuers(ctx context.Context, mrpufURL string) ([]Issuer, error) {
	log.Printf("Downloading MR-PUF from %s...", mrpufURL)

	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mrpufURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download MR-PUF: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return parseIssuerCSV(resp.Body)
}

func parseIssuerCSV(r io.Reader) ([]Issuer, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Normalize headers: uppercase, strip spaces and underscores for flexible matching
	colIndex := make(map[string]int)
	for i, col := range header {
		normalized := strings.TrimSpace(strings.ToUpper(col))
		colIndex[normalized] = i
		// Also store with underscores replaced by spaces and vice versa
		colIndex[strings.ReplaceAll(normalized, " ", "_")] = i
		colIndex[strings.ReplaceAll(normalized, "_", " ")] = i
	}

	var issuers []Issuer
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Try both naming conventions for the URL column
		url := getCSVCol(record, colIndex, "MR_URL_SUBMITTED")
		if url == "" {
			url = getCSVCol(record, colIndex, "URL_SUBMITTED")
		}
		if url == "" {
			url = getCSVCol(record, colIndex, "URL SUBMITTED")
		}
		if url == "" {
			continue
		}

		state := getCSVCol(record, colIndex, "STATE")
		issuerID := getCSVCol(record, colIndex, "ISSUER_ID")
		if issuerID == "" {
			issuerID = getCSVCol(record, colIndex, "ISSUER ID")
		}
		email := getCSVCol(record, colIndex, "TECH_POC_EMAIL")
		if email == "" {
			email = getCSVCol(record, colIndex, "TECH POC EMAIL")
		}

		issuers = append(issuers, Issuer{
			State:    state,
			IssuerID: issuerID,
			URL:      url,
			Email:    email,
		})
	}

	log.Printf("Discovered %d issuers from MR-PUF", len(issuers))
	return issuers, nil
}

func getCSVCol(record []string, index map[string]int, col string) string {
	i, ok := index[col]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}
