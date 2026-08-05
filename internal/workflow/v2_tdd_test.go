package workflow

import (
	"strings"
	"testing"
)

func TestSCN712_AuthorizedTDDBatchPreservesPerScenarioRGREvidence(t *testing.T) {
	// REQ-110, REQ-102, REQ-117 -> SCN-712
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801", "SCN-802"})
	updated, err := AcceptV2TDDBatch(repo, V2TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Worktree: v2WorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []V2ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red-801", Green: "green-801", Refactor: "refactor-801"}, {ScenarioID: "SCN-802", Red: "red-802", Green: "green-802", Refactor: "refactor-802"}}})
	if err != nil {
		t.Fatalf("AcceptV2TDDBatch() error = %v", err)
	}
	if updated.Status != V2TDDStatus || len(updated.AcceptedScenarioIDs) != 2 || updated.AcceptedScenarioIDs[0] != "SCN-801" || updated.AcceptedScenarioIDs[1] != "SCN-802" || len(updated.TDDEvidence) != 2 || updated.TDDEvidence[0].Red != "red-801" || updated.TDDEvidence[1].Refactor != "refactor-802" {
		t.Fatalf("updated ledger = %#v, want separately accepted TDD scenarios", updated)
	}
}

func TestSCN713_BlockedTDDRecordsOneTargetedVelaQuestion(t *testing.T) {
	// REQ-110, REQ-117 -> SCN-713
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801"})
	_, err := AcceptV2TDDBatch(repo, V2TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Worktree: v2WorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []V2ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}, TargetedVela: &V2TargetedVelaQuestion{Question: "who crosses this dependency boundary?", Boundary: "internal/workflow", Commit: v2TestCommit, Outcome: "uncertain"}})
	if err != nil {
		t.Fatalf("AcceptV2TDDBatch() error = %v", err)
	}
}

func TestSCN714_UnblockedTDDNeedsNoVelaQuestion(t *testing.T) {
	// REQ-110 -> SCN-714
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801"})
	_, err := AcceptV2TDDBatch(repo, V2TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Worktree: v2WorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []V2ScenarioRGREvidence{{ScenarioID: "SCN-801", Red: "red", Green: "green", Refactor: "refactor"}}})
	if err != nil {
		t.Fatalf("AcceptV2TDDBatch() error = %v", err)
	}
}

func TestV2TDDBatchRejectsUnapprovedOrIncompleteScenario(t *testing.T) {
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801"})
	_, err := AcceptV2TDDBatch(repo, V2TDDBatchRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Worktree: v2WorktreeObservation(), EvidenceRefs: []string{"evidence/tdd.json"}, Scenarios: []V2ScenarioRGREvidence{{ScenarioID: "SCN-unapproved", Red: "red", Green: "green", Refactor: ""}}})
	if err == nil || !strings.Contains(err.Error(), "invalid or incomplete") {
		t.Fatalf("AcceptV2TDDBatch() error = %v, want rejected scenario", err)
	}
}

func TestV2TDDCannotEnterReviewUntilEveryApprovedScenarioIsAccepted(t *testing.T) {
	repo, ledger := initializeV2TDDForTest(t, []string{"SCN-801", "SCN-802"})
	_, err := PersistV2Transition(repo, V2TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: V2TDDStatus, TargetStatus: V2ReviewStatus, LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, AuthorizedScope: []string{"SCN-801", "SCN-802"}, EvidenceRefs: []string{"evidence/tdd.json"}})
	if err == nil || !strings.Contains(err.Error(), "every approved scenario") {
		t.Fatalf("PersistV2Transition() error = %v, want incomplete TDD rejection", err)
	}
}

func initializeV2TDDForTest(t *testing.T, scenarios []string) (string, V2SubmissionLedger) {
	t.Helper()
	repo := initializeV2LedgerForTest(t)
	persistContractStateForTest(t, repo)
	ledger, err := ApproveV2ContractForTDD(repo, V2ContractApprovalRequest{SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-F", ApprovedScenarioIDs: scenarios, ContractCommit: v2TestCommit, Worktree: v2WorktreeIdentity(), Authorizer: v2OrchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"}, VerifyCommit: func(string) error { return nil }})
	if err != nil {
		t.Fatalf("ApproveV2ContractForTDD() error = %v", err)
	}
	return repo, ledger
}

func v2WorktreeIdentity() V2WorktreeIdentity {
	return V2WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: v2TestCommit}
}

func v2WorktreeObservation() V2WorktreeObservation {
	identity := v2WorktreeIdentity()
	return V2WorktreeObservation{Path: identity.Path, RepositoryID: identity.RepositoryID, Branch: identity.Branch, HeadCommit: identity.ExpectedCommit, Clean: true, Attached: true}
}
