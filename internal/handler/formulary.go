package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/drugalternative"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/ent/priorauthcriteria"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// SearchDrugs handles GET /drugs/search?q={query}.
func (h *Handler) SearchDrugs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		response.BadRequest(w, "Query must be at least 2 characters")
		return
	}

	drugs, err := h.db.Drug.Query().
		Where(
			drug.Or(
				drug.DrugNameContainsFold(query),
				drug.GenericNameContainsFold(query),
				drug.DrugClassContainsFold(query),
			),
		).
		Limit(30).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Record search history as side effect
	h.recordSearchHistory(r, query, len(drugs), nil, nil)

	response.JSON(w, http.StatusOK, dto.DrugsFromEnt(drugs))
}

// GetCoverage handles GET /plans/{id}/drugs/{drugId}/coverage.
func (h *Handler) GetCoverage(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid plan ID")
		return
	}
	drugID, err := strconv.Atoi(chi.URLParam(r, "drugId"))
	if err != nil {
		response.BadRequest(w, "Invalid drug ID")
		return
	}

	entry, err := h.db.FormularyEntry.Query().
		Where(
			formularyentry.HasPlanWith(plan.ID(planID)),
			formularyentry.HasDrugWith(drug.ID(drugID)),
			formularyentry.IsCurrent(true),
		).
		WithPlan().
		WithDrug().
		Only(r.Context())

	if ent.IsNotFound(err) {
		// Return synthetic not-covered response
		response.JSON(w, http.StatusOK, dto.FormularyEntryDTO{
			PlanID:    planID,
			DrugID:    drugID,
			IsCovered: false,
		})
		return
	}
	if err != nil {
		response.InternalError(w)
		return
	}

	// Record search history
	h.recordSearchHistory(r, "", 1, &planID, &drugID)

	response.JSON(w, http.StatusOK, dto.FormularyEntryFromEnt(entry))
}

// GetCoverageMulti handles GET /drugs/{id}/coverage?plan_ids=1,2,3.
func (h *Handler) GetCoverageMulti(w http.ResponseWriter, r *http.Request) {
	drugID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid drug ID")
		return
	}

	planIDsStr := r.URL.Query().Get("plan_ids")
	if planIDsStr == "" {
		response.BadRequest(w, "plan_ids parameter is required")
		return
	}

	parts := strings.Split(planIDsStr, ",")
	planIDs := make([]int, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		planIDs = append(planIDs, id)
	}

	entries, err := h.db.FormularyEntry.Query().
		Where(
			formularyentry.HasDrugWith(drug.ID(drugID)),
			formularyentry.HasPlanWith(plan.IDIn(planIDs...)),
			formularyentry.IsCurrent(true),
		).
		WithPlan().
		WithDrug().
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Record search history for each plan in the multi-lookup
	for _, pid := range planIDs {
		pidCopy := pid
		h.recordSearchHistory(r, "", len(entries), &pidCopy, &drugID)
	}

	result := make([]dto.FormularyEntryDTO, len(entries))
	for i, e := range entries {
		result[i] = dto.FormularyEntryFromEnt(e)
	}

	response.JSON(w, http.StatusOK, result)
}

// GetAlternatives handles GET /drugs/{id}/alternatives?plan_id=N.
func (h *Handler) GetAlternatives(w http.ResponseWriter, r *http.Request) {
	drugID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid drug ID")
		return
	}

	alternatives, err := h.db.DrugAlternative.Query().
		Where(drugalternative.HasDrugWith(drug.ID(drugID))).
		WithDrug().
		WithAlternativeDrug().
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.DrugAlternativeDTO, len(alternatives))
	for i, a := range alternatives {
		result[i] = dto.DrugAlternativeFromEnt(a)
	}

	response.JSON(w, http.StatusOK, result)
}

// GetPriorAuthCriteria handles GET /coverage/{entryId}/prior-auth.
func (h *Handler) GetPriorAuthCriteria(w http.ResponseWriter, r *http.Request) {
	entryID, err := strconv.Atoi(chi.URLParam(r, "entryId"))
	if err != nil {
		response.BadRequest(w, "Invalid entry ID")
		return
	}

	criteria, err := h.db.PriorAuthCriteria.Query().
		Where(priorauthcriteria.HasFormularyEntryWith(formularyentry.ID(entryID))).
		WithFormularyEntry().
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.PriorAuthCriteriaDTO, len(criteria))
	for i, c := range criteria {
		result[i] = dto.PriorAuthCriteriaFromEnt(c)
	}

	response.JSON(w, http.StatusOK, result)
}

// recordSearchHistory records a search as a side effect (async, best-effort).
func (h *Handler) recordSearchHistory(r *http.Request, searchText string, resultsCount int, planID *int, drugID *int) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		return
	}

	go func() {
		ctx := context.Background()
		builder := h.db.SearchHistory.Create().
			SetPhysician(phys).
			SetResultsCount(resultsCount)

		if searchText != "" {
			builder = builder.SetSearchText(searchText)
		}
		if planID != nil {
			p, err := h.db.Plan.Get(ctx, *planID)
			if err == nil {
				builder = builder.SetPlan(p)
				if p.StateCode != "" {
					builder = builder.SetStateCode(p.StateCode)
				}
			}
		}
		if drugID != nil {
			d, err := h.db.Drug.Get(ctx, *drugID)
			if err == nil {
				builder = builder.SetDrug(d)
			}
		}

		builder.Save(ctx) // best-effort, ignore errors
	}()
}
