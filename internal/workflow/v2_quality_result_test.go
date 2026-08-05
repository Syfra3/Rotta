package workflow

import "testing"

func TestV2QualityResultRequiresExactCompletedAdapterEvidence(t *testing.T) {
	policy := V2QualityPolicy{ID: "default", Version: "1", Analyzers: []V2QualityPolicyAnalyzer{{ID: "adapter-A", SupportedDimensions: []string{"security"}}}}
	result := V2QualityResult{Format: "quality-result/v1", AnalyzerID: "adapter-A", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: "sha256:policy", CandidateCommit: v2TestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/quality.json", Dimensions: []V2QualityDimensionResult{{Dimension: "security"}}}
	if err := ValidateV2QualityResult(result, policy, "sha256:policy", v2TestCommit); err != nil {
		t.Fatalf("ValidateV2QualityResult() error = %v", err)
	}
	result.ExecutionStatus = "failed"
	if err := ValidateV2QualityResult(result, policy, "sha256:policy", v2TestCommit); err == nil {
		t.Fatal("expected failed adapter result rejection")
	}
}

func TestV2QualityResultsRequireEveryConfiguredAnalyzer(t *testing.T) {
	policy := V2QualityPolicy{ID: "default", Version: "1", Analyzers: []V2QualityPolicyAnalyzer{{ID: "adapter-A", SupportedDimensions: []string{"security"}}, {ID: "adapter-B", SupportedDimensions: []string{"coverage"}}}, Dimensions: []V2QualityPolicyDimension{{ID: "security"}, {ID: "coverage"}}}
	result := V2QualityResult{Format: "quality-result/v1", AnalyzerID: "adapter-A", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: "sha256:policy", CandidateCommit: v2TestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/quality.json", Dimensions: []V2QualityDimensionResult{{Dimension: "security"}}}
	if _, err := MergeV2QualityResults([]V2QualityResult{result}, policy, "sha256:policy", v2TestCommit); err == nil {
		t.Fatal("expected missing analyzer rejection")
	}
}
