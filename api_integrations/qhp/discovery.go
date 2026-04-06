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

	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.TrimSpace(strings.ToUpper(col))] = i
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

		url := getCSVCol(record, colIndex, "MR_URL_SUBMITTED")
		if url == "" {
			continue
		}

		issuers = append(issuers, Issuer{
			State:    getCSVCol(record, colIndex, "STATE"),
			IssuerID: getCSVCol(record, colIndex, "ISSUER_ID"),
			URL:      url,
			Email:    getCSVCol(record, colIndex, "TECH_POC_EMAIL"),
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
