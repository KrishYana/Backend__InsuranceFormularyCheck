package qhp

// Issuer represents a row from the MR-PUF CSV.
type Issuer struct {
	State    string
	IssuerID string
	URL      string // MR_URL_Submitted — points to index.json
	Email    string
}

// IndexJSON is the top-level issuer index file.
type IndexJSON struct {
	ProviderURLs  []string `json:"provider_urls"`
	FormularyURLs []string `json:"formulary_urls"`
	PlanURLs      []string `json:"plan_urls"`
}

// PlanJSON represents a single plan from plans.json.
type PlanJSON struct {
	PlanIDType    string          `json:"plan_id_type"`
	PlanID        string          `json:"plan_id"`
	MarketingName string          `json:"marketing_name"`
	SummaryURL    string          `json:"summary_url"`
	PlanContact   string          `json:"plan_contact"`
	Formulary     []TierDefinition `json:"formulary"`
	LastUpdatedOn string          `json:"last_updated_on"`
}

// TierDefinition defines a drug tier and its cost sharing within a plan.
type TierDefinition struct {
	DrugTier    string        `json:"drug_tier"`
	MailOrder   bool          `json:"mail_order"`
	CostSharing []CostSharing `json:"cost_sharing"`
}

// CostSharing holds copay/coinsurance for a pharmacy type.
type CostSharing struct {
	PharmacyType    string  `json:"pharmacy_type"`
	CopayAmount     float64 `json:"copay_amount"`
	CopayOpt        string  `json:"copay_opt"`
	CoinsuranceRate float64 `json:"coinsurance_rate"`
	CoinsuranceOpt  string  `json:"coinsurance_opt"`
}

// DrugJSON represents a single drug from drugs.json.
type DrugJSON struct {
	RxNormID string     `json:"rxnorm_id"`
	DrugName string     `json:"drug_name"`
	Plans    []DrugPlan `json:"plans"`
}

// DrugPlan maps a drug to a specific plan with coverage details.
type DrugPlan struct {
	PlanIDType         string `json:"plan_id_type"`
	PlanID             string `json:"plan_id"`
	DrugTier           string `json:"drug_tier"`
	PriorAuthorization bool   `json:"prior_authorization"`
	StepTherapy        bool   `json:"step_therapy"`
	QuantityLimit      bool   `json:"quantity_limit"`
}

// TierMapping maps QHP drug_tier strings to our tier_level integers.
var TierMapping = map[string]int{
	"GENERIC":                1,
	"PREFERRED-GENERIC":      1,
	"NON-PREFERRED-GENERIC":  2,
	"PREFERRED-BRAND":        3,
	"BRAND":                  3,
	"NON-PREFERRED-BRAND":    4,
	"SPECIALTY":              5,
	"ZERO-COST-SHARE-PREVENTIVE": 1,
	"MEDICAL-SERVICE":        5,
}
