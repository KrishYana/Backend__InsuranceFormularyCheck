package dto

// InsightsSummaryDTO is the response shape for GET /insights/summary.
type InsightsSummaryDTO struct {
	TotalLookups        int                 `json:"totalLookups"`
	CoverageSuccessRate float64             `json:"coverageSuccessRate"`
	TopDrugs            []TopDrugDTO        `json:"topDrugs"`
	TopInsurers         []TopInsurerDTO     `json:"topInsurers"`
	TopPlans            []TopPlanDTO        `json:"topPlans"`
}

// TopDrugDTO represents a frequently searched drug.
type TopDrugDTO struct {
	Drug        DrugDTO `json:"drug"`
	SearchCount int     `json:"searchCount"`
}

// TopInsurerDTO represents a frequently searched insurer.
type TopInsurerDTO struct {
	Insurer     InsurerDTO `json:"insurer"`
	SearchCount int        `json:"searchCount"`
}

// TopPlanDTO represents a frequently checked plan.
type TopPlanDTO struct {
	Plan        PlanDTO `json:"plan"`
	SearchCount int     `json:"searchCount"`
}

// InsightsTrendsDTO is the response shape for GET /insights/trends.
type InsightsTrendsDTO struct {
	Period     string          `json:"period"`
	DataPoints []TrendPointDTO `json:"dataPoints"`
}

// TrendPointDTO is a single data point in a time series.
type TrendPointDTO struct {
	Date        string `json:"date"`
	LookupCount int    `json:"lookupCount"`
}
