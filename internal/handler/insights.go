package handler

import (
	"fmt"
	"net/http"

	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/ent/searchhistory"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetInsightsSummary handles GET /insights/summary.
// Returns computed analytics from the physician's search history.
func (h *Handler) GetInsightsSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	phys, ok := middleware.PhysicianFromCtx(ctx)
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	// Total lookups
	totalLookups, err := h.db.SearchHistory.Query().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		Count(ctx)
	if err != nil {
		response.InternalError(w)
		return
	}

	// Fetch recent history with drug + plan + insurer edges
	history, err := h.db.SearchHistory.Query().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		WithDrug().
		WithPlan(func(q *ent.PlanQuery) {
			q.WithInsurer()
		}).
		Order(ent.Desc(searchhistory.FieldSearchedAt)).
		Limit(500).
		All(ctx)
	if err != nil {
		response.InternalError(w)
		return
	}

	// --- Top 5 Drugs ---
	drugCounts := make(map[int]int)
	drugMap := make(map[int]*ent.Drug)
	for _, entry := range history {
		if d := entry.Edges.Drug; d != nil {
			drugCounts[d.ID]++
			drugMap[d.ID] = d
		}
	}

	topDrugs := topNByCount(drugCounts, 5)
	topDrugDTOs := make([]dto.TopDrugDTO, len(topDrugs))
	for i, item := range topDrugs {
		topDrugDTOs[i] = dto.TopDrugDTO{
			Drug:        dto.DrugFromEnt(drugMap[item.id]),
			SearchCount: item.count,
		}
	}

	// --- Top 5 Insurers (via Plan → Insurer edge) ---
	insurerCounts := make(map[int]int)
	insurerMap := make(map[int]*ent.Insurer)
	for _, entry := range history {
		if p := entry.Edges.Plan; p != nil {
			if ins := p.Edges.Insurer; ins != nil {
				insurerCounts[ins.ID]++
				insurerMap[ins.ID] = ins
			}
		}
	}

	topInsurers := topNByCount(insurerCounts, 5)
	topInsurerDTOs := make([]dto.TopInsurerDTO, len(topInsurers))
	for i, item := range topInsurers {
		topInsurerDTOs[i] = dto.TopInsurerDTO{
			Insurer:     dto.InsurerFromEnt(insurerMap[item.id], &item.count),
			SearchCount: item.count,
		}
	}

	// --- Top 5 Plans ---
	planCounts := make(map[int]int)
	planMap := make(map[int]*ent.Plan)
	for _, entry := range history {
		if p := entry.Edges.Plan; p != nil {
			planCounts[p.ID]++
			planMap[p.ID] = p
		}
	}

	topPlans := topNByCount(planCounts, 5)
	topPlanDTOs := make([]dto.TopPlanDTO, len(topPlans))
	for i, item := range topPlans {
		topPlanDTOs[i] = dto.TopPlanDTO{
			Plan:        dto.PlanFromEnt(planMap[item.id]),
			SearchCount: item.count,
		}
	}

	// --- Coverage Success Rate ---
	// Collect all (plan_id, drug_id) pairs from coverage lookups
	type pdPair struct{ planID, drugID int }
	var coveragePairs []pdPair
	for _, entry := range history {
		if entry.Edges.Plan != nil && entry.Edges.Drug != nil {
			coveragePairs = append(coveragePairs, pdPair{
				planID: entry.Edges.Plan.ID,
				drugID: entry.Edges.Drug.ID,
			})
		}
	}

	var coverageRate float64
	if len(coveragePairs) > 0 {
		// Batch check: for each pair, does a covered formulary entry exist?
		coveredCount := 0
		for _, pair := range coveragePairs {
			covered, err := h.db.FormularyEntry.Query().
				Where(
					formularyentry.HasPlanWith(plan.ID(pair.planID)),
					formularyentry.HasDrugWith(drug.ID(pair.drugID)),
					formularyentry.IsCovered(true),
					formularyentry.IsCurrent(true),
				).
				Exist(ctx)
			if err == nil && covered {
				coveredCount++
			}
		}
		coverageRate = float64(coveredCount) / float64(len(coveragePairs))
	}

	summary := dto.InsightsSummaryDTO{
		TotalLookups:        totalLookups,
		CoverageSuccessRate: coverageRate,
		TopDrugs:            topDrugDTOs,
		TopInsurers:         topInsurerDTOs,
		TopPlans:            topPlanDTOs,
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
