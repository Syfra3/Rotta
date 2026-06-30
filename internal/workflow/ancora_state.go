package workflow

import "encoding/json"

type AncoraWorkflowState struct {
	SpecPath       string            `json:"spec_path"`
	FeaturePaths   []string          `json:"feature_paths"`
	Phase          string            `json:"phase"`
	ApprovalStatus string            `json:"approval_status"`
	RiskLevel      string            `json:"risk_level"`
	RequirementIDs []string          `json:"requirement_ids"`
	ScenarioIDs    []string          `json:"scenario_ids"`
	ObservationIDs []string          `json:"observation_ids,omitempty"`
	Checksums      map[string]string `json:"checksums,omitempty"`
}

func SerializeAncoraWorkflowState(state AncoraWorkflowState) ([]byte, error) {
	return json.Marshal(state)
}
