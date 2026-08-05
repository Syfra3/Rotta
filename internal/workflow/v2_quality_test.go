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
	repo, ledger := initializeV2ReviewForTest(t)
	updated, err := ApplyV2QualityReview(repo, v2QualityRequest(ledger.LedgerVersion, V2QualityPass))
	if err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
	if updated.Status != V2ArchiveStatus || updated.ReviewedCommit != v2TestCommit || updated.QualityPolicyFingerprint != v2TestQualityPolicyFingerprint() {
		t.Fatalf("reviewed ledger = %#v, want exact reviewed Archive snapshot", updated)
	}
}

func TestSCN716_RequiredQualityFailureReturnsReviewToTDD(t *testing.T) {
	// REQ-111, REQ-113 -> SCN-716
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.Results[0].Dimensions[5] = V2QualityDimensionResult{Dimension: "security", Required: true, Applicable: true, Outcome: V2QualityFail, Reason: "blocking finding"}
	updated, err := ApplyV2QualityReview(repo, request)
	if err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
	if updated.Status != V2TDDStatus || updated.ReviewedCommit != "" {
		t.Fatalf("failed review ledger = %#v, want TDD without reviewed commit", updated)
	}
}

func TestSCN717_BlockedRequiredEvidenceRemainsInReview(t *testing.T) {
	// REQ-111, REQ-113, REQ-115 -> SCN-717
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.Results[0].Dimensions[8] = V2QualityDimensionResult{Dimension: "static_analysis", Required: true, Applicable: true, Outcome: V2QualityBlocked, Reason: "analyzer output unavailable"}
	updated, err := ApplyV2QualityReview(repo, request)
	if err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
	if updated.Status != V2ReviewStatus || updated.ReviewedCommit != "" {
		t.Fatalf("blocked review ledger = %#v, want Review without reviewed commit", updated)
	}
}

func TestSCN727_NotApplicableRequiresFalseApplicability(t *testing.T) {
	// REQ-111, REQ-117 -> SCN-727
	repo, ledger := initializeV2ReviewForTest(t)
	policy := strings.Replace(v2TestQualityPolicyContents(t), `"id": "coverage", "analyzer": "manual-evidence", "gate": "required", "applies_when": "all"`, `"id": "coverage", "analyzer": "manual-evidence", "gate": "required", "applies_when": "changed_path_matches_any", "changed_path_matches_any": ["tests/*"]`, 1)
	mustWrite(t, filepath.Join(repo, v2QualityPolicyPath), policy)
	_, fingerprint, err := LoadV2QualityPolicy(repo)
	if err != nil {
		t.Fatalf("LoadV2QualityPolicy() error = %v", err)
	}
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.PolicyFingerprint = fingerprint
	request.Results[0].PolicyFingerprint = fingerprint
	request.ChangedPaths = []string{"internal/workflow/v2_quality.go"}
	request.Results[0].Dimensions[9] = V2QualityDimensionResult{Dimension: "coverage", Required: true, Applicable: false, Outcome: V2QualityNotApplicable, Reason: "changed paths do not match policy predicate"}
	if _, err := ApplyV2QualityReview(repo, request); err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
}

func TestSCN728_MalformedRequiredAnalyzerResultBlocksReview(t *testing.T) {
	// REQ-111, REQ-113 -> SCN-728
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.Results[0].Dimensions[8] = V2QualityDimensionResult{Dimension: "static_analysis", Required: true, Applicable: true, Outcome: V2QualityBlocked, Reason: "unreadable normalized result"}
	updated, err := ApplyV2QualityReview(repo, request)
	if err != nil || updated.Status != V2ReviewStatus {
		t.Fatalf("ApplyV2QualityReview() = %#v, %v; want blocked Review", updated, err)
	}
}

func TestV2QualityRejectsApplicableNotApplicable(t *testing.T) {
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.Results[0].Dimensions[9].Outcome = V2QualityNotApplicable
	_, err := ApplyV2QualityReview(repo, request)
	if err == nil || !strings.Contains(err.Error(), "cannot be not_applicable") {
		t.Fatalf("ApplyV2QualityReview() error = %v, want invalid applicability rejection", err)
	}
}

func initializeV2ReviewForTest(t *testing.T) (string, V2SubmissionLedger) {
	t.Helper()
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801"})
	mustWrite(t, filepath.Join(repo, v2QualityPolicyPath), v2TestQualityPolicyContents(t))
	ledger, err := AcceptV2TDDBatch(repo, V2TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Worktree: v2WorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []V2ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}})
	if err != nil {
		t.Fatalf("AcceptV2TDDBatch() error = %v", err)
	}
	ledger, err = PersistV2Transition(repo, V2TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: V2TDDStatus, TargetStatus: V2ReviewStatus, LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, AuthorizedScope: []string{"SCN-801"}, EvidenceRefs: []string{"evidence/tdd.json"}})
	if err != nil {
		t.Fatalf("PersistV2Transition() error = %v", err)
	}
	return repo, ledger
}

func v2QualityRequest(version uint64, outcome V2QualityOutcome) V2QualityReviewRequest {
	dimensions := make([]V2QualityDimensionResult, 0, len(v2CanonicalQualityDimensions))
	for _, dimension := range v2CanonicalQualityDimensions {
		dimensions = append(dimensions, V2QualityDimensionResult{Dimension: dimension, Required: true, Applicable: true, Outcome: outcome})
	}
	fingerprint := v2TestQualityPolicyFingerprint()
	return V2QualityReviewRequest{SubmissionID: "submission-A", LedgerVersion: version, Authorizer: v2OrchestratorAuthorizer, CandidateCommit: v2TestCommit, PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: fingerprint, Results: []V2QualityResult{{Format: "quality-result/v1", AnalyzerID: "manual-evidence", PolicyID: "default", PolicyVersion: "1", PolicyFingerprint: fingerprint, CandidateCommit: v2TestCommit, ExecutionStatus: "completed", EvidenceRef: "evidence/adapter.json", Dimensions: dimensions}}, EvidenceRefs: []string{"evidence/quality.json"}}
}

func v2TestQualityPolicyFingerprint() string {
	contents, err := os.ReadFile(filepath.Join("..", "..", v2QualityPolicyPath))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(contents))
}

func TestV2QualityRejectsRequestThatDoesNotMatchCommittedPolicy(t *testing.T) {
	repo, ledger := initializeV2ReviewForTest(t)
	request := v2QualityRequest(ledger.LedgerVersion, V2QualityPass)
	request.PolicyFingerprint = "sha256:stale"
	_, err := ApplyV2QualityReview(repo, request)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("ApplyV2QualityReview() error = %v, want committed-policy rejection", err)
	}
}
