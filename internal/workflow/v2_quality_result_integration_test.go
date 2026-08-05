package workflow

import "testing"

func TestV2ReviewUsesValidatedAnalyzerResultsWhenProvided(t *testing.T) {
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	if _, err := ApplyV2QualityReview(repo, request); err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
}
