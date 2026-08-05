package workflow

import (
	"errors"
	"testing"
)

func TestSCN718_ExactPublishedSnapshotArchivesOnlyAfterPostConfirmationCheck(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-718
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, err := VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: v2TestCommit, EvidenceRef: "evidence/published.json"})
	if err != nil || ledger.Publication.CleanupOutcome != "pending_confirmation" {
		t.Fatalf("VerifyV2Publication() = %#v, %v", ledger, err)
	}
	removed := false
	ledger, err = FinalizeV2Archive(repo, V2ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: v2TestCommit, EvidenceRef: "evidence/recheck.json", RemoveWorktree: func(path string) error { removed = path == "/cache/submission-A"; return nil }})
	if err != nil || !removed || ledger.Status != V2ArchivedStatus || ledger.Publication.CleanupOutcome != "removed" {
		t.Fatalf("FinalizeV2Archive() = %#v, %v", ledger, err)
	}
}

func TestSCN719_PublishMismatchRetainsArchiveAndWorktree(t *testing.T) {
	// REQ-114 -> SCN-719
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, err := VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", EvidenceRef: "evidence/mismatch.json"})
	if err != nil || ledger.Status != V2ArchiveStatus || ledger.Publication.CleanupOutcome != "publication_mismatch" {
		t.Fatalf("VerifyV2Publication() = %#v, %v", ledger, err)
	}
}

func TestSCN720_ChangedPublicationAfterConfirmationDoesNotRemoveWorktree(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-720
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, _ = VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: v2TestCommit, EvidenceRef: "evidence/published.json"})
	removed := false
	ledger, err := FinalizeV2Archive(repo, V2ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", EvidenceRef: "evidence/changed.json", RemoveWorktree: func(string) error { removed = true; return errors.New("must not run") }})
	if err != nil || removed || ledger.Status != V2ArchiveStatus || ledger.Publication.CleanupOutcome != "publication_changed" {
		t.Fatalf("FinalizeV2Archive() = %#v, %v", ledger, err)
	}
}

func TestSCN730_RemoteRefRaceRetainsWorktree(t *testing.T) {
	// REQ-114, REQ-115, REQ-117 -> SCN-730
	TestSCN720_ChangedPublicationAfterConfirmationDoesNotRemoveWorktree(t)
}

func TestSCN721_AncestorPublicationIsNotExact(t *testing.T) {
	// REQ-114 -> SCN-721
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, err := VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: "cccccccccccccccccccccccccccccccccccccccc", EvidenceRef: "evidence/tip.json"})
	if err != nil || ledger.Publication.CleanupOutcome != "publication_mismatch" {
		t.Fatalf("VerifyV2Publication() = %#v, %v", ledger, err)
	}
}

func TestSCN722_CleanupDeclineRetainsArchive(t *testing.T) {
	// REQ-114, REQ-117 -> SCN-722
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, _ = VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: v2TestCommit, EvidenceRef: "evidence/published.json"})
	ledger, err := FinalizeV2Archive(repo, V2ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, CleanupConfirmed: false, EvidenceRef: "evidence/declined.json"})
	if err != nil || ledger.Status != V2ArchiveStatus || ledger.Publication.CleanupOutcome != "cleanup_declined" {
		t.Fatalf("FinalizeV2Archive() = %#v, %v", ledger, err)
	}
}

func TestSCN724_RemovalFailureDoesNotArchive(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-724
	repo, ledger := initializeV2ArchiveForTest(t)
	ledger, _ = VerifyV2Publication(repo, V2PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: v2TestCommit, EvidenceRef: "evidence/published.json"})
	ledger, err := FinalizeV2Archive(repo, V2ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: v2OrchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: v2TestCommit, EvidenceRef: "evidence/remove.json", RemoveWorktree: func(string) error { return errors.New("busy") }})
	if err != nil || ledger.Status != V2ArchiveStatus || ledger.Publication.CleanupOutcome != "removal_failed" {
		t.Fatalf("FinalizeV2Archive() = %#v, %v", ledger, err)
	}
}

func initializeV2ArchiveForTest(t *testing.T) (string, V2SubmissionLedger) {
	t.Helper()
	repo, ledger := initializeV2ReviewForTest(t)
	ledger, err := ApplyV2QualityReview(repo, v2QualityRequest(ledger.LedgerVersion, V2QualityPass))
	if err != nil {
		t.Fatalf("ApplyV2QualityReview() error = %v", err)
	}
	return repo, ledger
}
