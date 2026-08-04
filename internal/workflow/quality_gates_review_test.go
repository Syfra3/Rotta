package workflow

import (
	"fmt"
	"os"
	"os/exec"
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

// REQ-074 → SCN-505 → TestSCN505_MissingSecurityCheckConventionBlocksReviewWithoutExecution
func TestSCN505_MissingSecurityCheckConventionBlocksReviewWithoutExecution(t *testing.T) {
	// Scenario: Missing required command discovery blocks review instead of guessing
	repo := t.TempDir()
	writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), "baseline: baseline-sha\nsnapshot: snapshot-sha\nconventions:\n  build:\n    command: task build\n")

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
	for _, want := range []string{"security-check", "declare", "configure", "supported convention"} {
		if !strings.Contains(review.Result, want) {
			t.Errorf("review result %q does not contain %q", review.Result, want)
		}
	}
	if executed {
		t.Fatal("missing security-check command was executed, invented, substituted, or silently passed")
	}
}

// REQ-074 → SCN-504 → TestSCN504_ChangedFileScopeUsesRecordedSnapshots
func TestSCN504_ChangedFileScopeUsesRecordedSnapshots(t *testing.T) {
	// Scenario: Changed-file scope is measured from trusted snapshots
	repo := t.TempDir()
	runSCN504Git(t, repo, "init")
	runSCN504Git(t, repo, "config", "user.email", "test@example.com")
	runSCN504Git(t, repo, "config", "user.name", "Test User")
	writeSCN503File(t, filepath.Join(repo, "modified.txt"), "before\n")
	writeSCN503File(t, filepath.Join(repo, "deleted.txt"), "delete me\n")
	writeSCN503File(t, filepath.Join(repo, "renamed-from.txt"), "rename me\n")
	writeSCN503File(t, filepath.Join(repo, "working-tree-only.txt"), "snapshot content\n")
	runSCN504Git(t, repo, "add", ".")
	runSCN504Git(t, repo, "commit", "-m", "baseline")
	baseline := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))

	writeSCN503File(t, filepath.Join(repo, "modified.txt"), "after\n")
	if err := os.Remove(filepath.Join(repo, "deleted.txt")); err != nil {
		t.Fatalf("delete baseline file: %v", err)
	}
	runSCN504Git(t, repo, "mv", "renamed-from.txt", "renamed-to.txt")
	writeSCN503File(t, filepath.Join(repo, "added.txt"), "added\n")
	runSCN504Git(t, repo, "add", ".")
	runSCN504Git(t, repo, "commit", "-m", "review snapshot")
	snapshot := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))

	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), fmt.Sprintf("baseline: %s\nsnapshot: %s\n", baseline, snapshot))
	writeSCN503File(t, filepath.Join(repo, "working-tree-only.txt"), "must not be scoped\n")

	scope, err := ResolveChangedFileScope(repo, []string{"caller-supplied.txt"})
	if err != nil {
		t.Fatalf("resolve changed-file scope: %v", err)
	}
	if scope.Baseline != baseline || scope.Snapshot != snapshot {
		t.Errorf("comparison identities = %#v, want baseline %q and snapshot %q", scope, baseline, snapshot)
	}
	if !reflect.DeepEqual(scope.Changed, []string{"added.txt", "modified.txt"}) {
		t.Errorf("changed scope = %#v, want added and modified snapshot paths", scope.Changed)
	}
	if !reflect.DeepEqual(scope.Renamed, []ChangedFileRename{{From: "renamed-from.txt", To: "renamed-to.txt"}}) {
		t.Errorf("renamed scope = %#v, want recorded snapshot rename", scope.Renamed)
	}
	if !reflect.DeepEqual(scope.Deleted, []string{"deleted.txt"}) {
		t.Errorf("deleted scope = %#v, want recorded snapshot deletion", scope.Deleted)
	}
	if strings.Contains(strings.Join(scope.Changed, "\n"), "caller-supplied.txt") || strings.Contains(strings.Join(scope.Changed, "\n"), "working-tree-only.txt") {
		t.Errorf("scope used untrusted input or working-tree diff: %#v", scope)
	}

	persisted, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "changed-file-scope.yaml"))
	if err != nil {
		t.Fatalf("read persisted changed-file scope: %v", err)
	}
	for _, want := range []string{baseline, snapshot, "added.txt", "modified.txt", "renamed-from.txt", "renamed-to.txt", "deleted.txt"} {
		if !strings.Contains(string(persisted), want) {
			t.Errorf("persisted changed-file scope does not contain %q:\n%s", want, persisted)
		}
	}
}

func runSCN504Git(t *testing.T, repo string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
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
