package workflow

import (
	"encoding/json"
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

func TestSCN026_RefusesLoopWithoutScopedHumanApproval(t *testing.T) {
	// REQ-022 → SCN-026 → TestSCN026_RefusesLoopWithoutScopedHumanApproval
	// Scenario: Refuse autonomous execution without scoped human approval
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "autonomous_scenario_checkpoints.approved"), "SCN-025\n")

	launched := false
	committed := false
	decision, err := StartAutonomousScenarioLoop(repo, AutonomousScenarioLoopRequest{
		Scope: ContractScope{
			SpecPath:    "specs/autonomous_scenario_checkpoints.md",
			FeaturePath: "features/autonomous_scenario_checkpoints.feature",
			ScenarioID:  "SCN-026",
		},
		LaunchScenario: func() error {
			launched = true
			return nil
		},
		CreateScenarioCommit: func() error {
			committed = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("StartAutonomousScenarioLoop returned error: %v", err)
	}
	if decision.Approved {
		t.Fatalf("expected loop to refuse unapproved scenario, got %#v", decision)
	}
	if !strings.Contains(decision.Reason, "explicit human Gherkin approval is required") {
		t.Fatalf("expected explicit human Gherkin approval report, got %q", decision.Reason)
	}
	if launched {
		t.Fatal("expected loop not to launch a scenario agent without scoped approval")
	}
	if committed {
		t.Fatal("expected loop not to create a scenario commit without scoped approval")
	}
}

func TestSCN027_CheckpointsValidatedScenarioInOneLocalCommit(t *testing.T) {
	// REQ-022 → REQ-023 → SCN-027 → TestSCN027_CheckpointsValidatedScenarioInOneLocalCommit
	// Scenario: Checkpoint one approved scenario after strict TDD and objective validation pass
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint_test.go"), "package workflow\n")
	runGit(t, repo, "add", "internal/workflow/checkpoint.go", "internal/workflow/checkpoint_test.go")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint_test.go"), "package workflow\n\nfunc TestCheckpoint() {}\n")

	record, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-027",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go", "internal/workflow/checkpoint_test.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error: %v", err)
	}
	if record.ScenarioID != "SCN-027" || record.CommitID == "" {
		t.Fatalf("expected checkpoint record for SCN-027 with a local commit ID, got %#v", record)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("expected exactly one scenario commit, got %s commits", commits)
	}
	if changed := gitOutput(t, repo, "show", "--format=", "--name-only", "HEAD"); changed != "internal/workflow/checkpoint.go\ninternal/workflow/checkpoint_test.go" {
		t.Fatalf("expected only scenario paths in checkpoint commit, got %q", changed)
	}
	message := gitOutput(t, repo, "show", "-s", "--format=%B", "HEAD")
	for _, attribution := range []string{"AI-generated", "Generated-by", "Co-authored-by"} {
		if strings.Contains(strings.ToLower(message), strings.ToLower(attribution)) {
			t.Fatalf("expected no AI attribution in scenario commit, got %q", message)
		}
	}

	stateContent, err := os.ReadFile(filepath.Join(repo, ".rotta", "autonomous-phase3-state.json"))
	if err != nil {
		t.Fatalf("read workflow state: %v", err)
	}
	var state AutonomousPhase3WorkflowState
	if err := json.Unmarshal(stateContent, &state); err != nil {
		t.Fatalf("unmarshal workflow state: %v", err)
	}
	if state.Checkpoints["SCN-027"] != record.CommitID {
		t.Fatalf("expected workflow state to record SCN-027 commit %q, got %#v", record.CommitID, state)
	}
}

func TestSCN028_HaltsForUnexpectedTrackedChange(t *testing.T) {
	// REQ-024 → SCN-028 → TestSCN028_HaltsForUnexpectedTrackedChange
	// Scenario: Halt for an unexpected tracked change before checkpointing
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n")
	mustWrite(t, filepath.Join(repo, "README.md"), "# Baseline\n")
	runGit(t, repo, "add", "internal/workflow/checkpoint.go", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
	mustWrite(t, filepath.Join(repo, "README.md"), "# Unexpected change\n")

	_, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-028",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err == nil {
		t.Fatal("expected checkpoint evaluation to halt for unexpected tracked path README.md")
	}
	if !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("expected halt to identify README.md, got %q", err)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("expected no scenario commit, got %s commits", commits)
	}
	if status := gitOutput(t, repo, "status", "--short"); status != "M README.md\n M internal/workflow/checkpoint.go" {
		t.Fatalf("expected unexpected change to remain untouched, got %q", status)
	}
}

func TestSCN029_HaltsForUntrackedNonIgnoredFile(t *testing.T) {
	// REQ-024 → SCN-029 → TestSCN029_HaltsForUntrackedNonIgnoredFile
	// Scenario: Halt for an untracked non-ignored file before checkpointing
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", "internal/workflow/checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
	mustWrite(t, filepath.Join(repo, "scenario-report.txt"), "unexpected report\n")

	_, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-029",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err == nil {
		t.Fatal("expected checkpoint evaluation to halt for untracked path scenario-report.txt")
	}
	if !strings.Contains(err.Error(), "scenario-report.txt") {
		t.Fatalf("expected halt to identify scenario-report.txt, got %q", err)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("expected no scenario commit, got %s commits", commits)
	}
	if status := gitOutput(t, repo, "status", "--short"); status != "M internal/workflow/checkpoint.go\n?? scenario-report.txt" {
		t.Fatalf("expected worktree changes to remain untouched, got %q", status)
	}
}

func TestSCN030_DoesNotCheckpointWhenValidationFails(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_DoesNotCheckpointWhenValidationFails
	// Scenario: Do not advance when validation or local commit creation fails
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", "internal/workflow/checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")

	_, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-030",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      false,
		ValidationPassed: true,
	})
	if err == nil {
		t.Fatal("expected checkpoint evaluation to halt when required tests fail")
	}
	if !strings.Contains(err.Error(), "required tests") {
		t.Fatalf("expected failed validation report, got %q", err)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("expected no scenario commit, got %s commits", commits)
	}
	if _, err := os.Stat(filepath.Join(repo, ".rotta", "autonomous-phase3-state.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no checkpoint state to be written, got %v", err)
	}
	if status := gitOutput(t, repo, "status", "--short"); status != "M internal/workflow/checkpoint.go" {
		t.Fatalf("expected scenario change to remain uncheckpointed, got %q", status)
	}
}

func TestSCN031_ContinuesFromCleanSuccessfulCheckpoint(t *testing.T) {
	// REQ-025 → SCN-031 → TestSCN031_ContinuesFromCleanSuccessfulCheckpoint
	// Scenario: Continue automatically only from a clean successful checkpoint
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "internal/workflow/checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")

	record, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-031",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error: %v", err)
	}

	started := ""
	state, err := ContinueFromAutonomousScenarioCheckpoint(repo, record, []string{"SCN-032"}, func(scenarioID string) error {
		started = scenarioID
		return nil
	})
	if err != nil {
		t.Fatalf("ContinueFromAutonomousScenarioCheckpoint returned error: %v", err)
	}
	if status := gitOutput(t, repo, "status", "--short"); status != "" {
		t.Fatalf("expected clean non-ignored worktree at checkpoint boundary, got %q", status)
	}
	if state.Checkpoints["SCN-031"] != record.CommitID || state.CompletedScenario != "SCN-031" || strings.Join(state.RemainingScenarios, ",") != "SCN-032" || state.NextScenario != "SCN-032" {
		t.Fatalf("expected completed, remaining, and next scenario state, got %#v", state)
	}
	if started != "SCN-032" {
		t.Fatalf("expected next approved scenario to start automatically, got %q", started)
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
