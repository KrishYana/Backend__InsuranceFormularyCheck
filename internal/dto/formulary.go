package dto

import (
	"github.com/kyanaman/formularycheck/ent"
)

// FormularyEntryDTO matches the frontend FormularyEntry type.
type FormularyEntryDTO struct {
	EntryID            int      `json:"entryId"`
	PlanID             int      `json:"planId"`
	DrugID             int      `json:"drugId"`
	IsCovered          bool     `json:"isCovered"`
	TierLevel          *int     `json:"tierLevel"`
	TierName           *string  `json:"tierName"`
	PriorAuthRequired  bool     `json:"priorAuthRequired"`
	StepTherapy        bool     `json:"stepTherapy"`
	QuantityLimit      bool     `json:"quantityLimit"`
	QuantityLimitDetail *string `json:"quantityLimitDetail"`
	SpecialtyDrug      bool     `json:"specialtyDrug"`
	CopayAmount        *float64 `json:"copayAmount"`
	CoinsurancePct     *float64 `json:"coinsurancePct"`
	CopayMailOrder     *float64 `json:"copayMailOrder"`
	SourceType         string   `json:"sourceType"`
	SourceDate         string   `json:"sourceDate"`
	ResolvedViaGeneric bool     `json:"resolvedViaGeneric,omitempty"`
	GenericDrugName    *string  `json:"genericDrugName,omitempty"`
}

// FormularyEntryFromEnt converts an Ent FormularyEntry to a DTO.
func FormularyEntryFromEnt(e *ent.FormularyEntry) FormularyEntryDTO {
	dto := FormularyEntryDTO{
		EntryID:           e.ID,
		IsCovered:         e.IsCovered,
		PriorAuthRequired: e.PriorAuthRequired,
		StepTherapy:       e.StepTherapy,
		QuantityLimit:     e.QuantityLimit,
		SpecialtyDrug:     e.SpecialtyDrug,
		SourceType:        e.SourceType,
		SourceDate:        e.SourceDate.Format("2006-01-02"),
	}
	if e.TierLevel != 0 {
		tl := e.TierLevel
		dto.TierLevel = &tl
	}
	if e.TierName != "" {
		dto.TierName = &e.TierName
	}
	if e.QuantityLimitDetail != "" {
		dto.QuantityLimitDetail = &e.QuantityLimitDetail
	}
	if e.CopayAmount != nil {
		dto.CopayAmount = e.CopayAmount
	}
	if e.CoinsurancePct != nil {
		dto.CoinsurancePct = e.CoinsurancePct
	}
	if e.CopayMailOrder != nil {
		dto.CopayMailOrder = e.CopayMailOrder
	}
	// Set plan/drug IDs from edges if loaded
	if p := e.Edges.Plan; p != nil {
		dto.PlanID = p.ID
	}
	if d := e.Edges.Drug; d != nil {
		dto.DrugID = d.ID
	}
	return dto
}

// PriorAuthCriteriaDTO matches the frontend PriorAuthCriteria type.
type PriorAuthCriteriaDTO struct {
	CriteriaID           int      `json:"criteriaId"`
	EntryID              int      `json:"entryId"`
	CriteriaType         string   `json:"criteriaType"`
	CriteriaDescription  *string  `json:"criteriaDescription"`
	RequiredDiagnoses    []string `json:"requiredDiagnoses"`
	AgeMin               *int     `json:"ageMin"`
	AgeMax               *int     `json:"ageMax"`
	GenderRestriction    *string  `json:"genderRestriction"`
	StepTherapyDrugs     []string `json:"stepTherapyDrugs"`
	StepTherapyDescription *string `json:"stepTherapyDescription"`
	MaxQuantity          *int     `json:"maxQuantity"`
	QuantityPeriodDays   *int     `json:"quantityPeriodDays"`
	SourceDocumentUrl    *string  `json:"sourceDocumentUrl"`
}

// PriorAuthCriteriaFromEnt converts an Ent PriorAuthCriteria to a DTO.
func PriorAuthCriteriaFromEnt(c *ent.PriorAuthCriteria) PriorAuthCriteriaDTO {
	dto := PriorAuthCriteriaDTO{
		CriteriaID:        c.ID,
		CriteriaType:      c.CriteriaType,
		RequiredDiagnoses: c.RequiredDiagnoses,
		StepTherapyDrugs:  c.StepTherapyDrugs,
		AgeMin:            c.AgeMin,
		AgeMax:            c.AgeMax,
		MaxQuantity:       c.MaxQuantity,
		QuantityPeriodDays: c.QuantityPeriodDays,
	}
	if c.CriteriaDescription != "" {
		dto.CriteriaDescription = &c.CriteriaDescription
	}
	if c.GenderRestriction != "" {
		dto.GenderRestriction = &c.GenderRestriction
	}
	if c.StepTherapyDescription != "" {
		dto.StepTherapyDescription = &c.StepTherapyDescription
	}
	if c.SourceDocumentURL != "" {
		dto.SourceDocumentUrl = &c.SourceDocumentURL
	}
	// Set entry ID from edge if loaded
	if fe := c.Edges.FormularyEntry; fe != nil {
		dto.EntryID = fe.ID
	}
	return dto
}

// DrugAlternativeDTO matches the frontend DrugAlternative type.
type DrugAlternativeDTO struct {
	AlternativeID     int      `json:"alternativeId"`
	DrugID            int      `json:"drugId"`
	AlternativeDrugID int      `json:"alternativeDrugId"`
	RelationshipType  string   `json:"relationshipType"`
	Source            *string  `json:"source"`
	Notes             *string  `json:"notes"`
	AlternativeDrug   *DrugDTO `json:"alternativeDrug,omitempty"`
	CoverageStatus    *string  `json:"coverageStatus,omitempty"`
	AlternativeTierName *string `json:"alternativeTierName,omitempty"`
}

// DrugAlternativeFromEnt converts an Ent DrugAlternative to a DTO.
func DrugAlternativeFromEnt(a *ent.DrugAlternative) DrugAlternativeDTO {
	dto := DrugAlternativeDTO{
		AlternativeID:    a.ID,
		RelationshipType: string(a.RelationshipType),
	}
	if a.Source != "" {
		dto.Source = &a.Source
	}
	if a.Notes != "" {
		dto.Notes = &a.Notes
	}
	if d := a.Edges.Drug; d != nil {
		dto.DrugID = d.ID
	}
	if ad := a.Edges.AlternativeDrug; ad != nil {
		dto.AlternativeDrugID = ad.ID
		adDTO := DrugFromEnt(ad)
		dto.AlternativeDrug = &adDTO
	}
	return dto
}
