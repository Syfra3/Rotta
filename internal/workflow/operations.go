package workflow

import (
	"errors"
	"path/filepath"
	"strings"
)

type OperationRequest struct {
	SubmissionID, Worktree, RemoteRef string
	ExpectedLedgerVersion             uint64
	Destructive                       bool
}

// ValidateOperations splits multi-target destructive requests before worker dispatch.
func ValidateOperations(requests []OperationRequest, repoRoot string) error {
	for _, request := range requests {
		if request.SubmissionID == "" || request.ExpectedLedgerVersion == 0 || (request.Destructive && (request.Worktree == "" || request.RemoteRef == "")) {
			return errors.New("operation is incomplete; split it into a bounded single-submission task")
		}
		if request.Worktree != "" && !isSafeRepoPath(repoRoot, request.Worktree) {
			return errors.New("operation contains unsafe worktree evidence")
		}
		if strings.ContainsAny(request.RemoteRef, "\n\r;|&$") {
			return errors.New("operation contains unsafe remote evidence")
		}
	}
	return nil
}
func isSafeRepoPath(repoRoot, candidate string) bool {
	relative, err := filepath.Rel(repoRoot, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
