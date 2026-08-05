package workflow

import (
	"strings"
	"testing"
)

func TestQualityFindingMustRespectCommittedSeverityScale(t *testing.T) {
	policy, dimensions := SeverityTestInputs()
	dimensions[5].Findings = []QualityFinding{{ID: "SEC-1", Dimension: "security", Severity: "error", Message: "unsafe operation", Location: "internal/workflow/x.go:1"}}
	_, err := validateQualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "reports pass") {
		t.Fatalf("validateQualityDimensions() error = %v", err)
	}
	dimensions[5].Findings[0].Severity = "critical"
	_, err = validateQualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "absent from") {
		t.Fatalf("validateQualityDimensions() error = %v", err)
	}
}

func SeverityTestInputs() (QualityPolicy, []QualityDimensionResult) {
	policy := QualityPolicy{}
	results := make([]QualityDimensionResult, 0, len(CanonicalQualityDimensions))
	for _, id := range CanonicalQualityDimensions {
		policy.Dimensions = append(policy.Dimensions, QualityPolicyDimension{ID: id, Required: true, AppliesWhen: "all", SeverityScale: []string{"info", "warning", "error"}, BlockingSeverity: "error"})
		results = append(results, QualityDimensionResult{Dimension: id, Required: true, Applicable: true, Outcome: QualityPass})
	}
	return policy, results
}
