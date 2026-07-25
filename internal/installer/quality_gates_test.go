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
		"format: rotta.quality-gates/v1",
		"gates:",
		"- id: critical_path_statement_coverage",
		"thresholds: { minimum: 0.95 }",
		"CheckpointApprovedScenario",
		"ContinueFromAutonomousScenarioCheckpoint",
		"CompleteAutonomousPhase3Boundary",
		"run: \"go-mutesting ./<changed-module>\"",
		"changed_module: \"./<changed-module>\"",
		"score_pattern: 'The mutation score is ([0-9]+(?:\\.[0-9]+)?)'",
		"thresholds: { minimum: 0.80 }",
		"thresholds: { maximum: 0 }",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated quality gates missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "critical_path_branch_coverage") {
		t.Errorf("generated quality gates retain obsolete branch coverage gate:\n%s", got)
	}
}
