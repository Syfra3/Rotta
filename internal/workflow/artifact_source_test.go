package workflow

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources(t *testing.T) {
	// REQ-011 → REQ-012 → SCN-012 → TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources
	// Scenario: Active hard spec and feature files are tracked as the contract source of truth
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")

	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-011 @REQ-012 @SCN-012\nScenario: Active hard spec and feature files are tracked as the contract source of truth\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md", "features/workflow_artifact_lifecycle.feature")
	runGit(t, repo, "commit", "-m", "test: track approved contract artifacts")

	status, err := EvaluateContractSourceOfTruth(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-012",
	})
	if err != nil {
		t.Fatalf("EvaluateContractSourceOfTruth returned error: %v", err)
	}
	if !status.Authoritative {
		t.Fatalf("expected tracked spec and feature to be authoritative, got: %#v", status)
	}
	if !status.SpecTracked || !status.FeatureTracked {
		t.Fatalf("expected both contract files to be tracked, got: %#v", status)
	}
	if status.RequiresAncoraContractText {
		t.Fatalf("expected repository files to recover behavior without full Ancora contract text")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
