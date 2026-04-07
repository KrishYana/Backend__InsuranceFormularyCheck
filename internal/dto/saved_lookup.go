package dto

import "github.com/kyanaman/formularycheck/ent"

// SavedLookupDTO matches the frontend SavedLookup type.
type SavedLookupDTO struct {
	LookupID     int                `json:"lookupId"`
	PhysicianID  int                `json:"physicianId"`
	PlanID       int                `json:"planId"`
	DrugID       int                `json:"drugId"`
	Nickname     *string            `json:"nickname"`
	CreatedAt    string             `json:"createdAt"`
	Plan         *PlanDTO           `json:"plan,omitempty"`
	Drug         *DrugDTO           `json:"drug,omitempty"`
	CurrentEntry *FormularyEntryDTO `json:"currentEntry,omitempty"`
}

// SavedLookupFromEnt converts an Ent SavedLookup to a DTO.
func SavedLookupFromEnt(sl *ent.SavedLookup) SavedLookupDTO {
	dto := SavedLookupDTO{
		LookupID:  sl.ID,
		CreatedAt: sl.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if sl.Nickname != "" {
		dto.Nickname = &sl.Nickname
	}
	if p := sl.Edges.Physician; p != nil {
		dto.PhysicianID = p.ID
	}
	if p := sl.Edges.Plan; p != nil {
		dto.PlanID = p.ID
		planDTO := PlanFromEnt(p)
		dto.Plan = &planDTO
	}
	if d := sl.Edges.Drug; d != nil {
		dto.DrugID = d.ID
		drugDTO := DrugFromEnt(d)
		dto.Drug = &drugDTO
	}
	return dto
}

// CreateSavedLookupRequest is the request body for POST /saved-lookups.
type CreateSavedLookupRequest struct {
	PlanID   int     `json:"planId"`
	DrugID   int     `json:"drugId"`
	Nickname *string `json:"nickname"`
}

// UpdateSavedLookupRequest is the request body for PATCH /saved-lookups/{id}.
type UpdateSavedLookupRequest struct {
	Nickname *string `json:"nickname"`
}
