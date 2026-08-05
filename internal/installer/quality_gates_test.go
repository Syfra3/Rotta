package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-073 → SCN-501 → TestSCN501_GeneratedQualityGatesContainOnlyRequiredGenericCategories
func TestSCN501_GeneratedQualityGatesContainOnlyRequiredGenericCategories(t *testing.T) {
	// Scenario: A newly generated quality-gates configuration defines only required generic categories
	projectPath := t.TempDir()

	if _, err := installConfig(projectPath); err != nil {
		t.Fatalf("install config: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectPath, ".rotta", "quality-gates.yaml"))
	if err != nil {
		t.Fatalf("read generated quality gates: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "format: rotta.quality-gates/v1") {
		t.Errorf("generated quality gates do not use the current format:\n%s", got)
	}

	for _, want := range []string{
		"- id: build",
		"- id: tests",
		"- id: changed_file_scope",
		"- id: static_analysis",
		"- id: dependency_checks",
		"- id: security_checks",
	} {
		if strings.Count(got, want) != 1 {
			t.Errorf("generated quality gates must contain exactly one %q entry:\n%s", want, got)
		}
	}

	if count := strings.Count(got, "- id:"); count != 6 {
		t.Errorf("generated quality gates contain %d entries, want 6:\n%s", count, got)
	}
	for _, prohibited := range []string{"coverage", "mutation", "complexity", "named_function", "language_profile", "command:", "run:"} {
		if strings.Contains(got, prohibited) {
			t.Errorf("generated quality gates contain prohibited %q:\n%s", prohibited, got)
		}
	}
}
