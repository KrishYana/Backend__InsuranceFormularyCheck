package dto

import (
	"testing"
	"time"

	"github.com/kyanaman/formularycheck/ent"
)

func TestFormularyEntryFromEnt_FullFields(t *testing.T) {
	copay := 25.0
	coinsurance := 0.2
	copayMail := 15.0
	sourceDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

	entry := &ent.FormularyEntry{
		ID:                  100,
		TierLevel:           2,
		TierName:            "Preferred Generic",
		PriorAuthRequired:   true,
		StepTherapy:         false,
		QuantityLimit:       true,
		QuantityLimitDetail: "30 tablets per 30 days",
		SpecialtyDrug:       false,
		IsCovered:           true,
		CopayAmount:         &copay,
		CoinsurancePct:      &coinsurance,
		CopayMailOrder:      &copayMail,
		SourceType:          "CMS_PUF",
		SourceDate:          sourceDate,
	}
	// Simulate loaded edges
	entry.Edges.Plan = &ent.Plan{ID: 10}
	entry.Edges.Drug = &ent.Drug{ID: 20}

	dto := FormularyEntryFromEnt(entry)

	if dto.EntryID != 100 {
		t.Errorf("expected EntryID 100, got %d", dto.EntryID)
	}
	if dto.PlanID != 10 {
		t.Errorf("expected PlanID 10, got %d", dto.PlanID)
	}
	if dto.DrugID != 20 {
		t.Errorf("expected DrugID 20, got %d", dto.DrugID)
	}
	if !dto.IsCovered {
		t.Error("expected IsCovered true")
	}
	if dto.TierLevel == nil || *dto.TierLevel != 2 {
		t.Errorf("expected TierLevel 2, got %v", dto.TierLevel)
	}
	if dto.TierName == nil || *dto.TierName != "Preferred Generic" {
		t.Errorf("expected TierName 'Preferred Generic', got %v", dto.TierName)
	}
	if !dto.PriorAuthRequired {
		t.Error("expected PriorAuthRequired true")
	}
	if dto.StepTherapy {
		t.Error("expected StepTherapy false")
	}
	if !dto.QuantityLimit {
		t.Error("expected QuantityLimit true")
	}
	if dto.QuantityLimitDetail == nil || *dto.QuantityLimitDetail != "30 tablets per 30 days" {
		t.Errorf("expected QuantityLimitDetail, got %v", dto.QuantityLimitDetail)
	}
	if dto.SpecialtyDrug {
		t.Error("expected SpecialtyDrug false")
	}
	if dto.CopayAmount == nil || *dto.CopayAmount != 25.0 {
		t.Errorf("expected CopayAmount 25.0, got %v", dto.CopayAmount)
	}
	if dto.CoinsurancePct == nil || *dto.CoinsurancePct != 0.2 {
		t.Errorf("expected CoinsurancePct 0.2, got %v", dto.CoinsurancePct)
	}
	if dto.CopayMailOrder == nil || *dto.CopayMailOrder != 15.0 {
		t.Errorf("expected CopayMailOrder 15.0, got %v", dto.CopayMailOrder)
	}
	if dto.SourceType != "CMS_PUF" {
		t.Errorf("expected SourceType 'CMS_PUF', got '%s'", dto.SourceType)
	}
	if dto.SourceDate != "2025-03-15" {
		t.Errorf("expected SourceDate '2025-03-15', got '%s'", dto.SourceDate)
	}
}

func TestFormularyEntryFromEnt_MinimalFields(t *testing.T) {
	sourceDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	entry := &ent.FormularyEntry{
		ID:         200,
		IsCovered:  false,
		SourceType: "QHP",
		SourceDate: sourceDate,
	}

	dto := FormularyEntryFromEnt(entry)

	if dto.EntryID != 200 {
		t.Errorf("expected EntryID 200, got %d", dto.EntryID)
	}
	if dto.IsCovered {
		t.Error("expected IsCovered false")
	}
	if dto.TierLevel != nil {
		t.Errorf("expected TierLevel nil for zero value, got %v", dto.TierLevel)
	}
	if dto.TierName != nil {
		t.Errorf("expected TierName nil for empty, got %v", dto.TierName)
	}
	if dto.QuantityLimitDetail != nil {
		t.Errorf("expected QuantityLimitDetail nil for empty, got %v", dto.QuantityLimitDetail)
	}
	if dto.CopayAmount != nil {
		t.Errorf("expected CopayAmount nil, got %v", dto.CopayAmount)
	}
	if dto.CoinsurancePct != nil {
		t.Errorf("expected CoinsurancePct nil, got %v", dto.CoinsurancePct)
	}
	if dto.CopayMailOrder != nil {
		t.Errorf("expected CopayMailOrder nil, got %v", dto.CopayMailOrder)
	}
	// No edges loaded
	if dto.PlanID != 0 {
		t.Errorf("expected PlanID 0 (no edge), got %d", dto.PlanID)
	}
	if dto.DrugID != 0 {
		t.Errorf("expected DrugID 0 (no edge), got %d", dto.DrugID)
	}
}

func TestPriorAuthCriteriaFromEnt_FullFields(t *testing.T) {
	ageMin := 18
	ageMax := 65
	maxQty := 30
	qtyPeriod := 90

	criteria := &ent.PriorAuthCriteria{
		ID:                     1,
		CriteriaType:           "diagnosis",
		CriteriaDescription:    "Requires documented diabetes diagnosis",
		RequiredDiagnoses:      []string{"E11.9", "E11.65"},
		AgeMin:                 &ageMin,
		AgeMax:                 &ageMax,
		GenderRestriction:      "female",
		StepTherapyDrugs:       []string{"metformin", "glipizide"},
		StepTherapyDescription: "Must try metformin first",
		MaxQuantity:            &maxQty,
		QuantityPeriodDays:     &qtyPeriod,
		SourceDocumentURL:      "https://cms.gov/pa-criteria/123",
	}
	criteria.Edges.FormularyEntry = &ent.FormularyEntry{ID: 50}

	dto := PriorAuthCriteriaFromEnt(criteria)

	if dto.CriteriaID != 1 {
		t.Errorf("expected CriteriaID 1, got %d", dto.CriteriaID)
	}
	if dto.EntryID != 50 {
		t.Errorf("expected EntryID 50, got %d", dto.EntryID)
	}
	if dto.CriteriaType != "diagnosis" {
		t.Errorf("expected CriteriaType 'diagnosis', got '%s'", dto.CriteriaType)
	}
	if dto.CriteriaDescription == nil || *dto.CriteriaDescription != "Requires documented diabetes diagnosis" {
		t.Errorf("unexpected CriteriaDescription: %v", dto.CriteriaDescription)
	}
	if len(dto.RequiredDiagnoses) != 2 {
		t.Errorf("expected 2 RequiredDiagnoses, got %d", len(dto.RequiredDiagnoses))
	}
	if dto.AgeMin == nil || *dto.AgeMin != 18 {
		t.Errorf("expected AgeMin 18, got %v", dto.AgeMin)
	}
	if dto.AgeMax == nil || *dto.AgeMax != 65 {
		t.Errorf("expected AgeMax 65, got %v", dto.AgeMax)
	}
	if dto.GenderRestriction == nil || *dto.GenderRestriction != "female" {
		t.Errorf("expected GenderRestriction 'female', got %v", dto.GenderRestriction)
	}
	if len(dto.StepTherapyDrugs) != 2 {
		t.Errorf("expected 2 StepTherapyDrugs, got %d", len(dto.StepTherapyDrugs))
	}
	if dto.StepTherapyDescription == nil || *dto.StepTherapyDescription != "Must try metformin first" {
		t.Errorf("unexpected StepTherapyDescription: %v", dto.StepTherapyDescription)
	}
	if dto.MaxQuantity == nil || *dto.MaxQuantity != 30 {
		t.Errorf("expected MaxQuantity 30, got %v", dto.MaxQuantity)
	}
	if dto.QuantityPeriodDays == nil || *dto.QuantityPeriodDays != 90 {
		t.Errorf("expected QuantityPeriodDays 90, got %v", dto.QuantityPeriodDays)
	}
	if dto.SourceDocumentUrl == nil || *dto.SourceDocumentUrl != "https://cms.gov/pa-criteria/123" {
		t.Errorf("unexpected SourceDocumentUrl: %v", dto.SourceDocumentUrl)
	}
}

func TestPriorAuthCriteriaFromEnt_EmptyOptionals(t *testing.T) {
	criteria := &ent.PriorAuthCriteria{
		ID:           2,
		CriteriaType: "step_therapy",
	}

	dto := PriorAuthCriteriaFromEnt(criteria)

	if dto.CriteriaDescription != nil {
		t.Errorf("expected nil CriteriaDescription, got %v", dto.CriteriaDescription)
	}
	if dto.GenderRestriction != nil {
		t.Errorf("expected nil GenderRestriction, got %v", dto.GenderRestriction)
	}
	if dto.StepTherapyDescription != nil {
		t.Errorf("expected nil StepTherapyDescription, got %v", dto.StepTherapyDescription)
	}
	if dto.SourceDocumentUrl != nil {
		t.Errorf("expected nil SourceDocumentUrl, got %v", dto.SourceDocumentUrl)
	}
	if dto.EntryID != 0 {
		t.Errorf("expected EntryID 0 (no edge), got %d", dto.EntryID)
	}
}

func TestDrugAlternativeFromEnt_FullFields(t *testing.T) {
	alt := &ent.DrugAlternative{
		ID:               1,
		RelationshipType: "GENERIC_EQUIVALENT",
		Source:           "RxNorm",
		Notes:            "Same active ingredient",
	}
	alt.Edges.Drug = &ent.Drug{ID: 10, Rxcui: "111", DrugName: "Brand Drug"}
	alt.Edges.AlternativeDrug = &ent.Drug{ID: 20, Rxcui: "222", DrugName: "Generic Drug"}

	dto := DrugAlternativeFromEnt(alt)

	if dto.AlternativeID != 1 {
		t.Errorf("expected AlternativeID 1, got %d", dto.AlternativeID)
	}
	if dto.RelationshipType != "GENERIC_EQUIVALENT" {
		t.Errorf("expected RelationshipType 'GENERIC_EQUIVALENT', got '%s'", dto.RelationshipType)
	}
	if dto.Source == nil || *dto.Source != "RxNorm" {
		t.Errorf("expected Source 'RxNorm', got %v", dto.Source)
	}
	if dto.Notes == nil || *dto.Notes != "Same active ingredient" {
		t.Errorf("expected Notes, got %v", dto.Notes)
	}
	if dto.DrugID != 10 {
		t.Errorf("expected DrugID 10, got %d", dto.DrugID)
	}
	if dto.AlternativeDrugID != 20 {
		t.Errorf("expected AlternativeDrugID 20, got %d", dto.AlternativeDrugID)
	}
	if dto.AlternativeDrug == nil {
		t.Fatal("expected AlternativeDrug to be non-nil")
	}
	if dto.AlternativeDrug.Rxcui != "222" {
		t.Errorf("expected AlternativeDrug Rxcui '222', got '%s'", dto.AlternativeDrug.Rxcui)
	}
}

func TestDrugAlternativeFromEnt_NoEdges(t *testing.T) {
	alt := &ent.DrugAlternative{
		ID:               2,
		RelationshipType: "THERAPEUTIC_ALTERNATIVE",
	}

	dto := DrugAlternativeFromEnt(alt)

	if dto.DrugID != 0 {
		t.Errorf("expected DrugID 0 (no edge), got %d", dto.DrugID)
	}
	if dto.AlternativeDrugID != 0 {
		t.Errorf("expected AlternativeDrugID 0 (no edge), got %d", dto.AlternativeDrugID)
	}
	if dto.AlternativeDrug != nil {
		t.Errorf("expected AlternativeDrug nil, got %v", dto.AlternativeDrug)
	}
	if dto.Source != nil {
		t.Errorf("expected Source nil, got %v", dto.Source)
	}
	if dto.Notes != nil {
		t.Errorf("expected Notes nil, got %v", dto.Notes)
	}
}
