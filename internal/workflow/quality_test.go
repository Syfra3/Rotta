package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN715_PassingRequiredQualityMakesSnapshotEligibleForArchive(t *testing.T) {
	// REQ-111, REQ-112, REQ-114 -> SCN-715
	repo, ledger := initializeReviewForTest(t)
	updated, err := ApplyQualityReview(repo, QualityRequest(ledger.LedgerVersion, QualityPass))
	if err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
	if updated.Status != ArchiveStatus || updated.ReviewedCommit != workflowTestCommit || updated.QualityPolicyFingerprint != qualityPolicyFingerprint() {
		t.Fatalf("reviewed ledger = %#v, want exact reviewed Archive snapshot", updated)
	}
}

func TestSCN716_RequiredQualityFailureReturnsReviewToTDD(t *testing.T) {
	// REQ-111, REQ-113 -> SCN-716
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.Results[0].Dimensions[5] = QualityDimensionResult{Dimension: "security", Required: true, Applicable: true, Outcome: QualityFail, Reason: "blocking finding"}
	updated, err := ApplyQualityReview(repo, request)
	if err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
	if updated.Status != TDDStatus || updated.ReviewedCommit != "" {
		t.Fatalf("failed review ledger = %#v, want TDD without reviewed commit", updated)
	}
}

func TestSCN717_BlockedRequiredEvidenceRemainsInReview(t *testing.T) {
	// REQ-111, REQ-113, REQ-115 -> SCN-717
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.Results[0].Dimensions[8] = QualityDimensionResult{Dimension: "static_analysis", Required: true, Applicable: true, Outcome: QualityBlocked, Reason: "analyzer output unavailable"}
	updated, err := ApplyQualityReview(repo, request)
	if err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
	if updated.Status != ReviewStatus || updated.ReviewedCommit != "" {
		t.Fatalf("blocked review ledger = %#v, want Review without reviewed commit", updated)
	}
}

func TestSCN727_NotApplicableRequiresFalseApplicability(t *testing.T) {
	// REQ-111, REQ-117 -> SCN-727
	repo, ledger := initializeReviewForTest(t)
	policy := strings.Replace(qualityPolicyContents(t), `"id":"coverage","analyzer":"manual-evidence","gate":"required","applies_when":"all"`, `"id":"coverage","analyzer":"manual-evidence","gate":"required","applies_when":"changed_path_matches_any","changed_path_matches_any":["tests/*"]`, 1)
	mustWrite(t, filepath.Join(repo, QualityPolicyPath), policy)
	_, fingerprint, err := LoadQualityPolicy(repo)
	if err != nil {
		t.Fatalf("LoadQualityPolicy() error = %v", err)
	}
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.PolicyFingerprint = fingerprint
	request.Results[0].PolicyFingerprint = fingerprint
	request.ChangedPaths = []string{"internal/workflow/_quality.go"}
	request.Results[0].Dimensions[9] = QualityDimensionResult{Dimension: "coverage", Required: true, Applicable: false, Outcome: QualityNotApplicable, Reason: "changed paths do not match policy predicate"}
	if _, err := ApplyQualityReview(repo, request); err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
}

func TestSCN728_MalformedRequiredAnalyzerResultBlocksReview(t *testing.T) {
	// REQ-111, REQ-113 -> SCN-728
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.Results[0].Dimensions[8] = QualityDimensionResult{Dimension: "static_analysis", Required: true, Applicable: true, Outcome: QualityBlocked, Reason: "unreadable normalized result"}
	updated, err := ApplyQualityReview(repo, request)
	if err != nil || updated.Status != ReviewStatus {
		t.Fatalf("ApplyQualityReview() = %#v, %v; want blocked Review", updated, err)
	}
}

func TestQualityRejectsApplicableNotApplicable(t *testing.T) {
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.Results[0].Dimensions[9].Outcome = QualityNotApplicable
	_, err := ApplyQualityReview(repo, request)
	if err == nil || !strings.Contains(err.Error(), "cannot be not_applicable") {
		t.Fatalf("ApplyQualityReview() error = %v, want invalid applicability rejection", err)
	}
}

func initializeReviewForTest(t *testing.T) (string, SubmissionLedger) {
	t.Helper()
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801"})
	mustWrite(t, filepath.Join(repo, QualityPolicyPath), qualityPolicyContents(t))
	ledger, err := AcceptTDDBatch(repo, TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Worktree: testTDDWorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}})
	if err != nil {
		t.Fatalf("AcceptTDDBatch() error = %v", err)
	}
	ledger, err = PersistTransition(repo, TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: TDDStatus, TargetStatus: ReviewStatus, LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, AuthorizedScope: []string{"SCN-801"}, EvidenceRefs: []string{"evidence/tdd.json"}})
	if err != nil {
		t.Fatalf("PersistTransition() error = %v", err)
	}
	return repo, ledger
}

func QualityRequest(version uint64, outcome QualityOutcome) QualityReviewRequest {
	dimensions := make([]QualityDimensionResult, 0, len(CanonicalQualityDimensions))
	for _, dimension := range CanonicalQualityDimensions {
		dimensions = append(dimensions, QualityDimensionResult{Dimension: dimension, Required: true, Applicable: true, Outcome: outcome})
	}
	fingerprint := qualityPolicyFingerprint()
	return QualityReviewRequest{SubmissionID: "submission-A", LedgerVersion: version, Authorizer: orchestratorAuthorizer, CandidateCommit: workflowTestCommit, PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: fingerprint, Results: []QualityResult{{Format: "quality-result/v1", AnalyzerID: "manual-evidence", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: fingerprint, CandidateCommit: workflowTestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/adapter.json", Dimensions: dimensions}}, EvidenceRefs: []string{"evidence/quality.json"}}
}

func qualityPolicyFingerprint() string {
	contents, err := os.ReadFile(filepath.Join("..", "..", QualityPolicyPath))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(contents))
}

func TestQualityRejectsRequestThatDoesNotMatchCommittedPolicy(t *testing.T) {
	repo, ledger := initializeReviewForTest(t)
	request := QualityRequest(ledger.LedgerVersion, QualityPass)
	request.PolicyFingerprint = "sha256:stale"
	_, err := ApplyQualityReview(repo, request)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("ApplyQualityReview() error = %v, want committed-policy rejection", err)
	}
}
