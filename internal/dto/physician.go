package dto

import "github.com/kyanaman/formularycheck/ent"

// PhysicianDTO matches the frontend physician profile.
type PhysicianDTO struct {
	PhysicianID    int     `json:"physicianId"`
	Email          string  `json:"email"`
	DisplayName    string  `json:"displayName"`
	NPI            *string `json:"npi"`
	Specialty      *string `json:"specialty"`
	PrimaryState   *string `json:"primaryState"`
	IsNPIVerified  bool    `json:"isNpiVerified"`
}

// PhysicianFromEnt converts an Ent Physician to a DTO.
func PhysicianFromEnt(p *ent.Physician) PhysicianDTO {
	dto := PhysicianDTO{
		PhysicianID:   p.ID,
		Email:         p.Email,
		DisplayName:   p.DisplayName,
		IsNPIVerified: p.IsNpiVerified,
	}
	if p.Npi != "" {
		dto.NPI = &p.Npi
	}
	if p.Specialty != "" {
		dto.Specialty = &p.Specialty
	}
	if p.PrimaryState != "" {
		dto.PrimaryState = &p.PrimaryState
	}
	return dto
}

// PhysicianUpdateRequest is the request body for PATCH /physicians/me.
type PhysicianUpdateRequest struct {
	DisplayName  *string `json:"displayName"`
	NPI          *string `json:"npi"`
	Specialty    *string `json:"specialty"`
	PrimaryState *string `json:"primaryState"`
}
