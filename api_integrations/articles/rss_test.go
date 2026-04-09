package articles

import (
	"testing"
	"time"
)

func TestParseRSS_ValidRSS(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <item>
      <title>FDA Approves New Drug</title>
      <link>https://fda.gov/news/123</link>
      <description>The FDA approved a new medication today.</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>Drug Safety Alert</title>
      <link>https://fda.gov/safety/456</link>
      <description>&lt;p&gt;Important safety information.&lt;/p&gt;</description>
      <pubDate>Tue, 03 Jan 2006 10:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`)

	items, err := parseRSS(xml)
	if err != nil {
		t.Fatalf("parseRSS failed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].Title != "FDA Approves New Drug" {
		t.Errorf("expected first title 'FDA Approves New Drug', got '%s'", items[0].Title)
	}
	if items[0].Link != "https://fda.gov/news/123" {
		t.Errorf("expected first link, got '%s'", items[0].Link)
	}
	if items[1].Title != "Drug Safety Alert" {
		t.Errorf("expected second title 'Drug Safety Alert', got '%s'", items[1].Title)
	}
}

func TestParseRSS_EmptyFeed(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
  </channel>
</rss>`)

	items, err := parseRSS(xml)
	if err != nil {
		t.Fatalf("parseRSS failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseRSS_InvalidXML(t *testing.T) {
	xml := []byte(`this is not valid xml`)

	_, err := parseRSS(xml)
	if err == nil {
		t.Error("expected error for invalid XML, got nil")
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantYear int
		wantMonth time.Month
		wantDay  int
	}{
		{
			name:      "RFC1123Z",
			input:     "Mon, 02 Jan 2006 15:04:05 -0700",
			wantYear:  2006,
			wantMonth: time.January,
			wantDay:   2,
		},
		{
			name:      "RFC1123",
			input:     "Mon, 02 Jan 2006 15:04:05 MST",
			wantYear:  2006,
			wantMonth: time.January,
			wantDay:   2,
		},
		{
			name:      "RFC3339",
			input:     "2025-04-01T14:30:00Z",
			wantYear:  2025,
			wantMonth: time.April,
			wantDay:   1,
		},
		{
			name:      "single digit day",
			input:     "Tue, 3 Jan 2006 10:00:00 -0700",
			wantYear:  2006,
			wantMonth: time.January,
			wantDay:   3,
		},
		{
			name:      "ISO date with time and Z",
			input:     "2006-01-02T15:04:05Z",
			wantYear:  2006,
			wantMonth: time.January,
			wantDay:   2,
		},
		{
			name:      "date only",
			input:     "2025-12-31",
			wantYear:  2025,
			wantMonth: time.December,
			wantDay:   31,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDate(tt.input)
			if got.Year() != tt.wantYear {
				t.Errorf("parseDate(%q) year = %d, want %d", tt.input, got.Year(), tt.wantYear)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("parseDate(%q) month = %v, want %v", tt.input, got.Month(), tt.wantMonth)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("parseDate(%q) day = %d, want %d", tt.input, got.Day(), tt.wantDay)
			}
		})
	}
}

func TestParseDate_InvalidFallsBackToNow(t *testing.T) {
	before := time.Now().Add(-time.Second)
	got := parseDate("not-a-date")
	after := time.Now().Add(time.Second)

	if got.Before(before) || got.After(after) {
		t.Errorf("parseDate('not-a-date') = %v, expected to be approximately now", got)
	}
}

func TestParseDate_WhitespaceHandling(t *testing.T) {
	got := parseDate("  2025-04-01  ")
	if got.Year() != 2025 || got.Month() != time.April || got.Day() != 1 {
		t.Errorf("parseDate with whitespace returned %v, expected 2025-04-01", got)
	}
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no tags",
			input: "Hello, world!",
			want:  "Hello, world!",
		},
		{
			name:  "simple tags",
			input: "<p>Hello, <b>world</b>!</p>",
			want:  "Hello, world!",
		},
		{
			name:  "nested tags",
			input: "<div><p>Some <em>formatted</em> text</p></div>",
			want:  "Some formatted text",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only tags",
			input: "<br/><hr/>",
			want:  "",
		},
		{
			name:  "whitespace handling",
			input: "  <p>  trimmed  </p>  ",
			want:  "trimmed",
		},
		{
			name:  "HTML entities remain",
			input: "<p>Drug &amp; Safety</p>",
			want:  "Drug &amp; Safety",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanHTML(tt.input)
			if got != tt.want {
				t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
		want   string
	}{
		{
			name:   "shorter than limit",
			input:  "Hello",
			length: 10,
			want:   "Hello",
		},
		{
			name:   "exact limit",
			input:  "Hello",
			length: 5,
			want:   "Hello",
		},
		{
			name:   "longer than limit",
			input:  "Hello, world!",
			length: 5,
			want:   "Hello...",
		},
		{
			name:   "empty string",
			input:  "",
			length: 5,
			want:   "",
		},
		{
			name:   "zero limit",
			input:  "Hello",
			length: 0,
			want:   "Hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.length)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.length, got, tt.want)
			}
		})
	}
}
