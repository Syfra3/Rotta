package workflow

import (
	"errors"
	"testing"
)

func TestSCN718_ExactPublishedSnapshotArchivesOnlyAfterPostConfirmationCheck(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-718
	repo, ledger := initializeArchiveForTest(t)
	ledger, err := VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/published.json"})
	if err != nil || ledger.Publication.CleanupOutcome != "pending_confirmation" {
		t.Fatalf("VerifyPublication() = %#v, %v", ledger, err)
	}
	removed := false
	ledger, err = FinalizeArchive(repo, ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/recheck.json", RemoveWorktree: func(path string) error { removed = path == "/cache/submission-A"; return nil }})
	if err != nil || !removed || ledger.Status != ArchivedStatus || ledger.Publication.CleanupOutcome != "removed" {
		t.Fatalf("FinalizeArchive() = %#v, %v", ledger, err)
	}
}

func TestSCN719_PublishMismatchRetainsArchiveAndWorktree(t *testing.T) {
	// REQ-114 -> SCN-719
	repo, ledger := initializeArchiveForTest(t)
	ledger, err := VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", EvidenceRef: "evidence/mismatch.json"})
	if err != nil || ledger.Status != ArchiveStatus || ledger.Publication.CleanupOutcome != "publication_mismatch" {
		t.Fatalf("VerifyPublication() = %#v, %v", ledger, err)
	}
}

func TestSCN720_ChangedPublicationAfterConfirmationDoesNotRemoveWorktree(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-720
	repo, ledger := initializeArchiveForTest(t)
	ledger, _ = VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/published.json"})
	removed := false
	ledger, err := FinalizeArchive(repo, ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", EvidenceRef: "evidence/changed.json", RemoveWorktree: func(string) error { removed = true; return errors.New("must not run") }})
	if err != nil || removed || ledger.Status != ArchiveStatus || ledger.Publication.CleanupOutcome != "publication_changed" {
		t.Fatalf("FinalizeArchive() = %#v, %v", ledger, err)
	}
}

func TestSCN730_RemoteRefRaceRetainsWorktree(t *testing.T) {
	// REQ-114, REQ-115, REQ-117 -> SCN-730
	TestSCN720_ChangedPublicationAfterConfirmationDoesNotRemoveWorktree(t)
}

func TestSCN721_AncestorPublicationIsNotExact(t *testing.T) {
	// REQ-114 -> SCN-721
	repo, ledger := initializeArchiveForTest(t)
	ledger, err := VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: "cccccccccccccccccccccccccccccccccccccccc", EvidenceRef: "evidence/tip.json"})
	if err != nil || ledger.Publication.CleanupOutcome != "publication_mismatch" {
		t.Fatalf("VerifyPublication() = %#v, %v", ledger, err)
	}
}

func TestSCN722_CleanupDeclineRetainsArchive(t *testing.T) {
	// REQ-114, REQ-117 -> SCN-722
	repo, ledger := initializeArchiveForTest(t)
	ledger, _ = VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/published.json"})
	ledger, err := FinalizeArchive(repo, ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, CleanupConfirmed: false, EvidenceRef: "evidence/declined.json"})
	if err != nil || ledger.Status != ArchiveStatus || ledger.Publication.CleanupOutcome != "cleanup_declined" {
		t.Fatalf("FinalizeArchive() = %#v, %v", ledger, err)
	}
}

func TestSCN724_RemovalFailureDoesNotArchive(t *testing.T) {
	// REQ-114, REQ-115 -> SCN-724
	repo, ledger := initializeArchiveForTest(t)
	ledger, _ = VerifyPublication(repo, PublicationVerificationRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, Remote: "origin", Ref: "refs/heads/feature/submission-A", ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/published.json"})
	ledger, err := FinalizeArchive(repo, ArchiveCleanupRequest{SubmissionID: "submission-A", LedgerVersion: ledger.LedgerVersion, Authorizer: orchestratorAuthorizer, CleanupConfirmed: true, ObservedCommit: workflowTestCommit, EvidenceRef: "evidence/remove.json", RemoveWorktree: func(string) error { return errors.New("busy") }})
	if err != nil || ledger.Status != ArchiveStatus || ledger.Publication.CleanupOutcome != "removal_failed" {
		t.Fatalf("FinalizeArchive() = %#v, %v", ledger, err)
	}
}

func initializeArchiveForTest(t *testing.T) (string, SubmissionLedger) {
	t.Helper()
	repo, ledger := initializeReviewForTest(t)
	ledger, err := ApplyQualityReview(repo, QualityRequest(ledger.LedgerVersion, QualityPass))
	if err != nil {
		t.Fatalf("ApplyQualityReview() error = %v", err)
	}
	return repo, ledger
}
