package workflow

import "testing"

func TestSCN725_726_BoundedOperationsRejectUnsafeEvidence(t *testing.T) {
	if err := ValidateOperations([]OperationRequest{{SubmissionID: "A", Worktree: "/tmp/outside", RemoteRef: "origin/main", ExpectedLedgerVersion: 1, Destructive: true}}, t.TempDir()); err == nil {
		t.Fatal("want unsafe path rejection")
	}
	if err := ValidateOperations([]OperationRequest{{SubmissionID: "A", Worktree: t.TempDir(), RemoteRef: "origin/main; rm -rf /", ExpectedLedgerVersion: 1, Destructive: true}}, "/"); err == nil {
		t.Fatal("want unsafe remote rejection")
	}
}
