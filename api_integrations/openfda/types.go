package openfda

// NDCResponse is the top-level response from /drug/ndc.json
type NDCResponse struct {
	Meta    Meta        `json:"meta"`
	Results []NDCRecord `json:"results"`
}

// Meta contains pagination info.
type Meta struct {
	Results MetaResults `json:"results"`
}

// MetaResults contains result counts.
type MetaResults struct {
	Skip  int `json:"skip"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// NDCRecord is a single product from the NDC directory.
type NDCRecord struct {
	ProductNDC       string             `json:"product_ndc"`
	BrandName        string             `json:"brand_name"`
	GenericName      string             `json:"generic_name"`
	DosageForm       string             `json:"dosage_form"`
	Route            []string           `json:"route"`
	DEASchedule      string             `json:"dea_schedule"`
	ProductType      string             `json:"product_type"`
	MarketingStartDate string           `json:"marketing_start_date"`
	MarketingEndDate   string           `json:"marketing_end_date"`
	ActiveIngredients []ActiveIngredient `json:"active_ingredients"`
	Packaging        []Packaging        `json:"packaging"`
	OpenFDA          OpenFDAFields      `json:"openfda"`
}

// ActiveIngredient holds ingredient info.
type ActiveIngredient struct {
	Name     string `json:"name"`
	Strength string `json:"strength"`
}

// Packaging holds NDC packaging info.
type Packaging struct {
	PackageNDC         string `json:"package_ndc"`
	Description        string `json:"description"`
	MarketingStartDate string `json:"marketing_start_date"`
	MarketingEndDate   string `json:"marketing_end_date"`
	Sample             bool   `json:"sample"`
}

// OpenFDAFields holds harmonized fields including rxcui.
type OpenFDAFields struct {
	RxCUI            []string `json:"rxcui"`
	ManufacturerName []string `json:"manufacturer_name"`
	BrandName        []string `json:"brand_name"`
	GenericName      []string `json:"generic_name"`
	Route            []string `json:"route"`
	SubstanceName    []string `json:"substance_name"`
}

// BulkDownloadMeta is the top-level structure of the openFDA bulk download index.
type BulkDownloadMeta struct {
	Results BulkResults `json:"results"`
}

// BulkResults holds the download partitions.
type BulkResults struct {
	Drug BulkDrug `json:"drug"`
}

// BulkDrug holds drug-specific download info.
type BulkDrug struct {
	NDC BulkPartitions `json:"ndc"`
}

// BulkPartitions holds the list of download partitions.
type BulkPartitions struct {
	Partitions []BulkPartition `json:"partitions"`
}

// BulkPartition is a single downloadable chunk.
type BulkPartition struct {
	SizeInBytes int    `json:"size_in_bytes"`
	Records     int    `json:"records"`
	DisplayName string `json:"display_name"`
	File        string `json:"file"`
}
