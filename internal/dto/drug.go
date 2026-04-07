package dto

import "github.com/kyanaman/formularycheck/ent"

// DrugDTO matches the frontend Drug type.
type DrugDTO struct {
	DrugID      int      `json:"drugId"`
	Rxcui       string   `json:"rxcui"`
	DrugName    string   `json:"drugName"`
	GenericName *string  `json:"genericName"`
	BrandNames  []string `json:"brandNames"`
	DoseForm    *string  `json:"doseForm"`
	Strength    *string  `json:"strength"`
	Route       *string  `json:"route"`
	DrugClass   *string  `json:"drugClass"`
	IsSpecialty bool     `json:"isSpecialty"`
	IsControlled bool   `json:"isControlled"`
	DeaSchedule *string  `json:"deaSchedule"`
}

// DrugFromEnt converts an Ent Drug entity to a DrugDTO.
func DrugFromEnt(d *ent.Drug) DrugDTO {
	dto := DrugDTO{
		DrugID:       d.ID,
		Rxcui:        d.Rxcui,
		DrugName:     d.DrugName,
		BrandNames:   d.BrandNames,
		IsSpecialty:  d.IsSpecialty,
		IsControlled: d.IsControlled,
	}
	if d.GenericName != "" {
		dto.GenericName = &d.GenericName
	}
	if d.DoseForm != "" {
		dto.DoseForm = &d.DoseForm
	}
	if d.Strength != "" {
		dto.Strength = &d.Strength
	}
	if d.Route != "" {
		dto.Route = &d.Route
	}
	if d.DrugClass != "" {
		dto.DrugClass = &d.DrugClass
	}
	if d.DeaSchedule != "" {
		dto.DeaSchedule = &d.DeaSchedule
	}
	return dto
}

// DrugsFromEnt converts a slice of Ent Drug entities to DrugDTOs.
func DrugsFromEnt(drugs []*ent.Drug) []DrugDTO {
	result := make([]DrugDTO, len(drugs))
	for i, d := range drugs {
		result[i] = DrugFromEnt(d)
	}
	return result
}
