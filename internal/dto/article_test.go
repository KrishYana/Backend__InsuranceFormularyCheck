package dto

import (
	"testing"
	"time"

	"github.com/kyanaman/formularycheck/ent"
)

func TestArticleFromEnt_FullFields(t *testing.T) {
	pubDate := time.Date(2025, 4, 1, 14, 30, 0, 0, time.UTC)
	article := &ent.Article{
		ID:          1,
		Title:       "FDA Approves New Diabetes Drug",
		Summary:     "The FDA has approved a new GLP-1 receptor agonist for type 2 diabetes.",
		SourceName:  "FDA",
		SourceURL:   "https://fda.gov/news/12345",
		PublishedAt: pubDate,
		DrugClasses: []string{"GLP-1", "Antidiabetic"},
		ImageURL:    "https://fda.gov/images/12345.jpg",
	}

	dto := ArticleFromEnt(article)

	if dto.ArticleID != 1 {
		t.Errorf("expected ArticleID 1, got %d", dto.ArticleID)
	}
	if dto.Title != "FDA Approves New Diabetes Drug" {
		t.Errorf("expected Title, got '%s'", dto.Title)
	}
	if dto.Summary == nil {
		t.Fatal("expected Summary non-nil")
	}
	if *dto.Summary != "The FDA has approved a new GLP-1 receptor agonist for type 2 diabetes." {
		t.Errorf("unexpected Summary: '%s'", *dto.Summary)
	}
	if dto.SourceName != "FDA" {
		t.Errorf("expected SourceName 'FDA', got '%s'", dto.SourceName)
	}
	if dto.SourceUrl != "https://fda.gov/news/12345" {
		t.Errorf("expected SourceUrl, got '%s'", dto.SourceUrl)
	}
	if dto.PublishedAt != "2025-04-01T14:30:00Z" {
		t.Errorf("expected PublishedAt '2025-04-01T14:30:00Z', got '%s'", dto.PublishedAt)
	}
	if len(dto.DrugClasses) != 2 {
		t.Errorf("expected 2 DrugClasses, got %d", len(dto.DrugClasses))
	}
	if dto.ImageUrl == nil || *dto.ImageUrl != "https://fda.gov/images/12345.jpg" {
		t.Errorf("expected ImageUrl, got %v", dto.ImageUrl)
	}
}

func TestArticleFromEnt_EmptyOptionals(t *testing.T) {
	pubDate := time.Date(2025, 3, 20, 10, 0, 0, 0, time.UTC)
	article := &ent.Article{
		ID:          2,
		Title:       "Formulary Update",
		SourceName:  "STAT News",
		SourceURL:   "https://statnews.com/article/456",
		PublishedAt: pubDate,
	}

	dto := ArticleFromEnt(article)

	if dto.ArticleID != 2 {
		t.Errorf("expected ArticleID 2, got %d", dto.ArticleID)
	}
	if dto.Summary != nil {
		t.Errorf("expected Summary nil for empty, got %v", dto.Summary)
	}
	if dto.ImageUrl != nil {
		t.Errorf("expected ImageUrl nil for empty, got %v", dto.ImageUrl)
	}
	if dto.DrugClasses != nil {
		t.Errorf("expected DrugClasses nil, got %v", dto.DrugClasses)
	}
}

func TestArticleFromEnt_DateFormatting(t *testing.T) {
	tests := []struct {
		name     string
		pubDate  time.Time
		expected string
	}{
		{
			name:     "midnight UTC",
			pubDate:  time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: "2025-01-15T00:00:00Z",
		},
		{
			name:     "end of day",
			pubDate:  time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "2025-12-31T23:59:59Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := &ent.Article{
				ID:          1,
				Title:       "Test",
				SourceName:  "Test",
				SourceURL:   "https://test.com",
				PublishedAt: tt.pubDate,
			}
			dto := ArticleFromEnt(article)
			if dto.PublishedAt != tt.expected {
				t.Errorf("expected PublishedAt '%s', got '%s'", tt.expected, dto.PublishedAt)
			}
		})
	}
}
