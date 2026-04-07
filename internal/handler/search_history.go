package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/ent/searchhistory"
	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// ListSearchHistory handles GET /search-history.
func (h *Handler) ListSearchHistory(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	entries, err := h.db.SearchHistory.Query().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		WithPlan().
		WithDrug().
		Order(ent.Desc(searchhistory.FieldSearchedAt)).
		Limit(100).
		All(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]dto.SearchHistoryDTO, len(entries))
	for i, e := range entries {
		result[i] = dto.SearchHistoryFromEnt(e)
	}

	response.JSON(w, http.StatusOK, result)
}

// DeleteSearchHistoryEntry handles DELETE /search-history/{id}.
func (h *Handler) DeleteSearchHistoryEntry(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid history entry ID")
		return
	}

	// Verify ownership
	exists, err := h.db.SearchHistory.Query().
		Where(searchhistory.ID(id), searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		Exist(r.Context())
	if err != nil || !exists {
		response.NotFound(w, "History entry not found")
		return
	}

	if err := h.db.SearchHistory.DeleteOneID(id).Exec(r.Context()); err != nil {
		response.InternalError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ClearSearchHistory handles DELETE /search-history (clear all).
func (h *Handler) ClearSearchHistory(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	_, err := h.db.SearchHistory.Delete().
		Where(searchhistory.HasPhysicianWith(physician.ID(phys.ID))).
		Exec(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
