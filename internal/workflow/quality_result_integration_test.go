package workflow

import "testing"

func TestReviewUsesValidatedAnalyzerResultsWhenProvided(t *testing.T) {
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	if _, err := ApplyQualityReview(repo, request); err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
}
