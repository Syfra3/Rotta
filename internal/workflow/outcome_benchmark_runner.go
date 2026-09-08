package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const BenchmarkResultSchemaV1 = "rotta.retained-benchmark-result/v1"

type BenchmarkStatus string

const (
	BenchmarkPassed       BenchmarkStatus = "passed"
	BenchmarkFailed       BenchmarkStatus = "failed"
	BenchmarkNotEvaluable BenchmarkStatus = "not_evaluable"
)

type BenchmarkRequest struct {
	Worktree            string
	BenchmarkID         string
	Records             []OutcomeRecord
	CanonicalInputPaths []string
}

type BenchmarkResult struct {
	SchemaVersion        string          `json:"schema_version"`
	Status               BenchmarkStatus `json:"status"`
	Reason               string          `json:"reason"`
	Conclusion           string          `json:"conclusion"`
	CanonicalInputPaths  []string        `json:"canonical_input_paths"`
	CanonicalRecordPaths []string        `json:"canonical_record_paths"`
	CanonicalResultPath  string          `json:"canonical_result_path"`
}

// FixtureExpectedResult deliberately has no paths: it is a trusted expected
// outcome input, not a retained benchmark publication.
type FixtureExpectedResult struct {
	SchemaVersion string          `json:"schema_version"`
	Status        BenchmarkStatus `json:"status"`
	Reason        string          `json:"reason"`
	Conclusion    string          `json:"conclusion"`
}

var (
	benchmarkIDPattern     = regexp.MustCompile(`^[a-z0-9]+(?:[a-z0-9._-]*[a-z0-9])?$`)
	benchmarkStageFile     = writeStageJSON
	benchmarkStageRename   = publishStageNoReplace
	benchmarkBeforePublish func(stage, final string) error
	benchmarkStageOpen     = func(path string) (stageFile, error) {
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	}
)

var ErrBenchmarkNoReplaceUnsupported = errors.New("atomic no-replace benchmark publication is unsupported on this platform")

type stageFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// RunRetainedBenchmark atomically publishes an immutable exactly-three-record result.
func RunRetainedBenchmark(request BenchmarkRequest) (result BenchmarkResult, err error) {
	if len(request.Records) != 3 {
		return result, fmt.Errorf("benchmark requires exactly three outcome records")
	}
	if !benchmarkIDPattern.MatchString(request.BenchmarkID) {
		return result, fmt.Errorf("invalid benchmark id %q", request.BenchmarkID)
	}
	worktree, err := filepath.EvalSymlinks(request.Worktree)
	if err != nil {
		return result, fmt.Errorf("resolve worktree: %w", err)
	}
	worktree, err = filepath.Abs(worktree)
	if err != nil {
		return result, err
	}
	if err := validateRecords(request.Records); err != nil {
		return result, err
	}
	root, err := benchmarkRoot(worktree)
	if err != nil {
		return result, err
	}
	final := filepath.Join(root, request.BenchmarkID)
	if info, statErr := os.Lstat(final); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return result, fmt.Errorf("immutable benchmark output already exists: %s", final)
		}
		return result, fmt.Errorf("benchmark output path is not a directory: %s", final)
	} else if !os.IsNotExist(statErr) {
		return result, statErr
	}

	stage, err := os.MkdirTemp(root, "."+request.BenchmarkID+".stage-")
	if err != nil {
		return result, err
	}
	if err := ownedStage(root, stage); err != nil {
		return result, err
	}
	defer func() {
		if stage != "" {
			_ = removeOwnedStage(root, stage)
		}
	}()

	result = evaluateRecords(request.Records)
	result.SchemaVersion = BenchmarkResultSchemaV1
	result.CanonicalInputPaths = append([]string(nil), request.CanonicalInputPaths...)
	sort.Strings(result.CanonicalInputPaths)
	for _, record := range request.Records {
		finalRecord := filepath.Join(final, "record-"+record.RunID+".json")
		if err := benchmarkStageFile(filepath.Join(stage, filepath.Base(finalRecord)), record); err != nil {
			return BenchmarkResult{}, fmt.Errorf("write staged record: %w", err)
		}
		result.CanonicalRecordPaths = append(result.CanonicalRecordPaths, finalRecord)
	}
	sort.Strings(result.CanonicalRecordPaths)
	result.CanonicalResultPath = filepath.Join(final, "result.json")
	if err := benchmarkStageFile(filepath.Join(stage, "result.json"), result); err != nil {
		return BenchmarkResult{}, fmt.Errorf("write staged result: %w", err)
	}
	if err := validateStagedBenchmark(stage, final); err != nil {
		return BenchmarkResult{}, fmt.Errorf("validate staged benchmark: %w", err)
	}
	if err := syncDirectory(stage); err != nil {
		return BenchmarkResult{}, fmt.Errorf("sync staged benchmark: %w", err)
	}
	if benchmarkBeforePublish != nil {
		if err := benchmarkBeforePublish(stage, final); err != nil {
			return BenchmarkResult{}, fmt.Errorf("prepare benchmark publication: %w", err)
		}
	}
	if err := benchmarkStageRename(stage, final); err != nil {
		return BenchmarkResult{}, fmt.Errorf("atomically publish benchmark: %w", err)
	}
	stage = ""
	// The stage and its files were synced before rename. A parent-directory sync
	// after publication is best effort: reporting it as a failed publication
	// would falsely claim the identifier remains reusable after a successful rename.
	_ = syncDirectory(root)
	return result, nil
}

// ReadBenchmarkFixture validates the fixture input and expected-result schema.
func ReadBenchmarkFixture(directory string) ([]OutcomeRecord, FixtureExpectedResult, error) {
	var records []OutcomeRecord
	if err := readJSON(filepath.Join(directory, "records.json"), &records); err != nil {
		return nil, FixtureExpectedResult{}, fmt.Errorf("read fixture records: %w", err)
	}
	if err := validateRecords(records); err != nil {
		return nil, FixtureExpectedResult{}, fmt.Errorf("validate fixture records: %w", err)
	}
	var expected FixtureExpectedResult
	if err := readJSON(filepath.Join(directory, "expected-result.json"), &expected); err != nil {
		return nil, FixtureExpectedResult{}, fmt.Errorf("read fixture expected result: %w", err)
	}
	if err := expected.Validate(); err != nil {
		return nil, FixtureExpectedResult{}, fmt.Errorf("validate fixture expected result: %w", err)
	}
	return records, expected, nil
}

func (expected FixtureExpectedResult) Validate() error {
	if expected.SchemaVersion != BenchmarkResultSchemaV1 {
		return fmt.Errorf("unsupported expected result schema %q", expected.SchemaVersion)
	}
	if !validStatus(expected.Status) || expected.Reason == "" || expected.Conclusion == "" {
		return fmt.Errorf("invalid expected result")
	}
	return nil
}

// ValidatePersistedBenchmark parses and semantically validates the published canonical JSON.
func ValidatePersistedBenchmark(result BenchmarkResult) error {
	if err := validateResult(result, true); err != nil {
		return err
	}
	final := filepath.Dir(result.CanonicalResultPath)
	if !within(final, result.CanonicalResultPath) || filepath.Base(result.CanonicalResultPath) != "result.json" {
		return fmt.Errorf("invalid canonical result path")
	}
	var records []OutcomeRecord
	for _, path := range result.CanonicalRecordPaths {
		if filepath.Dir(path) != final || filepath.Base(path) == "" {
			return fmt.Errorf("record path escapes final benchmark")
		}
		if err := regularFile(path); err != nil {
			return err
		}
		var record OutcomeRecord
		if err := readJSON(path, &record); err != nil {
			return err
		}
		records = append(records, record)
	}
	if err := validateRecords(records); err != nil {
		return err
	}
	var persisted BenchmarkResult
	if err := regularFile(result.CanonicalResultPath); err != nil {
		return err
	}
	if err := readJSON(result.CanonicalResultPath, &persisted); err != nil {
		return err
	}
	if err := validateResult(persisted, true); err != nil {
		return err
	}
	if persisted.Status != result.Status || persisted.Reason != result.Reason || persisted.CanonicalResultPath != result.CanonicalResultPath || !samePaths(persisted.CanonicalRecordPaths, result.CanonicalRecordPaths) {
		return fmt.Errorf("persisted result differs from publication result")
	}
	expected := evaluateRecords(records)
	if persisted.Status != expected.Status || persisted.Reason != expected.Reason {
		return fmt.Errorf("persisted result does not match record semantics")
	}
	return nil
}

func validateStagedBenchmark(stage, final string) error {
	if err := ownedStage(filepath.Dir(final), stage); err != nil {
		return err
	}
	if err := regularFile(filepath.Join(stage, "result.json")); err != nil {
		return err
	}
	var result BenchmarkResult
	if err := readJSON(filepath.Join(stage, "result.json"), &result); err != nil {
		return err
	}
	if err := validateResult(result, true); err != nil {
		return err
	}
	if result.CanonicalResultPath != filepath.Join(final, "result.json") {
		return fmt.Errorf("staged result has non-canonical final path")
	}
	var records []OutcomeRecord
	for _, path := range result.CanonicalRecordPaths {
		if filepath.Dir(path) != final {
			return fmt.Errorf("staged record has non-canonical final path")
		}
		if err := regularFile(filepath.Join(stage, filepath.Base(path))); err != nil {
			return err
		}
		var record OutcomeRecord
		if err := readJSON(filepath.Join(stage, filepath.Base(path)), &record); err != nil {
			return err
		}
		records = append(records, record)
	}
	if err := validateRecords(records); err != nil {
		return err
	}
	expected := evaluateRecords(records)
	if result.Status != expected.Status || result.Reason != expected.Reason {
		return fmt.Errorf("staged result does not match record semantics")
	}
	return nil
}

func evaluateRecords(records []OutcomeRecord) BenchmarkResult {
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"task_request_fingerprint", identityValues(records, func(i OutcomeIdentity) string { return i.TaskRequestFingerprint })}, {"policy_contract_fingerprint", identityValues(records, func(i OutcomeIdentity) string { return i.PolicyContractFingerprint })}, {"baseline", identityValues(records, func(i OutcomeIdentity) string { return i.Baseline })}, {"provider", identityValues(records, func(i OutcomeIdentity) string { return i.Provider })}, {"model", identityValues(records, func(i OutcomeIdentity) string { return i.Model })}, {"family", identityValues(records, func(i OutcomeIdentity) string { return i.Family })}, {"integrations_fingerprint", identityValues(records, func(i OutcomeIdentity) string { return i.IntegrationsFingerprint })}, {"permissions_fingerprint", identityValues(records, func(i OutcomeIdentity) string { return i.PermissionsFingerprint })}, {"acceptance_checks_fingerprint", identityValues(records, func(i OutcomeIdentity) string { return i.AcceptanceChecksFingerprint })},
	} {
		if field.values[0] != field.values[1] || field.values[0] != field.values[2] {
			return nonEvaluable("identity_mismatch:" + field.name)
		}
	}
	for _, record := range records {
		for name, metric := range record.requiredMetrics() {
			if metric.Availability == TelemetryUnavailable {
				return nonEvaluable("required_telemetry_unavailable:" + name)
			}
		}
	}
	for _, record := range records {
		if !record.Outcome.Accepted {
			return BenchmarkResult{Status: BenchmarkFailed, Reason: "fixture_acceptance_failed", Conclusion: "fixture semantics only; no performance trend claim"}
		}
	}
	return BenchmarkResult{Status: BenchmarkPassed, Reason: "equivalent_fixture_acceptance_passed", Conclusion: "fixture semantics only; no performance trend claim"}
}
func nonEvaluable(reason string) BenchmarkResult {
	return BenchmarkResult{Status: BenchmarkNotEvaluable, Reason: reason, Conclusion: "no performance conclusion"}
}
func identityValues(records []OutcomeRecord, value func(OutcomeIdentity) string) []string {
	return []string{value(records[0].Identity), value(records[1].Identity), value(records[2].Identity)}
}
func validateRecords(records []OutcomeRecord) error {
	if len(records) != 3 {
		return fmt.Errorf("benchmark requires exactly three outcome records")
	}
	for _, record := range records {
		if err := record.validateStructure(); err != nil {
			return err
		}
	}
	return distinctIDs(records)
}
func distinctIDs(records []OutcomeRecord) error {
	for _, value := range []struct {
		name string
		get  func(OutcomeRecord) string
	}{{"run_id", func(r OutcomeRecord) string { return r.RunID }}, {"root_id", func(r OutcomeRecord) string { return r.RootID }}, {"evidence_id", func(r OutcomeRecord) string { return r.EvidenceID }}} {
		seen := map[string]bool{}
		for _, record := range records {
			if seen[value.get(record)] {
				return fmt.Errorf("duplicate %s", value.name)
			}
			seen[value.get(record)] = true
		}
	}
	return nil
}
func validStatus(status BenchmarkStatus) bool {
	return status == BenchmarkPassed || status == BenchmarkFailed || status == BenchmarkNotEvaluable
}
func validateResult(result BenchmarkResult, requirePaths bool) error {
	if result.SchemaVersion != BenchmarkResultSchemaV1 || !validStatus(result.Status) || result.Reason == "" || result.Conclusion == "" {
		return fmt.Errorf("invalid benchmark result")
	}
	if requirePaths && (len(result.CanonicalRecordPaths) != 3 || result.CanonicalResultPath == "") {
		return fmt.Errorf("missing canonical result paths")
	}
	return nil
}
func benchmarkRoot(worktree string) (string, error) {
	current := worktree
	for _, part := range []string{"reports", "benchmarks"} {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err = os.Mkdir(current, 0o700); err != nil {
				return "", err
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("benchmark output component is not a real directory: %s", current)
		}
	}
	if !within(worktree, current) {
		return "", fmt.Errorf("benchmark root escapes worktree")
	}
	return current, nil
}
func ownedStage(root, stage string) error {
	if !within(root, stage) || filepath.Dir(stage) != root || !strings.HasPrefix(filepath.Base(stage), ".") {
		return fmt.Errorf("invalid benchmark stage")
	}
	info, err := os.Lstat(stage)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("benchmark stage is not a regular directory")
	}
	return nil
}
func removeOwnedStage(root, stage string) error {
	if err := ownedStage(root, stage); err != nil {
		return err
	}
	return os.RemoveAll(stage)
}
func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
func writeStageJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := benchmarkStageOpen(path)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, destination)
}
func regularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}
	return nil
}
func samePaths(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
