package workflow

import (
	"errors"
	"path/filepath"
	"strings"
)

type V2OperationRequest struct {
	SubmissionID, Worktree, RemoteRef string
	ExpectedLedgerVersion             uint64
	Destructive                       bool
}

// ValidateV2Operations splits multi-target destructive requests before worker dispatch.
func ValidateV2Operations(requests []V2OperationRequest, repoRoot string) error {
	for _, request := range requests {
		if request.SubmissionID == "" || request.ExpectedLedgerVersion == 0 || (request.Destructive && (request.Worktree == "" || request.RemoteRef == "")) {
			return errors.New("v2 operation is incomplete; split it into a bounded single-submission task")
		}
		if request.Worktree != "" && !isV2SafeRepoPath(repoRoot, request.Worktree) {
			return errors.New("v2 operation contains unsafe worktree evidence")
		}
		if strings.ContainsAny(request.RemoteRef, "\n\r;|&$") {
			return errors.New("v2 operation contains unsafe remote evidence")
		}
	}
	return nil
}
func isV2SafeRepoPath(repoRoot, candidate string) bool {
	relative, err := filepath.Rel(repoRoot, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
