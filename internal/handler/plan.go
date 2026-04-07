package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/insurer"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/response"
)

// GetInsurers returns insurers that have active plans in the given state.
func (h *Handler) GetInsurers(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		response.BadRequest(w, "State code is required")
		return
	}

	insurers, err := h.db.Insurer.Query().
		Where(insurer.HasPlansWith(plan.StateCode(code), plan.IsActive(true))).
		WithPlans(func(q *ent.PlanQuery) {
			q.Where(plan.StateCode(code), plan.IsActive(true))
		}).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.InsurerDTO, len(insurers))
	for i, ins := range insurers {
		count := len(ins.Edges.Plans)
		result[i] = dto.InsurerFromEnt(ins, &count)
	}

	response.JSON(w, http.StatusOK, result)
}

// GetPlans returns plans for an insurer in a state.
func (h *Handler) GetPlans(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	insurerID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(w, "Invalid insurer ID")
		return
	}

	stateCode := r.URL.Query().Get("state")

	query := h.db.Plan.Query().
		Where(plan.HasInsurerWith(insurer.ID(insurerID)), plan.IsActive(true)).
		WithInsurer()

	if stateCode != "" {
		query = query.Where(plan.StateCode(stateCode))
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
