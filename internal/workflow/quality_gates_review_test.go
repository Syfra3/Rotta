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

// REQ-074 → SCN-506 → TestSCN506_AmbiguousStaticAnalysisConventionsBlockReviewWithoutExecution
func TestSCN506_AmbiguousStaticAnalysisConventionsBlockReviewWithoutExecution(t *testing.T) {
	// Scenario: Ambiguous command candidates block review
	repo := t.TempDir()
	writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), "baseline: baseline-sha\nsnapshot: snapshot-sha\nconventions:\n  static_analysis:\n    command: task lint\n  static_analysis:\n    command: make lint\n  security_checks:\n    command: task security\n")

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
	for _, want := range []string{"static-analysis", "ambiguous", "declare", "supported convention"} {
		if !strings.Contains(review.Result, want) {
			t.Errorf("review result %q does not contain %q", review.Result, want)
		}
	}
	if strings.Contains(review.Result, "task lint") || strings.Contains(review.Result, "make lint") {
		t.Errorf("review result selected a conflicting command: %q", review.Result)
	}
	if executed {
		t.Fatal("ambiguous static-analysis command was selected or executed")
	}
	if _, err := os.Stat(filepath.Join(repo, ".rotta", "current", "review-plan.yaml")); !os.IsNotExist(err) {
		t.Fatalf("ambiguous static-analysis commands persisted a selected review plan: %v", err)
	}
}

// REQ-075 → REQ-077 → SCN-507 → TestSCN507_SuccessfulGenericReviewWritesEvidenceAndEntersFinalHumanReview
func TestSCN507_SuccessfulGenericReviewWritesEvidenceAndEntersFinalHumanReview(t *testing.T) {
	// Scenario: Successful generic review writes evidence and enters final human review
	repo := t.TempDir()
	runSCN504Git(t, repo, "init")
	runSCN504Git(t, repo, "config", "user.email", "test@example.com")
	runSCN504Git(t, repo, "config", "user.name", "Test User")
	writeSCN503File(t, filepath.Join(repo, "tracked.txt"), "baseline\n")
	runSCN504Git(t, repo, "add", "tracked.txt")
	runSCN504Git(t, repo, "commit", "-m", "baseline")
	baseline := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))
	writeSCN503File(t, filepath.Join(repo, "tracked.txt"), "snapshot\n")
	runSCN504Git(t, repo, "add", "tracked.txt")
	runSCN504Git(t, repo, "commit", "-m", "snapshot")
	snapshot := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))

	writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ngate_order:\n  - build\n  - tests\n  - changed_file_scope\n  - static_analysis\n  - dependency_checks\n  - security_checks\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), fmt.Sprintf("baseline: %s\nsnapshot: %s\nconventions:\n  build:\n    command: task build\n  tests:\n    command: task test\n  changed_file_scope:\n    command: task scope\n  static_analysis:\n    command: task lint\n  dependency_checks:\n    command: task dependencies\n  security_checks:\n    command: task security\n", baseline, snapshot))
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), fmt.Sprintf("phase: review\nbaseline_commit: %s\ncompleted_scenarios: [SCN-501]\n", baseline))
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-501\nvalid current TDD evidence\n")

	var executed []string
	result, err := EvaluatePhase4Review(repo, func(command string) (string, error) {
		executed = append(executed, command)
		return "passed " + command, nil
	})
	if err != nil {
		t.Fatalf("evaluate Phase 4 review: %v", err)
	}
	if result.Readiness != "ready" || result.Snapshot != snapshot {
		t.Errorf("review result = %#v, want ready result for snapshot %q", result, snapshot)
	}
	if !reflect.DeepEqual(executed, []string{"task build", "task test", "task scope", "task lint", "task dependencies", "task security"}) {
		t.Errorf("executed commands = %#v, want all ordered generic gate commands", executed)
	}

	evidence, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "review-evidence.yaml"))
	if err != nil {
		t.Fatalf("read review evidence: %v", err)
	}
	for _, want := range append([]string{baseline, snapshot, result.ConfigurationFingerprint, result.PlanFingerprint, "overall_readiness: ready", "tracked.txt"}, executed...) {
		if !strings.Contains(string(evidence), want) {
			t.Errorf("review evidence does not contain %q:\n%s", want, evidence)
		}
	}
	evidenceText := string(evidence)
	lastOutcome := -1
	for index, category := range []string{"build", "tests", "changed_file_scope", "static_analysis", "dependency_checks", "security_checks"} {
		outcome := "category: " + category + "\n    status: passed\n    command: \"" + executed[index] + "\"\n    output: \"passed " + executed[index] + "\""
		outcomeIndex := strings.Index(evidenceText, outcome)
		if outcomeIndex <= lastOutcome {
			t.Errorf("review evidence does not record ordered passed %q outcome:\n%s", category, evidence)
		}
		lastOutcome = outcomeIndex
	}

	state, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if err != nil {
		t.Fatalf("read current state: %v", err)
	}
	for _, want := range []string{"phase: final_human_review", "reviewed_commit: " + snapshot, "overall_readiness: ready"} {
		if !strings.Contains(string(state), want) {
			t.Errorf("current state does not contain %q:\n%s", want, state)
		}
	}
	for _, forbidden := range []string{"phase: complete", "human_identity", "reviewer"} {
		if strings.Contains(string(state), forbidden) {
			t.Errorf("current state must not contain %q:\n%s", forbidden, state)
		}
	}
}

// REQ-075 → REQ-076 → SCN-509 → TestSCN509_FailedBuildGateWritesNotReadyEvidence
func TestSCN509_FailedBuildGateWritesNotReadyEvidence(t *testing.T) {
	// Scenario: A failed required gate produces not-ready evidence
	repo := t.TempDir()
	runSCN504Git(t, repo, "init")
	runSCN504Git(t, repo, "config", "user.email", "test@example.com")
	runSCN504Git(t, repo, "config", "user.name", "Test User")
	writeSCN503File(t, filepath.Join(repo, "tracked.txt"), "baseline\n")
	runSCN504Git(t, repo, "add", "tracked.txt")
	runSCN504Git(t, repo, "commit", "-m", "baseline")
	baseline := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))
	writeSCN503File(t, filepath.Join(repo, "tracked.txt"), "snapshot\n")
	runSCN504Git(t, repo, "add", "tracked.txt")
	runSCN504Git(t, repo, "commit", "-m", "snapshot")
	snapshot := strings.TrimSpace(runSCN504Git(t, repo, "rev-parse", "HEAD"))

	writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ngate_order:\n  - build\n  - tests\n  - changed_file_scope\n  - static_analysis\n  - dependency_checks\n  - security_checks\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), fmt.Sprintf("baseline: %s\nsnapshot: %s\nconventions:\n  build:\n    command: task build\n  tests:\n    command: task test\n  changed_file_scope:\n    command: task scope\n  static_analysis:\n    command: task lint\n  dependency_checks:\n    command: task dependencies\n  security_checks:\n    command: task security\n", baseline, snapshot))
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), fmt.Sprintf("phase: review\nbaseline_commit: %s\ncompleted_scenarios: [SCN-501]\n", baseline))
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-501\nvalid current TDD evidence\n")

	result, err := EvaluatePhase4Review(repo, func(command string) (string, error) {
		if command == "task build" {
			return "compile failed", fmt.Errorf("exit status 1")
		}
		t.Fatalf("executor ran %q after the failed build gate", command)
		return "", nil
	})
	if err != nil {
		t.Fatalf("evaluate Phase 4 review: %v", err)
	}
	if result.Readiness != "not_ready" || result.Snapshot != snapshot {
		t.Errorf("review result = %#v, want not_ready result for snapshot %q", result, snapshot)
	}

	evidence, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "review-evidence.yaml"))
	if err != nil {
		t.Fatalf("read review evidence: %v", err)
	}
	for _, want := range []string{"overall_readiness: not_ready", "category: build", "status: failed", "output: \"compile failed\"", "exit_result: \"exit status 1\"", "remediation:"} {
		if !strings.Contains(string(evidence), want) {
			t.Errorf("review evidence does not contain %q:\n%s", want, evidence)
		}
	}

	state, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if err != nil {
		t.Fatalf("read current state: %v", err)
	}
	for _, forbidden := range []string{"phase: final_human_review", "overall_readiness: ready", "reviewed_commit:"} {
		if strings.Contains(string(state), forbidden) {
			t.Errorf("current state must not contain %q:\n%s", forbidden, state)
		}
	}
}

// REQ-076 → SCN-510 → TestSCN510_ValidWaiverProducesReadyWithWaivers
func TestSCN510_ValidWaiverProducesReadyWithWaivers(t *testing.T) {
	// Scenario: A valid waiver remains visible and produces ready with waivers
	repo := t.TempDir()
	evidencePath := filepath.Join(repo, ".rotta", "current", "review-evidence.yaml")
	evidence := "format: rotta.review-evidence/v1\nsnapshot: \"abc123\"\nconfiguration_fingerprint: \"cfg-1\"\noverall_readiness: not_ready\ngates:\n  - category: build\n    status: passed\n  - category: tests\n    status: passed\n  - category: changed_file_scope\n    status: passed\n  - category: static_analysis\n    status: passed\n  - category: dependency_checks\n    status: failed\n    output: \"dependency audit failed\"\n    exit_result: \"exit status 1\"\n  - category: security_checks\n    status: passed\n"
	writeSCN503File(t, evidencePath, evidence)
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "waivers.yaml"), "format: rotta.review-waivers/v1\nwaivers:\n  - gate: dependency_checks\n    reason: \"Dependency advisory has a documented mitigation\"\n    scope: \"current pull request\"\n    timestamp: \"2026-08-04T12:00:00Z\"\n    snapshot: \"abc123\"\n    configuration_fingerprint: \"cfg-1\"\n")

	readiness, err := DerivePRReadiness(repo, "abc123", "cfg-1")
	if err != nil {
		t.Fatalf("derive PR readiness: %v", err)
	}
	if readiness.State != "ready_with_waivers" {
		t.Errorf("readiness state = %q, want ready_with_waivers", readiness.State)
	}
	if len(readiness.Gates) != 6 {
		t.Fatalf("derived gate count = %d, want 6", len(readiness.Gates))
	}
	if readiness.Gates[4].Status != "waived" || readiness.Gates[4].UnderlyingStatus != "failed" {
		t.Errorf("dependency-check gate = %#v, want waived status with failed underlying outcome", readiness.Gates[4])
	}
	if len(readiness.Waivers) != 1 || readiness.Waivers[0].Gate != "dependency_checks" || readiness.Waivers[0].Reason == "" {
		t.Errorf("derived waivers = %#v, want separate durable dependency-check waiver", readiness.Waivers)
	}

	persistedEvidence, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read persisted review evidence: %v", err)
	}
	if string(persistedEvidence) != evidence {
		t.Errorf("persisted evidence changed during waiver derivation:\n%s", persistedEvidence)
	}
	for _, forbidden := range []string{"reviewer", "identity", "actor"} {
		if strings.Contains(string(persistedEvidence), forbidden) {
			t.Errorf("persisted evidence must not contain %q:\n%s", forbidden, persistedEvidence)
		}
	}
}

// REQ-075 → SCN-508 → TestSCN508_RootAndArchivedTDDLogsDoNotSatisfyCurrentEvidence
func TestSCN508_RootAndArchivedTDDLogsDoNotSatisfyCurrentEvidence(t *testing.T) {
	// Scenario: Root and archived TDD logs cannot satisfy current review evidence
	repo := t.TempDir()
	writeSCN503File(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v2\ngate_order:\n  - build\n  - tests\n  - changed_file_scope\n  - static_analysis\n  - dependency_checks\n  - security_checks\ndiscovery:\n  supported_inputs:\n    - declared_project_metadata\n  rules:\n    - declared_convention_only\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "review-snapshot.yaml"), "baseline: baseline-sha\nsnapshot: snapshot-sha\nconventions:\n  build:\n    command: task build\n  tests:\n    command: task test\n  changed_file_scope:\n    command: task scope\n  static_analysis:\n    command: task lint\n  dependency_checks:\n    command: task dependencies\n  security_checks:\n    command: task security\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: review\nbaseline_commit: baseline-sha\ncompleted_scenarios: [SCN-501]\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "tdd-log.md"), "## SCN-501\nroot evidence\n")
	writeSCN503File(t, filepath.Join(repo, ".rotta", "archive", "prior", "tdd-log.md"), "## SCN-501\narchived evidence\n")

	executed := false
	result, err := EvaluatePhase4Review(repo, func(string) (string, error) {
		executed = true
		return "passed", nil
	})
	if err == nil || !strings.Contains(err.Error(), "current TDD evidence is missing SCN-501") {
		t.Fatalf("evaluate Phase 4 review error = %v, want missing current-submission evidence for SCN-501", err)
	}
	if executed {
		t.Fatal("root or archived TDD evidence authorized gate execution")
	}
	if result.Readiness == "ready" {
		t.Fatalf("review readiness = %q, root or archived TDD evidence must not report ready", result.Readiness)
	}
	if _, err := os.Stat(filepath.Join(repo, ".rotta", "current", "review-evidence.yaml")); !os.IsNotExist(err) {
		t.Fatalf("root or archived TDD evidence persisted ready review evidence: %v", err)
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
