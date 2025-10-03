package validation

// ValidationResult represents the validation result for a single migration file
type ValidationResult struct {
	File     string   `json:"file"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
	Valid    bool     `json:"valid"`
}

// ValidationResults aggregates results from multiple migration files
type ValidationResults struct {
	Results []ValidationResult `json:"results"`
	Summary string             `json:"summary"`
}

// HasErrors returns true if any validation result contains errors
func (vr *ValidationResults) HasErrors() bool {
	for _, result := range vr.Results {
		if len(result.Errors) > 0 {
			return true
		}
	}
	return false
}

// ErrorCount returns the total number of errors across all results
func (vr *ValidationResults) ErrorCount() int {
	count := 0
	for _, result := range vr.Results {
		count += len(result.Errors)
	}
	return count
}

// WarningCount returns the total number of warnings across all results
func (vr *ValidationResults) WarningCount() int {
	count := 0
	for _, result := range vr.Results {
		count += len(result.Warnings)
	}
	return count
}

// AddResult adds a validation result to the collection
func (vr *ValidationResults) AddResult(result ValidationResult) {
	vr.Results = append(vr.Results, result)
}
