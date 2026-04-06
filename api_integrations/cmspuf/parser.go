package cmspuf

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParsePlans parses the pipe-delimited Plan Information file.
func ParsePlans(data []byte) ([]PlanRow, error) {
	reader := newPipeReader(data)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIndex := buildColumnIndex(header)
	var plans []PlanRow

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		plans = append(plans, PlanRow{
			ContractID:   getCol(record, colIndex, "CONTRACT_ID"),
			PlanID:       getCol(record, colIndex, "PLAN_ID"),
			SegmentID:    getCol(record, colIndex, "SEGMENT_ID"),
			ContractName: getCol(record, colIndex, "CONTRACT_NAME"),
			PlanName:     getCol(record, colIndex, "PLAN_NAME"),
			FormularyID:  getCol(record, colIndex, "FORMULARY_ID"),
			PlanType:     getCol(record, colIndex, "PLAN_TYPE"),
			SNPType:      getCol(record, colIndex, "SNP_TYPE"),
		})
	}

	return plans, nil
}

// ParseFormulary parses the pipe-delimited Basic Drugs Formulary file.
// It calls the handler function for each batch of rows to enable streaming.
func ParseFormulary(data []byte, batchSize int, handler func([]FormularyRow) error) error {
	reader := newPipeReader(data)

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	colIndex := buildColumnIndex(header)
	batch := make([]FormularyRow, 0, batchSize)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		row := FormularyRow{
			FormularyID:       getCol(record, colIndex, "FORMULARY_ID"),
			RxCUI:             getCol(record, colIndex, "RXCUI"),
			NDC:               getCol(record, colIndex, "NDC"),
			TierLevelValue:    getColInt(record, colIndex, "TIER_LEVEL_VALUE"),
			PriorAuthYN:       getCol(record, colIndex, "PRIOR_AUTHORIZATION_YN") == "Y",
			StepTherapyYN:     getCol(record, colIndex, "STEP_THERAPY_YN") == "Y",
			QuantityLimitYN:   getCol(record, colIndex, "QUANTITY_LIMIT_YN") == "Y",
			QuantityLimitAmt:  getColFloat(record, colIndex, "QUANTITY_LIMIT_AMOUNT"),
			QuantityLimitDays: getColInt(record, colIndex, "QUANTITY_LIMIT_DAYS"),
		}

		batch = append(batch, row)

		if len(batch) >= batchSize {
			if err := handler(batch); err != nil {
				return fmt.Errorf("handle batch: %w", err)
			}
			batch = batch[:0]
		}
	}

	// Process remaining rows
	if len(batch) > 0 {
		if err := handler(batch); err != nil {
			return fmt.Errorf("handle final batch: %w", err)
		}
	}

	return nil
}

// ParseBeneficiaryCost parses the pipe-delimited Beneficiary Cost file.
func ParseBeneficiaryCost(data []byte) ([]BeneficiaryCostRow, error) {
	reader := newPipeReader(data)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIndex := buildColumnIndex(header)
	var rows []BeneficiaryCostRow

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		rows = append(rows, BeneficiaryCostRow{
			ContractID:      getCol(record, colIndex, "CONTRACT_ID"),
			PlanID:          getCol(record, colIndex, "PLAN_ID"),
			SegmentID:       getCol(record, colIndex, "SEGMENT_ID"),
			Tier:            getColInt(record, colIndex, "TIER"),
			DaysSupply:      getColInt(record, colIndex, "DAYS_SUPPLY"),
			CostTypePref:    getColInt(record, colIndex, "COST_TYPE_PREF"),
			CostAmtPref:     getColFloat(record, colIndex, "COST_AMT_PREF"),
			CostTypeNonPref: getColInt(record, colIndex, "COST_TYPE_NONPREF"),
			CostAmtNonPref:  getColFloat(record, colIndex, "COST_AMT_NONPREF"),
			CostTypeMail:    getColInt(record, colIndex, "COST_TYPE_MAIL"),
			CostAmtMail:     getColFloat(record, colIndex, "COST_AMT_MAIL"),
			CoveragePhase:   getCol(record, colIndex, "COVERAGE_PHASE"),
		})
	}

	return rows, nil
}

func newPipeReader(data []byte) *csv.Reader {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '|'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // allow variable fields
	r.TrimLeadingSpace = true
	return r
}

func buildColumnIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, col := range header {
		index[strings.TrimSpace(strings.ToUpper(col))] = i
	}
	return index
}

func getCol(record []string, index map[string]int, col string) string {
	i, ok := index[col]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func getColInt(record []string, index map[string]int, col string) int {
	s := getCol(record, index, col)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func getColFloat(record []string, index map[string]int, col string) float64 {
	s := getCol(record, index, col)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
