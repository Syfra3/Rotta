package workflow

import "testing"

func TestQualityResultRequiresExactCompletedAdapterEvidence(t *testing.T) {
	policy := QualityPolicy{ID: "default", Version: "1", Analyzers: []QualityPolicyAnalyzer{{ID: "adapter-A", SupportedDimensions: []string{"security"}}}}
	result := QualityResult{Format: "quality-result/v1", AnalyzerID: "adapter-A", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: "sha256:policy", CandidateCommit: workflowTestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/quality.json", Dimensions: []QualityDimensionResult{{Dimension: "security"}}}
	if err := ValidateQualityResult(result, policy, "sha256:policy", workflowTestCommit); err != nil {
		t.Fatalf("ValidateQualityResult() error = %v", err)
	}
	result.ExecutionStatus = "failed"
	if err := ValidateQualityResult(result, policy, "sha256:policy", workflowTestCommit); err == nil {
		t.Fatal("expected failed adapter result rejection")
	}
}

func TestQualityResultsRequireEveryConfiguredAnalyzer(t *testing.T) {
	policy := QualityPolicy{ID: "default", Version: "1", Analyzers: []QualityPolicyAnalyzer{{ID: "adapter-A", SupportedDimensions: []string{"security"}}, {ID: "adapter-B", SupportedDimensions: []string{"coverage"}}}, Dimensions: []QualityPolicyDimension{{ID: "security"}, {ID: "coverage"}}}
	result := QualityResult{Format: "quality-result/v1", AnalyzerID: "adapter-A", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: "sha256:policy", CandidateCommit: workflowTestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/quality.json", Dimensions: []QualityDimensionResult{{Dimension: "security"}}}
	if _, err := MergeQualityResults([]QualityResult{result}, policy, "sha256:policy", workflowTestCommit); err == nil {
		t.Fatal("expected missing analyzer rejection")
	}
}
