package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ091BenchmarkInputPersistsThreeRecordsAndComparesThem(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	input := OutcomeBenchmarkInput{Format: OutcomeBenchmarkInputFormat, Runs: runs}
	result, paths, err := PersistAndCompareOutcomeBenchmark(worktree, ".rotta/benchmarks/req-091", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != OutcomeBenchmarkPassed || len(paths) != REQ091BenchmarkRunCount {
		t.Fatalf("result = %#v; paths = %#v", result, paths)
	}
	for index, path := range paths {
		contents, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		record, err := UnmarshalOutcomeRecord(contents)
		if err != nil || record.RunID != runs[index].RunID {
			t.Fatalf("persisted record %d = %#v, %v", index, record, err)
		}
	}
}

func TestREQ091BenchmarkRejectsSymlinkedRecordsDirectory(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(worktree, "records")); err != nil {
		t.Fatal(err)
	}
	_, _, err := PersistAndCompareOutcomeBenchmark(worktree, "records", OutcomeBenchmarkInput{Format: OutcomeBenchmarkInputFormat, Runs: runs})
	if err == nil || !strings.Contains(err.Error(), "worktree-local") {
		t.Fatalf("PersistAndCompareOutcomeBenchmark() error = %v", err)
	}
}

func TestREQ091BenchmarkRejectsTamperedPersistedRecord(t *testing.T) {
	_, runs := req091ComparableRuns(t)
	encoded, err := MarshalOutcomeRecord(runs[0])
	if err != nil {
		t.Fatal(err)
	}
	retained := append(encoded, '\n')
	hash := sha256.Sum256(retained)
	path := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(path, append(retained, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPersistedOutcomeBenchmarkRecord(path, hash[:]); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("loadPersistedOutcomeBenchmarkRecord() error = %v", err)
	}
	tampered, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tamperedHash := sha256.Sum256(tampered)
	if _, err := loadPersistedOutcomeBenchmarkRecord(path, tamperedHash[:]); err == nil || !strings.Contains(err.Error(), "bytes do not match") {
		t.Fatalf("loadPersistedOutcomeBenchmarkRecord() canonical-byte error = %v", err)
	}
}

func TestREQ091BenchmarkInputRejectsMissingTelemetryWithoutPersisting(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	runs[1].Tokens.Reasoning = NotObservableTelemetry("OpenCode export omits reasoning tokens")
	data, err := json.Marshal(OutcomeBenchmarkInput{Format: OutcomeBenchmarkInputFormat, Runs: runs})
	if err != nil {
		t.Fatal(err)
	}
	input, err := UnmarshalOutcomeBenchmarkInput(data)
	if err != nil {
		t.Fatal(err)
	}
	result, paths, err := PersistAndCompareOutcomeBenchmark(worktree, ".rotta/benchmarks/req-091", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != OutcomeBenchmarkNotEvaluable || !containsReason(result.Reasons, "reasoning_tokens") || len(paths) != 3 {
		t.Fatalf("result = %#v; paths = %#v", result, paths)
	}
}

func TestREQ091BenchmarkInputRejectsInvalidCountAndConfinement(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	data, err := json.Marshal(OutcomeBenchmarkInput{Format: OutcomeBenchmarkInputFormat, Runs: runs[:2]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalOutcomeBenchmarkInput(data); err == nil || !strings.Contains(err.Error(), "exactly 3") {
		t.Fatalf("UnmarshalOutcomeBenchmarkInput() error = %v", err)
	}
	_, _, err = PersistAndCompareOutcomeBenchmark(worktree, "../outside", OutcomeBenchmarkInput{Format: OutcomeBenchmarkInputFormat, Runs: runs})
	if err == nil || !strings.Contains(err.Error(), "escapes worktree") {
		t.Fatalf("PersistAndCompareOutcomeBenchmark() error = %v", err)
	}
}

func TestREQ091BenchmarkInputRejectsUnknownAndAmbiguousFields(t *testing.T) {
	data := `{"format":"rotta.workflow-benchmark-input/v1","FORMAT":"rotta.workflow-benchmark-input/v1","runs":[]}`
	if _, err := UnmarshalOutcomeBenchmarkInput([]byte(data)); err == nil || !strings.Contains(err.Error(), "ambiguous JSON field alias") {
		t.Fatalf("UnmarshalOutcomeBenchmarkInput() error = %v", err)
	}
}
