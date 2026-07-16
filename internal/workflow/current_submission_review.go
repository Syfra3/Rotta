package workflow

import (
	"strings"
)

// CurrentSubmissionReview contains Judge evidence results for one active
// submission. ScenarioIDs is always the scope declared by its manifest.
type CurrentSubmissionReview struct {
	ScenarioIDs     []string
	MissingEvidence []string
	Warnings        []string
	Passed          bool
}

// ReviewCurrentSubmission checks current TDD evidence only for scenarios
// explicitly declared by the active submission manifest.
func ReviewCurrentSubmission(repoRoot string) (CurrentSubmissionReview, error) {
	submission, err := LoadCurrentSubmission(repoRoot)
	if err != nil {
		return CurrentSubmissionReview{}, err
	}

	review := CurrentSubmissionReview{
		ScenarioIDs: append([]string(nil), submission.Manifest.ScenarioIDs...),
	}
	evidence, _ := readRepositoryFile(repoRoot, ".rotta/current/tdd-log.md")
	for _, scenarioID := range review.ScenarioIDs {
		if !strings.Contains(string(evidence), scenarioID) {
			review.MissingEvidence = append(review.MissingEvidence, scenarioID)
		}
	}
	review.Passed = len(review.MissingEvidence) == 0
	return review, nil
}
