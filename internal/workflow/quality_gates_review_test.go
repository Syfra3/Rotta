package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// REQ-074 → SCN-503 → TestSCN503_DeclaredConventionsResolveReproducibleGenericGatePlans
func TestSCN503_DeclaredConventionsResolveReproducibleGenericGatePlans(t *testing.T) {
	// Scenario: Declared project conventions resolve a reproducible generic gate plan
	for _, convention := range []struct {
		category string
		command  string
	}{
		{category: "build", command: "task build"},
		{category: "tests", command: "task test"},
		{category: "static_analysis", command: "task lint"},
		{category: "dependency_checks", command: "task dependencies"},
		{category: "security_checks", command: "task security"},
	} {
		t.Run(convention.category, func(t *testing.T) {
			repo := t.TempDir()
			writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
			metadataPath := filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml")
			writeSCN503File(t, metadataPath, fmt.Sprintf("baseline: baseline-sha\nsnapshot: snapshot-sha\nconventions:\n  %s:\n    command: %s\n", convention.category, convention.command))

			first, err := ResolvePhase4ReviewPlan(repo)
			if err != nil {
				t.Fatalf("resolve first review plan: %v", err)
			}
			firstPersisted, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "review-plan.yaml"))
			if err != nil {
				t.Fatalf("read first persisted review plan: %v", err)
			}

			second, err := ResolvePhase4ReviewPlan(repo)
			if err != nil {
				t.Fatalf("resolve second review plan: %v", err)
			}
			secondPersisted, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "review-plan.yaml"))
			if err != nil {
				t.Fatalf("read second persisted review plan: %v", err)
			}

			if !reflect.DeepEqual(first, second) {
				t.Errorf("review plans differ for unchanged inputs:\nfirst: %#v\nsecond: %#v", first, second)
			}
			if string(firstPersisted) != string(secondPersisted) {
				t.Errorf("persisted review plans differ for unchanged inputs:\nfirst: %s\nsecond: %s", firstPersisted, secondPersisted)
			}
			if len(first.Gates) != 1 {
				t.Fatalf("resolved gate count = %d, want 1", len(first.Gates))
			}
			gate := first.Gates[0]
			if gate.Category != convention.category || gate.Command != convention.command {
				t.Errorf("resolved gate = %#v, want category %q and command %q", gate, convention.category, convention.command)
			}
			if gate.MetadataSource != metadataPath || gate.DiscoveryRule != "declared_convention_only" {
				t.Errorf("resolved gate evidence = %#v, want metadata source %q and discovery rule %q", gate, metadataPath, "declared_convention_only")
			}
			if first.Baseline != "baseline-sha" || first.Snapshot != "snapshot-sha" || first.ConfigurationFingerprint == "" || first.PlanFingerprint == "" {
				t.Errorf("review plan identities = %#v, want baseline, snapshot, configuration, and plan fingerprints", first)
			}
			for _, want := range []string{first.ConfigurationFingerprint, first.PlanFingerprint, metadataPath, "declared_convention_only"} {
				if !strings.Contains(string(firstPersisted), want) {
					t.Errorf("persisted plan does not contain %q:\n%s", want, firstPersisted)
				}
			}
		})
	}
}

func writeSCN503File(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s directory: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
