package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-037, REQ-038 → SCN-241 → TestSCN241_PrepareNewImplementationSubmissionCreatesIsolatedFeatureWorktree
func TestSCN241_PrepareNewImplementationSubmissionCreatesIsolatedFeatureWorktree(t *testing.T) {
	// Scenario: Create an isolated feature worktree before Phase 2 writes a contract
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}

	wantWorktree := filepath.Join(parent, "repository-worktree-handoff")
	if submission.WorktreePath != wantWorktree || !filepath.IsAbs(submission.WorktreePath) {
		t.Fatalf("worktree path = %q, want absolute sibling %q", submission.WorktreePath, wantWorktree)
	}
	if submission.BaseBranch != "main" || submission.FeatureBranch != "feature/worktree-handoff" {
		t.Fatalf("reported branches = %q/%q, want main/feature/worktree-handoff", submission.BaseBranch, submission.FeatureBranch)
	}
	if got := runGitOutput(t, submission.WorktreePath, "branch", "--show-current"); got != "feature/worktree-handoff" {
		t.Fatalf("isolated worktree branch = %q, want feature/worktree-handoff", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "specs")); !os.IsNotExist(err) {
		t.Fatalf("initiating worktree received a Phase 2 artifact directory: %v", err)
	}
}

// REQ-037, REQ-044 → SCN-242 → TestSCN242_PrepareNewImplementationSubmissionRejectsDetachedHEAD
func TestSCN242_PrepareNewImplementationSubmissionRejectsDetachedHEAD(t *testing.T) {
	// Scenario: Reject an unsafe starting condition without falling back to the initiating worktree
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")
	runGit(t, repo, "checkout", "--detach")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("PrepareNewImplementationSubmission error = %v, want detached HEAD validation failure", err)
	}
	if submission != (NewImplementationSubmission{}) {
		t.Fatalf("submission = %#v, want no fallback submission", submission)
	}
	if _, err := os.Stat(filepath.Join(parent, "repository-worktree-handoff")); !os.IsNotExist(err) {
		t.Fatalf("isolated worktree was created after detached HEAD validation: %v", err)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("initiating worktree status = %q, want no submission artifacts or code", got)
	}
}

// REQ-038, REQ-044 → SCN-243 → TestSCN243_PrepareNewImplementationSubmissionRejectsInvalidOrExistingFeatureBranch
func TestSCN243_PrepareNewImplementationSubmissionRejectsInvalidOrExistingFeatureBranch(t *testing.T) {
	// Scenario: Reject an invalid or unavailable feature branch
	for _, testCase := range []struct {
		name           string
		slug           string
		existingBranch bool
		wantError      string
	}{
		{name: "uppercase and whitespace", slug: "Feature Name", wantError: "invalid submission slug"},
		{name: "path traversal", slug: "../escape", wantError: "invalid submission slug"},
		{name: "existing feature branch", slug: "release-fix", existingBranch: true, wantError: "feature branch already exists"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "repository")
			if err := os.Mkdir(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "init", "-b", "main")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
			runGit(t, repo, "add", "README.md")
			runGit(t, repo, "commit", "-m", "test: establish integration base")
			if testCase.existingBranch {
				runGit(t, repo, "branch", "feature/"+testCase.slug)
			}

			submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
				Slug:              testCase.slug,
				IntegrationBranch: "main",
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("PrepareNewImplementationSubmission error = %v, want %q", err, testCase.wantError)
			}
			if submission != (NewImplementationSubmission{}) {
				t.Fatalf("submission = %#v, want no created or reused feature branch", submission)
			}
			if got := runGitOutput(t, repo, "branch", "--show-current"); got != "main" {
				t.Fatalf("initiating branch = %q, want main", got)
			}
			if got := runGitOutput(t, repo, "status", "--short"); got != "" {
				t.Fatalf("initiating worktree status = %q, want no submission artifacts or code", got)
			}
		})
	}
}

// REQ-039, REQ-044 → SCN-244 → TestSCN244_PrepareNewImplementationSubmissionRejectsCollidingSiblingWorktreePath
func TestSCN244_PrepareNewImplementationSubmissionRejectsCollidingSiblingWorktreePath(t *testing.T) {
	// Scenario: Reject a colliding sibling worktree path
	for _, testCase := range []struct {
		name    string
		occupy  func(t *testing.T, path string)
		inspect func(t *testing.T, path string)
	}{
		{
			name: "file",
			occupy: func(t *testing.T, path string) {
				mustWrite(t, path, "preserve me\n")
			},
			inspect: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil || string(content) != "preserve me\n" {
					t.Fatalf("colliding file was changed: content=%q, err=%v", content, err)
				}
			},
		},
		{
			name: "directory",
			occupy: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(path, "preserve.txt"), "preserve me\n")
			},
			inspect: func(t *testing.T, path string) {
				content, err := os.ReadFile(filepath.Join(path, "preserve.txt"))
				if err != nil || string(content) != "preserve me\n" {
					t.Fatalf("colliding directory was changed: content=%q, err=%v", content, err)
				}
			},
		},
		{
			name: "symlink",
			occupy: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "preserved-target")
				mustWrite(t, target, "preserve me\n")
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			inspect: func(t *testing.T, path string) {
				info, err := os.Lstat(path)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("colliding symlink was removed or replaced: info=%v, err=%v", info, err)
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "repository")
			if err := os.Mkdir(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "init", "-b", "main")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
			runGit(t, repo, "add", "README.md")
			runGit(t, repo, "commit", "-m", "test: establish integration base")

			collidingPath := filepath.Join(parent, "repository-worktree-handoff")
			testCase.occupy(t, collidingPath)

			submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
				Slug:              "worktree-handoff",
				IntegrationBranch: "main",
			})
			if err == nil || !strings.Contains(err.Error(), "worktree path collision") {
				t.Fatalf("PrepareNewImplementationSubmission error = %v, want path collision", err)
			}
			if submission != (NewImplementationSubmission{}) {
				t.Fatalf("submission = %#v, want no submission", submission)
			}
			testCase.inspect(t, collidingPath)
			if got := runGitOutput(t, repo, "branch", "--list", "feature/worktree-handoff"); got != "" {
				t.Fatalf("feature branch = %q, want no worktree operation", got)
			}
			if got := runGitOutput(t, repo, "status", "--short"); got != "" {
				t.Fatalf("initiating worktree status = %q, want no Phase 2 or Phase 3 artifacts", got)
			}
		})
	}
}

// REQ-039 → SCN-245 → TestSCN245_PrepareNewImplementationSubmissionRejectsWorktreeOwnedByAnotherSubmission
func TestSCN245_PrepareNewImplementationSubmissionRejectsWorktreeOwnedByAnotherSubmission(t *testing.T) {
	// Scenario: Allow concurrent submissions only with independent worktree ownership
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")

	alpha, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "alpha",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("prepare alpha submission: %v", err)
	}
	mustWrite(t, filepath.Join(alpha.WorktreePath, "alpha-only.txt"), "alpha state\n")
	secondInitiatingWorktree := filepath.Join(parent, "repository-second")
	runGit(t, repo, "worktree", "add", "-b", "initiator", secondInitiatingWorktree, "main")
	betaPath := filepath.Join(parent, "repository-second-beta")
	runGit(t, repo, "worktree", "add", "-b", "other-submission", betaPath, "main")
	if err := os.RemoveAll(betaPath); err != nil {
		t.Fatalf("remove stale worktree directory: %v", err)
	}

	blockedBeta, err := PrepareNewImplementationSubmission(secondInitiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "beta",
		IntegrationBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "worktree ownership conflict") {
		t.Fatalf("PrepareNewImplementationSubmission error = %v, want worktree ownership conflict", err)
	}
	if blockedBeta != (NewImplementationSubmission{}) {
		t.Fatalf("beta submission = %#v, want no submission using another worktree's path", blockedBeta)
	}
	runGit(t, repo, "worktree", "prune")

	beta, err := PrepareNewImplementationSubmission(secondInitiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "beta",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("prepare beta submission: %v", err)
	}
	if alpha.FeatureBranch == beta.FeatureBranch || alpha.WorktreePath == beta.WorktreePath {
		t.Fatalf("submissions share branch/path: alpha=%#v beta=%#v", alpha, beta)
	}
	if beta.WorktreePath != betaPath || beta.FeatureBranch != "feature/beta" {
		t.Fatalf("beta submission = %#v, want feature/beta at %q", beta, betaPath)
	}
	if got := runGitOutput(t, alpha.WorktreePath, "branch", "--show-current"); got != "feature/alpha" {
		t.Fatalf("alpha worktree branch = %q, want feature/alpha", got)
	}
	if got := runGitOutput(t, beta.WorktreePath, "branch", "--show-current"); got != "feature/beta" {
		t.Fatalf("beta worktree branch = %q, want feature/beta", got)
	}
	if content, err := os.ReadFile(filepath.Join(alpha.WorktreePath, "alpha-only.txt")); err != nil || string(content) != "alpha state\n" {
		t.Fatalf("alpha-only state changed: content=%q, err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(beta.WorktreePath, "alpha-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("beta worktree received alpha state: %v", err)
	}
	if got := runGitOutput(t, beta.WorktreePath, "status", "--short"); got != "" {
		t.Fatalf("beta worktree status = %q, want independent clean state", got)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
