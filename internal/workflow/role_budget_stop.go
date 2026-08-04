package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WorkflowRole identifies a workflow role with a recorded default budget.
type WorkflowRole string

const (
	RoleExploration    WorkflowRole = "exploration"
	RoleSpec           WorkflowRole = "spec"
	RoleOrchestration  WorkflowRole = "orchestration"
	RoleReview         WorkflowRole = "review"
	RoleImplementation WorkflowRole = "implementation"
)

// RoleBudgetStopRequest identifies the unfinished current slice and its
// already-recorded evidence when a role reaches its default step budget.
type RoleBudgetStopRequest struct {
	FeatureWorktree string
	Role            WorkflowRole
	StepsUsed       int
	Slice           string
	EvidencePath    string
}

// RoleBudgetStop reports a bounded, resumable stop. It is never success and
// always requires validation before the slice can proceed.
type RoleBudgetStop struct {
	RecordPath         string
	ManifestPath       string
	Slice              string
	EvidencePath       string
	Resumable          bool
	Success            bool
	ValidationRequired bool
}

type roleBudgetStopRecord struct {
	Role               WorkflowRole `json:"role"`
	StepsUsed          int          `json:"steps_used"`
	Budget             int          `json:"budget"`
	ManifestPath       string       `json:"manifest_path"`
	Slice              string       `json:"slice"`
	EvidencePath       string       `json:"evidence_path"`
	Resumable          bool         `json:"resumable"`
	Success            bool         `json:"success"`
	ValidationRequired bool         `json:"validation_required"`
}

// RecordRoleBudgetStop persists a narrow recovery reference without reading or
// writing lifecycle acceptance state or any prior role transcript.
func RecordRoleBudgetStop(request RoleBudgetStopRequest) (RoleBudgetStop, error) {
	budget, ok := defaultRoleBudget(request.Role)
	if !ok || request.StepsUsed != budget || request.Slice == "" {
		return RoleBudgetStop{}, fmt.Errorf("role budget stop requires a role at its recorded default budget and a slice")
	}

	worktree, err := filepath.Abs(request.FeatureWorktree)
	if err != nil {
		return RoleBudgetStop{}, fmt.Errorf("resolve role budget stop worktree: %w", err)
	}
	manifestPath := filepath.Join(worktree, ".rotta", "current", "manifest.yaml")
	if err := requireRegularFile(manifestPath); err != nil {
		return RoleBudgetStop{}, fmt.Errorf("current manifest: %w", err)
	}
	evidencePath, err := roleBudgetEvidencePath(worktree, request.EvidencePath)
	if err != nil {
		return RoleBudgetStop{}, err
	}
	if err := requireRegularFile(evidencePath); err != nil {
		return RoleBudgetStop{}, fmt.Errorf("current evidence: %w", err)
	}

	record := roleBudgetStopRecord{
		Role:               request.Role,
		StepsUsed:          request.StepsUsed,
		Budget:             budget,
		ManifestPath:       manifestPath,
		Slice:              request.Slice,
		EvidencePath:       evidencePath,
		Resumable:          true,
		Success:            false,
		ValidationRequired: true,
	}
	contents, err := json.Marshal(record)
	if err != nil {
		return RoleBudgetStop{}, fmt.Errorf("serialize role budget stop: %w", err)
	}
	recordPath := filepath.Join(worktree, ".rotta", "current", "evidence", "role-budget-stop-"+string(request.Role)+".json")
	if err := os.WriteFile(recordPath, contents, 0o600); err != nil {
		return RoleBudgetStop{}, fmt.Errorf("write role budget stop: %w", err)
	}
	return roleBudgetStopFromRecord(record, recordPath), nil
}

// ResumeRoleBudgetStop returns only the persisted stop references needed to
// resume validation; it does not load or return any previous transcript.
func ResumeRoleBudgetStop(recordPath string) (RoleBudgetStop, error) {
	contents, err := os.ReadFile(recordPath)
	if err != nil {
		return RoleBudgetStop{}, fmt.Errorf("read role budget stop: %w", err)
	}
	var record roleBudgetStopRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		return RoleBudgetStop{}, fmt.Errorf("parse role budget stop: %w", err)
	}
	if budget, ok := defaultRoleBudget(record.Role); !ok || record.StepsUsed != budget || record.Budget != budget || record.ManifestPath == "" || record.Slice == "" || record.EvidencePath == "" || !record.Resumable || record.Success || !record.ValidationRequired {
		return RoleBudgetStop{}, fmt.Errorf("role budget stop is not a valid unfinished resumable stop")
	}
	return roleBudgetStopFromRecord(record, recordPath), nil
}

func defaultRoleBudget(role WorkflowRole) (int, bool) {
	switch role {
	case RoleExploration:
		return 8, true
	case RoleSpec:
		return 12, true
	case RoleOrchestration, RoleReview:
		return 16, true
	case RoleImplementation:
		return 24, true
	default:
		return 0, false
	}
}

func roleBudgetEvidencePath(worktree, evidencePath string) (string, error) {
	evidencePath, err := filepath.Abs(evidencePath)
	if err != nil {
		return "", fmt.Errorf("resolve current evidence: %w", err)
	}
	evidenceRoot := filepath.Join(worktree, ".rotta", "current", "evidence")
	relative, err := filepath.Rel(evidenceRoot, evidencePath)
	if err != nil || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("current evidence must remain below %q", evidenceRoot)
	}
	return evidencePath, nil
}

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	return nil
}

func roleBudgetStopFromRecord(record roleBudgetStopRecord, recordPath string) RoleBudgetStop {
	return RoleBudgetStop{
		RecordPath:         recordPath,
		ManifestPath:       record.ManifestPath,
		Slice:              record.Slice,
		EvidencePath:       record.EvidencePath,
		Resumable:          record.Resumable,
		Success:            record.Success,
		ValidationRequired: record.ValidationRequired,
	}
}
