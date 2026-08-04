package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-087 → SCN-619 → TestSCN619_StrictModeKeepsOneScenarioPerFullCheckpoint
func TestSCN619_StrictModeKeepsOneScenarioPerFullCheckpoint(t *testing.T) {
	// Scenario: Strict mode keeps one scenario per full checkpoint
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "feature/strict-slices")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	sourcePath := filepath.Join(repo, "internal", "workflow", "slice.go")
	mustWrite(t, sourcePath, "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "internal/workflow/slice.go")
	runGit(t, repo, "commit", "-m", "test: establish strict slice baseline")

	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "checkpoint_mode: strict_per_scenario\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "approved_scenarios: [SCN-701, SCN-702]\nnext_scenario: SCN-701\n")

	newEvidence := func(scenarioID string) StrictScenarioEvidence {
		evidence := StrictScenarioEvidence{
			ScenarioID:              scenarioID,
			RedEvidencePath:         filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-red.txt"),
			GreenEvidencePath:       filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-green.txt"),
			RefactorEvidencePath:    filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-refactor.txt"),
			FocusedTestEvidencePath: filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-focused-test.txt"),
		}
		for _, path := range []string{evidence.RedEvidencePath, evidence.GreenEvidencePath, evidence.RefactorEvidencePath, evidence.FocusedTestEvidencePath} {
			mustWrite(t, path, scenarioID+" evidence\n")
		}
		return evidence
	}

	validations := 0
	mustWrite(t, sourcePath, "package workflow\n\nfunc firstStrictScenario() {}\n")
	first, err := ExecuteStrictScenario(StrictScenarioRequest{
		FeatureWorktree:      repo,
		Scenario:             newEvidence("SCN-701"),
		ExpectedChangedPaths: []string{"internal/workflow/slice.go"},
		RunFullValidation: func() error {
			validations++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteStrictScenario(first) error = %v", err)
	}
	if validations != 1 || len(first.Scenarios) != 1 || first.Scenarios[0].ScenarioID != "SCN-701" || first.Checkpoint == "" {
		t.Fatalf("first strict report = %#v with %d validations, want one scenario and one validation", first, validations)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("checkpoint commits after first strict scenario = %s, want 2", commits)
	}

	mustWrite(t, sourcePath, "package workflow\n\nfunc firstStrictScenario() {}\nfunc secondStrictScenario() {}\n")
	second, err := ExecuteStrictScenario(StrictScenarioRequest{
		FeatureWorktree:      repo,
		Scenario:             newEvidence("SCN-702"),
		ExpectedChangedPaths: []string{"internal/workflow/slice.go"},
		RunFullValidation: func() error {
			validations++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteStrictScenario(second) error = %v", err)
	}
	if validations != 2 || len(second.Scenarios) != 1 || second.Scenarios[0].ScenarioID != "SCN-702" || second.Checkpoint == "" || second.Checkpoint == first.Checkpoint {
		t.Fatalf("second strict report = %#v with %d validations, want a separate one-scenario validation and checkpoint", second, validations)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "3" {
		t.Fatalf("checkpoint commits after second strict scenario = %s, want 3", commits)
	}

	state, readErr := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if readErr != nil {
		t.Fatalf("read strict slice state: %v", readErr)
	}
	for _, required := range []string{"SCN-701", "SCN-702", first.Checkpoint, second.Checkpoint} {
		if !strings.Contains(string(state), required) {
			t.Fatalf("state = %s, missing strict scenario trace %q", state, required)
		}
	}
}
