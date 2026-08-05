package workflow

import "testing"

func TestChangedPathApplicabilityRequiresCommittedPatternMatch(t *testing.T) {
	policy := QualityPolicy{Dimensions: []QualityPolicyDimension{{ID: "coverage", Required: true, AppliesWhen: "changed_path_matches_any", ChangedPathMatchesAny: []string{"internal/*.go"}}}}
	dimensions := make([]QualityDimensionResult, 0, len(CanonicalQualityDimensions))
	for _, id := range CanonicalQualityDimensions {
		result := QualityDimensionResult{Dimension: id, Required: true, Applicable: true, Outcome: QualityPass}
		if id == "coverage" {
			result.Applicable = false
			result.Outcome = QualityNotApplicable
		}
		dimensions = append(dimensions, result)
		if id != "coverage" {
			policy.Dimensions = append(policy.Dimensions, QualityPolicyDimension{ID: id, Required: true, AppliesWhen: "all"})
		}
	}
	if _, err := validateQualityDimensions(dimensions, policy, []string{"docs/readme.md"}); err != nil {
		t.Fatalf("validateQualityDimensions() error = %v", err)
	}
	dimensions[9].Applicable, dimensions[9].Outcome = true, QualityPass
	if _, err := validateQualityDimensions(dimensions, policy, []string{"docs/readme.md"}); err == nil {
		t.Fatal("expected applicability mismatch")
	}
}
