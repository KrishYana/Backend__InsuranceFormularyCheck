package handler

import (
	"context"
	"net/http"
	"sort"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/article"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/ent/searchhistory"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetArticles handles GET /discover/articles.
// For authenticated physicians: fetches their most-searched drug classes from
// search_history, then scores and sorts articles by drug_class overlap.
// Falls back to chronological order for guests or physicians with no history.
func (h *Handler) GetArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	allArticles, err := h.db.Article.Query().
		Where(article.IsActive(true)).
		Order(ent.Desc(article.FieldPublishedAt)).
		Limit(50).
		All(ctx)
	if err != nil {
		response.InternalError(w)
		return
	}

	// Try to personalize if physician is in context
	phys, ok := middleware.PhysicianFromCtx(ctx)
	if ok && phys != nil {
		classes := h.getPhysicianDrugClasses(ctx, phys.ID)
		if len(classes) > 0 {
			allArticles = scoreAndSort(allArticles, classes)
		}
	}

	// Cap at 30 after scoring
	if len(allArticles) > 30 {
		allArticles = allArticles[:30]
	}

	result := make([]dto.ArticleDTO, len(allArticles))
	for i, a := range allArticles {
		result[i] = dto.ArticleFromEnt(a)
	}

	response.JSON(w, http.StatusOK, result)
}

// getPhysicianDrugClasses returns the unique drug classes from a physician's
// recent search history (up to 200 most recent searches).
func (h *Handler) getPhysicianDrugClasses(ctx context.Context, physID int) map[string]int {
	history, err := h.db.SearchHistory.Query().
		Where(
			searchhistory.HasPhysicianWith(physician.ID(physID)),
			searchhistory.HasDrug(),
		).
		WithDrug().
		Order(ent.Desc(searchhistory.FieldSearchedAt)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil
	}

	// Count occurrences of each drug class — more searches = higher weight
	classCounts := make(map[string]int)
	for _, entry := range history {
		if d := entry.Edges.Drug; d != nil && d.DrugClass != "" {
			classCounts[d.DrugClass]++
		}
	}

	return classCounts
}

// scoreAndSort ranks articles by overlap with physician's drug classes.
// Articles matching more (and more frequently searched) drug classes rank higher.
// Within the same score, newer articles come first.
func scoreAndSort(arts []*ent.Article, classCounts map[string]int) []*ent.Article {
	type scored struct {
		article *ent.Article
		score   int
	}

	items := make([]scored, len(arts))
	for i, a := range arts {
		score := 0
		for _, cls := range a.DrugClasses {
			if count, ok := classCounts[cls]; ok {
				score += count
			}
		}
		items[i] = scored{article: a, score: score}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].article.PublishedAt.After(items[j].article.PublishedAt)
	})

	result := make([]*ent.Article, len(items))
	for i, item := range items {
		result[i] = item.article
	}
	return result
}
