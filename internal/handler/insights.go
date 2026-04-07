package handler

import (
	"fmt"
	"net/http"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/ent/searchhistory"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetInsightsSummary handles GET /insights/summary.
// Returns computed analytics from the physician's search history.
func (h *Handler) GetInsightsSummary(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	// Total lookups
	totalLookups, err := h.db.SearchHistory.Query().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		Count(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Top 5 searched drugs
	history, err := h.db.SearchHistory.Query().
		Where(
			searchhistory.HasPhysicianWith(physician.ID(phys.ID)),
			searchhistory.HasDrug(),
		).
		WithDrug().
		Order(ent.Desc(searchhistory.FieldSearchedAt)).
		Limit(200).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Count drug occurrences
	drugCounts := make(map[int]int)
	drugMap := make(map[int]*ent.Drug)
	for _, entry := range history {
		if d := entry.Edges.Drug; d != nil {
			drugCounts[d.ID]++
			drugMap[d.ID] = d
		}
	}

	// Sort and take top 5
	topDrugs := topNByCount(drugCounts, 5)
	topDrugDTOs := make([]dto.TopDrugDTO, len(topDrugs))
	for i, item := range topDrugs {
		topDrugDTOs[i] = dto.TopDrugDTO{
			Drug:        dto.DrugFromEnt(drugMap[item.id]),
			SearchCount: item.count,
		}
	}

	summary := dto.InsightsSummaryDTO{
		TotalLookups:        totalLookups,
		CoverageSuccessRate: 0, // TODO: compute from formulary_entries
		TopDrugs:            topDrugDTOs,
		TopInsurers:         []dto.TopInsurerDTO{}, // TODO: compute from plan->insurer joins
	}

	response.JSON(w, http.StatusOK, summary)
}

// GetInsightsTrends handles GET /insights/trends.
// Returns weekly lookup counts for the last 12 weeks.
func (h *Handler) GetInsightsTrends(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	// Fetch all entries from last 12 weeks and group client-side
	// TODO: Replace with raw SQL using date_trunc('week', searched_at) for efficiency
	history, err := h.db.SearchHistory.Query().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		Order(ent.Desc(searchhistory.FieldSearchedAt)).
		Limit(500).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Group by ISO week
	weekCounts := make(map[string]int)
	for _, entry := range history {
		year, week := entry.SearchedAt.ISOWeek()
		key := weekKey(year, week)
		weekCounts[key]++
	}

	// Convert to sorted data points
	dataPoints := make([]dto.TrendPointDTO, 0, len(weekCounts))
	for date, count := range weekCounts {
		dataPoints = append(dataPoints, dto.TrendPointDTO{
			Date:        date,
			LookupCount: count,
		})
	}

	response.JSON(w, http.StatusOK, dto.InsightsTrendsDTO{
		Period:     "weekly",
		DataPoints: dataPoints,
	})
}

// --- helpers ---

type countItem struct {
	id    int
	count int
}

func topNByCount(counts map[int]int, n int) []countItem {
	items := make([]countItem, 0, len(counts))
	for id, count := range counts {
		items = append(items, countItem{id: id, count: count})
	}
	// Simple selection sort for small N
	for i := 0; i < len(items) && i < n; i++ {
		maxIdx := i
		for j := i + 1; j < len(items); j++ {
			if items[j].count > items[maxIdx].count {
				maxIdx = j
			}
		}
		items[i], items[maxIdx] = items[maxIdx], items[i]
	}
	if len(items) > n {
		items = items[:n]
	}
	return items
}

func weekKey(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}
