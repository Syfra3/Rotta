package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var submissionSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

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
	if !submissionSlugPattern.MatchString(request.Slug) {
		return NewImplementationSubmission{}, fmt.Errorf("invalid submission slug %q", request.Slug)
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
	if branches, err := gitSubmissionOutput(repoRoot, "branch", "--list", "--format=%(refname:short)", featureBranch); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("check feature branch availability %q: %w", featureBranch, err)
	} else if branches != "" {
		return NewImplementationSubmission{}, fmt.Errorf("feature branch already exists: %s", featureBranch)
	}
	worktreePath := filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-"+request.Slug)
	if _, err := os.Lstat(worktreePath); err == nil {
		return NewImplementationSubmission{}, fmt.Errorf("worktree path collision: %s", worktreePath)
	} else if !os.IsNotExist(err) {
		return NewImplementationSubmission{}, fmt.Errorf("inspect prescribed worktree path: %w", err)
	}
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
