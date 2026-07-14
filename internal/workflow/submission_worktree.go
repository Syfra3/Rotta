package workflow

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type NewImplementationSubmissionRequest struct {
	Slug              string
	IntegrationBranch string
}

type NewImplementationSubmission struct {
	WorktreePath  string
	BaseBranch    string
	FeatureBranch string
}

// PrepareNewImplementationSubmission creates the isolated worktree from which
// Phase 2 may write a new submission's durable artifacts.
func PrepareNewImplementationSubmission(initiatingWorktree string, request NewImplementationSubmissionRequest) (NewImplementationSubmission, error) {
	repoRoot, err := gitSubmissionOutput(initiatingWorktree, "rev-parse", "--show-toplevel")
	if err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("resolve initiating Git worktree: %w", err)
	}
	if status, err := gitSubmissionOutput(repoRoot, "status", "--short"); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("check initiating worktree cleanliness: %w", err)
	} else if status != "" {
		return NewImplementationSubmission{}, fmt.Errorf("initiating worktree has non-ignored changes")
	}
	if _, err := gitSubmissionOutput(repoRoot, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("validate initiating worktree HEAD: detached HEAD: %w", err)
	}

	baseBranch := request.IntegrationBranch
	if baseBranch == "" {
		baseBranch, err = gitSubmissionOutput(repoRoot, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
		if err != nil {
			return NewImplementationSubmission{}, fmt.Errorf("resolve repository-default integration branch: %w", err)
		}
	}
	if _, err := gitSubmissionOutput(repoRoot, "rev-parse", "--verify", baseBranch+"^{commit}"); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("resolve integration branch %q: %w", baseBranch, err)
	}

	featureBranch := "feature/" + request.Slug
	worktreePath := filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-"+request.Slug)
	if _, err := gitSubmissionOutput(repoRoot, "worktree", "add", "-b", featureBranch, worktreePath, baseBranch); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("create isolated feature worktree: %w", err)
	}
	return NewImplementationSubmission{
		WorktreePath:  worktreePath,
		BaseBranch:    baseBranch,
		FeatureBranch: featureBranch,
	}, nil
}

func gitSubmissionOutput(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
