package dto

import (
	"encoding/json"
	"testing"
)

func TestInsightsSummaryDTO_JSONSerialization(t *testing.T) {
	summary := InsightsSummaryDTO{
		TotalLookups:        150,
		CoverageSuccessRate: 0.73,
		TopDrugs: []TopDrugDTO{
			{
				Drug:        DrugDTO{DrugID: 1, Rxcui: "111", DrugName: "Atorvastatin"},
				SearchCount: 25,
			},
		},
		TopInsurers: []TopInsurerDTO{
			{
				Insurer:     InsurerDTO{InsurerID: 1, InsurerName: "Aetna"},
				SearchCount: 30,
			},
		},
		TopPlans: []TopPlanDTO{
			{
				Plan:        PlanDTO{PlanID: 1, PlanName: "Gold PPO"},
				SearchCount: 20,
			},
		},
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal InsightsSummaryDTO: %v", err)
	}

	var decoded InsightsSummaryDTO
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal InsightsSummaryDTO: %v", err)
	}

	if decoded.TotalLookups != 150 {
		t.Errorf("expected TotalLookups 150, got %d", decoded.TotalLookups)
	}
	if decoded.CoverageSuccessRate != 0.73 {
		t.Errorf("expected CoverageSuccessRate 0.73, got %f", decoded.CoverageSuccessRate)
	}
	if len(decoded.TopDrugs) != 1 {
		t.Errorf("expected 1 TopDrugs, got %d", len(decoded.TopDrugs))
	}
	if len(decoded.TopInsurers) != 1 {
		t.Errorf("expected 1 TopInsurers, got %d", len(decoded.TopInsurers))
	}
	if len(decoded.TopPlans) != 1 {
		t.Errorf("expected 1 TopPlans, got %d", len(decoded.TopPlans))
	}
}

func TestInsightsTrendsDTO_JSONSerialization(t *testing.T) {
	trends := InsightsTrendsDTO{
		Period: "weekly",
		DataPoints: []TrendPointDTO{
			{Date: "2025-W01", LookupCount: 10},
			{Date: "2025-W02", LookupCount: 15},
			{Date: "2025-W03", LookupCount: 8},
		},
	}

	data, err := json.Marshal(trends)
	if err != nil {
		t.Fatalf("failed to marshal InsightsTrendsDTO: %v", err)
	}

	var decoded InsightsTrendsDTO
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal InsightsTrendsDTO: %v", err)
	}

	if decoded.Period != "weekly" {
		t.Errorf("expected Period 'weekly', got '%s'", decoded.Period)
	}
	if len(decoded.DataPoints) != 3 {
		t.Fatalf("expected 3 DataPoints, got %d", len(decoded.DataPoints))
	}
	if decoded.DataPoints[0].Date != "2025-W01" {
		t.Errorf("expected first date '2025-W01', got '%s'", decoded.DataPoints[0].Date)
	}
	if decoded.DataPoints[1].LookupCount != 15 {
		t.Errorf("expected second LookupCount 15, got %d", decoded.DataPoints[1].LookupCount)
	}
}

func TestTopDrugDTO_JSONKeys(t *testing.T) {
	dto := TopDrugDTO{
		Drug:        DrugDTO{DrugID: 5, Rxcui: "999", DrugName: "TestDrug"},
		SearchCount: 42,
	}

	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	if _, ok := m["drug"]; !ok {
		t.Error("expected 'drug' key in JSON output")
	}
	if _, ok := m["searchCount"]; !ok {
		t.Error("expected 'searchCount' key in JSON output")
	}
}

func TestInsightsSummaryDTO_ZeroValues(t *testing.T) {
	summary := InsightsSummaryDTO{}

	if summary.TotalLookups != 0 {
		t.Errorf("expected TotalLookups 0, got %d", summary.TotalLookups)
	}
	if summary.CoverageSuccessRate != 0 {
		t.Errorf("expected CoverageSuccessRate 0, got %f", summary.CoverageSuccessRate)
	}
	if summary.TopDrugs != nil {
		t.Errorf("expected TopDrugs nil, got %v", summary.TopDrugs)
	}
	if summary.TopInsurers != nil {
		t.Errorf("expected TopInsurers nil, got %v", summary.TopInsurers)
	}
	if summary.TopPlans != nil {
		t.Errorf("expected TopPlans nil, got %v", summary.TopPlans)
	}
}
