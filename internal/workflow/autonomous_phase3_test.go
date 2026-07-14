package workflow

import (
	"encoding/json"
	"fmt"
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

// REQ-040, REQ-041 → SCN-247 → TestSCN247_RejectsScenarioCheckpointOnProtectedBranch
func TestSCN247_RejectsScenarioCheckpointOnProtectedBranch(t *testing.T) {
	// Scenario: Commit a validated scenario only on the recorded feature branch
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", "checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish scenario baseline")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")

	_, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-247",
		ExpectedPaths:    []string{"checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "integration",
			FeatureBranch: "main",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "protected branch") {
		t.Fatalf("CheckpointApprovedScenario error = %v, want protected branch rejection", err)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("expected no scenario commit, got %s commits", commits)
	}
}

// REQ-040, REQ-041 → SCN-247 → TestSCN247_CheckpointsOnlyFromRecordedFeatureWorktree
func TestSCN247_CheckpointsOnlyFromRecordedFeatureWorktree(t *testing.T) {
	// Scenario: Commit a validated scenario only on the recorded feature branch
	repo, submission := prepareSCN246Submission(t)
	mustWrite(t, filepath.Join(submission.WorktreePath, "README.md"), "feature change\n")

	record, err := CheckpointApprovedScenario(submission.WorktreePath, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-247",
		ExpectedPaths:    []string{"README.md"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
		Submission:       submission,
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error: %v", err)
	}
	if record.CommitID == "" {
		t.Fatalf("expected a local scenario checkpoint, got %#v", record)
	}
	if branch := gitOutput(t, submission.WorktreePath, "branch", "--show-current"); branch != submission.FeatureBranch {
		t.Fatalf("checkpoint branch = %q, want %q", branch, submission.FeatureBranch)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "main"); commits != "1" {
		t.Fatalf("base branch received scenario checkpoint: %s commits", commits)
	}
}

// REQ-040, REQ-041 → SCN-247 → TestSCN247_RejectsIncompleteFeatureWorktreeRecord
func TestSCN247_RejectsIncompleteFeatureWorktreeRecord(t *testing.T) {
	// Scenario: Commit a validated scenario only on the recorded feature branch
	_, submission := prepareSCN246Submission(t)
	mustWrite(t, filepath.Join(submission.WorktreePath, "README.md"), "feature change\n")
	submission.BaseBranch = ""

	_, err := CheckpointApprovedScenario(submission.WorktreePath, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-247",
		ExpectedPaths:    []string{"README.md"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
		Submission:       submission,
	})
	if err == nil || !strings.Contains(err.Error(), "recorded isolated feature worktree") {
		t.Fatalf("CheckpointApprovedScenario error = %v, want incomplete record rejection", err)
	}
	if commits := gitOutput(t, submission.WorktreePath, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("expected no scenario commit, got %s commits", commits)
	}
}

// REQ-040, REQ-041 → SCN-247 → TestSCN247_RejectsBaseAndDetachedCheckpointBranches
func TestSCN247_RejectsBaseAndDetachedCheckpointBranches(t *testing.T) {
	// Scenario: Commit a validated scenario only on the recorded feature branch
	for _, testCase := range []struct {
		name   string
		mutate func(t *testing.T, submission NewImplementationSubmission)
	}{
		{
			name: "base branch",
			mutate: func(t *testing.T, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", submission.BaseBranch)
			},
		},
		{
			name: "detached HEAD",
			mutate: func(t *testing.T, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", "--detach")
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, submission := prepareSCN246Submission(t)
			testCase.mutate(t, submission)

			_, err := CheckpointApprovedScenario(submission.WorktreePath, ScenarioCheckpointRequest{
				ScenarioID:       "SCN-247",
				ExpectedPaths:    []string{"README.md"},
				TDDComplete:      true,
				TestsPassed:      true,
				ValidationPassed: true,
				Submission:       submission,
			})
			if err == nil {
				t.Fatal("expected checkpoint rejection")
			}
			if commits := gitOutput(t, submission.WorktreePath, "rev-list", "--count", "HEAD"); commits != "1" {
				t.Fatalf("expected no scenario commit, got %s commits", commits)
			}
		})
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

func TestSCN032_SendsFinalCheckpointToReviewWithoutPublishing(t *testing.T) {
	// REQ-025 → REQ-027 → SCN-032 → TestSCN032_SendsFinalCheckpointToReviewWithoutPublishing
	// Scenario: Send the final checkpointed scenario to review without publishing
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
		ScenarioID:       "SCN-032",
		ExpectedPaths:    []string{"internal/workflow/checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error: %v", err)
	}

	reviewStarted := false
	decision, err := CompleteAutonomousPhase3Boundary(repo, record, func() error {
		reviewStarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteAutonomousPhase3Boundary returned error: %v", err)
	}
	if !reviewStarted || decision.Phase != "Phase 4 review" {
		t.Fatalf("expected final checkpoint to advance to Phase 4 review, got %#v", decision)
	}
	if decision.FinalHumanApproval {
		t.Fatalf("expected Phase 4 review gate, not final human approval, got %#v", decision)
	}
	if state := gitOutput(t, repo, "status", "--short"); state != "" {
		t.Fatalf("expected final checkpoint boundary to remain clean, got %q", state)
	}
	if remotes := gitOutput(t, repo, "remote"); remotes != "" {
		t.Fatalf("expected no remote branch publication, got remotes %q", remotes)
	}
	if tags := gitOutput(t, repo, "tag"); tags != "" {
		t.Fatalf("expected no tag publication, got tags %q", tags)
	}
}

func TestSCN033_CheckpointsExpectedSensitiveScopeAfterOrdinaryValidation(t *testing.T) {
	// REQ-026 → SCN-033 → TestSCN033_CheckpointsExpectedSensitiveScopeAfterOrdinaryValidation
	// Scenario: Checkpoint an expected sensitive-scope scenario after ordinary validation passes
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "internal", "auth", "session.go"), "package auth\n")
	mustWrite(t, filepath.Join(repo, "internal", "auth", "session_test.go"), "package auth\n")
	runGit(t, repo, "add", ".gitignore", "internal/auth/session.go", "internal/auth/session_test.go")
	runGit(t, repo, "commit", "-m", "test: establish sensitive scenario baseline")
	mustWrite(t, filepath.Join(repo, "internal", "auth", "session.go"), "package auth\n\nfunc session() {}\n")
	mustWrite(t, filepath.Join(repo, "internal", "auth", "session_test.go"), "package auth\n\nfunc TestSession() {}\n")

	record, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-033",
		ExpectedPaths:    []string{"internal/auth/session.go", "internal/auth/session_test.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error for expected auth paths: %v", err)
	}
	if record.CommitID == "" {
		t.Fatalf("expected a local checkpoint for sensitive scenario, got %#v", record)
	}
	if changed := gitOutput(t, repo, "show", "--format=", "--name-only", "HEAD"); changed != "internal/auth/session.go\ninternal/auth/session_test.go" {
		t.Fatalf("expected sensitive scenario paths in checkpoint commit, got %q", changed)
	}

	reviewStarted := false
	decision, err := CompleteAutonomousPhase3Boundary(repo, record, func() error {
		reviewStarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteAutonomousPhase3Boundary returned error: %v", err)
	}
	if !reviewStarted || decision.Phase != "Phase 4 review" || decision.FinalHumanApproval {
		t.Fatalf("expected sensitive scenario to preserve the Phase 4 review gate, got %#v", decision)
	}
}

func TestSCN034_ReportsHumanMayPushOnceAfterReview(t *testing.T) {
	// REQ-027 → SCN-034 → TestSCN034_ReportsHumanMayPushOnceAfterReview
	// Scenario: Require a human to push once after review completes
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", "checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish checkpointed review baseline")

	report := ReportAutonomousWorkflowCompletion()
	if !report.HumanMayPushOnce {
		t.Fatalf("expected report to permit one manual human push, got %#v", report)
	}
	if !strings.Contains(report.Message, "human may manually push the feature branch once") {
		t.Fatalf("expected manual-push report, got %q", report.Message)
	}
	if remotes := gitOutput(t, repo, "remote"); remotes != "" {
		t.Fatalf("expected final report not to publish remotely, got remotes %q", remotes)
	}
}

func TestSCN026_ReportsApprovalGateErrorAndApprovedDecision(t *testing.T) {
	// REQ-022 → SCN-026 → TestSCN026_ReportsApprovalGateErrorAndApprovedDecision
	// Scenario: Refuse autonomous execution without scoped human approval
	scope := ContractScope{
		SpecPath:    "specs/autonomous_scenario_checkpoints.md",
		FeaturePath: "features/autonomous_scenario_checkpoints.feature",
		ScenarioID:  "SCN-026",
	}

	t.Run("returns approval inspection errors", func(t *testing.T) {
		repoFile := filepath.Join(t.TempDir(), "not-a-repository")
		mustWrite(t, repoFile, "not a directory\n")

		if _, err := StartAutonomousScenarioLoop(repoFile, AutonomousScenarioLoopRequest{Scope: scope}); err == nil {
			t.Fatal("expected approval inspection error when repository root is a file")
		}
	})

	t.Run("reports scoped approval", func(t *testing.T) {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "autonomous_scenario_checkpoints.approved"), "SCN-026\n")

		decision, err := StartAutonomousScenarioLoop(repo, AutonomousScenarioLoopRequest{Scope: scope})
		if err != nil {
			t.Fatalf("StartAutonomousScenarioLoop returned error: %v", err)
		}
		if !decision.Approved || decision.Reason != "scoped human approval recorded" {
			t.Fatalf("expected scoped approval decision, got %#v", decision)
		}
	})
}

func TestSCN030_ReportsCheckpointFailurePaths(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_ReportsCheckpointFailurePaths
	// Scenario: Do not advance when validation or local commit creation fails
	request := ScenarioCheckpointRequest{ScenarioID: "SCN-030", ExpectedPaths: []string{"checkpoint.go"}, TDDComplete: true, TestsPassed: true, ValidationPassed: true}

	t.Run("rejects missing TDD evidence and objective validation", func(t *testing.T) {
		withoutTDD := request
		withoutTDD.TDDComplete = false
		if _, err := CheckpointApprovedScenario(t.TempDir(), withoutTDD); err == nil || !strings.Contains(err.Error(), "strict Red, Green, and Refactor") {
			t.Fatalf("expected missing TDD evidence error, got %v", err)
		}

		withoutValidation := request
		withoutValidation.ValidationPassed = false
		if _, err := CheckpointApprovedScenario(t.TempDir(), withoutValidation); err == nil || !strings.Contains(err.Error(), "active objective validation") {
			t.Fatalf("expected objective validation error, got %v", err)
		}
	})

	t.Run("reports repository inspection and staging errors", func(t *testing.T) {
		repoFile := filepath.Join(t.TempDir(), "not-a-repository")
		mustWrite(t, repoFile, "not a directory\n")
		if _, err := CheckpointApprovedScenario(repoFile, request); err == nil || !strings.Contains(err.Error(), "inspect untracked scenario changes") {
			t.Fatalf("expected untracked inspection error, got %v", err)
		}
		if _, err := trackedChangedPaths(repoFile); err == nil || !strings.Contains(err.Error(), "inspect tracked scenario changes") {
			t.Fatalf("expected tracked inspection error, got %v", err)
		}

		repo := t.TempDir()
		runGit(t, repo, "init")
		if _, err := CheckpointApprovedScenario(repo, request); err == nil || !strings.Contains(err.Error(), "stage scenario changes") {
			t.Fatalf("expected staging error, got %v", err)
		}
	})

	t.Run("reports commit and state-write errors", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@example.invalid")
		runGit(t, repo, "config", "user.name", "Test User")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
		runGit(t, repo, "add", "checkpoint.go")
		runGit(t, repo, "commit", "-m", "test: establish checkpoint baseline")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
		runGit(t, repo, "config", "user.name", "")
		runGit(t, repo, "config", "user.email", "")
		if _, err := CheckpointApprovedScenario(repo, request); err == nil || !strings.Contains(err.Error(), "create scenario checkpoint") {
			t.Fatalf("expected commit creation error, got %v", err)
		}

		repo = t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@example.invalid")
		runGit(t, repo, "config", "user.name", "Test User")
		mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta\n")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
		runGit(t, repo, "add", ".gitignore", "checkpoint.go")
		runGit(t, repo, "commit", "-m", "test: establish state-write baseline")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
		mustWrite(t, filepath.Join(repo, ".rotta"), "not a directory\n")
		if _, err := CheckpointApprovedScenario(repo, request); err == nil || !strings.Contains(err.Error(), "create autonomous Phase 3 workflow state directory") {
			t.Fatalf("expected state-write error, got %v", err)
		}
	})
}

func TestSCN030_ValidatesCheckpointEvidence(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_ValidatesCheckpointEvidence
	// Scenario: Do not advance when validation or local commit creation fails
	valid := ScenarioCheckpointRequest{TDDComplete: true, TestsPassed: true, ValidationPassed: true}
	if err := validateScenarioCheckpointEvidence(valid); err != nil {
		t.Fatalf("expected valid checkpoint evidence, got %v", err)
	}
}

func TestSCN030_ReportsCheckpointRevisionLookupFailure(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_ReportsCheckpointRevisionLookupFailure
	// Scenario: Do not advance when validation or local commit creation fails
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", "checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: establish revision lookup baseline")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git executable: %v", err)
	}
	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "git")
	mustWrite(t, wrapperPath, fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"rev-parse\" ]; then\n  printf '%%s\\n' 'forced revision lookup failure' >&2\n  exit 1\nfi\nexec %q \"$@\"\n", gitPath))
	if err := os.Chmod(wrapperPath, 0o755); err != nil {
		t.Fatalf("make git wrapper executable: %v", err)
	}
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{
		ScenarioID:       "SCN-030",
		ExpectedPaths:    []string{"checkpoint.go"},
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "read scenario checkpoint commit") {
		t.Fatalf("expected checkpoint revision lookup error, got %v", err)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("expected the local checkpoint commit before revision lookup failed, got %s commits", commits)
	}
	if _, err := os.Stat(filepath.Join(repo, ".rotta", "autonomous-phase3-state.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no checkpoint state after revision lookup failed, got %v", err)
	}
}

func TestSCN031_StopsAtDirtyBoundaryAndCallbackFailure(t *testing.T) {
	// REQ-025 → SCN-031 → TestSCN031_StopsAtDirtyBoundaryAndCallbackFailure
	// Scenario: Continue automatically only from a clean successful checkpoint
	record := ScenarioCheckpointRecord{ScenarioID: "SCN-031", CommitID: "abc123"}

	t.Run("reports checkpoint status inspection failure", func(t *testing.T) {
		repoFile := filepath.Join(t.TempDir(), "not-a-repository")
		mustWrite(t, repoFile, "not a directory\n")
		if _, err := ContinueFromAutonomousScenarioCheckpoint(repoFile, record, []string{"SCN-032"}, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "check scenario checkpoint boundary") {
			t.Fatalf("expected checkpoint status inspection error, got %v", err)
		}
	})

	t.Run("rejects a dirty checkpoint boundary", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		mustWrite(t, filepath.Join(repo, "unexpected.txt"), "unexpected\n")
		if _, err := ContinueFromAutonomousScenarioCheckpoint(repo, record, []string{"SCN-032"}, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "non-ignored changes") {
			t.Fatalf("expected dirty boundary error, got %v", err)
		}
	})

	t.Run("returns next-scenario callback failure", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		configureTestGitIdentity(t, repo)
		mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
		runGit(t, repo, "add", ".gitignore")
		runGit(t, repo, "commit", "-m", "test: ignore workflow state")
		callbackErr := fmt.Errorf("next scenario failed")
		if _, err := ContinueFromAutonomousScenarioCheckpoint(repo, record, []string{"SCN-032"}, func(string) error { return callbackErr }); err != callbackErr {
			t.Fatalf("expected callback error %v, got %v", callbackErr, err)
		}
	})
}

func TestSCN031_StopsWhenContinuingStateWriteFails(t *testing.T) {
	// REQ-025 → SCN-031 → TestSCN031_StopsWhenContinuingStateWriteFails
	// Scenario: Continue automatically only from a clean successful checkpoint
	repo := t.TempDir()
	runGit(t, repo, "init")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta\n")
	runGit(t, repo, "add", ".gitignore")
	runGit(t, repo, "commit", "-m", "test: ignore workflow state")
	mustWrite(t, filepath.Join(repo, ".rotta"), "not a directory\n")

	started := false
	_, err := ContinueFromAutonomousScenarioCheckpoint(repo, ScenarioCheckpointRecord{ScenarioID: "SCN-031", CommitID: "abc123"}, []string{"SCN-032"}, func(string) error {
		started = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "create autonomous Phase 3 workflow state directory") {
		t.Fatalf("expected continuing state-write error, got %v", err)
	}
	if started {
		t.Fatal("expected next scenario not to start when workflow state cannot be written")
	}
}

func TestSCN032_StopsAtDirtyFinalBoundaryAndReviewFailure(t *testing.T) {
	// REQ-025 → REQ-027 → SCN-032 → TestSCN032_StopsAtDirtyFinalBoundaryAndReviewFailure
	// Scenario: Send the final checkpointed scenario to review without publishing
	record := ScenarioCheckpointRecord{ScenarioID: "SCN-032", CommitID: "abc123"}

	t.Run("reports final-boundary status inspection failure", func(t *testing.T) {
		repoFile := filepath.Join(t.TempDir(), "not-a-repository")
		mustWrite(t, repoFile, "not a directory\n")
		if _, err := CompleteAutonomousPhase3Boundary(repoFile, record, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "check final scenario checkpoint boundary") {
			t.Fatalf("expected final-boundary status inspection error, got %v", err)
		}
	})

	t.Run("rejects a dirty final boundary", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		mustWrite(t, filepath.Join(repo, "unexpected.txt"), "unexpected\n")
		if _, err := CompleteAutonomousPhase3Boundary(repo, record, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "non-ignored changes") {
			t.Fatalf("expected dirty final boundary error, got %v", err)
		}
	})

	t.Run("returns review callback failure", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		configureTestGitIdentity(t, repo)
		mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
		runGit(t, repo, "add", ".gitignore")
		runGit(t, repo, "commit", "-m", "test: ignore workflow state")
		callbackErr := fmt.Errorf("review failed")
		if _, err := CompleteAutonomousPhase3Boundary(repo, record, func() error { return callbackErr }); err != callbackErr {
			t.Fatalf("expected review callback error %v, got %v", callbackErr, err)
		}
	})
}

func TestSCN032_StopsWhenFinalStateWriteFails(t *testing.T) {
	// REQ-025 → REQ-027 → SCN-032 → TestSCN032_StopsWhenFinalStateWriteFails
	// Scenario: Send the final checkpointed scenario to review without publishing
	repo := t.TempDir()
	runGit(t, repo, "init")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta\n")
	runGit(t, repo, "add", ".gitignore")
	runGit(t, repo, "commit", "-m", "test: ignore workflow state")
	mustWrite(t, filepath.Join(repo, ".rotta"), "not a directory\n")

	reviewStarted := false
	_, err := CompleteAutonomousPhase3Boundary(repo, ScenarioCheckpointRecord{ScenarioID: "SCN-032", CommitID: "abc123"}, func() error {
		reviewStarted = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "create autonomous Phase 3 workflow state directory") {
		t.Fatalf("expected final state-write error, got %v", err)
	}
	if reviewStarted {
		t.Fatal("expected review not to start when workflow state cannot be written")
	}
}

func TestSCN025_PreparationUsesRequestedRepositoryAndVerifiesEveryArtifact(t *testing.T) {
	// REQ-021 → SCN-025 → TestSCN025_PreparationUsesRequestedRepositoryAndVerifiesEveryArtifact
	// Scenario: Start autonomous Phase 3 in a new clean isolated feature worktree
	t.Run("creates the requested branch in the requested repository", func(t *testing.T) {
		repo := checkpointTestRepository(t)
		mustWrite(t, filepath.Join(repo, "specs", "unique-contract.md"), "# contract\n")
		mustWrite(t, filepath.Join(repo, "features", "unique-contract.feature"), "Feature: unique contract\n")
		runGit(t, repo, "add", "specs/unique-contract.md", "features/unique-contract.feature")
		runGit(t, repo, "commit", "-m", "test: add unique contract")

		worktreePath := filepath.Join(t.TempDir(), "requested-worktree")
		branch := "feat/requested-worktree"
		_, err := PrepareAutonomousPhase3Worktree(repo, AutonomousPhase3WorktreeRequest{
			Scope:  ContractScope{SpecPath: "specs/unique-contract.md", FeaturePath: "features/unique-contract.feature", ScenarioID: "SCN-025"},
			Branch: branch, WorktreePath: worktreePath,
		})
		if err != nil {
			t.Fatalf("PrepareAutonomousPhase3Worktree returned error: %v", err)
		}
		if !strings.Contains(gitOutput(t, repo, "branch", "--list", branch), branch) {
			t.Fatalf("expected requested branch %q in requested repository", branch)
		}
		if !strings.Contains(gitOutput(t, repo, "worktree", "list", "--porcelain"), "worktree "+worktreePath) {
			t.Fatalf("expected requested worktree %q in requested repository", worktreePath)
		}
	})

	t.Run("rejects a missing second contract artifact", func(t *testing.T) {
		repo := checkpointTestRepository(t)
		mustWrite(t, filepath.Join(repo, "specs", "present-contract.md"), "# contract\n")
		runGit(t, repo, "add", "specs/present-contract.md")
		runGit(t, repo, "commit", "-m", "test: add only spec")

		_, err := PrepareAutonomousPhase3Worktree(repo, AutonomousPhase3WorktreeRequest{
			Scope:  ContractScope{SpecPath: "specs/present-contract.md", FeaturePath: "features/missing-contract.feature", ScenarioID: "SCN-025"},
			Branch: "feat/missing-contract", WorktreePath: filepath.Join(t.TempDir(), "missing-contract"),
		})
		if err == nil || !strings.Contains(err.Error(), "verify approved contract artifact features/missing-contract.feature") {
			t.Fatalf("expected missing feature artifact to halt preparation, got %v", err)
		}
	})
}

func TestSCN030_CheckpointPreservesRepositoryAndStateWriteFailures(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_CheckpointPreservesRepositoryAndStateWriteFailures
	// Scenario: Do not advance when validation or local commit creation fails
	t.Run("stops when tracked-change inspection fails", func(t *testing.T) {
		repo := checkpointTestRepository(t)
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
		runGit(t, repo, "add", "checkpoint.go")
		runGit(t, repo, "commit", "-m", "test: checkpoint baseline")

		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Fatalf("locate git executable: %v", err)
		}
		wrapperDir := t.TempDir()
		mustWrite(t, filepath.Join(wrapperDir, "git"), fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"diff\" ]; then\n  printf 'forced tracked inspection failure\\n' >&2\n  exit 1\nfi\nexec %q \"$@\"\n", gitPath))
		if err := os.Chmod(filepath.Join(wrapperDir, "git"), 0o755); err != nil {
			t.Fatalf("make git wrapper executable: %v", err)
		}
		t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err = CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{ScenarioID: "SCN-030", ExpectedPaths: []string{"checkpoint.go"}, TDDComplete: true, TestsPassed: true, ValidationPassed: true})
		if err == nil || !strings.Contains(err.Error(), "inspect tracked scenario changes") {
			t.Fatalf("expected tracked-change inspection failure, got %v", err)
		}
	})

	t.Run("stops when the state file cannot be written", func(t *testing.T) {
		repo := checkpointTestRepository(t)
		mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
		runGit(t, repo, "add", ".gitignore", "checkpoint.go")
		runGit(t, repo, "commit", "-m", "test: checkpoint baseline")
		mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")
		if err := os.MkdirAll(filepath.Join(repo, ".rotta", "autonomous-phase3-state.json"), 0o755); err != nil {
			t.Fatalf("create state-file directory: %v", err)
		}

		_, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{ScenarioID: "SCN-030", ExpectedPaths: []string{"checkpoint.go"}, TDDComplete: true, TestsPassed: true, ValidationPassed: true})
		if err == nil || !strings.Contains(err.Error(), "write autonomous Phase 3 workflow state") {
			t.Fatalf("expected state-file write failure, got %v", err)
		}
	})
}

func TestSCN030_CheckpointRecordUsesTheScenarioRepositoryHead(t *testing.T) {
	// REQ-023 → REQ-025 → SCN-030 → TestSCN030_CheckpointRecordUsesTheScenarioRepositoryHead
	// Scenario: Do not advance when validation or local commit creation fails
	repo := checkpointTestRepository(t)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "checkpoint.go")
	runGit(t, repo, "commit", "-m", "test: checkpoint baseline")
	mustWrite(t, filepath.Join(repo, "checkpoint.go"), "package workflow\n\nfunc checkpoint() {}\n")

	record, err := CheckpointApprovedScenario(repo, ScenarioCheckpointRequest{ScenarioID: "SCN-030", ExpectedPaths: []string{"checkpoint.go"}, TDDComplete: true, TestsPassed: true, ValidationPassed: true})
	if err != nil {
		t.Fatalf("CheckpointApprovedScenario returned error: %v", err)
	}
	if record.CommitID != gitOutput(t, repo, "rev-parse", "HEAD") {
		t.Fatalf("expected checkpoint record to use requested repository HEAD, got %#v", record)
	}
}

func TestSCN025_PreparationStopsForCreateAndBoundaryStatusFailures(t *testing.T) {
	// REQ-021 → SCN-025 → TestSCN025_PreparationStopsForCreateAndBoundaryStatusFailures
	// Scenario: Start autonomous Phase 3 in a new clean isolated feature worktree
	t.Run("reports worktree creation failure", func(t *testing.T) {
		repoRootFile := filepath.Join(t.TempDir(), "not-a-repository")
		mustWrite(t, repoRootFile, "not a directory\n")
		_, err := PrepareAutonomousPhase3Worktree(repoRootFile, AutonomousPhase3WorktreeRequest{Branch: "feat/no-repo", WorktreePath: filepath.Join(t.TempDir(), "worktree")})
		if err == nil || !strings.Contains(err.Error(), "create isolated Phase 3 worktree") {
			t.Fatalf("expected worktree creation failure, got %v", err)
		}
	})

	t.Run("rejects a non-clean status result", func(t *testing.T) {
		repo := checkpointTestRepository(t)
		mustWrite(t, filepath.Join(repo, "specs", "clean-contract.md"), "# contract\n")
		mustWrite(t, filepath.Join(repo, "features", "clean-contract.feature"), "Feature: clean contract\n")
		runGit(t, repo, "add", "specs/clean-contract.md", "features/clean-contract.feature")
		runGit(t, repo, "commit", "-m", "test: add clean contract")

		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Fatalf("locate git executable: %v", err)
		}
		wrapperDir := t.TempDir()
		mustWrite(t, filepath.Join(wrapperDir, "git"), fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"status\" ]; then\n  printf ' M unexpected.txt\\n'\n  exit 0\nfi\nexec %q \"$@\"\n", gitPath))
		if err := os.Chmod(filepath.Join(wrapperDir, "git"), 0o755); err != nil {
			t.Fatalf("make git wrapper executable: %v", err)
		}
		t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err = PrepareAutonomousPhase3Worktree(repo, AutonomousPhase3WorktreeRequest{
			Scope:  ContractScope{SpecPath: "specs/clean-contract.md", FeaturePath: "features/clean-contract.feature", ScenarioID: "SCN-025"},
			Branch: "feat/dirty-status", WorktreePath: filepath.Join(t.TempDir(), "dirty-status"),
		})
		if err == nil || !strings.Contains(err.Error(), "isolated Phase 3 worktree has non-ignored changes") {
			t.Fatalf("expected non-clean worktree status error, got %v", err)
		}
	})
}

func TestSCN025_RejectsUnsafeWorktreeInputs(t *testing.T) {
	// REQ-021 → SCN-025 → TestSCN025_RejectsUnsafeWorktreeInputs
	// Scenario: Start autonomous Phase 3 in a new clean isolated feature worktree
	_, err := PrepareAutonomousPhase3Worktree(t.TempDir(), AutonomousPhase3WorktreeRequest{
		Branch:       "-malicious-branch",
		WorktreePath: filepath.Join(t.TempDir(), "phase3"),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid Phase 3 worktree branch") {
		t.Fatalf("expected unsafe branch to be rejected before invoking git, got %v", err)
	}
}

func TestSCN027_RejectsUnsafeCheckpointInputs(t *testing.T) {
	// REQ-022 → REQ-023 → SCN-027 → TestSCN027_RejectsUnsafeCheckpointInputs
	// Scenario: Checkpoint one approved scenario after strict TDD and objective validation pass
	if err := stageScenarioChanges(t.TempDir(), []string{"../outside"}); err == nil || !strings.Contains(err.Error(), "invalid scenario path") {
		t.Fatalf("expected unsafe scenario path to be rejected before invoking git, got %v", err)
	}
	if _, err := createScenarioCheckpointCommit(t.TempDir(), "SCN-027\ninjected"); err == nil || !strings.Contains(err.Error(), "invalid scenario ID") {
		t.Fatalf("expected unsafe scenario ID to be rejected before invoking git, got %v", err)
	}
}

func TestSCN027_WritesPrivateWorkflowState(t *testing.T) {
	// REQ-022 → REQ-023 → SCN-027 → TestSCN027_WritesPrivateWorkflowState
	// Scenario: Checkpoint one approved scenario after strict TDD and objective validation pass
	repo := t.TempDir()
	if err := writeAutonomousPhase3WorkflowState(repo, ScenarioCheckpointRecord{ScenarioID: "SCN-027", CommitID: "abc123"}); err != nil {
		t.Fatalf("writeAutonomousPhase3WorkflowState returned error: %v", err)
	}

	directory, err := os.Stat(filepath.Join(repo, ".rotta"))
	if err != nil {
		t.Fatalf("stat workflow state directory: %v", err)
	}
	if directory.Mode().Perm() != 0o750 {
		t.Fatalf("expected private workflow state directory mode 0750, got %04o", directory.Mode().Perm())
	}
	stateFile, err := os.Stat(filepath.Join(repo, ".rotta", "autonomous-phase3-state.json"))
	if err != nil {
		t.Fatalf("stat workflow state file: %v", err)
	}
	if stateFile.Mode().Perm() != 0o600 {
		t.Fatalf("expected private workflow state file mode 0600, got %04o", stateFile.Mode().Perm())
	}
}

func checkpointTestRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	configureTestGitIdentity(t, repo)
	return repo
}

func configureTestGitIdentity(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
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
