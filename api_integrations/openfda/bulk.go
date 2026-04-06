package openfda

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const bulkDownloadURL = "https://api.fda.gov/download.json"

// BulkDownloader handles downloading and parsing the full openFDA NDC dataset.
type BulkDownloader struct {
	http *http.Client
}

// NewBulkDownloader creates a new bulk downloader.
func NewBulkDownloader() *BulkDownloader {
	return &BulkDownloader{
		http: &http.Client{Timeout: 10 * time.Minute},
	}
}

// FetchAllNDCRecords downloads all NDC partitions and returns parsed records.
func (b *BulkDownloader) FetchAllNDCRecords(ctx context.Context) ([]NDCRecord, error) {
	partitions, err := b.getPartitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get partitions: %w", err)
	}

	log.Printf("Found %d NDC bulk download partitions", len(partitions))

	var allRecords []NDCRecord
	for i, p := range partitions {
		log.Printf("Downloading partition %d/%d (%s, %d records)...", i+1, len(partitions), p.DisplayName, p.Records)

		records, err := b.downloadPartition(ctx, p.File)
		if err != nil {
			log.Printf("WARN: failed to download partition %s: %v", p.DisplayName, err)
			continue
		}

		allRecords = append(allRecords, records...)
		log.Printf("Partition %d: got %d records (total so far: %d)", i+1, len(records), len(allRecords))
	}

	return allRecords, nil
}

func (b *BulkDownloader) getPartitions(ctx context.Context) ([]BulkPartition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bulkDownloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var meta BulkDownloadMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode download index: %w", err)
	}

	return meta.Results.Drug.NDC.Partitions, nil
}

func (b *BulkDownloader) downloadPartition(ctx context.Context, url string) ([]NDCRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Partitions are ZIP files containing a single JSON file
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open file in zip: %w", err)
		}
		defer rc.Close()

		var wrapper struct {
			Results []NDCRecord `json:"results"`
		}
		if err := json.NewDecoder(rc).Decode(&wrapper); err != nil {
			return nil, fmt.Errorf("decode JSON: %w", err)
		}

		return wrapper.Results, nil
	}

	return nil, fmt.Errorf("no files in zip")
}
