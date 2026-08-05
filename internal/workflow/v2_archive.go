package workflow

import (
	"errors"
	"fmt"
	"strings"
)

type V2Publication struct {
	Remote             string `json:"remote"`
	Ref                string `json:"ref"`
	ReviewedCommit     string `json:"reviewed_commit"`
	InitialObserved    string `json:"initial_observed_commit"`
	InitialEvidenceRef string `json:"initial_evidence_ref"`
	CleanupOutcome     string `json:"cleanup_outcome,omitempty"`
	PostObserved       string `json:"post_observed_commit,omitempty"`
	PostEvidenceRef    string `json:"post_evidence_ref,omitempty"`
}

type V2PublicationVerificationRequest struct {
	SubmissionID   string
	LedgerVersion  uint64
	Authorizer     string
	Remote         string
	Ref            string
	ObservedCommit string
	EvidenceRef    string
}

type V2ArchiveCleanupRequest struct {
	SubmissionID     string
	LedgerVersion    uint64
	Authorizer       string
	CleanupConfirmed bool
	ObservedCommit   string
	EvidenceRef      string
	RemoveWorktree   func(path string) error
}

// VerifyV2Publication records the exact remote/ref result before any cleanup
// confirmation is requested. An ancestor or push result is never sufficient.
func VerifyV2Publication(repoRoot string, request V2PublicationVerificationRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer || strings.TrimSpace(request.Remote) == "" || strings.TrimSpace(request.Ref) == "" || !isFullCommitID(request.ObservedCommit) || strings.TrimSpace(request.EvidenceRef) == "" {
		return V2SubmissionLedger{}, errors.New("v2 publication verification is incomplete or unauthorized")
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
	if ledger.Status != V2ArchiveStatus || ledger.LedgerVersion != request.LedgerVersion || !isFullCommitID(ledger.ReviewedCommit) {
		return V2SubmissionLedger{}, fmt.Errorf("v2 publication verification rejected: expected Archive at ledger version %d with reviewed commit", request.LedgerVersion)
	}
	ledger.Publication = &V2Publication{Remote: request.Remote, Ref: request.Ref, ReviewedCommit: ledger.ReviewedCommit, InitialObserved: strings.ToLower(request.ObservedCommit), InitialEvidenceRef: request.EvidenceRef}
	if !strings.EqualFold(request.ObservedCommit, ledger.ReviewedCommit) {
		ledger.Publication.CleanupOutcome = "publication_mismatch"
	} else {
		ledger.Publication.CleanupOutcome = "pending_confirmation"
	}
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

// FinalizeV2Archive re-verifies the same remote/ref after explicit human
// confirmation. Removal is attempted only after that exact match.
func FinalizeV2Archive(repoRoot string, request V2ArchiveCleanupRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer || strings.TrimSpace(request.EvidenceRef) == "" {
		return V2SubmissionLedger{}, errors.New("v2 archive cleanup is incomplete or unauthorized")
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
	if ledger.Status != V2ArchiveStatus || ledger.LedgerVersion != request.LedgerVersion || ledger.Publication == nil || ledger.Publication.CleanupOutcome != "pending_confirmation" {
		return V2SubmissionLedger{}, fmt.Errorf("v2 archive cleanup rejected: exact publication verification and a pending cleanup confirmation are required")
	}
	if !request.CleanupConfirmed {
		ledger.Publication.CleanupOutcome = "cleanup_declined"
		ledger.Publication.PostEvidenceRef = request.EvidenceRef
	} else if !isFullCommitID(request.ObservedCommit) || !strings.EqualFold(request.ObservedCommit, ledger.ReviewedCommit) {
		ledger.Publication.CleanupOutcome = "publication_changed"
		ledger.Publication.PostObserved = strings.ToLower(request.ObservedCommit)
		ledger.Publication.PostEvidenceRef = request.EvidenceRef
	} else if request.RemoveWorktree == nil {
		return V2SubmissionLedger{}, errors.New("v2 archive cleanup requires a recorded-worktree removal operation")
	} else if err := request.RemoveWorktree(ledger.Worktree.Path); err != nil {
		ledger.Publication.CleanupOutcome = "removal_failed"
		ledger.Publication.PostObserved = strings.ToLower(request.ObservedCommit)
		ledger.Publication.PostEvidenceRef = request.EvidenceRef
	} else {
		ledger.Publication.CleanupOutcome = "removed"
		ledger.Publication.PostObserved = strings.ToLower(request.ObservedCommit)
		ledger.Publication.PostEvidenceRef = request.EvidenceRef
		ledger.Status = V2ArchivedStatus
	}
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}
