package workflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ093_CompactEvidenceContainsOnlyCanonicalReferencesAndExposedSizes(t *testing.T) {
	promptTokens := uint64(31)
	result, err := NewCompactEvidenceResult(compactEvidenceInput(promptTokens))
	if err != nil {
		t.Fatal(err)
	}
	view := compactEvidenceView(t, result)
	if view.PromptTokens.Status != TokenSizeObserved || view.PromptTokens.Tokens == nil || *view.PromptTokens.Tokens != promptTokens {
		t.Fatalf("prompt measurement = %#v", view.PromptTokens)
	}
	if view.CapsuleTokens.Status != TokenSizeNotObservable || view.CapsuleTokens.Tokens != nil {
		t.Fatalf("unexposed capsule measurement = %#v", view.CapsuleTokens)
	}
	if len(view.CompactionEvidence) != 1 || view.Evidence.Path != view.CanonicalOutcome.EvidencePath {
		t.Fatalf("compact evidence result lost a durable reference: %#v", view)
	}
}

func TestREQ093_CompactEvidenceRejectsMismatchedAuthority(t *testing.T) {
	input := compactEvidenceInput(1)
	input.Evidence.Path = ".rotta/current/evidence/b.json"
	if _, err := NewCompactEvidenceResult(input); err == nil || !strings.Contains(err.Error(), "canonical outcome") {
		t.Fatalf("NewCompactEvidenceResult() error = %v, want authority mismatch", err)
	}
}

func TestREQ093_CompactEvidenceRejectsNestedCapsule(t *testing.T) {
	input := compactEvidenceInput(1)
	nested, err := NewCompactEvidenceResult(compactEvidenceInput(1))
	if err != nil {
		t.Fatal(err)
	}
	input.CanonicalOutcome.compactCapsule = nested
	if _, err := NewCompactEvidenceResult(input); err == nil || !strings.Contains(err.Error(), "must not embed") {
		t.Fatalf("nested capsule error = %v", err)
	}
}

func TestREQ093_CompactEvidenceRejectsOversizedOrRawCapsuleFields(t *testing.T) {
	for name, mutate := range map[string]func(*CompactEvidenceInput){
		"canonical outcome": func(input *CompactEvidenceInput) {
			input.CanonicalOutcome.Remediation = strings.Repeat("x", maxCompactCapsuleTextBytes+1)
		},
		"changed paths":   func(input *CompactEvidenceInput) { input.ChangedPaths = make([]string, maxCompactCapsulePaths+1) },
		"scope":           func(input *CompactEvidenceInput) { input.Scope = make([]string, maxCompactCapsuleScope+1) },
		"risk raw log":    func(input *CompactEvidenceInput) { input.Risk = "line one\nraw log line" },
		"ansi raw log":    func(input *CompactEvidenceInput) { input.Risk = "\x1b[31mfailed\x1b[0m" },
		"single line log": func(input *CompactEvidenceInput) { input.Risk = "time=2026-08-06T12:00:00Z level=error msg=failed" },
		"invalid utf8":    func(input *CompactEvidenceInput) { input.Remediation = string([]byte{0xff}) },
		"remediation": func(input *CompactEvidenceInput) {
			input.Remediation = strings.Repeat("x", maxCompactCapsuleTextBytes+1)
		},
		"evidence refs": func(input *CompactEvidenceInput) {
			input.CompactionEvidence = make([]DurableEvidenceReference, maxCompactEvidenceRefs+1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := compactEvidenceInput(1)
			mutate(&input)
			if _, err := NewCompactEvidenceResult(input); err == nil {
				t.Fatal("oversized or raw capsule input was accepted")
			}
		})
	}
	tooLarge := maxObservedTokenCount + 1
	if _, err := NewCompactEvidenceResult(compactEvidenceInput(tooLarge)); err == nil {
		t.Fatal("unbounded observed token count was accepted")
	}
}

func TestREQ093_CompactEvidenceRejectsSingleLineRawDataInEveryTextBearingField(t *testing.T) {
	for name, mutate := range map[string]func(*CompactEvidenceInput){
		"arbitrary risk payload": func(input *CompactEvidenceInput) { input.Risk = "the command printed a single line" },
		"secret remediation":     func(input *CompactEvidenceInput) { input.Remediation = "api_key=secret-value" },
		"raw command output diagnostic": func(input *CompactEvidenceInput) {
			input.CanonicalOutcome.Diagnostics = []string{"fatal: command output"}
		},
		"log-like diagnostic": func(input *CompactEvidenceInput) {
			input.CanonicalOutcome.Diagnostics = []string{"time=2026-08-06 level=error msg=failed"}
		},
		"secret in canonical remediation": func(input *CompactEvidenceInput) { input.CanonicalOutcome.Remediation = "token=secret-value" },
		"secret as changed path":          func(input *CompactEvidenceInput) { input.ChangedPaths = []string{"api_key=secret-value"} },
		"arbitrary evidence check":        func(input *CompactEvidenceInput) { input.CompactionEvidence[0].Check = "curl https://example.invalid" },
		"raw data as evidence path":       func(input *CompactEvidenceInput) { input.Evidence.Path = "stdout: secret-value" },
	} {
		t.Run(name, func(t *testing.T) {
			input := compactEvidenceInput(1)
			mutate(&input)
			if _, err := NewCompactEvidenceResult(input); err == nil {
				t.Fatal("single-line raw data was accepted into a compact capsule")
			}
		})
	}

	result := compactEvidenceInput(1).CanonicalOutcome
	result.Diagnostics = []string{"api_key=secret-value"}
	presented := compactWorkflowPresentation(result)
	capsule, ok := presented.CompactCapsule()
	if !ok {
		t.Fatalf("raw compact data did not produce a bounded capsule omission diagnostic: %#v", presented)
	}
	view := compactEvidenceView(t, capsule)
	if len(view.CanonicalOutcome.Diagnostics) != 1 || view.CanonicalOutcome.Diagnostics[0] != compactDiagnosticsOmitted {
		t.Fatalf("raw compact data did not produce a bounded capsule omission diagnostic: %#v", presented)
	}
	serialized, err := json.Marshal(capsule)
	if err != nil || strings.Contains(string(serialized), "secret-value") {
		t.Fatalf("raw compact data entered a capsule: %s, %v", serialized, err)
	}
}

func TestREQ093_CompactEvidenceRejectsAggregateOversize(t *testing.T) {
	input := compactEvidenceInput(1)
	input.ChangedPaths = make([]string, maxCompactCapsulePaths)
	for index := range input.ChangedPaths {
		input.ChangedPaths[index] = strings.Repeat("p", maxCompactCapsuleTextBytes)
	}
	if _, err := NewCompactEvidenceResult(input); err == nil || !strings.Contains(err.Error(), "aggregate") {
		t.Fatalf("aggregate capsule oversize error = %v", err)
	}
}

func TestREQ093_CompactEvidenceFactoryCopiesMutableInputs(t *testing.T) {
	input := compactEvidenceInput(1)
	result, err := NewCompactEvidenceResult(input)
	if err != nil {
		t.Fatal(err)
	}
	input.CanonicalOutcome.CanonicalInputs.Scope[0] = "api_key=secret-value"
	input.ChangedPaths[0] = "raw command output"
	input.Scope[0] = "token=secret-value"
	input.CompactionEvidence[0].Path = "stdout: secret-value"
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"secret-value", "raw command output", "stdout:"} {
		if strings.Contains(string(serialized), raw) {
			t.Fatalf("mutated caller input entered a compact capsule: %s", serialized)
		}
	}
}

func TestREQ093_WorkflowPresentationBuildsCapsuleOrReportsBoundedRejection(t *testing.T) {
	result := compactWorkflowPresentation(compactEvidenceInput(1).CanonicalOutcome)
	capsule, ok := result.CompactCapsule()
	if !ok || compactEvidenceView(t, capsule).Evidence.Path != result.EvidencePath {
		t.Fatalf("production workflow result did not carry compact capsule: %#v", result)
	}
	oversized := compactEvidenceInput(1).CanonicalOutcome
	oversized.CanonicalInputs.Scope = make([]string, maxCompactCapsuleScope)
	for index := range oversized.CanonicalInputs.Scope {
		oversized.CanonicalInputs.Scope[index] = strings.Repeat("p", maxCompactCapsuleTextBytes)
	}
	oversized.Diagnostics = make([]string, maxCompactDiagnostics)
	for index := range oversized.Diagnostics {
		oversized.Diagnostics[index] = strings.Repeat("x", maxCompactCapsuleTextBytes)
	}
	oversized = compactWorkflowPresentation(oversized)
	if _, ok := oversized.CompactCapsule(); ok || !strings.Contains(strings.Join(oversized.Diagnostics, " "), "compact capsule omitted") || len(oversized.Diagnostics) > maxCompactDiagnostics {
		t.Fatalf("oversized production capsule was not transparently rejected: %#v", oversized)
	}
}

func compactEvidenceView(t *testing.T, result CompactEvidenceResult) compactEvidenceJSON {
	t.Helper()
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var view compactEvidenceJSON
	if err := json.Unmarshal(serialized, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func TestREQ093_RTKPresentationLoadsRecordedStateAndPreservesFallback(t *testing.T) {
	record := RTKExecutableRecord{Status: RTKStatusSuccess, ExecutablePath: "/trusted/rtk", Version: "rtk 1.0", ExecutableHash: "same"}
	report := LifecycleCommandReport{EvidencePath: ".rotta/current/evidence/result.json", EvidenceHash: strings.Repeat("c", 64), ChatSummary: "unfiltered bounded result", Passing: true}
	resolver := &fakeRTKResolver{executable: &fakeRTKExecutable{path: "/trusted/rtk", version: "rtk 1.0", hash: "same"}}
	filter := &fakeRTKFilter{output: "filtered view"}
	presentation := RTKPresentation{StatePath: "state.json", Loader: func(path string) (RTKExecutableRecord, error) {
		if path != "state.json" {
			t.Fatalf("state path = %q", path)
		}
		return record, nil
	}, Resolver: resolver, Filter: filter}
	filtered := presentation.Present(report, report.ChatSummary)
	if !filtered.UsedRTK || filtered.Output != "filtered view" || filtered.EvidencePath != report.EvidencePath || filtered.EvidenceHash != report.EvidenceHash || resolver.calls != 1 || filter.calls != 1 {
		t.Fatalf("filtered presentation = %#v resolver=%d filter=%d", filtered, resolver.calls, filter.calls)
	}
	for _, test := range []struct {
		name     string
		loader   func(string) (RTKExecutableRecord, error)
		resolver *fakeRTKResolver
		filter   *fakeRTKFilter
	}{
		{"missing state", func(string) (RTKExecutableRecord, error) { return RTKExecutableRecord{}, errRTKUnavailable }, resolver, filter},
		{"skipped state", func(string) (RTKExecutableRecord, error) { return RTKExecutableRecord{Status: RTKStatusSkipped}, nil }, resolver, filter},
		{"version mismatch", func(string) (RTKExecutableRecord, error) { return record, nil }, &fakeRTKResolver{executable: &fakeRTKExecutable{path: record.ExecutablePath, version: "rtk 2.0", hash: record.ExecutableHash}}, filter},
		{"fingerprint mismatch", func(string) (RTKExecutableRecord, error) { return record, nil }, &fakeRTKResolver{executable: &fakeRTKExecutable{path: record.ExecutablePath, version: record.Version, hash: "replaced"}}, filter},
		{"filter failure", func(string) (RTKExecutableRecord, error) { return record, nil }, resolver, &fakeRTKFilter{err: errRTKUnavailable}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fallback := (RTKPresentation{StatePath: "state.json", Loader: test.loader, Resolver: test.resolver, Filter: test.filter}).Present(report, report.ChatSummary)
			if fallback.UsedRTK || fallback.Output != report.ChatSummary || fallback.EvidencePath != report.EvidencePath || fallback.EvidenceHash != report.EvidenceHash {
				t.Fatalf("fallback = %#v", fallback)
			}
		})
	}
}

func TestREQ093_RTKStateReaderRejectsOversizedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtk.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxRTKStateBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRTKExecutableRecord(path); err == nil {
		t.Fatal("oversized RTK state was accepted")
	}
}

func compactEvidenceInput(tokens uint64) CompactEvidenceInput {
	return CompactEvidenceInput{
		CanonicalOutcome:   WorkflowCommandResult{Format: WorkflowCommandFormat, Command: "scoped-verify", Status: "failed", EvidencePath: ".rotta/current/evidence/check.json", EvidenceHash: strings.Repeat("a", 64), Remediation: "correct the in-scope failure and rerun scoped verification", CanonicalInputs: WorkflowCommandInputs{Worktree: ".", Feature: "slice-four", ContractPath: "specs/contract.md", Baseline: strings.Repeat("b", 40), Scope: []string{"internal/workflow"}}},
		Evidence:           DurableEvidenceReference{Check: "scoped-verify", Path: ".rotta/current/evidence/check.json", Hash: strings.Repeat("a", 64), Status: OutcomeFailed},
		ChangedPaths:       []string{"internal/workflow/command_evidence.go"},
		Scope:              []string{"internal/workflow"},
		Risk:               compactEvidenceRisk,
		Remediation:        "correct the in-scope failure and rerun scoped verification",
		CompactionEvidence: []DurableEvidenceReference{{Check: "tool-output-pruning", Path: ".rotta/current/evidence/pruning.json", Hash: strings.Repeat("b", 64), Status: OutcomePassed}},
		PromptTokens:       &tokens,
	}
}

type fakeRTKResolver struct {
	executable RTKExecutable
	err        error
	calls      int
}

func (f *fakeRTKResolver) Resolve(string) (RTKExecutable, error) {
	f.calls++
	return f.executable, f.err
}

type fakeRTKExecutable struct {
	path, version, hash string
	versionErr, hashErr error
	closed              bool
}

func (f *fakeRTKExecutable) Path() string                 { return f.path }
func (f *fakeRTKExecutable) Version() (string, error)     { return f.version, f.versionErr }
func (f *fakeRTKExecutable) Fingerprint() (string, error) { return f.hash, f.hashErr }
func (f *fakeRTKExecutable) Run([]string, string, int) (string, error) {
	return "", errors.New("not used by fake filter")
}
func (f *fakeRTKExecutable) Close() error { f.closed = true; return nil }

type fakeRTKFilter struct {
	output string
	err    error
	calls  int
}

func (f *fakeRTKFilter) Filter(RTKExecutable, string) (string, error) {
	f.calls++
	return f.output, f.err
}

type rtkTestError string

func (e rtkTestError) Error() string { return string(e) }

const errRTKUnavailable = rtkTestError("rtk unavailable")

func TestREQ093_LoadRTKExecutableRecordRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtk.json")
	want := RTKExecutableRecord{Status: RTKStatusSkipped, FailureReason: "not selected"}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRTKExecutableRecord(path)
	if err != nil || got != want {
		t.Fatalf("state = %#v, %v", got, err)
	}
}
