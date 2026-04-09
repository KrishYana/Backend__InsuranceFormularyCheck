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
		// Try generic fallback: look up the branded drug's generic equivalent
		genericEntry := h.resolveViaGeneric(r.Context(), planID, drugID)
		if genericEntry != nil {
			// Record search history even for generic-resolved lookups
			h.recordSearchHistory(r, "", 1, &planID, &drugID)
			response.JSON(w, http.StatusOK, *genericEntry)
			return
		}

		// No coverage found even via generic
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
// Dynamically generates generic equivalents from drug data in addition
// to any pre-populated DrugAlternative records.
func (h *Handler) GetAlternatives(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	drugID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid drug ID")
		return
	}

	// Optional plan_id for coverage status on alternatives
	var planID int
	if pidStr := r.URL.Query().Get("plan_id"); pidStr != "" {
		planID, _ = strconv.Atoi(pidStr)
	}

	// 1. Get pre-populated alternatives from DB
	alternatives, err := h.db.DrugAlternative.Query().
		Where(drugalternative.HasDrugWith(drug.ID(drugID))).
		WithDrug().
		WithAlternativeDrug().
		All(ctx)
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.DrugAlternativeDTO, 0, len(alternatives)+5)

	// 2. Dynamically find generic equivalents via generic_name matching
	requestedDrug, err := h.db.Drug.Get(ctx, drugID)
	if err == nil && requestedDrug.GenericName != "" {
		// Find drugs that share the same generic_name (generic ↔ brand siblings)
		genericSiblings, err := h.db.Drug.Query().
			Where(
				drug.Or(
					drug.DrugName(requestedDrug.GenericName),
					drug.GenericName(requestedDrug.GenericName),
				),
				drug.IDNEQ(drugID),
			).
			All(ctx)
		if err == nil {
			for _, g := range genericSiblings {
				altDTO := dto.DrugAlternativeDTO{
					DrugID:            drugID,
					AlternativeDrugID: g.ID,
					RelationshipType:  "GENERIC_EQUIVALENT",
				}
				src := "RxNorm"
				altDTO.Source = &src
				gDTO := dto.DrugFromEnt(g)
				altDTO.AlternativeDrug = &gDTO

				// Check coverage if plan_id provided
				if planID > 0 {
					altDTO.CoverageStatus, altDTO.AlternativeTierName = h.checkAlternativeCoverage(ctx, planID, g.ID)
				}

				result = append(result, altDTO)
			}
		}
	}

	// 3. Append pre-populated alternatives with coverage status
	for _, a := range alternatives {
		altDTO := dto.DrugAlternativeFromEnt(a)
		if planID > 0 && a.Edges.AlternativeDrug != nil {
			altDTO.CoverageStatus, altDTO.AlternativeTierName = h.checkAlternativeCoverage(ctx, planID, a.Edges.AlternativeDrug.ID)
		}
		result = append(result, altDTO)
	}

	response.JSON(w, http.StatusOK, result)
}

// resolveViaGeneric attempts to find coverage for the generic equivalent
// of a branded drug. Returns nil if no generic match or no coverage found.
func (h *Handler) resolveViaGeneric(ctx context.Context, planID, brandedDrugID int) *dto.FormularyEntryDTO {
	// Look up the branded drug
	brandedDrug, err := h.db.Drug.Get(ctx, brandedDrugID)
	if err != nil || brandedDrug.GenericName == "" {
		return nil
	}

	// If drug_name == generic_name, this IS the generic — no fallback needed
	if brandedDrug.DrugName == brandedDrug.GenericName {
		return nil
	}

	// Find the generic drug by matching drug_name to the branded drug's generic_name
	genericDrug, err := h.db.Drug.Query().
		Where(
			drug.DrugName(brandedDrug.GenericName),
			drug.IDNEQ(brandedDrugID),
		).
		First(ctx)
	if err != nil {
		return nil
	}

	// Check if the generic drug is covered on this plan
	entry, err := h.db.FormularyEntry.Query().
		Where(
			formularyentry.HasPlanWith(plan.ID(planID)),
			formularyentry.HasDrugWith(drug.ID(genericDrug.ID)),
			formularyentry.IsCurrent(true),
		).
		WithPlan().
		WithDrug().
		Only(ctx)
	if err != nil {
		return nil
	}

	// Build DTO with the generic resolution flag
	result := dto.FormularyEntryFromEnt(entry)
	result.DrugID = brandedDrugID // Keep original drug ID for the physician's context
	result.ResolvedViaGeneric = true
	gn := genericDrug.DrugName
	result.GenericDrugName = &gn
	return &result
}

// checkAlternativeCoverage checks if a drug is covered on a plan and returns status + tier.
func (h *Handler) checkAlternativeCoverage(ctx context.Context, planID, altDrugID int) (*string, *string) {
	entry, err := h.db.FormularyEntry.Query().
		Where(
			formularyentry.HasPlanWith(plan.ID(planID)),
			formularyentry.HasDrugWith(drug.ID(altDrugID)),
			formularyentry.IsCurrent(true),
		).
		Only(ctx)
	if err != nil {
		status := "not_covered"
		return &status, nil
	}
	if entry.IsCovered {
		status := "covered"
		var tier *string
		if entry.TierName != "" {
			tier = &entry.TierName
		}
		return &status, tier
	}
	status := "not_covered"
	return &status, nil
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
