package workflow

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type V2WorktreeIdentity struct {
	Path           string `json:"path,omitempty"`
	RepositoryID   string `json:"repository_id,omitempty"`
	Branch         string `json:"branch,omitempty"`
	ExpectedCommit string `json:"expected_commit,omitempty"`
}

type V2WorktreeObservation struct {
	Path         string
	RepositoryID string
	Branch       string
	HeadCommit   string
	Clean        bool
	Attached     bool
}

// V2ContractApprovalRequest contains the durable facts an ops task returns
// after the human approved exactly the contract fingerprint in question.
type V2ContractApprovalRequest struct {
	SubmissionID        string
	LedgerVersion       uint64
	ApprovedFingerprint string
	ApprovedScenarioIDs []string
	ContractCommit      string
	Worktree            V2WorktreeIdentity
	Authorizer          string
	EvidenceRefs        []string
	VerifyCommit        func(commit string) error
}

func ApproveV2ContractForTDD(repoRoot string, request V2ContractApprovalRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer {
		return V2SubmissionLedger{}, errors.New("v2 contract approval is unauthorized: expected orchestrator authorizer")
	}
	if len(request.ApprovedScenarioIDs) == 0 || len(request.EvidenceRefs) == 0 || !isFullCommitID(request.ContractCommit) || request.VerifyCommit == nil {
		return V2SubmissionLedger{}, errors.New("v2 contract approval is incomplete: scenario scope, evidence, full contract commit, and commit verification are required")
	}
	if err := validateV2WorktreeIdentity(request.Worktree, request.ContractCommit); err != nil {
		return V2SubmissionLedger{}, err
	}
	if err := request.VerifyCommit(request.ContractCommit); err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("verify contract commit: %w", err)
	}

	unlock, err := lockV2Ledger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadV2SubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	if ledger.Status != V2ContractStatus || ledger.LedgerVersion != request.LedgerVersion {
		return V2SubmissionLedger{}, fmt.Errorf("v2 contract approval rejected: expected Contract at ledger version %d, observed %s at version %d", request.LedgerVersion, ledger.Status, ledger.LedgerVersion)
	}
	contract, err := loadV2ContractArtifact(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("v2 contract approval rejected: %w", err)
	}
	if contract.Fingerprint != request.ApprovedFingerprint {
		return V2SubmissionLedger{}, errors.New("v2 contract approval is stale: the human-approved fingerprint does not match the durable contract")
	}

	ledger.ContractFingerprint = contract.Fingerprint
	ledger.ContractCommit = strings.ToLower(request.ContractCommit)
	ledger.ApprovedScenarioIDs = append([]string(nil), request.ApprovedScenarioIDs...)
	worktree := request.Worktree
	ledger.Worktree = &worktree
	ledger.Transitions = append(ledger.Transitions, V2Transition{
		FromStatus: V2ContractStatus, ToStatus: V2TDDStatus, LedgerVersion: ledger.LedgerVersion,
		Authorizer: request.Authorizer, AuthorizedScope: append([]string(nil), request.ApprovedScenarioIDs...), EvidenceRefs: append([]string(nil), request.EvidenceRefs...),
	})
	ledger.Status = V2TDDStatus
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

// ValidateV2RecordedWorktree rejects any convenient substitute. The caller
// supplies delegated ops observations; this function does not select a path.
func ValidateV2RecordedWorktree(identity V2WorktreeIdentity, observed V2WorktreeObservation) error {
	if err := validateV2WorktreeIdentity(identity, identity.ExpectedCommit); err != nil {
		return err
	}
	if filepath.Clean(observed.Path) != filepath.Clean(identity.Path) || observed.RepositoryID != identity.RepositoryID || observed.Branch != identity.Branch || !strings.EqualFold(observed.HeadCommit, identity.ExpectedCommit) || !observed.Clean || !observed.Attached {
		return errors.New("recorded v2 worktree validation failed; stop the task without substitution or deletion")
	}
	return nil
}

func validateV2WorktreeIdentity(identity V2WorktreeIdentity, expectedCommit string) error {
	if !filepath.IsAbs(identity.Path) || strings.TrimSpace(identity.RepositoryID) == "" || strings.TrimSpace(identity.Branch) == "" || !isFullCommitID(identity.ExpectedCommit) || !strings.EqualFold(identity.ExpectedCommit, expectedCommit) {
		return errors.New("v2 worktree identity is incomplete or does not match the expected contract commit")
	}
	return nil
}
