package cmspuf

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// targetFiles are the inner ZIP filename patterns we want to extract.
var targetFiles = []string{
	"plan information",
	"basic drugs formulary",
	"beneficiary cost",
}

// Downloader handles downloading and extracting CMS PUF files.
type Downloader struct {
	http *http.Client
	url  string
}

// NewDownloader creates a new PUF downloader.
// url should point to the SPUF ZIP file on data.cms.gov.
func NewDownloader(url string) *Downloader {
	return &Downloader{
		http: &http.Client{Timeout: 30 * time.Minute},
		url:  url,
	}
}

// ExtractedFiles holds the raw content of the three files we need.
type ExtractedFiles struct {
	PlanInfo       []byte
	FormularyDrugs []byte
	BenefitCost    []byte
}

// CheckFingerprint sends a HEAD request to the PUF URL and returns a fingerprint
// string (Last-Modified, ETag, or Content-Length) that can be compared across runs
// to avoid re-downloading the ~2.3GB ZIP if unchanged.
func (d *Downloader) CheckFingerprint(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, d.url, nil)
	if err != nil {
		return "", fmt.Errorf("create HEAD request: %w", err)
	}

	resp, err := d.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("HEAD request: %w", err)
	}
	defer resp.Body.Close()

	// Some CDNs don't support HEAD, fall back gracefully
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Try Last-Modified first, then ETag, then Content-Length
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		return lm, nil
	}
	if etag := resp.Header.Get("ETag"); etag != "" {
		return etag, nil
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		return "size:" + cl, nil
	}

	return "", fmt.Errorf("no Last-Modified, ETag, or Content-Length header available")
}

// Download fetches the PUF ZIP and extracts only the target inner ZIPs.
func (d *Downloader) Download(ctx context.Context) (*ExtractedFiles, error) {
	slog.Info("downloading PUF", "url", d.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	slog.Info("downloaded PUF, extracting target files", "size_mb", len(body)/(1024*1024))

	return d.extractTargetFiles(body)
}

func (d *Downloader) extractTargetFiles(data []byte) (*ExtractedFiles, error) {
	outerZip, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open outer zip: %w", err)
	}

	result := &ExtractedFiles{}

	for _, f := range outerZip.File {
		nameLower := strings.ToLower(f.Name)

		var target *[]byte
		for _, pattern := range targetFiles {
			if strings.Contains(nameLower, pattern) {
				switch {
				case strings.Contains(pattern, "plan information"):
					target = &result.PlanInfo
				case strings.Contains(pattern, "basic drugs"):
					target = &result.FormularyDrugs
				case strings.Contains(pattern, "beneficiary cost"):
					target = &result.BenefitCost
				}
				break
			}
		}

		if target == nil {
			continue
		}

		slog.Info("extracting file", "name", f.Name)

		content, err := extractInnerZip(f)
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", f.Name, err)
		}

		*target = content
	}

	if result.PlanInfo == nil {
		return nil, fmt.Errorf("plan information file not found in ZIP")
	}
	if result.FormularyDrugs == nil {
		return nil, fmt.Errorf("basic drugs formulary file not found in ZIP")
	}
	if result.BenefitCost == nil {
		return nil, fmt.Errorf("beneficiary cost file not found in ZIP")
	}

	return result, nil
}

// extractInnerZip opens an inner ZIP file and returns the content of the first file inside.
func extractInnerZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	innerData, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	// If this is a ZIP inside the ZIP, extract its first file
	if strings.HasSuffix(strings.ToLower(f.Name), ".zip") {
		innerZip, err := zip.NewReader(bytes.NewReader(innerData), int64(len(innerData)))
		if err != nil {
			return nil, fmt.Errorf("open inner zip: %w", err)
		}

		for _, inner := range innerZip.File {
			irc, err := inner.Open()
			if err != nil {
				return nil, err
			}
			defer irc.Close()

			return io.ReadAll(irc)
		}

		return nil, fmt.Errorf("inner zip is empty")
	}

	return innerData, nil
}
