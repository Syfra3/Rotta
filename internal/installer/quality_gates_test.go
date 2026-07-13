package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallConfigGeneratesActionableCoverageAndMutationGates(t *testing.T) {
	projectPath := t.TempDir()

	if _, err := installConfig(projectPath); err != nil {
		t.Fatalf("install config: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectPath, ".rotta", "quality-gates.yaml"))
	if err != nil {
		t.Fatalf("read generated quality gates: %v", err)
	}

	got := string(data)
	for _, want := range []string{
		"critical_path_statement_coverage: 0.95",
		"- critical_path_statement_coverage",
		"CheckpointApprovedScenario",
		"ContinueFromAutonomousScenarioCheckpoint",
		"CompleteAutonomousPhase3Boundary",
		"runner_command: go-mutesting",
		"changed_module_target: ./<changed-module>",
		"score_pattern: 'The mutation score is ([0-9]+(?:\\.[0-9]+)?)'",
		"score_threshold: 0.80",
		"critical_survivors_max: 0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated quality gates missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "critical_path_branch_coverage") {
		t.Errorf("generated quality gates retain obsolete branch coverage gate:\n%s", got)
	}
}
