package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/workflow"
)

func TestWorkflowPreflightCLIEmitsVersionedJSONAndBoundedHumanResult(t *testing.T) {
	repo, baseline := workflowCLITestRepository(t)
	t.Chdir(repo)
	args := []string{"workflow", "preflight", "--worktree", ".", "--feature", "workflow-command", "--contract", "specs/contract.md", "--baseline", baseline, "--scope", "internal/demo"}
	var stdout, stderr bytes.Buffer
	if err := runCLI(args, &stdout, &stderr); err != nil {
		t.Fatalf("runCLI(%v) error = %v; stderr = %s", args, err, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("workflow output lines = %d, want JSON plus concise human result: %q", len(lines), stdout.String())
	}
	var result struct {
		Format       string `json:"format"`
		Command      string `json:"command"`
		Status       string `json:"status"`
		EvidencePath string `json:"evidence_path"`
		EvidenceHash string `json:"evidence_hash"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("first workflow output line is not JSON: %v\n%s", err, lines[0])
	}
	if result.Format != "rotta.workflow-command/v1" || result.Command != "preflight" || result.Status != "passed" || result.EvidencePath == "" || len(result.EvidenceHash) != 64 {
		t.Fatalf("workflow JSON result = %#v", result)
	}
	if !strings.Contains(lines[1], "preflight: passed") || !strings.Contains(lines[1], result.EvidencePath+"#"+result.EvidenceHash) || len(lines[1]) > 1024 {
		t.Fatalf("human workflow result is not concise/canonical: %q", lines[1])
	}
}

func TestREQ096_WorkflowCLIConstructsOneSafeTaskScopedAdvisoryContext(t *testing.T) {
	repo, baseline := workflowCLITestRepository(t)
	t.Chdir(repo)
	args := []string{"workflow", "preflight", "--worktree", ".", "--feature", "workflow-command", "--contract", "specs/contract.md", "--baseline", baseline, "--scope", "internal/demo"}
	var stdout, stderr bytes.Buffer
	if err := runCLI(args, &stdout, &stderr); err != nil {
		t.Fatalf("runCLI(%v) error = %v; stderr = %s", args, err, stderr.String())
	}
	var result struct {
		Advisory struct {
			Recovery struct {
				Source      string `json:"source"`
				Degraded    bool   `json:"degraded"`
				EvidenceGap string `json:"EvidenceGap"`
			} `json:"recovery"`
		} `json:"advisory"`
	}
	line, _, _ := strings.Cut(stdout.String(), "\n")
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		t.Fatalf("unmarshal workflow result: %v", err)
	}
	if result.Advisory.Recovery.Source != "workspace_git" || !result.Advisory.Recovery.Degraded || !strings.Contains(result.Advisory.Recovery.EvidenceGap, "unavailable") {
		t.Fatalf("CLI advisory did not use the safe no-integration fallback: %#v", result.Advisory)
	}
}

func TestWorkflowCLIRejectsAbsoluteAndTraversalPaths(t *testing.T) {
	repo, baseline := workflowCLITestRepository(t)
	t.Chdir(repo)
	base := []string{"workflow", "preflight", "--worktree", ".", "--feature", "workflow-command", "--contract", "specs/contract.md", "--baseline", baseline, "--scope", "internal/demo"}
	for _, test := range []struct {
		name   string
		modify func([]string) []string
	}{
		{"absolute contract", func(args []string) []string { args[7] = filepath.Join(repo, "specs", "contract.md"); return args }},
		{"traversal scope", func(args []string) []string { args[len(args)-1] = "../outside"; return args }},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string(nil), base...)
			err := runCLI(test.modify(args), &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "path") && !strings.Contains(err.Error(), "scope") {
				t.Fatalf("runCLI(%v) error = %v, want confined-path rejection", args, err)
			}
		})
	}
}

func TestWorkflowCLIOutputIsBoundedAtMaximumCanonicalHandoffID(t *testing.T) {
	result := workflow.WorkflowCommandResult{
		Format: "rotta.workflow-command/v1", Command: "handoff-validate", Status: "passed",
		CanonicalInputs: workflow.WorkflowCommandInputs{HandoffID: strings.Repeat("a", 126)},
		EvidencePath:    ".rotta/current/evidence/command.json", EvidenceHash: strings.Repeat("0", 64),
		Remediation: "continue from the validated handoff evidence",
	}
	var stdout bytes.Buffer
	if err := writeWorkflowCommandResult(&stdout, result); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || len(lines[0]) > 8*1024 || len(lines[1]) > 1024 {
		t.Fatalf("maximum-handoff output is unbounded: JSON=%d human=%d", len(lines[0]), len(lines[1]))
	}
	if !strings.Contains(lines[0], result.CanonicalInputs.HandoffID) || !strings.Contains(lines[1], result.EvidenceHash) {
		t.Fatalf("maximum-handoff output omitted canonical data: %q", stdout.String())
	}
}

func TestREQ091BenchmarkCLIParsesCanonicalInputPersistsRecordsAndReportsNotEvaluable(t *testing.T) {
	worktree := t.TempDir()
	input := workflow.OutcomeBenchmarkInput{Format: workflow.OutcomeBenchmarkInputFormat, Runs: []workflow.WorkflowOutcomeRecord{
		benchmarkCLIRecord("one"), benchmarkCLIRecord("two"), benchmarkCLIRecord("three"),
	}}
	contents, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "input.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(worktree)
	var stdout, stderr bytes.Buffer
	args := []string{"workflow", "benchmark", "--worktree", ".", "--input", "input.json"}
	if err := runCLI(args, &stdout, &stderr); err != nil {
		t.Fatalf("runCLI: %v; %s", err, stderr.String())
	}
	var result struct {
		Status  string   `json:"status"`
		Records []string `json:"records"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || len(result.Records) != 3 {
		t.Fatalf("benchmark output = %s", stdout.String())
	}
	for _, path := range result.Records {
		if _, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(path))); err != nil {
			t.Fatalf("persisted record %q: %v", path, err)
		}
	}
}

func TestREQ091BenchmarkCLIRejectsTraversalAndInvalidInput(t *testing.T) {
	worktree := t.TempDir()
	t.Chdir(worktree)
	if err := os.WriteFile(filepath.Join(worktree, "input.json"), []byte(`{"format":"rotta.workflow-benchmark-input/v1","runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"workflow", "benchmark", "--worktree", ".", "--input", "../input.json"},
		{"workflow", "benchmark", "--worktree", ".", "--input", "input.json"},
	} {
		err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil {
			t.Fatalf("runCLI(%v) unexpectedly succeeded", args)
		}
	}
}

func TestREQ091BenchmarkCLIRejectsSymlinkedInputOutsideWorktree(t *testing.T) {
	worktree := t.TempDir()
	external := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(external, []byte(`{"format":"rotta.workflow-benchmark-input/v1","runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(worktree, "input.json")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(worktree)
	err := runCLI([]string{"workflow", "benchmark", "--worktree", ".", "--input", "input.json"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "escapes worktree") {
		t.Fatalf("runCLI() error = %v", err)
	}
}

func TestREQ091BenchmarkCLIRejectsSymlinkedWorktreeOutsideInvokingRoot(t *testing.T) {
	invokingRoot := t.TempDir()
	externalWorktree := t.TempDir()
	if err := os.Symlink(externalWorktree, filepath.Join(invokingRoot, "outside")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(invokingRoot)
	err := runCLI([]string{"workflow", "benchmark", "--worktree", "outside", "--input", "input.json"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "escapes worktree") {
		t.Fatalf("runCLI() error = %v", err)
	}
}

func benchmarkCLIRecord(suffix string) workflow.WorkflowOutcomeRecord {
	metric := func(value uint64, source string) workflow.TelemetryMetric {
		return workflow.ObservedTelemetry(value, source)
	}
	return workflow.WorkflowOutcomeRecord{
		Format: "rotta.workflow-outcome/v1", RunID: "run-" + suffix, RootSessionID: "root-" + suffix,
		RunIdentity:   workflow.OutcomeRunIdentity{FeatureID: "req-091", FeatureRequestFingerprint: "request", ContractFingerprint: "contract", PolicyFingerprint: "policy", RepositoryBaseline: "baseline", ProviderIdentifier: "openai", ModelIdentifier: "openai/gpt-5.6-terra", ModelFamily: "gpt-5.6", EnabledIntegrations: []string{}, OperationalPermissions: []string{}, AcceptanceChecks: []string{"go test ./internal/workflow"}},
		ChildSessions: metric(1, "opencode.session.children"), RoleInvocations: map[string]workflow.TelemetryMetric{},
		Tokens:         workflow.OutcomeTokens{Input: metric(1, "opencode.usage.input"), Output: metric(1, "opencode.usage.output"), Reasoning: metric(1, "opencode.usage.reasoning")},
		CacheTokens:    workflow.OutcomeCacheTokens{Read: workflow.NotObservableTelemetry("OpenCode does not expose cache read tokens"), Write: workflow.NotObservableTelemetry("OpenCode does not expose cache write tokens")},
		HumanDecisions: metric(0, "workflow.human_decisions"), Continuations: metric(0, "workflow.continuations"), CorrectionCycles: metric(0, "workflow.correction_cycles"), DeterministicCommands: metric(1, "workflow.commands"),
		DeterministicValidation: workflow.DeterministicValidationOutcome{Status: workflow.OutcomeNotObservable},
		IndependentFinalReview:  workflow.IndependentFinalReviewOutcome{Status: workflow.OutcomeNotObservable},
	}
}

func workflowCLITestRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		workflowCLIGit(t, repo, args...)
	}
	for path, contents := range map[string]string{
		"go.mod":                    "module example.com/workflow-cli\n\ngo 1.25.0\n",
		"specs/contract.md":         "# contract\n",
		"internal/demo/demo.go":     "package demo\n",
		"internal/demo/ignored.txt": "fixture\n",
	} {
		fullPath := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	workflowCLIGit(t, repo, "add", ".")
	workflowCLIGit(t, repo, "commit", "-m", "baseline")
	return repo, workflowCLIGit(t, repo, "rev-parse", "HEAD")
}

func workflowCLIGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
