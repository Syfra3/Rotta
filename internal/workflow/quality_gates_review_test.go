package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-073 → SCN-502 → TestSCN502_V1ConfigurationBlocksReviewWithoutExecution
func TestSCN502_V1ConfigurationBlocksReviewWithoutExecution(t *testing.T) {
	// Scenario: A v1 configuration is rejected with migration remediation
	repo := t.TempDir()
	configPath := filepath.Join(repo, ".rotta", "quality-gates.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("create quality-gates directory: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("format: rotta.quality-gates/v1\ncommand: must-not-run\n"), 0o600); err != nil {
		t.Fatalf("write v1 quality-gates configuration: %v", err)
	}

	executed := false
	review, err := RequestPhase4Review(repo, func(string) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("request Phase 4 review: %v", err)
	}
	if review.State != QualityGatesReviewBlocked {
		t.Errorf("review state = %q, want %q", review.State, QualityGatesReviewBlocked)
	}
	for _, want := range []string{
		"v1 is unsupported",
		"not automatically migrated",
		".rotta/quality-gates.yaml",
	} {
		if !strings.Contains(review.Result, want) {
			t.Errorf("review result %q does not contain %q", review.Result, want)
		}
	}
	if executed {
		t.Fatal("v1 configuration command was executed")
	}
}
