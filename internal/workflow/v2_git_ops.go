package workflow

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// VerifyV2GitCommit supplies the real Git verifier required by contract approval.
func VerifyV2GitCommit(repoRoot, commit string) error {
	if !isFullCommitID(commit) {
		return fmt.Errorf("invalid immutable commit %q", commit)
	}
	return runV2Git(repoRoot, "cat-file", "-e", commit+"^{commit}")
}

// ObserveV2GitWorktree supplies facts for ValidateV2RecordedWorktree.
func ObserveV2GitWorktree(worktree string) (V2WorktreeObservation, error) {
	head, err := v2GitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return V2WorktreeObservation{}, err
	}
	branch, err := v2GitOutput(worktree, "branch", "--show-current")
	if err != nil {
		return V2WorktreeObservation{}, err
	}
	status, err := v2GitOutput(worktree, "status", "--porcelain")
	if err != nil {
		return V2WorktreeObservation{}, err
	}
	repositoryID, err := v2GitOutput(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return V2WorktreeObservation{}, err
	}
	return V2WorktreeObservation{Path: worktree, RepositoryID: repositoryID, Branch: branch, HeadCommit: head, Clean: status == "", Attached: branch != ""}, nil
}

// RemoveV2GitWorktree removes only the exact recorded path after Archive has
// already validated remote publication and explicit cleanup consent.
func RemoveV2GitWorktree(repoRoot string, identity V2WorktreeIdentity) error {
	if err := validateV2WorktreeIdentity(identity, identity.ExpectedCommit); err != nil {
		return err
	}
	return runV2Git(repoRoot, "worktree", "remove", "--force", identity.Path)
}

func runV2Git(directory string, arguments ...string) error {
	_, err := v2GitOutput(directory, arguments...)
	return err
}
func v2GitOutput(directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
