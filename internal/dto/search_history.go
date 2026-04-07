package dto

import "github.com/kyanaman/formularycheck/ent"

// SearchHistoryDTO matches the frontend SearchHistoryEntry type.
type SearchHistoryDTO struct {
	SearchID     int      `json:"searchId"`
	StateCode    *string  `json:"stateCode"`
	PlanID       *int     `json:"planId"`
	DrugID       *int     `json:"drugId"`
	SearchText   *string  `json:"searchText"`
	ResultsCount *int     `json:"resultsCount"`
	SearchedAt   string   `json:"searchedAt"`
	Plan         *PlanDTO `json:"plan,omitempty"`
	Drug         *DrugDTO `json:"drug,omitempty"`
}

// SearchHistoryFromEnt converts an Ent SearchHistory to a DTO.
func SearchHistoryFromEnt(sh *ent.SearchHistory) SearchHistoryDTO {
	dto := SearchHistoryDTO{
		SearchID:     sh.ID,
		ResultsCount: sh.ResultsCount,
		SearchedAt:   sh.SearchedAt.Format("2006-01-02T15:04:05Z"),
	}
	if sh.StateCode != "" {
		dto.StateCode = &sh.StateCode
	}
	if sh.SearchText != "" {
		dto.SearchText = &sh.SearchText
	}
	if p := sh.Edges.Plan; p != nil {
		id := p.ID
		dto.PlanID = &id
		planDTO := PlanFromEnt(p)
		dto.Plan = &planDTO
	}
	if d := sh.Edges.Drug; d != nil {
		id := d.ID
		dto.DrugID = &id
		drugDTO := DrugFromEnt(d)
		dto.Drug = &drugDTO
	}
	return dto
}
