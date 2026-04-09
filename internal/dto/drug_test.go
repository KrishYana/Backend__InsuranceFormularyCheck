package dto

import (
	"testing"

	"github.com/kyanaman/formularycheck/ent"
)

func TestDrugFromEnt_FullFields(t *testing.T) {
	drug := &ent.Drug{
		ID:           1,
		Rxcui:        "12345",
		DrugName:     "Metformin 500 MG Oral Tablet",
		GenericName:  "Metformin",
		BrandNames:   []string{"Glucophage", "Fortamet"},
		DoseForm:     "Oral Tablet",
		Strength:     "500 MG",
		Route:        "Oral",
		DrugClass:    "Biguanides",
		IsSpecialty:  false,
		IsControlled: false,
		DeaSchedule:  "",
	}

	dto := DrugFromEnt(drug)

	if dto.DrugID != 1 {
		t.Errorf("expected DrugID 1, got %d", dto.DrugID)
	}
	if dto.Rxcui != "12345" {
		t.Errorf("expected Rxcui '12345', got '%s'", dto.Rxcui)
	}
	if dto.DrugName != "Metformin 500 MG Oral Tablet" {
		t.Errorf("expected DrugName 'Metformin 500 MG Oral Tablet', got '%s'", dto.DrugName)
	}
	if dto.GenericName == nil {
		t.Fatal("expected GenericName to be non-nil")
	}
	if *dto.GenericName != "Metformin" {
		t.Errorf("expected GenericName 'Metformin', got '%s'", *dto.GenericName)
	}
	if len(dto.BrandNames) != 2 {
		t.Errorf("expected 2 BrandNames, got %d", len(dto.BrandNames))
	}
	if dto.DoseForm == nil || *dto.DoseForm != "Oral Tablet" {
		t.Errorf("expected DoseForm 'Oral Tablet', got %v", dto.DoseForm)
	}
	if dto.Strength == nil || *dto.Strength != "500 MG" {
		t.Errorf("expected Strength '500 MG', got %v", dto.Strength)
	}
	if dto.Route == nil || *dto.Route != "Oral" {
		t.Errorf("expected Route 'Oral', got %v", dto.Route)
	}
	if dto.DrugClass == nil || *dto.DrugClass != "Biguanides" {
		t.Errorf("expected DrugClass 'Biguanides', got %v", dto.DrugClass)
	}
	if dto.IsSpecialty {
		t.Error("expected IsSpecialty false")
	}
	if dto.IsControlled {
		t.Error("expected IsControlled false")
	}
	if dto.DeaSchedule != nil {
		t.Errorf("expected DeaSchedule nil for empty string, got %v", dto.DeaSchedule)
	}
}

func TestDrugFromEnt_EmptyOptionalFields(t *testing.T) {
	drug := &ent.Drug{
		ID:       2,
		Rxcui:    "99999",
		DrugName: "TestDrug",
	}

	dto := DrugFromEnt(drug)

	if dto.DrugID != 2 {
		t.Errorf("expected DrugID 2, got %d", dto.DrugID)
	}
	if dto.GenericName != nil {
		t.Errorf("expected GenericName nil, got %v", dto.GenericName)
	}
	if dto.DoseForm != nil {
		t.Errorf("expected DoseForm nil, got %v", dto.DoseForm)
	}
	if dto.Strength != nil {
		t.Errorf("expected Strength nil, got %v", dto.Strength)
	}
	if dto.Route != nil {
		t.Errorf("expected Route nil, got %v", dto.Route)
	}
	if dto.DrugClass != nil {
		t.Errorf("expected DrugClass nil, got %v", dto.DrugClass)
	}
	if dto.DeaSchedule != nil {
		t.Errorf("expected DeaSchedule nil, got %v", dto.DeaSchedule)
	}
}

func TestDrugFromEnt_ControlledSubstance(t *testing.T) {
	drug := &ent.Drug{
		ID:           3,
		Rxcui:        "55555",
		DrugName:     "Oxycodone 5 MG Oral Tablet",
		IsControlled: true,
		DeaSchedule:  "II",
		IsSpecialty:  false,
	}

	dto := DrugFromEnt(drug)

	if !dto.IsControlled {
		t.Error("expected IsControlled true")
	}
	if dto.DeaSchedule == nil || *dto.DeaSchedule != "II" {
		t.Errorf("expected DeaSchedule 'II', got %v", dto.DeaSchedule)
	}
}

func TestDrugsFromEnt(t *testing.T) {
	drugs := []*ent.Drug{
		{ID: 1, Rxcui: "111", DrugName: "Drug A"},
		{ID: 2, Rxcui: "222", DrugName: "Drug B"},
		{ID: 3, Rxcui: "333", DrugName: "Drug C"},
	}

	dtos := DrugsFromEnt(drugs)

	if len(dtos) != 3 {
		t.Fatalf("expected 3 DTOs, got %d", len(dtos))
	}

	for i, dto := range dtos {
		if dto.Rxcui != drugs[i].Rxcui {
			t.Errorf("drug %d: expected Rxcui '%s', got '%s'", i, drugs[i].Rxcui, dto.Rxcui)
		}
		if dto.DrugName != drugs[i].DrugName {
			t.Errorf("drug %d: expected DrugName '%s', got '%s'", i, drugs[i].DrugName, dto.DrugName)
		}
	}
}

func TestDrugsFromEnt_Empty(t *testing.T) {
	dtos := DrugsFromEnt([]*ent.Drug{})
	if len(dtos) != 0 {
		t.Errorf("expected 0 DTOs for empty input, got %d", len(dtos))
	}
}
