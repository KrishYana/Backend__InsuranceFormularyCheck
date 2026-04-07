package dto

import "github.com/kyanaman/formularycheck/ent"

// ArticleDTO matches the frontend article shape for the Discover tab.
type ArticleDTO struct {
	ArticleID   int      `json:"articleId"`
	Title       string   `json:"title"`
	Summary     *string  `json:"summary"`
	SourceName  string   `json:"sourceName"`
	SourceUrl   string   `json:"sourceUrl"`
	PublishedAt string   `json:"publishedAt"`
	DrugClasses []string `json:"drugClasses"`
	ImageUrl    *string  `json:"imageUrl"`
}

// ArticleFromEnt converts an Ent Article to a DTO.
func ArticleFromEnt(a *ent.Article) ArticleDTO {
	dto := ArticleDTO{
		ArticleID:   a.ID,
		Title:       a.Title,
		SourceName:  a.SourceName,
		SourceUrl:   a.SourceURL,
		PublishedAt: a.PublishedAt.Format("2006-01-02T15:04:05Z"),
		DrugClasses: a.DrugClasses,
	}
	if a.Summary != "" {
		dto.Summary = &a.Summary
	}
	if a.ImageURL != "" {
		dto.ImageUrl = &a.ImageURL
	}
	return dto
}
