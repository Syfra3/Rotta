package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const OutcomeBenchmarkInputFormat = "rotta.workflow-benchmark-input/v1"

// OutcomeBenchmarkInput is a transport-only boundary for retained telemetry.
// It contains measurements exported by a host adapter; Rotta neither contacts a
// provider nor infers absent telemetry.
type OutcomeBenchmarkInput struct {
	Format string                  `json:"format"`
	Runs   []WorkflowOutcomeRecord `json:"runs"`
}

// OutcomeTelemetryAdapter is the optional boundary an OpenCode (or other host)
// integration implements. Implementations must obtain all provider settings and
// credentials outside Rotta and either mark unavailable fields not_observable
// or return ErrOutcomeTelemetryNotEvaluable when collection cannot proceed.
type OutcomeTelemetryAdapter interface {
	CollectOutcomeTelemetry(OutcomeRunIdentity, string) (WorkflowOutcomeRecord, error)
}

var ErrOutcomeTelemetryNotEvaluable = errors.New("outcome telemetry is not evaluable from the configured host")

func UnmarshalOutcomeBenchmarkInput(data []byte) (OutcomeBenchmarkInput, error) {
	if err := rejectAmbiguousJSON(data, outcomeBenchmarkInputShape()); err != nil {
		return OutcomeBenchmarkInput{}, fmt.Errorf("decode outcome benchmark input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var input OutcomeBenchmarkInput
	if err := decoder.Decode(&input); err != nil {
		return OutcomeBenchmarkInput{}, fmt.Errorf("decode outcome benchmark input: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return OutcomeBenchmarkInput{}, fmt.Errorf("decode outcome benchmark input: multiple JSON values")
	}
	if input.Format != OutcomeBenchmarkInputFormat {
		return OutcomeBenchmarkInput{}, fmt.Errorf("outcome benchmark input format must be %q", OutcomeBenchmarkInputFormat)
	}
	if len(input.Runs) != REQ091BenchmarkRunCount {
		return OutcomeBenchmarkInput{}, fmt.Errorf("REQ-091 benchmark input requires exactly %d retained runs", REQ091BenchmarkRunCount)
	}
	for index, run := range input.Runs {
		if err := ValidateOutcomeRecord(run); err != nil {
			return OutcomeBenchmarkInput{}, fmt.Errorf("run %d is invalid: %w", index+1, err)
		}
	}
	return input, nil
}

// PersistAndCompareOutcomeBenchmark confines immutable retained records below
// recordsDirectory, then invokes the existing comparator on exactly those three
// records. Evidence remains separately retained and is verified by comparison.
func PersistAndCompareOutcomeBenchmark(worktree, recordsDirectory string, input OutcomeBenchmarkInput) (OutcomeBenchmarkResult, []string, error) {
	if input.Format != OutcomeBenchmarkInputFormat || len(input.Runs) != REQ091BenchmarkRunCount {
		return OutcomeBenchmarkResult{}, nil, fmt.Errorf("invalid REQ-091 benchmark input")
	}
	root, err := canonicalOutcomeBenchmarkDirectory(worktree, recordsDirectory)
	if err != nil {
		return OutcomeBenchmarkResult{}, nil, err
	}
	if err := createSecureOutcomeBenchmarkDirectory(root); err != nil {
		return OutcomeBenchmarkResult{}, nil, err
	}
	paths := make([]string, 0, len(input.Runs))
	persisted := make([]WorkflowOutcomeRecord, 0, len(input.Runs))
	names := make([]string, len(input.Runs))
	seenNames := make(map[string]bool, len(input.Runs))
	for _, run := range input.Runs {
		if err := ValidateOutcomeRecord(run); err != nil {
			return OutcomeBenchmarkResult{}, nil, err
		}
		name, err := outcomeBenchmarkRecordFilename(run.RunID)
		if err != nil {
			return OutcomeBenchmarkResult{}, nil, err
		}
		names[len(persisted)] = name
		if seenNames[name] {
			return OutcomeBenchmarkResult{}, nil, fmt.Errorf("outcome benchmark input duplicates retained run ID %q", run.RunID)
		}
		seenNames[name] = true
		path := filepath.Join(root, name)
		if _, err := os.Lstat(path); err == nil {
			return OutcomeBenchmarkResult{}, nil, fmt.Errorf("outcome benchmark record already exists: %s", filepath.ToSlash(filepath.Join(recordsDirectory, name)))
		} else if !os.IsNotExist(err) {
			return OutcomeBenchmarkResult{}, nil, fmt.Errorf("inspect outcome benchmark record: %w", err)
		}
		persisted = append(persisted, run)
	}
	reloaded := make([]WorkflowOutcomeRecord, 0, len(persisted))
	for index, run := range persisted {
		name := names[index]
		path := filepath.Join(root, name)
		encoded, err := MarshalOutcomeRecord(run)
		if err != nil {
			return OutcomeBenchmarkResult{}, nil, err
		}
		retained := append(encoded, '\n')
		hash := sha256.Sum256(retained)
		if err := writeNewOutcomeBenchmarkRecord(path, retained); err != nil {
			return OutcomeBenchmarkResult{}, nil, err
		}
		recorded, err := loadPersistedOutcomeBenchmarkRecord(path, hash[:])
		if err != nil {
			return OutcomeBenchmarkResult{}, nil, err
		}
		reloaded = append(reloaded, recorded)
		paths = append(paths, filepath.ToSlash(filepath.Join(recordsDirectory, name)))
	}
	return CompareOutcomeBenchmarkInWorktree(worktree, reloaded), paths, nil
}

func canonicalOutcomeBenchmarkDirectory(worktree, directory string) (string, error) {
	if strings.TrimSpace(worktree) == "" || strings.TrimSpace(directory) == "" || filepath.IsAbs(directory) {
		return "", fmt.Errorf("outcome benchmark records directory must be a non-empty relative path")
	}
	root, err := filepath.Abs(worktree)
	if err != nil {
		return "", fmt.Errorf("resolve worktree: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve canonical worktree: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("outcome benchmark worktree is not a directory")
	}
	if filepath.Clean(directory) != directory || directory == "." {
		return "", fmt.Errorf("outcome benchmark records directory must be a canonical relative path")
	}
	target := filepath.Join(root, directory)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("outcome benchmark records directory escapes worktree")
	}
	return target, nil
}

// createSecureOutcomeBenchmarkDirectory creates each component with Lstat
// checks. It never follows a pre-existing symlink while creating durable
// benchmark records.
func createSecureOutcomeBenchmarkDirectory(directory string) error {
	root := filepath.VolumeName(directory) + string(filepath.Separator)
	if root == string(filepath.Separator) {
		root = string(filepath.Separator)
	}
	current := root
	for _, component := range strings.Split(strings.TrimPrefix(directory, root), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create outcome benchmark record directory: %w", err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect outcome benchmark record directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("outcome benchmark records directory is not a worktree-local directory")
		}
	}
	return nil
}

func outcomeBenchmarkRecordFilename(runID string) (string, error) {
	if strings.TrimSpace(runID) == "" || runID != filepath.Base(runID) || strings.ContainsAny(runID, `/\\`) || runID == "." || runID == ".." {
		return "", fmt.Errorf("outcome benchmark run ID is not safe for durable storage")
	}
	return runID + ".json", nil
}

func writeNewOutcomeBenchmarkRecord(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write outcome benchmark record: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return fmt.Errorf("write outcome benchmark record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close outcome benchmark record: %w", err)
	}
	return nil
}

// loadPersistedOutcomeBenchmarkRecord verifies the exact retained bytes before
// strict decoding. Re-marshalling must reproduce the bytes (apart from the
// required final newline), so whitespace or schema changes cannot be compared.
func loadPersistedOutcomeBenchmarkRecord(path string, expectedHash []byte) (WorkflowOutcomeRecord, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return WorkflowOutcomeRecord{}, fmt.Errorf("read persisted outcome benchmark record: %w", err)
	}
	actualHash := sha256.Sum256(contents)
	if !bytes.Equal(actualHash[:], expectedHash) {
		return WorkflowOutcomeRecord{}, fmt.Errorf("persisted outcome benchmark record hash mismatch")
	}
	record, err := UnmarshalOutcomeRecord(contents)
	if err != nil {
		return WorkflowOutcomeRecord{}, fmt.Errorf("decode persisted outcome benchmark record: %w", err)
	}
	canonical, err := MarshalOutcomeRecord(record)
	if err != nil {
		return WorkflowOutcomeRecord{}, fmt.Errorf("recompute persisted outcome benchmark record: %w", err)
	}
	if !bytes.Equal(contents, append(canonical, '\n')) {
		return WorkflowOutcomeRecord{}, fmt.Errorf("persisted outcome benchmark record bytes do not match canonical record")
	}
	return record, nil
}

func outcomeBenchmarkInputShape() outcomeJSONShape {
	record := workflowOutcomeRecordShape()
	return outcomeJSONShape{fields: map[string]outcomeJSONShape{"format": {}, "runs": {element: &record}}}
}
