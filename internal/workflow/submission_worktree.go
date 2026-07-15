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

// BeginSpecificationPhase prepares the recorded isolated feature worktree
// before handing that worktree to the specification writer.
func BeginSpecificationPhase(initiatingWorktree string, request NewImplementationSubmissionRequest, writeContract func(recordedWorktree string) error) (NewImplementationSubmission, error) {
	submission, err := PrepareNewImplementationSubmission(initiatingWorktree, request)
	if err != nil {
		return NewImplementationSubmission{}, err
	}
	if err := writeContract(submission.WorktreePath); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("write specification contract in recorded feature worktree: %w", err)
	}
	return submission, nil
}

// ValidatePhase3SubagentBoundary verifies that a returned Phase 3 subagent
// remains in its recorded isolated feature worktree before another subagent
// may start.
func ValidatePhase3SubagentBoundary(submission NewImplementationSubmission, returnedWorktree string, launchNextSubagent func() error) error {
	worktreePath, err := filepath.EvalSymlinks(submission.WorktreePath)
	if err != nil {
		return fmt.Errorf("feature worktree identity failure: resolve recorded worktree: %w", err)
	}
	returnedWorktree, err = filepath.EvalSymlinks(returnedWorktree)
	if err != nil || returnedWorktree != worktreePath {
		return fmt.Errorf("feature worktree identity failure: returned worktree does not match the recorded worktree")
	}
	repoRoot, err := gitSubmissionOutput(returnedWorktree, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("feature worktree identity failure: resolve recorded worktree: %w", err)
	}
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil || repoRoot != worktreePath {
		return fmt.Errorf("feature worktree identity failure: recorded worktree does not match its repository root")
	}
	branch, err := gitSubmissionOutput(returnedWorktree, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != submission.FeatureBranch {
		return fmt.Errorf("feature branch identity failure: got %q, want %q", branch, submission.FeatureBranch)
	}
	if launchNextSubagent != nil {
		return launchNextSubagent()
	}
	return nil
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
	if worktrees, err := gitSubmissionOutput(repoRoot, "worktree", "list", "--porcelain"); err != nil {
		return NewImplementationSubmission{}, fmt.Errorf("inspect worktree ownership: %w", err)
	} else if strings.Contains("\n"+worktrees+"\n", "\nworktree "+worktreePath+"\n") {
		return NewImplementationSubmission{}, fmt.Errorf("worktree ownership conflict: %s", worktreePath)
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
