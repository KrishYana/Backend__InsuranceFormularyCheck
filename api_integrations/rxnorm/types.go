package rxnorm

// AllConceptsResponse is the response from /REST/allconcepts.json
type AllConceptsResponse struct {
	MinConceptGroup struct {
		MinConcept []MinConcept `json:"minConcept"`
	} `json:"minConceptGroup"`
}

// MinConcept is a minimal concept from allconcepts.
type MinConcept struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
	TTY   string `json:"tty"`
}

// PropertiesResponse is the response from /REST/rxcui/{rxcui}/properties.json
type PropertiesResponse struct {
	Properties *ConceptProperties `json:"properties"`
}

// ConceptProperties holds full concept properties.
type ConceptProperties struct {
	RxCUI    string `json:"rxcui"`
	Name     string `json:"name"`
	Synonym  string `json:"synonym"`
	TTY      string `json:"tty"`
	Language string `json:"language"`
	Suppress string `json:"suppress"`
}

// NDCResponse is the response from /REST/rxcui/{rxcui}/ndcs.json
type NDCResponse struct {
	NDCGroup struct {
		NDCList struct {
			NDC []string `json:"ndc"`
		} `json:"ndcList"`
	} `json:"ndcGroup"`
}

// ClassResponse is the response from /REST/rxclass/class/byRxcui.json
type ClassResponse struct {
	RxclassDrugInfoList struct {
		RxclassDrugInfo []RxclassDrugInfo `json:"rxclassDrugInfo"`
	} `json:"rxclassDrugInfoList"`
}

// RxclassDrugInfo holds a drug-class relationship.
type RxclassDrugInfo struct {
	RxclassMinConceptItem struct {
		ClassName string `json:"className"`
		ClassID   string `json:"classId"`
		ClassType string `json:"classType"`
	} `json:"rxclassMinConceptItem"`
	Rela string `json:"rela"`
}

// AllRelatedResponse is the response from /REST/rxcui/{rxcui}/allrelated.json
type AllRelatedResponse struct {
	AllRelatedGroup struct {
		ConceptGroup []ConceptGroup `json:"conceptGroup"`
	} `json:"allRelatedGroup"`
}

// ConceptGroup is a group of concepts by term type.
type ConceptGroup struct {
	TTY            string          `json:"tty"`
	ConceptProperties []ConceptProperties `json:"conceptProperties"`
}
