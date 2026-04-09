package dto

import (
	"testing"

	"github.com/kyanaman/formularycheck/ent"
)

func TestInsurerFromEnt_FullFields(t *testing.T) {
	insurer := &ent.Insurer{
		ID:              1,
		InsurerName:     "Blue Cross Blue Shield",
		ParentCompany:   "BCBS Association",
		HiosIssuerID:    "12345",
		FhirEndpointURL: "https://fhir.bcbs.com",
		WebsiteURL:      "https://bcbs.com",
	}
	planCount := 42

	dto := InsurerFromEnt(insurer, &planCount)

	if dto.InsurerID != 1 {
		t.Errorf("expected InsurerID 1, got %d", dto.InsurerID)
	}
	if dto.InsurerName != "Blue Cross Blue Shield" {
		t.Errorf("expected InsurerName 'Blue Cross Blue Shield', got '%s'", dto.InsurerName)
	}
	if dto.ParentCompany == nil || *dto.ParentCompany != "BCBS Association" {
		t.Errorf("expected ParentCompany 'BCBS Association', got %v", dto.ParentCompany)
	}
	if dto.HiosIssuerId == nil || *dto.HiosIssuerId != "12345" {
		t.Errorf("expected HiosIssuerId '12345', got %v", dto.HiosIssuerId)
	}
	if dto.FhirEndpointUrl == nil || *dto.FhirEndpointUrl != "https://fhir.bcbs.com" {
		t.Errorf("expected FhirEndpointUrl, got %v", dto.FhirEndpointUrl)
	}
	if dto.WebsiteUrl == nil || *dto.WebsiteUrl != "https://bcbs.com" {
		t.Errorf("expected WebsiteUrl, got %v", dto.WebsiteUrl)
	}
	if dto.PlanCount == nil || *dto.PlanCount != 42 {
		t.Errorf("expected PlanCount 42, got %v", dto.PlanCount)
	}
}

func TestInsurerFromEnt_EmptyOptionals(t *testing.T) {
	insurer := &ent.Insurer{
		ID:          2,
		InsurerName: "Aetna",
	}

	dto := InsurerFromEnt(insurer, nil)

	if dto.InsurerID != 2 {
		t.Errorf("expected InsurerID 2, got %d", dto.InsurerID)
	}
	if dto.ParentCompany != nil {
		t.Errorf("expected ParentCompany nil, got %v", dto.ParentCompany)
	}
	if dto.HiosIssuerId != nil {
		t.Errorf("expected HiosIssuerId nil, got %v", dto.HiosIssuerId)
	}
	if dto.FhirEndpointUrl != nil {
		t.Errorf("expected FhirEndpointUrl nil, got %v", dto.FhirEndpointUrl)
	}
	if dto.WebsiteUrl != nil {
		t.Errorf("expected WebsiteUrl nil, got %v", dto.WebsiteUrl)
	}
	if dto.PlanCount != nil {
		t.Errorf("expected PlanCount nil, got %v", dto.PlanCount)
	}
}

func TestPlanFromEnt_FullFields(t *testing.T) {
	planYear := 2025
	plan := &ent.Plan{
		ID:        10,
		PlanName:  "Gold PPO",
		StateCode: "CA",
		PlanType:  "PPO",
		MarketType: "Individual",
		MetalLevel: "Gold",
		PlanYear:  &planYear,
		IsActive:  true,
	}
	// Simulate loaded insurer edge
	plan.Edges.Insurer = &ent.Insurer{ID: 5}

	dto := PlanFromEnt(plan)

	if dto.PlanID != 10 {
		t.Errorf("expected PlanID 10, got %d", dto.PlanID)
	}
	if dto.PlanName != "Gold PPO" {
		t.Errorf("expected PlanName 'Gold PPO', got '%s'", dto.PlanName)
	}
	if dto.StateCode != "CA" {
		t.Errorf("expected StateCode 'CA', got '%s'", dto.StateCode)
	}
	if dto.PlanType == nil || *dto.PlanType != "PPO" {
		t.Errorf("expected PlanType 'PPO', got %v", dto.PlanType)
	}
	if dto.MarketType == nil || *dto.MarketType != "Individual" {
		t.Errorf("expected MarketType 'Individual', got %v", dto.MarketType)
	}
	if dto.MetalLevel == nil || *dto.MetalLevel != "Gold" {
		t.Errorf("expected MetalLevel 'Gold', got %v", dto.MetalLevel)
	}
	if dto.PlanYear != 2025 {
		t.Errorf("expected PlanYear 2025, got %d", dto.PlanYear)
	}
	if !dto.IsActive {
		t.Error("expected IsActive true")
	}
	if dto.InsurerID != 5 {
		t.Errorf("expected InsurerID 5, got %d", dto.InsurerID)
	}
}

func TestPlanFromEnt_EmptyOptionals(t *testing.T) {
	plan := &ent.Plan{
		ID:       20,
		PlanName: "Basic",
		IsActive: false,
	}

	dto := PlanFromEnt(plan)

	if dto.PlanID != 20 {
		t.Errorf("expected PlanID 20, got %d", dto.PlanID)
	}
	if dto.PlanType != nil {
		t.Errorf("expected PlanType nil, got %v", dto.PlanType)
	}
	if dto.MarketType != nil {
		t.Errorf("expected MarketType nil, got %v", dto.MarketType)
	}
	if dto.MetalLevel != nil {
		t.Errorf("expected MetalLevel nil, got %v", dto.MetalLevel)
	}
	if dto.PlanYear != 0 {
		t.Errorf("expected PlanYear 0, got %d", dto.PlanYear)
	}
	if dto.IsActive {
		t.Error("expected IsActive false")
	}
	if dto.InsurerID != 0 {
		t.Errorf("expected InsurerID 0 (no insurer edge), got %d", dto.InsurerID)
	}
}

func TestPlansFromEnt(t *testing.T) {
	plans := []*ent.Plan{
		{ID: 1, PlanName: "Plan A", IsActive: true},
		{ID: 2, PlanName: "Plan B", IsActive: false},
	}

	dtos := PlansFromEnt(plans)

	if len(dtos) != 2 {
		t.Fatalf("expected 2 DTOs, got %d", len(dtos))
	}
	if dtos[0].PlanName != "Plan A" {
		t.Errorf("expected first PlanName 'Plan A', got '%s'", dtos[0].PlanName)
	}
	if dtos[1].PlanName != "Plan B" {
		t.Errorf("expected second PlanName 'Plan B', got '%s'", dtos[1].PlanName)
	}
}

func TestPlansFromEnt_Empty(t *testing.T) {
	dtos := PlansFromEnt([]*ent.Plan{})
	if len(dtos) != 0 {
		t.Errorf("expected 0 DTOs, got %d", len(dtos))
	}
}
