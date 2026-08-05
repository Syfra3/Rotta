package workflow

import (
	"errors"
	"fmt"
)

type QualityResult struct {
	Format            string                   `json:"format"`
	AnalyzerID        string                   `json:"analyzer_id"`
	PolicyID          string                   `json:"policy_id"`
	PolicyVersion     string                   `json:"policy_version"`
	PolicyFingerprint string                   `json:"policy_fingerprint"`
	CandidateCommit   string                   `json:"candidate_commit"`
	ExecutionStatus   string                   `json:"execution_status"`
	EvidenceRef       string                   `json:"evidence_ref"`
	Dimensions        []QualityDimensionResult `json:"dimensions"`
}

// ValidateQualityResult accepts only complete, normalized adapter evidence.
// Failed execution and incomplete parseable output are intentionally rejected
// so the caller reports affected required dimensions as blocked.
func ValidateQualityResult(result QualityResult, policy QualityPolicy, fingerprint, candidateCommit string) error {
	if result.Format != "quality-result/v1" || result.PolicyID != policy.ID || result.PolicyVersion != policy.Version || result.PolicyFingerprint != fingerprint || result.CandidateCommit != candidateCommit || !isFullCommitID(result.CandidateCommit) || result.ExecutionStatus != "completed" || result.EvidenceRef == "" {
		return errors.New(" quality result is incomplete, mismatched, or not successfully executed")
	}
	var analyzer *QualityPolicyAnalyzer
	for index := range policy.Analyzers {
		if policy.Analyzers[index].ID == result.AnalyzerID {
			analyzer = &policy.Analyzers[index]
			break
		}
	}
	if analyzer == nil {
		return fmt.Errorf(" quality result names unknown analyzer %q", result.AnalyzerID)
	}
	if len(result.Dimensions) != len(analyzer.SupportedDimensions) {
		return errors.New(" quality result does not cover every supported dimension")
	}
	seen := make(map[string]bool, len(result.Dimensions))
	for _, dimension := range result.Dimensions {
		if !containsScenario(analyzer.SupportedDimensions, dimension.Dimension) || seen[dimension.Dimension] {
			return errors.New(" quality result has duplicate or unsupported dimensions")
		}
		seen[dimension.Dimension] = true
	}
	return nil
}

// MergeQualityResults validates every configured analyzer result before
// exposing normalized dimensions to the Review lifecycle operation.
func MergeQualityResults(results []QualityResult, policy QualityPolicy, fingerprint, candidateCommit string) ([]QualityDimensionResult, error) {
	if len(results) != len(policy.Analyzers) {
		return nil, errors.New(" quality review is blocked: every configured analyzer requires one result")
	}
	seen := make(map[string]bool, len(results))
	merged := make([]QualityDimensionResult, 0, len(policy.Dimensions))
	for _, result := range results {
		if seen[result.AnalyzerID] {
			return nil, errors.New(" quality review is blocked: duplicate analyzer result")
		}
		if err := ValidateQualityResult(result, policy, fingerprint, candidateCommit); err != nil {
			return nil, err
		}
		seen[result.AnalyzerID] = true
		merged = append(merged, result.Dimensions...)
	}
	if len(merged) != len(policy.Dimensions) {
		return nil, errors.New(" quality review is blocked: analyzer results do not cover every policy dimension")
	}
	return merged, nil
}
