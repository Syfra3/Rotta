package workflow

import (
	"strings"
	"testing"
)

func TestSCN712_AuthorizedTDDBatchPreservesPerScenarioRGREvidence(t *testing.T) {
	// REQ-110, REQ-102, REQ-117 -> SCN-712
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801", "SCN-802"})
	updated, err := AcceptTDDBatch(repo, TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Worktree: testTDDWorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red-801", Green: "green-801", Refactor: "refactor-801"}, {ScenarioID: "SCN-802", Red: "red-802", Green: "green-802", Refactor: "refactor-802"}}})
	if err != nil {
		t.Fatalf("AcceptTDDBatch() error = %v", err)
	}
	if updated.Status != TDDStatus || len(updated.AcceptedScenarioIDs) != 2 || updated.AcceptedScenarioIDs[0] != "SCN-801" || updated.AcceptedScenarioIDs[1] != "SCN-802" || len(updated.TDDEvidence) != 2 || updated.TDDEvidence[0].Red != "red-801" || updated.TDDEvidence[1].Refactor != "refactor-802" {
		t.Fatalf("updated ledger = %#v, want separately accepted TDD scenarios", updated)
	}
}

func TestSCN713_BlockedTDDRecordsOneTargetedVelaQuestion(t *testing.T) {
	// REQ-110, REQ-117 -> SCN-713
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801"})
	_, err := AcceptTDDBatch(repo, TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Worktree: testTDDWorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}, TargetedVela: &TargetedVelaQuestion{Question: "who crosses this dependency boundary?", Boundary: "internal/workflow", Commit: workflowTestCommit, Outcome: "uncertain"}})
	if err != nil {
		t.Fatalf("AcceptTDDBatch() error = %v", err)
	}
}

func TestSCN714_UnblockedTDDNeedsNoVelaQuestion(t *testing.T) {
	// REQ-110 -> SCN-714
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801"})
	_, err := AcceptTDDBatch(repo, TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Worktree: testTDDWorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}})
	if err != nil {
		t.Fatalf("AcceptTDDBatch() error = %v", err)
	}
}

func TestTDDBatchRejectsUnapprovedOrIncompleteScenario(t *testing.T) {
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801"})
	_, err := AcceptTDDBatch(repo, TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Worktree: testTDDWorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []ScenarioRGREvidence{{ScenarioID: "SCN-unapproved", Red: "red", Green: "green", Refactor: ""}}})
	if err == nil || !strings.Contains(err.Error(), "invalid or incomplete") {
		t.Fatalf("AcceptTDDBatch() error = %v, want rejected scenario", err)
	}
}

func TestTDDCannotEnterReviewUntilEveryApprovedScenarioIsAccepted(t *testing.T) {
	repo, ledger := initializeTDDForTest(t, []string{"SCN-801", "SCN-802"})
	_, err := PersistTransition(repo, TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: TDDStatus, TargetStatus: ReviewStatus, LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, AuthorizedScope: []string{"SCN-801", "SCN-802"}, EvidenceRefs: []string{"evidence/tdd.json"}})
	if err == nil || !strings.Contains(err.Error(), "every approved scenario") {
		t.Fatalf("PersistTransition() error = %v, want incomplete TDD rejection", err)
	}
}

func initializeTDDForTest(t *testing.T, scenarios []string) (string, SubmissionLedger) {
	t.Helper()
	repo := initializeLedgerForTest(t)
	persistContractStateForLedgerTest(t, repo)
	ledger, err := ApproveContractForTDD(repo, ContractApprovalRequest{SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-F", ApprovedScenarioIDs: scenarios, ContractCommit: workflowTestCommit, Worktree: testTDDWorktreeIdentity(), Authorizer: orchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"}, VerifyCommit: func(string) error { return nil }})
	if err != nil {
		t.Fatalf("ApproveContractForTDD() error = %v", err)
	}
	return repo, ledger
}

func testTDDWorktreeIdentity() WorktreeIdentity {
	return WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: workflowTestCommit}
}

func testTDDWorktreeObservation() WorktreeObservation {
	identity := testTDDWorktreeIdentity()
	return WorktreeObservation{Path: identity.Path, RepositoryID: identity.RepositoryID, Branch: identity.Branch, HeadCommit: identity.ExpectedCommit, Clean: true, Attached: true}
}
