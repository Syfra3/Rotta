package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const v2TestCommit = "0123456789abcdef0123456789abcdef01234567"

func TestSCN701_ExplicitNEWIgnoresLegacyArtifactsAndInitializesDraft(t *testing.T) {
	// REQ-101, REQ-104 -> SCN-701
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: complete\n")

	ledger, err := InitializeV2NewSubmission(repo, V2NewSubmissionRequest{
		SubmissionID: "submission-A",
		Draft:        "replace the legacy workflow",
		BaseCommit:   v2TestCommit,
	})
	if err != nil {
		t.Fatalf("InitializeV2NewSubmission() error = %v", err)
	}
	if ledger.Status != V2DraftStatus || ledger.LedgerVersion != 1 || ledger.BaseCommit != v2TestCommit {
		t.Fatalf("initial ledger = %#v, want Draft version 1 at selected base", ledger)
	}
	if len(ledger.Transitions) != 0 {
		t.Fatalf("initial ledger unexpectedly has lifecycle transitions: %#v", ledger.Transitions)
	}

	contents, err := os.ReadFile(v2LedgerPath(repo, "submission-A"))
	if err != nil {
		t.Fatalf("read created ledger: %v", err)
	}
	for _, prohibited := range []string{"contract_fingerprint", "contract_commit", "approved_scenarios", "reviewed_commit"} {
		if strings.Contains(string(contents), prohibited) {
			t.Fatalf("initial Draft ledger contains later lifecycle fact %q: %s", prohibited, contents)
		}
	}
}

func TestSCN743_ExplicitNEWRefusesAnyExistingV2IdentityArtifact(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-743
	for _, artifact := range []string{
		filepath.Join(".rotta", "v2", "submissions", "submission-A.yaml"),
		filepath.Join(".rotta", "v2", "contracts", "submission-A.yaml"),
	} {
		t.Run(filepath.Base(filepath.Dir(artifact)), func(t *testing.T) {
			repo := t.TempDir()
			mustWrite(t, filepath.Join(repo, artifact), "malformed: [\n")
			_, err := InitializeV2NewSubmission(repo, V2NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: v2TestCommit})
			if err == nil || !strings.Contains(err.Error(), "not fresh") {
				t.Fatalf("InitializeV2NewSubmission() error = %v, want non-fresh rejection", err)
			}
		})
	}
}

func TestSCN709_WorkerCannotAdvanceLifecycleState(t *testing.T) {
	// REQ-103, REQ-115 -> SCN-709
	repo := initializeV2LedgerForTest(t)
	_, err := PersistV2Transition(repo, V2TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: V2DraftStatus, TargetStatus: V2ArchiveStatus,
		LedgerVersion: 1, Authorizer: "review-worker", AuthorizedScope: []string{"SCN-709"}, EvidenceRefs: []string{"evidence/review.json"},
	})
	if err == nil || !strings.Contains(err.Error(), "expected orchestrator") {
		t.Fatalf("PersistV2Transition() error = %v, want unauthorized worker rejection", err)
	}
	ledger, err := LoadV2SubmissionLedger(repo, "submission-A")
	if err != nil {
		t.Fatalf("LoadV2SubmissionLedger() error = %v", err)
	}
	if ledger.Status != V2DraftStatus || ledger.LedgerVersion != 1 {
		t.Fatalf("ledger changed after worker request: %#v", ledger)
	}
}

func TestSCN710_StaleTransitionCannotOverwriteNewerState(t *testing.T) {
	// REQ-103, REQ-104, REQ-115 -> SCN-710
	repo := initializeV2LedgerForTest(t)
	_, err := PersistV2Transition(repo, V2TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: V2DraftStatus, TargetStatus: V2ContractStatus,
		LedgerVersion: 1, Authorizer: v2OrchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/draft.json"},
	})
	if err != nil {
		t.Fatalf("persist valid transition: %v", err)
	}
	_, err = PersistV2Transition(repo, V2TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: V2DraftStatus, TargetStatus: V2ContractStatus,
		LedgerVersion: 1, Authorizer: v2OrchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/delayed.json"},
	})
	if err == nil || !strings.Contains(err.Error(), "stale ledger version") {
		t.Fatalf("PersistV2Transition() error = %v, want stale-version rejection", err)
	}
	ledger, err := LoadV2SubmissionLedger(repo, "submission-A")
	if err != nil {
		t.Fatalf("LoadV2SubmissionLedger() error = %v", err)
	}
	if ledger.Status != V2ContractStatus || ledger.LedgerVersion != 2 || len(ledger.Transitions) != 1 {
		t.Fatalf("ledger after stale request = %#v, want one persisted transition", ledger)
	}
}

func TestInitializeV2NewSubmissionRejectsInvalidCommitAndDoesNotCreateState(t *testing.T) {
	repo := t.TempDir()
	_, err := InitializeV2NewSubmission(repo, V2NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: "short"})
	if err == nil || !strings.Contains(err.Error(), "full immutable") {
		t.Fatalf("InitializeV2NewSubmission() error = %v, want invalid commit rejection", err)
	}
	_, err = os.Stat(v2LedgerPath(repo, "submission-A"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ledger exists after invalid request: %v", err)
	}
}

func TestSCN736_ResumeDerivesContractStateWithoutAncoraPointer(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-736
	repo := initializeV2LedgerForTest(t)
	persistContractStateForTest(t, repo)

	resumed, err := ResumeV2Submission(repo, "submission-A", nil)
	if err != nil {
		t.Fatalf("ResumeV2Submission() error = %v", err)
	}
	if resumed.Ledger.Status != V2ContractStatus || resumed.Ledger.LedgerVersion != 2 || resumed.Contract.Fingerprint != "contract-F" {
		t.Fatalf("resumed state = %#v, want durable Contract state", resumed)
	}
	if resumed.AncoraPointerState != V2AncoraUnavailable {
		t.Fatalf("Ancora pointer state = %q, want unavailable", resumed.AncoraPointerState)
	}
}

func TestSCN737_StaleAncoraPointerCannotOverrideLedger(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-737
	repo := initializeV2LedgerForTest(t)
	persistContractStateForTest(t, repo)

	resumed, err := ResumeV2Submission(repo, "submission-A", &V2AncoraPointer{SubmissionID: "submission-A", LedgerVersion: 14, Status: V2ArchiveStatus, ContractFingerprint: "contract-F"})
	if err != nil {
		t.Fatalf("ResumeV2Submission() error = %v", err)
	}
	if resumed.Ledger.Status != V2ContractStatus || resumed.AncoraPointerState != V2AncoraStale {
		t.Fatalf("resumed state = %#v, want durable Contract state and stale Ancora diagnostic", resumed)
	}
}

func TestSCN738_ResumeFailsClosedWithoutRequiredDurableState(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-738
	for _, testCase := range []struct {
		name  string
		setup func(t *testing.T, repo string)
	}{
		{name: "missing ledger", setup: func(t *testing.T, repo string) {}},
		{name: "malformed ledger", setup: func(t *testing.T, repo string) { mustWrite(t, v2LedgerPath(repo, "submission-A"), "not json") }},
		{name: "missing contract", setup: func(t *testing.T, repo string) {
			initializeV2LedgerAt(t, repo)
			transitionV2LedgerToContract(t, repo)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := t.TempDir()
			testCase.setup(t, repo)
			_, err := ResumeV2Submission(repo, "submission-A", &V2AncoraPointer{SubmissionID: "submission-A", LedgerVersion: 8, Status: V2ContractStatus})
			if err == nil || !strings.Contains(err.Error(), "cannot establish durable v2 lifecycle state") || !strings.Contains(err.Error(), "safe next action") {
				t.Fatalf("ResumeV2Submission() error = %v, want fail-closed diagnostic", err)
			}
		})
	}
}

func TestSCN707_ExactContractApprovalAtomicallyAuthorizesTDD(t *testing.T) {
	// REQ-105, REQ-104 -> SCN-707
	repo := initializeV2LedgerForTest(t)
	persistContractStateForTest(t, repo)
	ledger, err := ApproveV2ContractForTDD(repo, V2ContractApprovalRequest{
		SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-F", ApprovedScenarioIDs: []string{"SCN-701"}, ContractCommit: v2TestCommit,
		Worktree: V2WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: v2TestCommit}, Authorizer: v2OrchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"},
		VerifyCommit: func(commit string) error {
			if commit != v2TestCommit {
				t.Fatalf("verified commit = %s", commit)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ApproveV2ContractForTDD() error = %v", err)
	}
	if ledger.Status != V2TDDStatus || ledger.LedgerVersion != 3 || ledger.ContractFingerprint != "contract-F" || ledger.ContractCommit != v2TestCommit || len(ledger.ApprovedScenarioIDs) != 1 {
		t.Fatalf("approved ledger = %#v, want atomically recorded TDD authorization", ledger)
	}
}

func TestSCN708_ChangedContractRequiresRenewedApproval(t *testing.T) {
	// REQ-105, REQ-102 -> SCN-708
	repo := initializeV2LedgerForTest(t)
	persistContractStateForTest(t, repo)
	_, err := ApproveV2ContractForTDD(repo, V2ContractApprovalRequest{SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-old", ApprovedScenarioIDs: []string{"SCN-701"}, ContractCommit: v2TestCommit, Worktree: V2WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: v2TestCommit}, Authorizer: v2OrchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"}, VerifyCommit: func(string) error { return nil }})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApproveV2ContractForTDD() error = %v, want stale approval rejection", err)
	}
}

func TestSCN711_RecordedWorktreeCannotBeSubstituted(t *testing.T) {
	// REQ-106, REQ-107, REQ-115 -> SCN-711
	identity := V2WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: v2TestCommit}
	if err := ValidateV2RecordedWorktree(identity, V2WorktreeObservation{Path: "/other/worktree", RepositoryID: "repo-A", Branch: "feature/submission-A", HeadCommit: v2TestCommit, Clean: true, Attached: true}); err == nil || !strings.Contains(err.Error(), "without substitution") {
		t.Fatalf("ValidateV2RecordedWorktree() error = %v, want substitution rejection", err)
	}
	if err := ValidateV2RecordedWorktree(identity, V2WorktreeObservation{Path: identity.Path, RepositoryID: identity.RepositoryID, Branch: identity.Branch, HeadCommit: identity.ExpectedCommit, Clean: true, Attached: true}); err != nil {
		t.Fatalf("ValidateV2RecordedWorktree() error = %v", err)
	}
}

func initializeV2LedgerForTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	initializeV2LedgerAt(t, repo)
	return repo
}

func initializeV2LedgerAt(t *testing.T, repo string) {
	t.Helper()
	if _, err := InitializeV2NewSubmission(repo, V2NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: v2TestCommit}); err != nil {
		t.Fatalf("initialize test ledger: %v", err)
	}
}

func persistContractStateForTest(t *testing.T, repo string) {
	t.Helper()
	if err := RecordV2ContractArtifact(repo, V2ContractArtifact{SubmissionID: "submission-A", Fingerprint: "contract-F", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/vela_v2_workflow.feature"}}); err != nil {
		t.Fatalf("RecordV2ContractArtifact() error = %v", err)
	}
	transitionV2LedgerToContract(t, repo)
}

func transitionV2LedgerToContract(t *testing.T, repo string) {
	t.Helper()
	if _, err := PersistV2Transition(repo, V2TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: V2DraftStatus, TargetStatus: V2ContractStatus, LedgerVersion: 1, Authorizer: v2OrchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/draft.json"}}); err != nil {
		t.Fatalf("persist Contract transition: %v", err)
	}
}
