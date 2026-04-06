package cmspuf

// PlanRow represents a row from the Plan Information file.
type PlanRow struct {
	ContractID   string
	PlanID       string
	SegmentID    string
	ContractName string
	PlanName     string
	FormularyID  string
	PlanType     string
	SNPType      string
}

// FormularyRow represents a row from the Basic Drugs Formulary file.
type FormularyRow struct {
	FormularyID       string
	RxCUI             string
	NDC               string
	TierLevelValue    int
	PriorAuthYN       bool
	StepTherapyYN     bool
	QuantityLimitYN   bool
	QuantityLimitAmt  float64
	QuantityLimitDays int
}

// BeneficiaryCostRow represents a row from the Beneficiary Cost file.
type BeneficiaryCostRow struct {
	ContractID     string
	PlanID         string
	SegmentID      string
	Tier           int
	DaysSupply     int
	CostTypePref   int // 1=copay, 2=coinsurance
	CostAmtPref    float64
	CostTypeNonPref int
	CostAmtNonPref  float64
	CostTypeMail   int
	CostAmtMail    float64
	CoveragePhase  string
}

// TierName maps tier level codes to human-readable names.
var TierName = map[int]string{
	1: "Preferred Generic",
	2: "Generic",
	3: "Preferred Brand",
	4: "Non-Preferred Drug",
	5: "Specialty Tier",
}
