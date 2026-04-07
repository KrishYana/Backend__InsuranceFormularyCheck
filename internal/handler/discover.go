package handler

import (
	"net/http"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetArticles handles GET /discover/articles.
// Returns personalized articles based on physician's search history drug classes.
// Falls back to most recent articles for guests or new users.
func (h *Handler) GetArticles(w http.ResponseWriter, r *http.Request) {
	articles, err := h.db.Article.Query().
		Where(article.IsActive(true)).
		Order(ent.Desc(article.FieldPublishedAt)).
		Limit(30).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// TODO: Personalization — when physician is in context, fetch their
	// most-searched drug classes from search_history and score/sort articles
	// by drug_class overlap using Postgres && array operator.
	// For now, return chronologically sorted articles.

	result := make([]dto.ArticleDTO, len(articles))
	for i, a := range articles {
		result[i] = dto.ArticleFromEnt(a)
	}

	response.JSON(w, http.StatusOK, result)
}
