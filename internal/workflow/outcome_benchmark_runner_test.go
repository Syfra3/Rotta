package workflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHarnessReliabilityFixturesExecuteExpectedResultsAndPersistCanonicalJSON(t *testing.T) {
	for _, fixture := range []string{"fixture-equivalent-v1", "fixture-identity-mismatch-v1", "fixture-unavailable-telemetry-v1"} {
		records, expected, err := ReadBenchmarkFixture(filepath.Join("..", "..", "reports", "benchmarks", fixture))
		if err != nil {
			t.Fatalf("read %s fixture: %v", fixture, err)
		}
		result, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: t.TempDir(), BenchmarkID: fixture, Records: records})
		if err != nil {
			t.Fatalf("run %s fixture: %v", fixture, err)
		}
		if result.Status != expected.Status || result.Reason != expected.Reason {
			t.Fatalf("%s result = %s/%s; want %s/%s", fixture, result.Status, result.Reason, expected.Status, expected.Reason)
		}
		if err := ValidatePersistedBenchmark(result); err != nil {
			t.Fatalf("validate persisted %s fixture: %v", fixture, err)
		}
	}
}

func TestHarnessReliabilityFixtureAndInputMalformedJSONRejectWithoutPublication(t *testing.T) {
	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "records.json"), []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "expected-result.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadBenchmarkFixture(fixture); err == nil {
		t.Fatal("malformed fixture records must fail")
	}
	validFixture := t.TempDir()
	data, err := json.Marshal([]OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validFixture, "records.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validFixture, "expected-result.json"), []byte(`{"schema_version":"rotta.retained-benchmark-result/v1","status":"invalid","reason":"bad","conclusion":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadBenchmarkFixture(validFixture); err == nil {
		t.Fatal("invalid expected-result status must fail")
	}
	worktree := t.TempDir()
	bad := validOutcomeRecord("run-1", "root-1", "evidence-1")
	bad.SchemaVersion = "invalid"
	if _, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "malformed-v1", Records: []OutcomeRecord{bad, validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}}); err == nil {
		t.Fatal("malformed record must fail")
	}
	if _, err := os.Stat(filepath.Join(worktree, "reports", "benchmarks", "malformed-v1")); !os.IsNotExist(err) {
		t.Fatalf("malformed record published final output: %v", err)
	}
}

func TestHarnessReliabilityAtomicFailureDoesNotConsumeID(t *testing.T) {
	for _, injected := range []struct {
		name string
		set  func()
	}{
		{"write", func() {
			benchmarkStageOpen = func(string) (stageFile, error) {
				return failingStageFile{writeErr: errors.New("injected write error")}, nil
			}
		}},
		{"close", func() {
			benchmarkStageOpen = func(string) (stageFile, error) {
				return failingStageFile{closeErr: errors.New("injected close error")}, nil
			}
		}},
		{"rename", func() {
			benchmarkStageRename = func(string, string) error { return errors.New("injected rename error") }
		}},
	} {
		t.Run(injected.name, func(t *testing.T) {
			oldOpen, oldRename := benchmarkStageOpen, benchmarkStageRename
			t.Cleanup(func() { benchmarkStageOpen, benchmarkStageRename = oldOpen, oldRename })
			injected.set()
			worktree := t.TempDir()
			request := BenchmarkRequest{Worktree: worktree, BenchmarkID: "retry-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}}
			if _, err := RunRetainedBenchmark(request); err == nil {
				t.Fatal("injected publication failure must fail")
			}
			final := filepath.Join(worktree, "reports", "benchmarks", "retry-v1")
			if _, err := os.Stat(final); !os.IsNotExist(err) {
				t.Fatalf("failure consumed final ID: %v", err)
			}
			entries, err := os.ReadDir(filepath.Join(worktree, "reports", "benchmarks"))
			if err != nil {
				t.Fatalf("read benchmark root after failure: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("failure left a partial stage: %#v", entries)
			}
			benchmarkStageOpen, benchmarkStageRename = oldOpen, oldRename
			result, err := RunRetainedBenchmark(request)
			if err != nil || result.Status != BenchmarkPassed {
				t.Fatalf("ID was not reusable after %s failure: %#v, %v", injected.name, result, err)
			}
		})
	}
}

func TestHarnessReliabilityFinalCreatedAfterPrecheckIsNeverReplaced(t *testing.T) {
	oldHook := benchmarkBeforePublish
	t.Cleanup(func() { benchmarkBeforePublish = oldHook })
	worktree := t.TempDir()
	request := BenchmarkRequest{Worktree: worktree, BenchmarkID: "raced-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}}
	final := filepath.Join(worktree, "reports", "benchmarks", request.BenchmarkID)
	benchmarkBeforePublish = func(_, observedFinal string) error {
		if observedFinal != final {
			t.Fatalf("unexpected final path: %s", observedFinal)
		}
		if err := os.Mkdir(final, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(final, "foreign.txt"), []byte("foreign contents"), 0o600)
	}
	if _, err := RunRetainedBenchmark(request); err == nil {
		t.Fatal("late final destination must reject publication")
	}
	data, err := os.ReadFile(filepath.Join(final, "foreign.txt"))
	if err != nil || string(data) != "foreign contents" {
		t.Fatalf("late final was mutated: %q, %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(worktree, "reports", "benchmarks"))
	if err != nil || len(entries) != 1 || entries[0].Name() != request.BenchmarkID {
		t.Fatalf("late final race left unexpected benchmark entries: %#v, %v", entries, err)
	}
}

func TestHarnessReliabilityFinalSymlinkCreatedAfterPrecheckIsNeverFollowed(t *testing.T) {
	oldHook := benchmarkBeforePublish
	t.Cleanup(func() { benchmarkBeforePublish = oldHook })
	worktree, external := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "foreign.txt"), []byte("foreign contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := BenchmarkRequest{Worktree: worktree, BenchmarkID: "raced-link-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}}
	final := filepath.Join(worktree, "reports", "benchmarks", request.BenchmarkID)
	benchmarkBeforePublish = func(_, _ string) error { return os.Symlink(external, final) }
	if _, err := RunRetainedBenchmark(request); err == nil {
		t.Fatal("late final symlink must reject publication")
	}
	info, err := os.Lstat(final)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("late final symlink was replaced: %#v, %v", info, err)
	}
	data, err := os.ReadFile(filepath.Join(external, "foreign.txt"))
	if err != nil || string(data) != "foreign contents" {
		t.Fatalf("late final symlink target was mutated: %q, %v", data, err)
	}
}

type failingStageFile struct{ writeErr, closeErr error }

func (f failingStageFile) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}
func (f failingStageFile) Sync() error  { return nil }
func (f failingStageFile) Close() error { return f.closeErr }

func TestHarnessReliabilityFailedStatusHasReason(t *testing.T) {
	failed := validOutcomeRecord("run-2", "root-2", "evidence-2")
	failed.Outcome.Accepted = false
	result, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: t.TempDir(), BenchmarkID: "failed-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), failed, validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err != nil || result.Status != BenchmarkFailed || result.Reason == "" {
		t.Fatalf("expected failed result with reason, got %#v, %v", result, err)
	}
}

func TestHarnessReliabilityStaticFixturesContainExactlyThreeRecords(t *testing.T) {
	for _, fixture := range []string{"fixture-equivalent-v1", "fixture-identity-mismatch-v1", "fixture-unavailable-telemetry-v1"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "reports", "benchmarks", fixture, "records.json"))
		if err != nil {
			t.Fatalf("read %s fixture: %v", fixture, err)
		}
		var records []OutcomeRecord
		if err := json.Unmarshal(data, &records); err != nil {
			t.Fatalf("parse %s fixture: %v", fixture, err)
		}
		if len(records) != 3 {
			t.Fatalf("%s fixture has %d records; want exactly 3", fixture, len(records))
		}
	}
}

func TestHarnessReliabilityRunRetainedBenchmarkPersistsExactlyThreeImmutableRecords(t *testing.T) {
	// Gherkin: Given exactly three immutable canonical records with distinct IDs,
	// when the offline benchmark runs, then it writes only confined records/report assets.
	worktree := t.TempDir()
	result, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "fixture-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if result.Status != BenchmarkPassed || result.Reason == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.CanonicalRecordPaths) != 3 {
		t.Fatalf("want 3 records, got %#v", result.CanonicalRecordPaths)
	}
	if _, err := os.Stat(result.CanonicalResultPath); err != nil {
		t.Fatalf("result absent: %v", err)
	}
	if _, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "fixture-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}}); err == nil {
		t.Fatal("existing canonical output must be immutable")
	}
}

func TestHarnessReliabilityRunRetainedBenchmarkMismatchIsNotEvaluableWithNamedField(t *testing.T) {
	// Gherkin: Given a mismatch in an equivalence identity fingerprint, when it runs,
	// then the result is not_evaluable with a named mismatch field.
	worktree := t.TempDir()
	mismatched := validOutcomeRecord("run-2", "root-2", "evidence-2")
	mismatched.Identity.Model = "other"
	result, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "mismatch-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), mismatched, validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if result.Status != BenchmarkNotEvaluable || result.Reason != "identity_mismatch:model" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHarnessReliabilityRunRetainedBenchmarkUnavailableTelemetryIsNotEvaluable(t *testing.T) {
	// Gherkin: Given unavailable required telemetry, when it runs, then it remains
	// unavailable and returns not_evaluable rather than a performance conclusion.
	worktree := t.TempDir()
	unavailable := validOutcomeRecord("run-2", "root-2", "evidence-2")
	unavailable.Cache.ReadTokens = SourceMetric{Availability: TelemetryUnavailable}
	result, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "unavailable-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), unavailable, validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if result.Status != BenchmarkNotEvaluable || result.Reason != "required_telemetry_unavailable:cache.read_tokens" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHarnessReliabilityRunRetainedBenchmarkRejectsRunIDPathTraversal(t *testing.T) {
	worktree := t.TempDir()
	bad := validOutcomeRecord("../escape", "root-1", "evidence-1")
	_, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "traversal-v1", Records: []OutcomeRecord{bad, validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err == nil {
		t.Fatal("run ID path traversal must be rejected")
	}
}

func TestHarnessReliabilityRunRetainedBenchmarkRejectsOutputOutsideWorktree(t *testing.T) {
	worktree := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(worktree, "reports")); err != nil {
		t.Fatal(err)
	}
	_, err := RunRetainedBenchmark(BenchmarkRequest{Worktree: worktree, BenchmarkID: "escape-v1", Records: []OutcomeRecord{validOutcomeRecord("run-1", "root-1", "evidence-1"), validOutcomeRecord("run-2", "root-2", "evidence-2"), validOutcomeRecord("run-3", "root-3", "evidence-3")}})
	if err == nil {
		t.Fatal("symlinked report output must be rejected")
	}
}
