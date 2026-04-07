package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/drug"
	"github.com/kyanaman/formularycheck/ent/formularyentry"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/ent/plan"
	"github.com/kyanaman/formularycheck/ent/savedlookup"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// ListSavedLookups handles GET /saved-lookups.
func (h *Handler) ListSavedLookups(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	lookups, err := h.db.SavedLookup.Query().
		Where(savedlookup.HasPhysicianWith(physician.ID(phys.ID))).
		WithPhysician().
		WithPlan().
		WithDrug().
		Order(ent.Desc(savedlookup.FieldCreatedAt)).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.SavedLookupDTO, len(lookups))
	for i, sl := range lookups {
		d := dto.SavedLookupFromEnt(sl)

		// Fetch current coverage for this plan+drug pair
		if sl.Edges.Plan != nil && sl.Edges.Drug != nil {
			entry, err := h.db.FormularyEntry.Query().
				Where(
					formularyentry.HasPlanWith(plan.ID(sl.Edges.Plan.ID)),
					formularyentry.HasDrugWith(drug.ID(sl.Edges.Drug.ID)),
					formularyentry.IsCurrent(true),
				).
				Only(r.Context())
			if err == nil {
				entryDTO := dto.FormularyEntryFromEnt(entry)
				d.CurrentEntry = &entryDTO
			}
		}

		result[i] = d
	}

	response.JSON(w, http.StatusOK, result)
}

// CreateSavedLookup handles POST /saved-lookups.
func (h *Handler) CreateSavedLookup(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	var req dto.CreateSavedLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	p, err := h.db.Plan.Get(r.Context(), req.PlanID)
	if err != nil {
		response.NotFound(w, "Plan not found")
		return
	}

	d, err := h.db.Drug.Get(r.Context(), req.DrugID)
	if err != nil {
		response.NotFound(w, "Drug not found")
		return
	}

	builder := h.db.SavedLookup.Create().
		SetPhysician(phys).
		SetPlan(p).
		SetDrug(d)

	if req.Nickname != nil {
		builder = builder.SetNickname(*req.Nickname)
	}

	sl, err := builder.Save(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	// Re-query with edges for response
	sl, _ = h.db.SavedLookup.Query().
		Where(savedlookup.ID(sl.ID)).
		WithPhysician().
		WithPlan().
		WithDrug().
		Only(r.Context())

	response.JSON(w, http.StatusCreated, dto.SavedLookupFromEnt(sl))
}

// UpdateSavedLookup handles PATCH /saved-lookups/{id}.
func (h *Handler) UpdateSavedLookup(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid lookup ID")
		return
	}

	// Verify ownership
	sl, err := h.db.SavedLookup.Query().
		Where(savedlookup.ID(id), savedlookup.HasPhysicianWith(physician.ID(phys.ID))).
		Only(r.Context())
	if ent.IsNotFound(err) {
		response.NotFound(w, "Saved lookup not found")
		return
	}
	if err != nil {
		response.InternalError(w)
		return
	}

	var req dto.UpdateSavedLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	update := sl.Update()
	if req.Nickname != nil {
		update = update.SetNickname(*req.Nickname)
	}

	updated, err := update.Save(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	response.JSON(w, http.StatusOK, dto.SavedLookupFromEnt(updated))
}

// DeleteSavedLookup handles DELETE /saved-lookups/{id}.
func (h *Handler) DeleteSavedLookup(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid lookup ID")
		return
	}

	// Verify ownership
	exists, err := h.db.SavedLookup.Query().
		Where(savedlookup.ID(id), savedlookup.HasPhysicianWith(physician.ID(phys.ID))).
		Exist(r.Context())
	if err != nil || !exists {
		response.NotFound(w, "Saved lookup not found")
		return
	}

	if err := h.db.SavedLookup.DeleteOneID(id).Exec(r.Context()); err != nil {
		response.InternalError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
