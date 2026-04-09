package openfda

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// CheckLastModified sends a HEAD request to the bulk download metadata endpoint
// and returns the Last-Modified header value. This can be compared against the
// stored value from the previous sync to determine if a re-download is needed.
func (b *BulkDownloader) CheckLastModified(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, bulkDownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create HEAD request: %w", err)
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("HEAD request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Try Last-Modified first, fall back to ETag
	lastModified := resp.Header.Get("Last-Modified")
	if lastModified != "" {
		return lastModified, nil
	}

	etag := resp.Header.Get("ETag")
	if etag != "" {
		return etag, nil
	}

	// If neither header is available, return content-length as a rough proxy
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		return "size:" + contentLength, nil
	}

	return "", fmt.Errorf("no Last-Modified, ETag, or Content-Length header available")
}

// FetchAllNDCRecords downloads all NDC partitions and returns parsed records.
func (b *BulkDownloader) FetchAllNDCRecords(ctx context.Context) ([]NDCRecord, error) {
	partitions, err := b.getPartitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get partitions: %w", err)
	}

	slog.Info("found NDC bulk download partitions", "count", len(partitions))

	var allRecords []NDCRecord
	for i, p := range partitions {
		slog.Info("downloading partition", "partition", i+1, "total", len(partitions), "name", p.DisplayName, "records", p.Records)

		records, err := b.downloadPartition(ctx, p.File)
		if err != nil {
			slog.Warn("failed to download partition", "name", p.DisplayName, "error", err)
			continue
		}

		allRecords = append(allRecords, records...)
		slog.Info("partition downloaded", "partition", i+1, "records", len(records), "total_so_far", len(allRecords))
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
