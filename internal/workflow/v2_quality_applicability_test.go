package workflow

import "testing"

func TestV2ChangedPathApplicabilityRequiresCommittedPatternMatch(t *testing.T) {
	policy := V2QualityPolicy{Dimensions: []V2QualityPolicyDimension{{ID: "coverage", Required: true, AppliesWhen: "changed_path_matches_any", ChangedPathMatchesAny: []string{"internal/*.go"}}}}
	dimensions := make([]V2QualityDimensionResult, 0, len(v2CanonicalQualityDimensions))
	for _, id := range v2CanonicalQualityDimensions {
		result := V2QualityDimensionResult{Dimension: id, Required: true, Applicable: true, Outcome: V2QualityPass}
		if id == "coverage" {
			result.Applicable = false
			result.Outcome = V2QualityNotApplicable
		}
		dimensions = append(dimensions, result)
		if id != "coverage" {
			policy.Dimensions = append(policy.Dimensions, V2QualityPolicyDimension{ID: id, Required: true, AppliesWhen: "all"})
		}
	}
	if _, err := validateV2QualityDimensions(dimensions, policy, []string{"docs/readme.md"}); err != nil {
		t.Fatalf("validateV2QualityDimensions() error = %v", err)
	}
	dimensions[9].Applicable, dimensions[9].Outcome = true, V2QualityPass
	if _, err := validateV2QualityDimensions(dimensions, policy, []string{"docs/readme.md"}); err == nil {
		t.Fatal("expected applicability mismatch")
	}
}
