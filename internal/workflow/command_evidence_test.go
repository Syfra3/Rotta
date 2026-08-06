package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-085 → SCN-614 → TestSCN614_CapturesFullFailureEvidenceBeforeBoundedSummary
func TestSCN614_CapturesFullFailureEvidenceBeforeBoundedSummary(t *testing.T) {
	// Scenario: OpenCode context limits retain complete local failure evidence
	repo := t.TempDir()
	command := "i=0; while [ $i -lt 100 ]; do printf 'stdout-%03d\\n' \"$i\"; i=$((i+1)); done; printf 'failure-excerpt\\n' >&2; exit 7"
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree:  repo,
		Command:          []string{"sh", "-c", command},
		WorkingDirectory: repo,
	})
	if err != nil {
		t.Fatalf("CaptureLifecycleCommand() error = %v", err)
	}
	if report.Passing {
		t.Fatal("failing command was accepted as passing evidence")
	}
	if report.EvidencePath == "" || report.EvidenceHash == "" {
		t.Fatalf("evidence reference = %#v, want durable path and hash", report)
	}
	evidence, err := os.ReadFile(report.EvidencePath)
	if err != nil {
		t.Fatalf("read complete command evidence: %v", err)
	}
	for _, want := range []string{"stdout-000", "stdout-099", "failure-excerpt", `"command"`, `"working_directory"`, `"started_at"`, `"finished_at"`, `"exit_status": 7`, `"timed_out": false`, report.EvidenceHash} {
		if !strings.Contains(string(evidence), want) {
			t.Errorf("complete evidence missing %q:\n%s", want, evidence)
		}
	}
	if lines := strings.Count(report.ChatSummary, "\n") + 1; lines > 80 {
		t.Errorf("chat summary lines = %d, want at most 80", lines)
	}
	if bytes := len(report.ChatSummary); bytes > 8*1024 {
		t.Errorf("chat summary bytes = %d, want at most 8192", bytes)
	}
	for _, want := range []string{"truncated: true", "total_lines:", "total_bytes:", "exit_status: 7", "failure_excerpt: failure-excerpt", "evidence_path: " + report.EvidencePath, "evidence_hash: " + report.EvidenceHash} {
		if !strings.Contains(report.ChatSummary, want) {
			t.Errorf("chat summary missing %q:\n%s", want, report.ChatSummary)
		}
	}
}

// REQ-085 → SCN-614 → TestSCN614_LowerHostLimitRetainsEvidenceReference
func TestSCN614_LowerHostLimitRetainsEvidenceReference(t *testing.T) {
	// Scenario: OpenCode context limits retain complete local failure evidence
	repo := t.TempDir()
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree:  repo,
		Command:          []string{"sh", "-c", "printf 'one\\ntwo\\nthree\\n'"},
		WorkingDirectory: repo,
		HostMaxLines:     1,
		HostMaxBytes:     512,
	})
	if err != nil {
		t.Fatalf("CaptureLifecycleCommand() error = %v", err)
	}
	if !report.Passing {
		t.Fatal("successful command with durable evidence was not accepted")
	}
	if _, err := os.Stat(report.EvidencePath); err != nil {
		t.Fatalf("durable evidence missing before passing result: %v", err)
	}
	if lines := strings.Count(report.ChatSummary, "\n") + 1; lines > 1 {
		t.Errorf("chat summary lines = %d, want lower host limit of 1", lines)
	}
	if bytes := len(report.ChatSummary); bytes > 512 {
		t.Errorf("chat summary bytes = %d, want lower host limit of 512", bytes)
	}
	for _, want := range []string{"truncated: true", "total_lines: 3", "total_bytes:", "exit_status: 0", "failure_excerpt: none", "evidence_path: " + report.EvidencePath, "evidence_hash: " + report.EvidenceHash} {
		if !strings.Contains(report.ChatSummary, want) {
			t.Errorf("host-limited chat summary missing %q:\n%s", want, report.ChatSummary)
		}
	}
}

func TestREQ093_CaptureLifecycleCommandUsesOptionalRecordedRTKPresentation(t *testing.T) {
	repo := t.TempDir()
	record := RTKExecutableRecord{Status: RTKStatusSuccess, ExecutablePath: "/trusted/rtk", Version: "rtk 1.0", ExecutableHash: "same"}
	resolver := &fakeRTKResolver{executable: &fakeRTKExecutable{path: record.ExecutablePath, version: record.Version, hash: record.ExecutableHash}}
	filter := &fakeRTKFilter{output: "filtered chat view"}
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree: repo, Command: []string{"sh", "-c", "printf original-output"}, WorkingDirectory: repo,
		RTKPresentation: &RTKPresentation{StatePath: "fake-state", Loader: func(string) (RTKExecutableRecord, error) { return record, nil }, Resolver: resolver, Filter: filter},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ChatSummary != "filtered chat view" || !report.Passing || resolver.calls != 1 || filter.calls != 1 {
		t.Fatalf("presentation = %#v resolver=%d filter=%d", report, resolver.calls, filter.calls)
	}
	evidence, err := os.ReadFile(report.EvidencePath)
	if err != nil || !strings.Contains(string(evidence), "original-output") {
		t.Fatalf("durable underlying evidence = %q, %v", evidence, err)
	}
}

// REQ-085 → SCN-614 → TestSCN614_SerializationFailureDoesNotAuthorizePassing
func TestSCN614_SerializationFailureDoesNotAuthorizePassing(t *testing.T) {
	// Scenario: OpenCode context limits retain complete local failure evidence
	repo := t.TempDir()
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree:  repo,
		Command:          []string{"sh", "-c", "printf success"},
		WorkingDirectory: repo,
		CommandMetadata:  json.RawMessage(`{`),
	})
	if err == nil {
		t.Fatal("CaptureLifecycleCommand() error = nil, want serialization failure")
	}
	if report.Passing {
		t.Fatal("serialization failure after command success authorized passing")
	}
	entries, readErr := os.ReadDir(filepath.Join(repo, ".rotta", "current", "evidence"))
	if readErr != nil {
		t.Fatalf("read evidence directory: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("serialized evidence = %v, want none after serialization failure", entries)
	}
}

func TestCaptureLifecycleCommandRejectsSymlinkedEvidenceDirectory(t *testing.T) {
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".rotta", "current"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(repo, ".rotta", "current", "evidence")); err != nil {
		t.Fatal(err)
	}
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree: repo, Command: []string{"sh", "-c", "printf success"}, WorkingDirectory: repo,
	})
	if err == nil || report.Passing || !strings.Contains(err.Error(), "worktree-local directory") {
		t.Fatalf("CaptureLifecycleCommand() = %#v, %v; want rejected external evidence path", report, err)
	}
	entries, readErr := os.ReadDir(external)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("external evidence directory received data: %v", entries)
	}
}
