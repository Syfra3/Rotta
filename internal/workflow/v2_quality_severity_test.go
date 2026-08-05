package workflow

import (
	"strings"
	"testing"
)

func TestV2QualityFindingMustRespectCommittedSeverityScale(t *testing.T) {
	policy, dimensions := v2SeverityTestInputs()
	dimensions[5].Findings = []V2QualityFinding{{ID: "SEC-1", Dimension: "security", Severity: "error", Message: "unsafe operation", Location: "internal/workflow/x.go:1"}}
	_, err := validateV2QualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "reports pass") {
		t.Fatalf("validateV2QualityDimensions() error = %v", err)
	}
	dimensions[5].Findings[0].Severity = "critical"
	_, err = validateV2QualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "absent from") {
		t.Fatalf("validateV2QualityDimensions() error = %v", err)
	}
}

func v2SeverityTestInputs() (V2QualityPolicy, []V2QualityDimensionResult) {
	policy := V2QualityPolicy{}
	results := make([]V2QualityDimensionResult, 0, len(v2CanonicalQualityDimensions))
	for _, id := range v2CanonicalQualityDimensions {
		policy.Dimensions = append(policy.Dimensions, V2QualityPolicyDimension{ID: id, Required: true, AppliesWhen: "all", SeverityScale: []string{"info", "warning", "error"}, BlockingSeverity: "error"})
		results = append(results, V2QualityDimensionResult{Dimension: id, Required: true, Applicable: true, Outcome: V2QualityPass})
	}
	return policy, results
}
