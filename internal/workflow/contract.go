package workflow

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type WorktreeIdentity struct {
	Path           string `json:"path,omitempty"`
	RepositoryID   string `json:"repository_id,omitempty"`
	Branch         string `json:"branch,omitempty"`
	ExpectedCommit string `json:"expected_commit,omitempty"`
}

type WorktreeObservation struct {
	Path         string
	RepositoryID string
	Branch       string
	HeadCommit   string
	Clean        bool
	Attached     bool
}

// ContractApprovalRequest contains the durable facts an ops task returns
// after the human approved exactly the contract fingerprint in question.
type ContractApprovalRequest struct {
	SubmissionID        string
	LedgerVersion       uint64
	ApprovedFingerprint string
	ApprovedScenarioIDs []string
	ContractCommit      string
	Worktree            WorktreeIdentity
	Authorizer          string
	EvidenceRefs        []string
	VerifyCommit        func(commit string) error
}

func ApproveContractForTDD(repoRoot string, request ContractApprovalRequest) (SubmissionLedger, error) {
	if err := validateSubmissionID(request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}
	if request.Authorizer != orchestratorAuthorizer {
		return SubmissionLedger{}, errors.New("contract approval is unauthorized: expected orchestrator authorizer")
	}
	if len(request.ApprovedScenarioIDs) == 0 || len(request.EvidenceRefs) == 0 || !isFullCommitID(request.ContractCommit) || request.VerifyCommit == nil {
		return SubmissionLedger{}, errors.New("contract approval is incomplete: scenario scope, evidence, full contract commit, and commit verification are required")
	}
	if err := validateWorktreeIdentity(request.Worktree, request.ContractCommit); err != nil {
		return SubmissionLedger{}, err
	}
	if err := request.VerifyCommit(request.ContractCommit); err != nil {
		return SubmissionLedger{}, fmt.Errorf("verify contract commit: %w", err)
	}

	unlock, err := lockLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadSubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	if ledger.Status != ContractStatus || ledger.LedgerVersion != request.LedgerVersion {
		return SubmissionLedger{}, fmt.Errorf("contract approval rejected: expected Contract at ledger version %d, observed %s at version %d", request.LedgerVersion, ledger.Status, ledger.LedgerVersion)
	}
	contract, err := loadContractArtifact(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, fmt.Errorf("contract approval rejected: %w", err)
	}
	if contract.Fingerprint != request.ApprovedFingerprint {
		return SubmissionLedger{}, errors.New("contract approval is stale: the human-approved fingerprint does not match the durable contract")
	}

	ledger.ContractFingerprint = contract.Fingerprint
	ledger.ContractCommit = strings.ToLower(request.ContractCommit)
	ledger.ApprovedScenarioIDs = append([]string(nil), request.ApprovedScenarioIDs...)
	worktree := request.Worktree
	ledger.Worktree = &worktree
	ledger.Transitions = append(ledger.Transitions, Transition{
		FromStatus: ContractStatus, ToStatus: TDDStatus, LedgerVersion: ledger.LedgerVersion,
		Authorizer: request.Authorizer, AuthorizedScope: append([]string(nil), request.ApprovedScenarioIDs...), EvidenceRefs: append([]string(nil), request.EvidenceRefs...),
	})
	ledger.Status = TDDStatus
	ledger.LedgerVersion++
	if err := writeLedgerAtomically(ledgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return SubmissionLedger{}, err
	}
	return ledger, nil
}

// ValidateRecordedWorktree rejects any convenient substitute. The caller
// supplies delegated ops observations; this function does not select a path.
func ValidateRecordedWorktree(identity WorktreeIdentity, observed WorktreeObservation) error {
	if err := validateWorktreeIdentity(identity, identity.ExpectedCommit); err != nil {
		return err
	}
	if filepath.Clean(observed.Path) != filepath.Clean(identity.Path) || observed.RepositoryID != identity.RepositoryID || observed.Branch != identity.Branch || !strings.EqualFold(observed.HeadCommit, identity.ExpectedCommit) || !observed.Clean || !observed.Attached {
		return errors.New("recorded worktree validation failed; stop the task without substitution or deletion")
	}
	return nil
}

func validateWorktreeIdentity(identity WorktreeIdentity, expectedCommit string) error {
	if !filepath.IsAbs(identity.Path) || strings.TrimSpace(identity.RepositoryID) == "" || strings.TrimSpace(identity.Branch) == "" || !isFullCommitID(identity.ExpectedCommit) || !strings.EqualFold(identity.ExpectedCommit, expectedCommit) {
		return errors.New("worktree identity is incomplete or does not match the expected contract commit")
	}
	return nil
}
