package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/insurer"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetInsurers returns insurers that have active plans in the given state,
// deduplicated by name and sectioned into local vs. national buckets.
func (h *Handler) GetInsurers(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		response.BadRequest(w, "State code is required")
		return
	}

	// Include plans with matching state_code OR NULL state_code (national plans like Medicare Part D)
	insurers, err := h.db.Insurer.Query().
		Where(insurer.HasPlansWith(
			plan.Or(plan.StateCode(code), plan.StateCodeIsNil()),
			plan.IsActive(true),
		)).
		WithPlans(func(q *ent.PlanQuery) {
			q.Where(plan.Or(plan.StateCode(code), plan.StateCodeIsNil()), plan.IsActive(true))
		}).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Deduplicate insurers by normalized name, summing plan counts
	deduped := deduplicateInsurers(insurers)

	// Section into local (has plans with matching state_code) vs national (only NULL state_code plans)
	var local, national []dto.InsurerDTO
	for _, ins := range deduped {
		hasLocalPlan := false
		for _, p := range ins.Edges.Plans {
			if p.StateCode == code {
				hasLocalPlan = true
				break
			}
		}
		totalCount := len(ins.Edges.Plans)
		d := dto.InsurerFromEnt(ins, &totalCount)
		if hasLocalPlan {
			local = append(local, d)
		} else {
			national = append(national, d)
		}
	}

	if local == nil {
		local = []dto.InsurerDTO{}
	}
	if national == nil {
		national = []dto.InsurerDTO{}
	}

	result := dto.StateSectionedInsurersDTO{
		StateCode:        code,
		LocalInsurers:    local,
		NationalInsurers: national,
	}

	response.JSON(w, http.StatusOK, result)
}

// deduplicateInsurers groups insurers by lowercase name, keeping the insurer
// with the most plans from each group but aggregating all plans across duplicates.
func deduplicateInsurers(insurers []*ent.Insurer) []*ent.Insurer {
	type group struct {
		representative *ent.Insurer
		allPlans       []*ent.Plan
	}

	groups := make(map[string]*group)
	// Track insertion order so results are deterministic
	var order []string

	for _, ins := range insurers {
		key := strings.ToLower(ins.InsurerName)
		g, exists := groups[key]
		if !exists {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.allPlans = append(g.allPlans, ins.Edges.Plans...)
		// Pick the insurer record with the most plans as the representative
		if g.representative == nil || len(ins.Edges.Plans) > len(g.representative.Edges.Plans) {
			g.representative = ins
		}
	}

	result := make([]*ent.Insurer, 0, len(groups))
	for _, key := range order {
		g := groups[key]
		// Attach the aggregated plan list to the representative
		g.representative.Edges.Plans = g.allPlans
		result = append(result, g.representative)
	}
	return result
}

// GetPlans returns plans for an insurer in a state.
// Accepts either a path param "id" (insurer ID) or a query param "insurer_name".
// When insurer_name is provided, plans from ALL insurer records matching that name
// are returned (handles the duplicate-insurer-name case).
func (h *Handler) GetPlans(w http.ResponseWriter, r *http.Request) {
	stateCode := r.URL.Query().Get("state")
	insurerName := r.URL.Query().Get("insurer_name")

	var query *ent.PlanQuery

	if insurerName != "" {
		// Fetch plans from all insurers matching this name (case-insensitive via lowercase compare)
		matchingInsurers, err := h.db.Insurer.Query().
			Where(insurer.InsurerNameEqualFold(insurerName)).
			IDs(r.Context())
		if err != nil {
			response.InternalError(w)
			return
		}
		if len(matchingInsurers) == 0 {
			response.NotFound(w, "No insurer found with that name")
			return
		}
		query = h.db.Plan.Query().
			Where(plan.HasInsurerWith(insurer.IDIn(matchingInsurers...)), plan.IsActive(true)).
			WithInsurer()
	} else {
		idStr := chi.URLParam(r, "id")
		insurerID, err := strconv.Atoi(idStr)
		if err != nil {
			response.BadRequest(w, "Invalid insurer ID")
			return
		}
		query = h.db.Plan.Query().
			Where(plan.HasInsurerWith(insurer.ID(insurerID)), plan.IsActive(true)).
			WithInsurer()
	}

	if stateCode != "" {
		query = query.Where(plan.Or(plan.StateCode(stateCode), plan.StateCodeIsNil()))
	}

	plans, err := query.All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	response.JSON(w, http.StatusOK, dto.PlansFromEnt(plans))
}

// LookupMedicarePlan finds a plan by Medicare Contract+Plan+Segment IDs.
func (h *Handler) LookupMedicarePlan(w http.ResponseWriter, r *http.Request) {
	contractID := r.URL.Query().Get("contract_id")
	planID := r.URL.Query().Get("plan_id")
	segmentID := r.URL.Query().Get("segment_id")

	if contractID == "" || planID == "" || segmentID == "" {
		response.BadRequest(w, "contract_id, plan_id, and segment_id are required")
		return
	}

	p, err := h.db.Plan.Query().
		Where(
			plan.ContractID(contractID),
			plan.PlanID(planID),
			plan.SegmentID(segmentID),
		).
		WithInsurer().
		Only(r.Context())
	if ent.IsNotFound(err) {
		response.NotFound(w, "No plan found with those Medicare identifiers")
		return
	}
	if err != nil {
		response.InternalError(w)
		return
	}

	response.JSON(w, http.StatusOK, dto.PlanFromEnt(p))
}

// LookupHiosPlan finds a plan by HIOS plan ID.
func (h *Handler) LookupHiosPlan(w http.ResponseWriter, r *http.Request) {
	hiosID := r.URL.Query().Get("hios_plan_id")
	if hiosID == "" {
		response.BadRequest(w, "hios_plan_id is required")
		return
	}

	p, err := h.db.Plan.Query().
		Where(plan.PlanID(hiosID)).
		WithInsurer().
		First(r.Context())
	if ent.IsNotFound(err) {
		response.NotFound(w, "No plan found with that HIOS ID")
		return
	}
	if err != nil {
		response.InternalError(w)
		return
	}

	response.JSON(w, http.StatusOK, dto.PlanFromEnt(p))
}

// LookupGroupPlan finds a plan by group ID (future feature).
func (h *Handler) LookupGroupPlan(w http.ResponseWriter, r *http.Request) {
	response.NotFound(w, "Group plan lookup is not yet supported. This feature is planned for a future release.")
}
