package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN025_PreparesCleanIsolatedFeatureWorktree(t *testing.T) {
	// REQ-021 → SCN-025 → TestSCN025_PreparesCleanIsolatedFeatureWorktree
	// Scenario: Start autonomous Phase 3 in a new clean isolated feature worktree
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "specs", "autonomous_scenario_checkpoints.md"), "# Approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "autonomous_scenario_checkpoints.feature"), "Feature: Autonomous Phase 3 scenario checkpoints\n")
	runGit(t, repo, "add", "specs/autonomous_scenario_checkpoints.md", "features/autonomous_scenario_checkpoints.feature")
	runGit(t, repo, "commit", "-m", "test: add approved autonomous contract")

	worktreePath := filepath.Join(t.TempDir(), "phase3")
	preparation, err := PrepareAutonomousPhase3Worktree(repo, AutonomousPhase3WorktreeRequest{
		Scope: ContractScope{
			SpecPath:    "specs/autonomous_scenario_checkpoints.md",
			FeaturePath: "features/autonomous_scenario_checkpoints.feature",
			ScenarioID:  "SCN-025",
		},
		Branch:       "feat/autonomous-phase3",
		WorktreePath: worktreePath,
	})
	if err != nil {
		t.Fatalf("PrepareAutonomousPhase3Worktree returned error: %v", err)
	}

	if preparation.Branch != "feat/autonomous-phase3" {
		t.Fatalf("expected selected branch to be reported, got %#v", preparation)
	}
	if preparation.WorktreePath != worktreePath {
		t.Fatalf("expected selected worktree to be reported, got %#v", preparation)
	}
	for _, path := range []string{
		filepath.Join(worktreePath, "specs", "autonomous_scenario_checkpoints.md"),
		filepath.Join(worktreePath, "features", "autonomous_scenario_checkpoints.feature"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected approved contract artifact in isolated worktree at %s: %v", path, err)
		}
	}
	if status := gitOutput(t, worktreePath, "status", "--short"); status != "" {
		t.Fatalf("expected isolated worktree to have no non-ignored changes, got %q", status)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
