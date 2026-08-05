package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const workflowTestCommit = "0123456789abcdef0123456789abcdef01234567"

func TestSCN701_ExplicitNEWIgnoresLegacyArtifactsAndInitializesDraft(t *testing.T) {
	// REQ-101, REQ-104 -> SCN-701
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: complete\n")

	ledger, err := InitializeNewSubmission(repo, NewSubmissionRequest{
		SubmissionID: "submission-A",
		Draft:        "replace the legacy workflow",
		BaseCommit:   workflowTestCommit,
	})
	if err != nil {
		t.Fatalf("InitializeNewSubmission() error = %v", err)
	}
	if ledger.Status != DraftStatus || ledger.LedgerVersion != 1 || ledger.BaseCommit != workflowTestCommit {
		t.Fatalf("initial ledger = %#v, want Draft version 1 at selected base", ledger)
	}
	if len(ledger.Transitions) != 0 {
		t.Fatalf("initial ledger unexpectedly has lifecycle transitions: %#v", ledger.Transitions)
	}

	contents, err := os.ReadFile(ledgerPath(repo, "submission-A"))
	if err != nil {
		t.Fatalf("read created ledger: %v", err)
	}
	for _, prohibited := range []string{"contract_fingerprint", "contract_commit", "approved_scenarios", "reviewed_commit"} {
		if strings.Contains(string(contents), prohibited) {
			t.Fatalf("initial Draft ledger contains later lifecycle fact %q: %s", prohibited, contents)
		}
	}
}

func TestSCN743_ExplicitNEWRefusesAnyExistingWorkflowIdentityArtifact(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-743
	for _, artifact := range []string{
		filepath.Join(".rotta", "workflow", "submissions", "submission-A.yaml"),
		filepath.Join(".rotta", "workflow", "contracts", "submission-A.yaml"),
	} {
		t.Run(filepath.Base(filepath.Dir(artifact)), func(t *testing.T) {
			repo := t.TempDir()
			mustWrite(t, filepath.Join(repo, artifact), "malformed: [\n")
			_, err := InitializeNewSubmission(repo, NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: workflowTestCommit})
			if err == nil || !strings.Contains(err.Error(), "not fresh") {
				t.Fatalf("InitializeNewSubmission() error = %v, want non-fresh rejection", err)
			}
		})
	}
}

func TestSCN709_WorkerCannotAdvanceLifecycleState(t *testing.T) {
	// REQ-103, REQ-115 -> SCN-709
	repo := initializeLedgerForTest(t)
	_, err := PersistTransition(repo, TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: DraftStatus, TargetStatus: ArchiveStatus,
		LedgerVersion: 1, Authorizer: "review-worker", AuthorizedScope: []string{"SCN-709"}, EvidenceRefs: []string{"evidence/review.json"},
	})
	if err == nil || !strings.Contains(err.Error(), "expected orchestrator") {
		t.Fatalf("PersistTransition() error = %v, want unauthorized worker rejection", err)
	}
	ledger, err := LoadSubmissionLedger(repo, "submission-A")
	if err != nil {
		t.Fatalf("LoadSubmissionLedger() error = %v", err)
	}
	if ledger.Status != DraftStatus || ledger.LedgerVersion != 1 {
		t.Fatalf("ledger changed after worker request: %#v", ledger)
	}
}

func TestSCN710_StaleTransitionCannotOverwriteNewerState(t *testing.T) {
	// REQ-103, REQ-104, REQ-115 -> SCN-710
	repo := initializeLedgerForTest(t)
	_, err := PersistTransition(repo, TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: DraftStatus, TargetStatus: ContractStatus,
		LedgerVersion: 1, Authorizer: orchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/draft.json"},
	})
	if err != nil {
		t.Fatalf("persist valid transition: %v", err)
	}
	_, err = PersistTransition(repo, TransitionRequest{
		SubmissionID: "submission-A", ExpectedStatus: DraftStatus, TargetStatus: ContractStatus,
		LedgerVersion: 1, Authorizer: orchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/delayed.json"},
	})
	if err == nil || !strings.Contains(err.Error(), "stale ledger version") {
		t.Fatalf("PersistTransition() error = %v, want stale-version rejection", err)
	}
	ledger, err := LoadSubmissionLedger(repo, "submission-A")
	if err != nil {
		t.Fatalf("LoadSubmissionLedger() error = %v", err)
	}
	if ledger.Status != ContractStatus || ledger.LedgerVersion != 2 || len(ledger.Transitions) != 1 {
		t.Fatalf("ledger after stale request = %#v, want one persisted transition", ledger)
	}
}

func TestInitializeNewSubmissionRejectsInvalidCommitAndDoesNotCreateState(t *testing.T) {
	repo := t.TempDir()
	_, err := InitializeNewSubmission(repo, NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: "short"})
	if err == nil || !strings.Contains(err.Error(), "full immutable") {
		t.Fatalf("InitializeNewSubmission() error = %v, want invalid commit rejection", err)
	}
	_, err = os.Stat(ledgerPath(repo, "submission-A"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ledger exists after invalid request: %v", err)
	}
}

func TestSCN736_ResumeDerivesContractStateWithoutAncoraPointer(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-736
	repo := initializeLedgerForTest(t)
	persistContractStateForLedgerTest(t, repo)

	resumed, err := ResumeSubmission(repo, "submission-A", nil)
	if err != nil {
		t.Fatalf("ResumeSubmission() error = %v", err)
	}
	if resumed.Ledger.Status != ContractStatus || resumed.Ledger.LedgerVersion != 2 || resumed.Contract.Fingerprint != "contract-F" {
		t.Fatalf("resumed state = %#v, want durable Contract state", resumed)
	}
	if resumed.AncoraPointerState != AncoraUnavailable {
		t.Fatalf("Ancora pointer state = %q, want unavailable", resumed.AncoraPointerState)
	}
}

func TestSCN737_StaleAncoraPointerCannotOverrideLedger(t *testing.T) {
	// REQ-104, REQ-115 -> SCN-737
	repo := initializeLedgerForTest(t)
	persistContractStateForLedgerTest(t, repo)

	resumed, err := ResumeSubmission(repo, "submission-A", &AncoraPointer{SubmissionID: "submission-A", LedgerVersion: 14, Status: ArchiveStatus, ContractFingerprint: "contract-F"})
	if err != nil {
		t.Fatalf("ResumeSubmission() error = %v", err)
	}
	if resumed.Ledger.Status != ContractStatus || resumed.AncoraPointerState != AncoraStale {
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
		{name: "malformed ledger", setup: func(t *testing.T, repo string) { mustWrite(t, ledgerPath(repo, "submission-A"), "not json") }},
		{name: "missing contract", setup: func(t *testing.T, repo string) {
			initializeLedgerAt(t, repo)
			transitionLedgerToContract(t, repo)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := t.TempDir()
			testCase.setup(t, repo)
			_, err := ResumeSubmission(repo, "submission-A", &AncoraPointer{SubmissionID: "submission-A", LedgerVersion: 8, Status: ContractStatus})
			if err == nil || !strings.Contains(err.Error(), "cannot establish durable lifecycle state") || !strings.Contains(err.Error(), "safe next action") {
				t.Fatalf("ResumeSubmission() error = %v, want fail-closed diagnostic", err)
			}
		})
	}
}

func TestSCN707_ExactContractApprovalAtomicallyAuthorizesTDD(t *testing.T) {
	// REQ-105, REQ-104 -> SCN-707
	repo := initializeLedgerForTest(t)
	persistContractStateForLedgerTest(t, repo)
	ledger, err := ApproveContractForTDD(repo, ContractApprovalRequest{
		SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-F", ApprovedScenarioIDs: []string{"SCN-701"}, ContractCommit: workflowTestCommit,
		Worktree: WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: workflowTestCommit}, Authorizer: orchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"},
		VerifyCommit: func(commit string) error {
			if commit != workflowTestCommit {
				t.Fatalf("verified commit = %s", commit)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ApproveContractForTDD() error = %v", err)
	}
	if ledger.Status != TDDStatus || ledger.LedgerVersion != 3 || ledger.ContractFingerprint != "contract-F" || ledger.ContractCommit != workflowTestCommit || len(ledger.ApprovedScenarioIDs) != 1 {
		t.Fatalf("approved ledger = %#v, want atomically recorded TDD authorization", ledger)
	}
}

func TestSCN708_ChangedContractRequiresRenewedApproval(t *testing.T) {
	// REQ-105, REQ-102 -> SCN-708
	repo := initializeLedgerForTest(t)
	persistContractStateForLedgerTest(t, repo)
	_, err := ApproveContractForTDD(repo, ContractApprovalRequest{SubmissionID: "submission-A", LedgerVersion: 2, ApprovedFingerprint: "contract-old", ApprovedScenarioIDs: []string{"SCN-701"}, ContractCommit: workflowTestCommit, Worktree: WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: workflowTestCommit}, Authorizer: orchestratorAuthorizer, EvidenceRefs: []string{"evidence/approval.json"}, VerifyCommit: func(string) error { return nil }})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApproveContractForTDD() error = %v, want stale approval rejection", err)
	}
}

func TestSCN711_RecordedWorktreeCannotBeSubstituted(t *testing.T) {
	// REQ-106, REQ-107, REQ-115 -> SCN-711
	identity := WorktreeIdentity{Path: "/cache/submission-A", RepositoryID: "repo-A", Branch: "feature/submission-A", ExpectedCommit: workflowTestCommit}
	if err := ValidateRecordedWorktree(identity, WorktreeObservation{Path: "/other/worktree", RepositoryID: "repo-A", Branch: "feature/submission-A", HeadCommit: workflowTestCommit, Clean: true, Attached: true}); err == nil || !strings.Contains(err.Error(), "without substitution") {
		t.Fatalf("ValidateRecordedWorktree() error = %v, want substitution rejection", err)
	}
	if err := ValidateRecordedWorktree(identity, WorktreeObservation{Path: identity.Path, RepositoryID: identity.RepositoryID, Branch: identity.Branch, HeadCommit: identity.ExpectedCommit, Clean: true, Attached: true}); err != nil {
		t.Fatalf("ValidateRecordedWorktree() error = %v", err)
	}
}

func initializeLedgerForTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	initializeLedgerAt(t, repo)
	return repo
}

func initializeLedgerAt(t *testing.T, repo string) {
	t.Helper()
	if _, err := InitializeNewSubmission(repo, NewSubmissionRequest{SubmissionID: "submission-A", Draft: "draft", BaseCommit: workflowTestCommit}); err != nil {
		t.Fatalf("initialize test ledger: %v", err)
	}
}

func persistContractStateForLedgerTest(t *testing.T, repo string) {
	t.Helper()
	if err := RecordContractArtifact(repo, ContractArtifact{SubmissionID: "submission-A", Fingerprint: "contract-F", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/vela__workflow.feature"}}); err != nil {
		t.Fatalf("RecordContractArtifact() error = %v", err)
	}
	transitionLedgerToContract(t, repo)
}

func transitionLedgerToContract(t *testing.T, repo string) {
	t.Helper()
	if _, err := PersistTransition(repo, TransitionRequest{SubmissionID: "submission-A", ExpectedStatus: DraftStatus, TargetStatus: ContractStatus, LedgerVersion: 1, Authorizer: orchestratorAuthorizer, AuthorizedScope: []string{"SCN-701"}, EvidenceRefs: []string{"evidence/draft.json"}}); err != nil {
		t.Fatalf("persist Contract transition: %v", err)
	}
}
