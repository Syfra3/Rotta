package workflow

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// VerifyGitCommit supplies the real Git verifier required by contract approval.
func VerifyGitCommit(repoRoot, commit string) error {
	if !isFullCommitID(commit) {
		return fmt.Errorf("invalid immutable commit %q", commit)
	}
	return executeWorkflowGit(repoRoot, "cat-file", "-e", commit+"^{commit}")
}

// ObserveGitWorktree supplies facts for ValidateRecordedWorktree.
func ObserveGitWorktree(worktree string) (WorktreeObservation, error) {
	head, err := GitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return WorktreeObservation{}, err
	}
	branch, err := GitOutput(worktree, "branch", "--show-current")
	if err != nil {
		return WorktreeObservation{}, err
	}
	status, err := GitOutput(worktree, "status", "--porcelain")
	if err != nil {
		return WorktreeObservation{}, err
	}
	repositoryID, err := GitOutput(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return WorktreeObservation{}, err
	}
	return WorktreeObservation{Path: worktree, RepositoryID: repositoryID, Branch: branch, HeadCommit: head, Clean: status == "", Attached: branch != ""}, nil
}

// RemoveGitWorktree removes only the exact recorded path after Archive has
// already validated remote publication and explicit cleanup consent.
func RemoveGitWorktree(repoRoot string, identity WorktreeIdentity) error {
	if err := validateWorktreeIdentity(identity, identity.ExpectedCommit); err != nil {
		return err
	}
	return executeWorkflowGit(repoRoot, "worktree", "remove", "--force", identity.Path)
}

func executeWorkflowGit(directory string, arguments ...string) error {
	_, err := GitOutput(directory, arguments...)
	return err
}
func GitOutput(directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
