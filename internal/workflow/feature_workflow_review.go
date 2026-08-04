package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const featureWorkflowPolicyPath = ".rotta/quality-gates.yaml"

var retiredFeatureWorkflowArtifactPaths = []string{
	".rotta/tdd-log.md",
	".rotta/state.yaml",
	"specs/.approved",
	".rotta/review-evidence.yaml",
	"reports/judge_report.md",
	"assets/config/state-machine.yaml",
	".rotta/state-machine.yaml",
}

// FeatureWorkflowReview reports a review prepared from only the current
// feature runtime and its manifest-bound quality-gates policy.
type FeatureWorkflowReview struct {
	FeatureID        string
	ScenarioOrSlice  string
	BaselineSHA      string
	PolicyPath       string
	EvidencePaths    []string
	RetiredArtifacts []string
	MissingEvidence  []string
	Passed           bool
}

// ReviewFeatureWorkflow checks the current feature's TDD evidence. Retired
// root artifacts are reported only; they are never read as review evidence.
func ReviewFeatureWorkflow(repoRoot string) (FeatureWorkflowReview, error) {
	continued, err := ResumeFeatureWorkflow(repoRoot)
	if err != nil {
		return FeatureWorkflowReview{}, err
	}
	if err := requireQualityGatesInterface(repoRoot); err != nil {
		return FeatureWorkflowReview{}, err
	}

	stateContents, err := readRepositoryFile(repoRoot, ".rotta/current/state.yaml")
	if err != nil {
		return FeatureWorkflowReview{}, fmt.Errorf("read feature workflow state: %w", err)
	}
	state, err := parseFeatureWorkflowState(string(stateContents))
	if err != nil {
		return FeatureWorkflowReview{}, err
	}

	review := FeatureWorkflowReview{
		FeatureID:        continued.FeatureID,
		ScenarioOrSlice:  continued.ScenarioOrSlice,
		BaselineSHA:      state.BaselineSHA,
		PolicyPath:       featureWorkflowPolicyPath,
		EvidencePaths:    []string{".rotta/current/tdd-log.md"},
		RetiredArtifacts: append([]string(nil), continued.RetiredArtifacts...),
	}
	evidence, err := readRepositoryFile(repoRoot, review.EvidencePaths[0])
	if err != nil {
		return FeatureWorkflowReview{}, fmt.Errorf("read feature workflow TDD evidence: %w", err)
	}
	if !strings.Contains(string(evidence), review.ScenarioOrSlice) {
		review.MissingEvidence = []string{review.ScenarioOrSlice}
	}
	review.Passed = len(review.MissingEvidence) == 0
	return review, nil
}

func findRetiredFeatureWorkflowArtifacts(repoRoot string) []string {
	var retired []string
	for _, path := range retiredFeatureWorkflowArtifactPaths {
		if _, err := os.Lstat(filepath.Join(repoRoot, filepath.FromSlash(path))); err == nil {
			retired = append(retired, path)
		}
	}
	return retired
}
