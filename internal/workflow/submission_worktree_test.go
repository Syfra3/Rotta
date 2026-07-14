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

// REQ-037, REQ-038 → SCN-241 → TestSCN241_PrepareNewImplementationSubmissionResolvesRepositoryDefaultIntegrationBranch
func TestSCN241_PrepareNewImplementationSubmissionResolvesRepositoryDefaultIntegrationBranch(t *testing.T) {
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
	runGit(t, repo, "commit", "-m", "test: establish repository default")
	runGit(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{Slug: "repository-default"})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}

	if submission.BaseBranch != "origin/main" {
		t.Fatalf("base branch = %q, want repository default %q", submission.BaseBranch, "origin/main")
	}
	if submission.FeatureBranch != "feature/repository-default" {
		t.Fatalf("feature branch = %q, want %q", submission.FeatureBranch, "feature/repository-default")
	}
	wantCommit := runGitOutput(t, repo, "rev-parse", "origin/main")
	if got := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD"); got != wantCommit {
		t.Fatalf("feature worktree commit = %q, want repository-default integration commit %q", got, wantCommit)
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_PresentsManualGitHubPRHandoff
func TestSCN248_PresentsManualGitHubPRHandoff(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	repo := prepareSCN248Repository(t)
	submission := NewImplementationSubmission{
		WorktreePath:  repo,
		BaseBranch:    "main",
		FeatureBranch: "feature/worktree-handoff",
	}

	handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission:     submission,
		ReviewedPaths:  []string{"internal/workflow/submission_worktree.go"},
		HostDisclaimer: "This host cannot delegate GitHub publication; use your own credentials.",
	})
	if err != nil {
		t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
	}

	for _, want := range []string{
		"cd \"" + repo + "\"",
		"git status --short",
		"git add -- \"internal/workflow/submission_worktree.go\"",
		"git commit",
		"git push origin feature/worktree-handoff",
		"gh pr create --base main --head feature/worktree-handoff",
		"https://github.com/",
		"This host cannot delegate GitHub publication; use your own credentials.",
	} {
		if !strings.Contains(handoff, want) {
			t.Fatalf("handoff missing %q:\n%s", want, handoff)
		}
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual handoff changed the worktree: %q", got)
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_PresentsManualHandoffForSupportedGitHubURLForms
func TestSCN248_PresentsManualHandoffForSupportedGitHubURLForms(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	for _, test := range []struct {
		name       string
		remoteURL  string
		wantWebURL string
	}{
		{
			name:       "SSH URL",
			remoteURL:  "ssh://git@github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
		{
			name:       "HTTPS URL",
			remoteURL:  "https://github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
		{
			name:       "HTTP URL",
			remoteURL:  "http://github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := prepareSCN248Repository(t)
			runGit(t, repo, "remote", "set-url", "origin", test.remoteURL)

			handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
				Submission: NewImplementationSubmission{
					WorktreePath:  repo,
					BaseBranch:    "main",
					FeatureBranch: "feature/worktree-handoff",
				},
			})
			if err != nil {
				t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
			}
			if !strings.Contains(handoff, test.wantWebURL) {
				t.Fatalf("handoff missing GitHub web UI URL %q:\n%s", test.wantWebURL, handoff)
			}
		})
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_RejectsUnsafeManualHandoffCommands
func TestSCN248_RejectsUnsafeManualHandoffCommands(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	repo := prepareSCN248Repository(t)
	_, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "main; unsafe-command",
			FeatureBranch: "feature/worktree-handoff",
		},
		HostDisclaimer: "This host cannot delegate GitHub publication; use your own credentials.",
	})
	if err == nil {
		t.Fatal("expected unsafe base branch to be rejected before printing a command")
	}
}

// REQ-042, REQ-044 → SCN-249 → TestSCN249_ReportsManualCommandFailureWithoutMutatingSubmission
func TestSCN249_ReportsManualCommandFailureWithoutMutatingSubmission(t *testing.T) {
	// Scenario: Preserve the feature worktree when manual PR creation fails
	repo := prepareSCN248Repository(t)
	submission := NewImplementationSubmission{
		WorktreePath:  repo,
		BaseBranch:    "main",
		FeatureBranch: "feature/worktree-handoff",
	}

	guidance, err := ReportManualGitHubPRFailure(submission, "gh pr create: authentication required")
	if err != nil {
		t.Fatalf("ReportManualGitHubPRFailure returned error: %v", err)
	}
	for _, want := range []string{
		"manual command failed: gh pr create: authentication required",
		"cd \"" + repo + "\"",
		"git status --short",
		"git branch --show-current",
		"feature/worktree-handoff",
		"preserved",
	} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("failure guidance missing %q:\n%s", want, guidance)
		}
	}
	if got := runGitOutput(t, repo, "branch", "--show-current"); got != submission.FeatureBranch {
		t.Fatalf("manual failure guidance changed branch to %q, want %q", got, submission.FeatureBranch)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual failure guidance changed the worktree: %q", got)
	}
	if !strings.Contains(guidance, "Do not retry automatically, switch publication mechanisms, merge, or modify main.") {
		t.Fatalf("failure guidance omitted the safe manual-only boundary:\n%s", guidance)
	}
}

// REQ-042, REQ-044 → SCN-249 → TestSCN249_RejectsIncompleteFailureGuidanceSubmission
func TestSCN249_RejectsIncompleteFailureGuidanceSubmission(t *testing.T) {
	// Scenario: Preserve the feature worktree when manual PR creation fails
	repo := prepareSCN248Repository(t)
	for _, test := range []struct {
		name       string
		submission NewImplementationSubmission
	}{
		{
			name: "relative worktree",
			submission: NewImplementationSubmission{
				WorktreePath:  "relative-worktree",
				BaseBranch:    "main",
				FeatureBranch: "feature/worktree-handoff",
			},
		},
		{
			name: "unsafe base branch",
			submission: NewImplementationSubmission{
				WorktreePath:  repo,
				BaseBranch:    "main; unsafe-command",
				FeatureBranch: "feature/worktree-handoff",
			},
		},
		{
			name: "unsafe feature branch",
			submission: NewImplementationSubmission{
				WorktreePath:  repo,
				BaseBranch:    "main",
				FeatureBranch: "feature/worktree handoff",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			guidance, err := ReportManualGitHubPRFailure(test.submission, "push failed")
			if err == nil || guidance != "" {
				t.Fatalf("ReportManualGitHubPRFailure = %q, %v; want empty guidance and validation error", guidance, err)
			}
			if !strings.Contains(err.Error(), "requires the recorded feature worktree and branches") {
				t.Fatalf("validation error = %q, want recorded submission guidance", err)
			}
		})
	}
	if got := runGitOutput(t, repo, "branch", "--show-current"); got != "feature/worktree-handoff" {
		t.Fatalf("validation failure changed branch to %q", got)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("validation failure changed the worktree: %q", got)
	}
}

// REQ-042, REQ-043 → SCN-250 → TestSCN250_ReportsRemoteResolutionRequiredWithoutPublicationCommands
func TestSCN250_ReportsRemoteResolutionRequiredWithoutPublicationCommands(t *testing.T) {
	// Scenario: Block guessed PR publication when no GitHub remote is unambiguous
	repo := prepareSCN248Repository(t)
	runGit(t, repo, "remote", "add", "upstream", "git@github.com:example/upstream.git")

	handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "main",
			FeatureBranch: "feature/worktree-handoff",
		},
	})
	if err != nil {
		t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
	}
	if !strings.Contains(handoff, "remote selection requires user resolution") {
		t.Fatalf("handoff did not require remote resolution:\n%s", handoff)
	}
	for _, forbidden := range []string{"git push", "gh pr create", "github.com"} {
		if strings.Contains(handoff, forbidden) {
			t.Fatalf("handoff guessed a publication action %q:\n%s", forbidden, handoff)
		}
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual handoff changed the worktree: %q", got)
	}
}

func prepareSCN248Repository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish handoff baseline")
	runGit(t, repo, "checkout", "-b", "feature/worktree-handoff")
	runGit(t, repo, "remote", "add", "origin", "git@github.com:example/repository.git")
	return repo
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

// REQ-040, REQ-041 → SCN-246 → TestSCN246_HaltsWhenPhase3SubagentBoundaryLosesFeatureWorktreeIdentity
func TestSCN246_HaltsWhenPhase3SubagentBoundaryLosesFeatureWorktreeIdentity(t *testing.T) {
	// Scenario: Halt when a Phase 3 subagent boundary loses feature-worktree identity
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
	runGit(t, repo, "checkout", "-b", "initiator")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}
	runGit(t, submission.WorktreePath, "checkout", "main")
	before := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD")
	nextSubagentLaunched := false

	err = ValidatePhase3SubagentBoundary(submission, submission.WorktreePath, func() error {
		nextSubagentLaunched = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "feature branch identity") {
		t.Fatalf("ValidatePhase3SubagentBoundary error = %v, want feature branch identity failure", err)
	}
	if nextSubagentLaunched {
		t.Fatal("next subagent launched after feature worktree identity failure")
	}
	if after := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD"); after != before {
		t.Fatalf("boundary validation changed HEAD from %q to %q", before, after)
	}
	if branch := runGitOutput(t, submission.WorktreePath, "branch", "--show-current"); branch != "main" {
		t.Fatalf("boundary validation changed branch to %q, want main", branch)
	}
}

// REQ-040, REQ-041 → SCN-246 → TestSCN246_HaltsForOtherDetachedOrWrongPhase3Worktree
func TestSCN246_HaltsForOtherDetachedOrWrongPhase3Worktree(t *testing.T) {
	// Scenario: Halt when a Phase 3 subagent boundary loses feature-worktree identity
	for _, testCase := range []struct {
		name     string
		mutate   func(t *testing.T, repo string, submission NewImplementationSubmission)
		returned func(repo string, submission NewImplementationSubmission) string
	}{
		{
			name: "other branch",
			mutate: func(t *testing.T, _ string, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", "-b", "other")
			},
			returned: func(_ string, submission NewImplementationSubmission) string { return submission.WorktreePath },
		},
		{
			name: "detached HEAD",
			mutate: func(t *testing.T, _ string, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", "--detach")
			},
			returned: func(_ string, submission NewImplementationSubmission) string { return submission.WorktreePath },
		},
		{
			name:     "wrong worktree",
			mutate:   func(t *testing.T, _ string, _ NewImplementationSubmission) {},
			returned: func(repo string, _ NewImplementationSubmission) string { return repo },
		},
		{
			name:   "missing worktree",
			mutate: func(t *testing.T, _ string, _ NewImplementationSubmission) {},
			returned: func(repo string, _ NewImplementationSubmission) string {
				return filepath.Join(filepath.Dir(repo), "missing-worktree")
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo, submission := prepareSCN246Submission(t)
			testCase.mutate(t, repo, submission)
			nextSubagentLaunched := false

			err := ValidatePhase3SubagentBoundary(submission, testCase.returned(repo, submission), func() error {
				nextSubagentLaunched = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "identity failure") {
				t.Fatalf("ValidatePhase3SubagentBoundary error = %v, want identity failure", err)
			}
			if nextSubagentLaunched {
				t.Fatal("next subagent launched after feature worktree identity failure")
			}
		})
	}
}

func prepareSCN246Submission(t *testing.T) (string, NewImplementationSubmission) {
	t.Helper()
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
	runGit(t, repo, "checkout", "-b", "initiator")
	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}
	return repo, submission
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
