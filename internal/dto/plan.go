package dto

import "github.com/kyanaman/formularycheck/ent"

// InsurerDTO matches the frontend Insurer type.
type InsurerDTO struct {
	InsurerID       int     `json:"insurerId"`
	InsurerName     string  `json:"insurerName"`
	ParentCompany   *string `json:"parentCompany"`
	HiosIssuerId    *string `json:"hiosIssuerId"`
	FhirEndpointUrl *string `json:"fhirEndpointUrl"`
	WebsiteUrl      *string `json:"websiteUrl"`
	PlanCount       *int    `json:"planCount,omitempty"`
}

// InsurerFromEnt converts an Ent Insurer entity to an InsurerDTO.
func InsurerFromEnt(i *ent.Insurer, planCount *int) InsurerDTO {
	dto := InsurerDTO{
		InsurerID:   i.ID,
		InsurerName: i.InsurerName,
		PlanCount:   planCount,
	}
	if i.ParentCompany != "" {
		dto.ParentCompany = &i.ParentCompany
	}
	if i.HiosIssuerID != "" {
		dto.HiosIssuerId = &i.HiosIssuerID
	}
	if i.FhirEndpointURL != "" {
		dto.FhirEndpointUrl = &i.FhirEndpointURL
	}
	if i.WebsiteURL != "" {
		dto.WebsiteUrl = &i.WebsiteURL
	}
	return dto
}

// PlanDTO matches the frontend Plan type.
type PlanDTO struct {
	PlanID     int     `json:"planId"`
	InsurerID  int     `json:"insurerId"`
	StateCode  string  `json:"stateCode"`
	PlanName   string  `json:"planName"`
	PlanType   *string `json:"planType"`
	MarketType *string `json:"marketType"`
	MetalLevel *string `json:"metalLevel"`
	PlanYear   int     `json:"planYear"`
	IsActive   bool    `json:"isActive"`
}

// PlanFromEnt converts an Ent Plan entity to a PlanDTO.
func PlanFromEnt(p *ent.Plan) PlanDTO {
	dto := PlanDTO{
		PlanID:   p.ID,
		PlanName: p.PlanName,
		IsActive: p.IsActive,
	}
	if p.StateCode != "" {
		dto.StateCode = p.StateCode
	}
	if p.PlanType != "" {
		dto.PlanType = &p.PlanType
	}
	if p.MarketType != "" {
		dto.MarketType = &p.MarketType
	}
	if p.MetalLevel != "" {
		dto.MetalLevel = &p.MetalLevel
	}
	if p.PlanYear != nil {
		dto.PlanYear = *p.PlanYear
	}
	// Get insurer ID from edge if loaded
	if insurer := p.Edges.Insurer; insurer != nil {
		dto.InsurerID = insurer.ID
	}
	return dto
}

// PlansFromEnt converts a slice of Ent Plan entities to PlanDTOs.
func PlansFromEnt(plans []*ent.Plan) []PlanDTO {
	result := make([]PlanDTO, len(plans))
	for i, p := range plans {
		result[i] = PlanFromEnt(p)
	}
	return result
}
