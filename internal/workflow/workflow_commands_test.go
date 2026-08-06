package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowCommandsProduceBoundedDurableEvidence(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	request := workflowCommandRequest(baseline)

	preflight, err := RunWorkflowPreflight(repo, request)
	if err != nil {
		t.Fatalf("RunWorkflowPreflight() error = %v", err)
	}
	assertWorkflowCommandResult(t, preflight, "preflight", "passed")
	if len(preflight.Diagnostics) != 0 {
		t.Fatalf("preflight diagnostics = %v, want none", preflight.Diagnostics)
	}

	requireScopedVerificationSandbox(t, repo)
	verification, err := RunScopedVerification(repo, requestWithPackages(request, "./internal/demo"))
	if err != nil {
		t.Fatalf("RunScopedVerification() error = %v", err)
	}
	assertWorkflowCommandResult(t, verification, "scoped-verify", "passed")
	evidence, err := readRepositoryFile(repo, verification.EvidencePath)
	if err != nil {
		t.Fatalf("read scoped evidence: %v", err)
	}
	for _, want := range []string{`"command": [`, `/go"`, `"test"`, `"content_hash": "` + verification.EvidenceHash, `"command_metadata":`} {
		if !strings.Contains(string(evidence), want) {
			t.Errorf("durable scoped evidence missing %q:\n%s", want, evidence)
		}
	}

	publication, err := RunPublicationPlan(repo, request)
	if err != nil {
		t.Fatalf("RunPublicationPlan() error = %v", err)
	}
	assertWorkflowCommandResult(t, publication, "publication-plan", "passed")
	if !strings.Contains(publication.Remediation, "separate exact authority") {
		t.Fatalf("publication remediation = %q, want non-authorizing guidance", publication.Remediation)
	}
	status, err := gitSubmissionOutput(repo, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(status, "\n") {
		if line != "" && !isWorkflowCommandEvidenceStatus(line) {
			t.Fatalf("command changed protected worktree state: %q", status)
		}
	}
}

func TestREQ096_WorkflowCommandsUseInjectedTaskAdvisoryContextOnceWithinBudget(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	contents, err := readRepositoryFile(repo, "specs/contract.md")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	ancora := &fakeAncoraAdvisory{recovery: AncoraAdvisoryRecovery{References: []AncoraAdvisoryReference{{Path: "specs/contract.md", SHA256: hex.EncodeToString(sum[:])}}}}
	vela := &fakeVelaAdvisory{evidence: VelaEvidence{Subject: "internal/demo", Files: []string{"internal/demo/demo.go"}, Confidence: "medium", SafeAction: "inspect source"}}
	request := workflowCommandRequest(baseline)
	request.Advisory = NewWorkflowAdvisoryContext(repo, ancora, vela)
	request.VelaQuestions = []VelaQuestion{{Role: VelaExploreRole, Kind: VelaDependency, Subject: "internal/demo", Question: "What depends on demo?"}}
	for attempt := 0; attempt < maxVelaExploreCalls+1; attempt++ {
		result, err := RunWorkflowPreflight(repo, request)
		if err != nil {
			t.Fatalf("workflow preflight %d: %v", attempt, err)
		}
		if result.Advisory.Recovery.Source != "ancora" || len(result.Advisory.Vela) != 1 {
			t.Fatalf("workflow advisory result = %#v", result.Advisory)
		}
	}
	if ancora.recoverCalls != 1 || vela.calls != maxVelaExploreCalls {
		t.Fatalf("workflow advisory calls: Ancora=%d Vela=%d", ancora.recoverCalls, vela.calls)
	}
}

func TestScopedVerificationContainsTestWritesToDurableEvidenceOnly(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	requireScopedVerificationSandbox(t, repo)
	hostStatePath := filepath.Join(filepath.Dir(repo), "host-state.txt")
	mustWrite(t, filepath.Join(repo, "internal", "demo", "containment_test.go"), fmt.Sprintf(`package demo

import (
	"os"
	"testing"
)

func TestScopedVerificationCannotWriteRepository(t *testing.T) {
	if err := os.WriteFile("protected-state.txt", []byte("mutation"), 0o600); err == nil {
		t.Fatal("sandbox allowed a repository write")
	}
	if err := os.WriteFile(%q, []byte("ephemeral sandbox write"), 0o600); err != nil {
		t.Fatalf("sandbox did not provide an isolated ephemeral path: %%v", err)
	}
}
`, hostStatePath))
	if _, err := gitSubmissionOutput(repo, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "commit", "-m", "add containment test"); err != nil {
		t.Fatal(err)
	}

	result, err := RunScopedVerification(repo, requestWithPackages(workflowCommandRequest(baseline), "./internal/demo"))
	if err != nil {
		t.Fatalf("RunScopedVerification() error = %v", err)
	}
	assertWorkflowCommandResult(t, result, "scoped-verify", "passed")
	if _, err := os.Stat(filepath.Join(repo, "internal", "demo", "protected-state.txt")); !os.IsNotExist(err) {
		t.Fatalf("sandboxed Go test changed protected repository state: %v", err)
	}
	if _, err := os.Stat(hostStatePath); !os.IsNotExist(err) {
		t.Fatalf("sandboxed Go test changed protected host state: %v", err)
	}
	evidence, err := readRepositoryFile(repo, result.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`/bwrap"`, `"--ro-bind"`, `"--tmpfs"`, `/go"`, `"test"`} {
		if !strings.Contains(string(evidence), want) {
			t.Errorf("sandbox evidence missing %q:\n%s", want, evidence)
		}
	}
}

func TestScopedVerificationRefusesWhenBubblewrapIsUnavailable(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	original := resolveScopedVerificationSandbox
	resolveScopedVerificationSandbox = func() (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { resolveScopedVerificationSandbox = original })

	result, err := RunScopedVerification(repo, requestWithPackages(workflowCommandRequest(baseline), "./internal/demo"))
	if err == nil || result.Format != "" || !strings.Contains(err.Error(), "Bubblewrap containment") || !strings.Contains(err.Error(), "not started") {
		t.Fatalf("RunScopedVerification() = %#v, %v; want deterministic pre-execution containment refusal", result, err)
	}
}

func TestScopedVerificationDefaultIsInertDespiteValidLookingHostRTKState(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	stateHome := t.TempDir()
	marker := filepath.Join(t.TempDir(), "rtk-executed")
	fakeRTK := filepath.Join(t.TempDir(), "rtk")
	mustWrite(t, fakeRTK, fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker))
	if err := os.Chmod(fakeRTK, 0o700); err != nil {
		t.Fatal(err)
	}
	state := RTKExecutableRecord{Status: RTKStatusSuccess, ExecutablePath: fakeRTK, Version: "rtk 1", ExecutableHash: strings.Repeat("a", 64)}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateHome, "rotta"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateHome, "rotta", "rtk.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", stateHome)
	request := requestWithPackages(workflowCommandRequest(baseline), "./internal/demo")
	request.RTKPresentation = nil // assert the library default is inert, not host configured.
	_, _ = RunScopedVerification(repo, request)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("package workflow test executed host-looking RTK state: %v", err)
	}
}

func TestScopedVerificationRejectsPathInjectedBubblewrapBeforeExecution(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	fakeDirectory := t.TempDir()
	marker := filepath.Join(fakeDirectory, "host-marker")
	fakeBubblewrap := filepath.Join(fakeDirectory, "bwrap")
	mustWrite(t, fakeBubblewrap, fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker))
	if err := os.Chmod(fakeBubblewrap, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeDirectory+string(os.PathListSeparator)+"/usr/bin:/bin")

	original := resolveScopedVerificationSandbox
	resolveScopedVerificationSandbox = func() (string, error) { return exec.LookPath("bwrap") }
	t.Cleanup(func() { resolveScopedVerificationSandbox = original })

	result, err := RunScopedVerification(repo, requestWithPackages(workflowCommandRequest(baseline), "./internal/demo"))
	if err == nil || result.Format != "" || !strings.Contains(err.Error(), "trusted Bubblewrap containment") || !strings.Contains(err.Error(), "not started") {
		t.Fatalf("RunScopedVerification() = %#v, %v; want fake PATH Bubblewrap rejection before execution", result, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("untrusted PATH Bubblewrap executed and created host marker: %v", err)
	}
}

func TestCanonicalTrustedSystemBubblewrapRejectsNonTrustedAndSymlinkEscapePaths(t *testing.T) {
	fakeDirectory := t.TempDir()
	fakeBubblewrap := filepath.Join(fakeDirectory, "bwrap")
	mustWrite(t, fakeBubblewrap, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(fakeBubblewrap, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(fakeDirectory, "bwrap-link")
	if err := os.Symlink(fakeBubblewrap, symlink); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{fakeBubblewrap, symlink} {
		if _, err := canonicalTrustedSystemBubblewrap(path); err == nil || !strings.Contains(err.Error(), "non-trusted system location") || !strings.Contains(err.Error(), fakeBubblewrap) {
			t.Errorf("canonicalTrustedSystemBubblewrap(%q) error = %v, want non-trusted path rejection", path, err)
		}
	}
}

func TestWorkflowCommandsRejectDirtyDetachedCrossWorktreeAndStaleEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(t *testing.T, repo, baseline string, request WorkflowCommandRequest)
	}{
		{
			name: "dirty",
			run: func(t *testing.T, repo, _ string, request WorkflowCommandRequest) {
				t.Helper()
				mustWrite(t, filepath.Join(repo, "internal", "demo", "demo.go"), "package demo\n// dirty\n")
				result, err := RunWorkflowPreflight(repo, request)
				if !IsWorkflowCommandFailure(err) || result.Status != "failed" || !strings.Contains(strings.Join(result.Diagnostics, " "), "non-evidence") {
					t.Fatalf("dirty preflight = %#v, %v", result, err)
				}
			},
		},
		{
			name: "detached",
			run: func(t *testing.T, repo, _ string, request WorkflowCommandRequest) {
				t.Helper()
				if _, err := gitSubmissionOutput(repo, "checkout", "--detach"); err != nil {
					t.Fatal(err)
				}
				if _, err := RunWorkflowPreflight(repo, request); err == nil || !strings.Contains(err.Error(), "detached HEAD") {
					t.Fatalf("detached preflight error = %v", err)
				}
			},
		},
		{
			name: "cross worktree",
			run: func(t *testing.T, repo, _ string, request WorkflowCommandRequest) {
				t.Helper()
				sibling := filepath.Join(filepath.Dir(repo), "sibling")
				if _, err := gitSubmissionOutput(repo, "worktree", "add", "-b", "feature/sibling", sibling); err != nil {
					t.Fatal(err)
				}
				request.Worktree = filepath.ToSlash(filepath.Join("..", filepath.Base(sibling)))
				if _, err := RunWorkflowPreflight(repo, request); err == nil || !strings.Contains(err.Error(), "worktree") {
					t.Fatalf("cross-worktree preflight error = %v", err)
				}
			},
		},
		{
			name: "stale evidence",
			run: func(t *testing.T, repo, _ string, request WorkflowCommandRequest) {
				t.Helper()
				first, err := RunWorkflowPreflight(repo, request)
				if err != nil {
					t.Fatal(err)
				}
				request.Scope = []string{"other"}
				request.EvidencePath, request.EvidenceHash = first.EvidencePath, first.EvidenceHash
				result, err := RunWorkflowPreflight(repo, request)
				if !IsWorkflowCommandFailure(err) || result.Status != "failed" || !strings.Contains(strings.Join(result.Diagnostics, " "), "stale") {
					t.Fatalf("stale-evidence preflight = %#v, %v", result, err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline := workflowCommandRepository(t)
			test.run(t, repo, baseline, workflowCommandRequest(baseline))
		})
	}
}

func TestWorkflowHandoffValidationUsesCanonicalRecoveryAndRejectsMalformedMirror(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	snapshot, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	index := NewOrchestratorHandoffIndex(repo, nil)
	record := validHandoff(baseline, snapshot)
	record.Scope = []string{"internal/demo/"}
	if result, err := index.Record(record); err != nil || result.Blocked {
		t.Fatalf("seed handoff = %#v, %v", result, err)
	}
	request := workflowCommandRequest(baseline)
	request.HandoffID = "checkout"
	result, err := RunWorkflowHandoffValidation(repo, request)
	if err != nil {
		t.Fatalf("RunWorkflowHandoffValidation() error = %v", err)
	}
	assertWorkflowCommandResult(t, result, "handoff-validate", "passed")

	mustWrite(t, filepath.Join(repo, ".rotta", "handoffs", "bad.yaml"), "not: a handoff\n")
	result, err = RunWorkflowHandoffValidation(repo, request)
	if !IsWorkflowCommandFailure(err) || result.Status != "failed" || !strings.Contains(strings.Join(result.Diagnostics, " "), "malformed") {
		t.Fatalf("malformed handoff validation = %#v, %v", result, err)
	}
}

func TestWorkflowHandoffValidationBoundsCanonicalHandoffID(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	request := workflowCommandRequest(baseline)
	request.HandoffID = strings.Repeat("a", maxCanonicalHandoffIDBytes+1)
	result, err := RunWorkflowHandoffValidation(repo, request)
	if err == nil || result.Format != "" || !strings.Contains(err.Error(), "at most 128 bytes") {
		t.Fatalf("RunWorkflowHandoffValidation() = %#v, %v; want oversized handoff rejection", result, err)
	}
}

func TestWorkflowHandoffValidationPersistsMaximumCanonicalHandoffID(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	snapshot, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	taskID := strings.Repeat("a", maxCanonicalHandoffIDBytes-2)
	record := validHandoff(baseline, snapshot)
	record.HandoffID, record.Scope = taskID+"/1", []string{"internal/demo/"}
	if result, err := NewOrchestratorHandoffIndex(repo, nil).Record(record); err != nil || result.Blocked {
		t.Fatalf("seed maximum handoff = %#v, %v", result, err)
	}
	request := workflowCommandRequest(baseline)
	request.HandoffID = taskID
	result, err := RunWorkflowHandoffValidation(repo, request)
	if err != nil {
		t.Fatalf("RunWorkflowHandoffValidation() error = %v", err)
	}
	assertWorkflowCommandResult(t, result, "handoff-validate", "passed")
	if got := len(result.CanonicalInputs.HandoffID); got != maxCanonicalHandoffIDBytes-2 {
		t.Fatalf("persisted canonical handoff length = %d, want %d", got, maxCanonicalHandoffIDBytes-2)
	}
	evidence, err := readRepositoryFile(repo, result.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) > 8*1024 || !strings.Contains(string(evidence), taskID) {
		t.Fatalf("durable evidence is not bounded/canonical: %d bytes", len(evidence))
	}
}

func TestWorkflowEvidenceRejectsTamperedPersistedOutput(t *testing.T) {
	repo, baseline := workflowCommandRepository(t)
	request := workflowCommandRequest(baseline)
	first, err := RunWorkflowPreflight(repo, request)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := readRepositoryFile(repo, first.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var evidence lifecycleCommandEvidence
	if err := json.Unmarshal(contents, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence.Stdout = "tampered durable command output"
	tampered, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, first.EvidencePath), tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	request.EvidencePath, request.EvidenceHash = first.EvidencePath, first.EvidenceHash
	result, err := RunWorkflowPreflight(repo, request)
	if !IsWorkflowCommandFailure(err) || result.Status != "failed" || !strings.Contains(strings.Join(result.Diagnostics, " "), "persisted stdout/stderr") {
		t.Fatalf("tampered-evidence preflight = %#v, %v", result, err)
	}
}

func workflowCommandRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		if _, err := gitSubmissionOutput(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/workflow-command\n\ngo 1.25.0\n")
	mustWrite(t, filepath.Join(repo, "specs", "contract.md"), "# contract\n")
	mustWrite(t, filepath.Join(repo, "internal", "demo", "demo.go"), "package demo\n")
	mustWrite(t, filepath.Join(repo, "other", "scope.go"), "package other\n")
	if _, err := gitSubmissionOutput(repo, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "commit", "-m", "baseline"); err != nil {
		t.Fatal(err)
	}
	baseline, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return repo, baseline
}

func workflowCommandRequest(baseline string) WorkflowCommandRequest {
	return WorkflowCommandRequest{Worktree: ".", Feature: "workflow-command", ContractPath: "specs/contract.md", Baseline: baseline, Scope: []string{"internal/demo"}, RTKPresentation: DisabledRTKPresentation()}
}

func requestWithPackages(request WorkflowCommandRequest, packages ...string) WorkflowCommandRequest {
	request.VerificationPkgs = packages
	return request
}

func requireScopedVerificationSandbox(t *testing.T, root string) {
	t.Helper()
	if _, err := scopedVerificationSandboxCommand(root, []string{"./internal/demo"}); err != nil {
		t.Skipf("scoped verification containment unavailable on this platform: %v", err)
	}
}

func assertWorkflowCommandResult(t *testing.T, result WorkflowCommandResult, command, status string) {
	t.Helper()
	if result.Format != WorkflowCommandFormat || result.Command != command || result.Status != status || result.EvidencePath == "" || len(result.EvidenceHash) != 64 {
		t.Fatalf("workflow result = %#v", result)
	}
	if len(result.Diagnostics) > maxWorkflowCommandDiagnostics {
		t.Fatalf("diagnostics = %v, want bounded", result.Diagnostics)
	}
	capsule, ok := result.CompactCapsule()
	if !ok {
		t.Fatalf("workflow result lacks its production compact capsule: %#v", result)
	}
	view := compactEvidenceView(t, capsule)
	if view.Evidence.Path != result.EvidencePath || view.Evidence.Hash != result.EvidenceHash {
		t.Fatalf("workflow capsule evidence = %#v, want %s#%s", view.Evidence, result.EvidencePath, result.EvidenceHash)
	}
	if len(result.Remediation) > 512 {
		t.Fatalf("remediation exceeds output bound: %d", len(result.Remediation))
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 8*1024 {
		t.Fatalf("compact result encoding = %d bytes, %v", len(encoded), err)
	}
}
