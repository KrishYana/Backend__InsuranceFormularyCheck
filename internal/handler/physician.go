package handler

import (
	"encoding/json"
	"net/http"

	"github.com/kyanaman/formularycheck/internal/dto"
	"github.com/kyanaman/formularycheck/internal/middleware"
	"github.com/kyanaman/formularycheck/internal/response"
)

// AuthCallback handles POST /auth/callback — ensures physician record exists.
func (h *Handler) AuthCallback(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	// Optionally update display name/email from request body
	var body struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
		update := phys.Update()
		if body.DisplayName != "" {
			update = update.SetDisplayName(body.DisplayName)
		}
		if body.Email != "" {
			update = update.SetEmail(body.Email)
		}
		updated, err := update.Save(r.Context())
		if err == nil {
			phys = updated
		}
	}

	response.JSON(w, http.StatusOK, dto.PhysicianFromEnt(phys))
}

// GetProfile handles GET /physicians/me.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}
	response.JSON(w, http.StatusOK, dto.PhysicianFromEnt(phys))
}

// UpdateProfile handles PATCH /physicians/me.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	phys, ok := middleware.PhysicianFromCtx(r.Context())
	if !ok {
		response.Unauthorized(w, "Invalid session")
		return
	}

	var req dto.PhysicianUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	update := phys.Update()
	if req.DisplayName != nil {
		update = update.SetDisplayName(*req.DisplayName)
	}
	if req.NPI != nil {
		update = update.SetNpi(*req.NPI).SetIsNpiVerified(false)
	}
	if req.Specialty != nil {
		update = update.SetSpecialty(*req.Specialty)
	}
	if req.PrimaryState != nil {
		update = update.SetPrimaryState(*req.PrimaryState)
	}

	updated, err := update.Save(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	response.JSON(w, http.StatusOK, dto.PhysicianFromEnt(updated))
}
