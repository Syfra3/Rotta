package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-087 → SCN-618 → TestSCN618_CompactSliceRetainsScenarioEvidenceAndOneCheckpoint
func TestSCN618_CompactSliceRetainsScenarioEvidenceAndOneCheckpoint(t *testing.T) {
	// Scenario: A compact execution slice preserves per-scenario traceability with one full checkpoint
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "feature/compact-slice")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "internal/workflow/slice.go")
	runGit(t, repo, "commit", "-m", "test: establish compact slice baseline")

	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "checkpoint_mode: compact_slice\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "approved_scenarios: [SCN-701, SCN-702, SCN-703]\nnext_scenario: SCN-701\n")
	capsuleDecision := filepath.Join(repo, ".rotta", "current", "evidence", "capsule-decision-none-required.json")
	localInspection := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
	mustWrite(t, localInspection, `{"owners":"resolved","invariants":"resolved"}`+"\n")
	decision, err := json.Marshal(localScopeCapsuleDecision{
		CapsuleDecision:        CapsuleDecisionNoneRequired,
		ScenarioOrSlice:        "SCN-701",
		StatePath:              filepath.Join(repo, ".rotta", "current", "state.yaml"),
		EvidencePath:           localInspection,
		FocusedActions:         1,
		TopLevelComponentCount: 1,
		DirectDependentCount:   0,
	})
	if err != nil {
		t.Fatalf("marshal none-required capsule decision: %v", err)
	}
	mustWrite(t, capsuleDecision, string(decision))

	scenarios := []CompactSliceScenarioEvidence{}
	for _, scenarioID := range []string{"SCN-701", "SCN-702", "SCN-703"} {
		paths := CompactSliceScenarioEvidence{ScenarioID: scenarioID}
		paths.RedEvidencePath = filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-red.txt")
		paths.GreenEvidencePath = filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-green.txt")
		paths.RefactorEvidencePath = filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-refactor.txt")
		paths.FocusedTestEvidencePath = filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-focused-test.txt")
		for _, evidencePath := range []string{paths.RedEvidencePath, paths.GreenEvidencePath, paths.RefactorEvidencePath, paths.FocusedTestEvidencePath} {
			mustWrite(t, evidencePath, scenarioID+" evidence\n")
		}
		scenarios = append(scenarios, paths)
	}
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n\nfunc compactSlice() {}\n")

	validations := 0
	report, err := ExecuteCompactSlice(CompactSliceRequest{
		FeatureWorktree:      repo,
		SliceID:              "compact-slice-701-703",
		ComponentScope:       "internal/workflow",
		ScenarioEvidence:     scenarios,
		ExpectedChangedPaths: []string{"internal/workflow/slice.go"},
		CapsuleDecisionPath:  capsuleDecision,
		RunFullValidation: func() error {
			validations++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteCompactSlice() error = %v", err)
	}
	if validations != 1 {
		t.Fatalf("full validations = %d, want one after all scenarios are green", validations)
	}
	if report.Checkpoint == "" || len(report.Scenarios) != 3 || report.CapsuleDecisionPath != capsuleDecision {
		t.Fatalf("report = %#v, want one traceable three-scenario compact checkpoint", report)
	}
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("checkpoint commits = %s, want one compact-slice checkpoint", commits)
	}

	state, readErr := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if readErr != nil {
		t.Fatalf("read compact slice state: %v", readErr)
	}
	for _, required := range append([]string{report.SliceID, capsuleDecision, report.Checkpoint}, evidencePathsForCompactSlice(scenarios)...) {
		if !strings.Contains(string(state), required) {
			t.Fatalf("state = %s, missing compact-slice trace %q", state, required)
		}
	}
}

// REQ-087 → SCN-618 → TestSCN618_CompactSliceAcceptsCreatedCapsuleDecision
func TestSCN618_CompactSliceAcceptsCreatedCapsuleDecision(t *testing.T) {
	// Scenario: A compact execution slice preserves per-scenario traceability with one full checkpoint
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "feature/compact-capsule-slice")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "internal/workflow/slice.go")
	runGit(t, repo, "commit", "-m", "test: establish compact capsule slice baseline")

	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "checkpoint_mode: compact_slice\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "approved_scenarios: [SCN-701]\nnext_scenario: SCN-701\n")
	localInspection := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
	mustWrite(t, localInspection, `{"owner":"unresolved"}`+"\n")
	capsule, err := CreateExplorationCapsule(ExplorationCapsuleRequest{
		FeatureWorktree:       repo,
		CapsuleID:             "compact-slice-capsule",
		ScenarioOrSlice:       "SCN-701",
		FocusedActions:        1,
		OwnerResolved:         false,
		InvariantResolved:     true,
		Objective:             "resolve the compact slice owner",
		InScope:               []string{"internal/workflow"},
		OutOfScope:            []string{"strict mode"},
		Invariants:            []string{"capsule is fingerprint-bound"},
		Risks:                 []string{"owner remains unresolved"},
		ManifestFingerprint:   "manifest-fingerprint",
		ContractFingerprint:   "contract-fingerprint",
		PolicyFingerprint:     "policy-fingerprint",
		RequiredEvidencePaths: []string{localInspection},
		Delegate:              func(ImplementationCapsuleInput) error { return nil },
	})
	if err != nil {
		t.Fatalf("CreateExplorationCapsule() error = %v", err)
	}

	scenario := CompactSliceScenarioEvidence{
		ScenarioID:              "SCN-701",
		RedEvidencePath:         filepath.Join(repo, ".rotta", "current", "evidence", "SCN-701-red.txt"),
		GreenEvidencePath:       filepath.Join(repo, ".rotta", "current", "evidence", "SCN-701-green.txt"),
		RefactorEvidencePath:    filepath.Join(repo, ".rotta", "current", "evidence", "SCN-701-refactor.txt"),
		FocusedTestEvidencePath: filepath.Join(repo, ".rotta", "current", "evidence", "SCN-701-focused-test.txt"),
	}
	for _, evidencePath := range evidencePathsForCompactSlice([]CompactSliceScenarioEvidence{scenario}) {
		if evidencePath != scenario.ScenarioID {
			mustWrite(t, evidencePath, "SCN-701 evidence\n")
		}
	}
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n\nfunc compactCapsuleSlice() {}\n")

	report, err := ExecuteCompactSlice(CompactSliceRequest{
		FeatureWorktree:      repo,
		SliceID:              "compact-slice-701",
		ComponentScope:       "internal/workflow",
		ScenarioEvidence:     []CompactSliceScenarioEvidence{scenario},
		ExpectedChangedPaths: []string{"internal/workflow/slice.go"},
		CapsuleDecisionPath:  capsule.DecisionPath,
		RunFullValidation:    func() error { return nil },
	})
	if err != nil {
		t.Fatalf("ExecuteCompactSlice() error = %v", err)
	}
	if report.CapsuleDecisionPath != capsule.DecisionPath {
		t.Fatalf("capsule decision trace = %q, want %q", report.CapsuleDecisionPath, capsule.DecisionPath)
	}
	decision, readErr := os.ReadFile(capsule.DecisionPath)
	if readErr != nil || !strings.Contains(string(decision), capsule.CapsuleID) || !strings.Contains(string(decision), capsule.CapsuleFingerprint) {
		t.Fatalf("created capsule decision = %q, read error = %v, want durable identity and fingerprint", decision, readErr)
	}
	state, readErr := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if readErr != nil || !strings.Contains(string(state), capsule.DecisionPath) {
		t.Fatalf("compact slice state = %q, read error = %v, want created decision trace", state, readErr)
	}
}

// REQ-087 → SCN-620 → TestSCN620_FailedSliceValidationPreservesWorkWithoutCheckpoint
func TestSCN620_FailedSliceValidationPreservesWorkWithoutCheckpoint(t *testing.T) {
	// Scenario: Slice failure preserves work without a partial checkpoint
	repo, request := compactSliceFailureRequest(t, "compact-slice-701-702")
	request.RunFullValidation = func() error {
		return fmt.Errorf("full validation failed")
	}

	_, err := ExecuteCompactSlice(request)
	if err == nil || !strings.Contains(err.Error(), "compact-slice-701-702") || !strings.Contains(err.Error(), "resume") || !strings.Contains(err.Error(), "handoff") || !strings.Contains(err.Error(), "recovery") {
		t.Fatalf("ExecuteCompactSlice() error = %v, want affected slice and actionable resume/handoff/recovery guidance", err)
	}
	assertCompactSliceFailurePreserved(t, repo, request.ScenarioEvidence)
}

// REQ-087 → SCN-620 → TestSCN620_MissingLaterFocusedEvidencePreservesWorkWithoutCheckpoint
func TestSCN620_MissingLaterFocusedEvidencePreservesWorkWithoutCheckpoint(t *testing.T) {
	// Scenario: Slice failure preserves work without a partial checkpoint
	repo, request := compactSliceFailureRequest(t, "compact-slice-701-702")
	if err := os.Remove(request.ScenarioEvidence[1].FocusedTestEvidencePath); err != nil {
		t.Fatalf("remove later focused-test evidence: %v", err)
	}

	_, err := ExecuteCompactSlice(request)
	if err == nil || !strings.Contains(err.Error(), "SCN-702") || !strings.Contains(err.Error(), "resume") || !strings.Contains(err.Error(), "handoff") || !strings.Contains(err.Error(), "recovery") {
		t.Fatalf("ExecuteCompactSlice() error = %v, want affected later scenario and actionable guidance", err)
	}
	assertCompactSliceFailurePreserved(t, repo, request.ScenarioEvidence)
}

// REQ-087 → SCN-620 → TestSCN620_FailedLaterFocusedTestPreservesWorkWithoutCheckpoint
func TestSCN620_FailedLaterFocusedTestPreservesWorkWithoutCheckpoint(t *testing.T) {
	// Scenario: Slice failure preserves work without a partial checkpoint
	repo, request := compactSliceFailureRequest(t, "compact-slice-701-702")
	mustWrite(t, request.ScenarioEvidence[1].FocusedTestEvidencePath, `{"exit_status":1,"timed_out":false}`+"\n")

	_, err := ExecuteCompactSlice(request)
	if err == nil || !strings.Contains(err.Error(), "SCN-702") || !strings.Contains(err.Error(), "focused test failed") || !strings.Contains(err.Error(), "resume") || !strings.Contains(err.Error(), "handoff") || !strings.Contains(err.Error(), "recovery") {
		t.Fatalf("ExecuteCompactSlice() error = %v, want failed later focused-test scenario and actionable guidance", err)
	}
	assertCompactSliceFailurePreserved(t, repo, request.ScenarioEvidence)
}

// REQ-087 → SCN-620 → TestSCN620_ScopeDriftPreservesWorkWithoutCheckpoint
func TestSCN620_ScopeDriftPreservesWorkWithoutCheckpoint(t *testing.T) {
	// Scenario: Slice failure preserves work without a partial checkpoint
	repo, request := compactSliceFailureRequest(t, "compact-slice-701-702")
	mustWrite(t, filepath.Join(repo, "unexpected.txt"), "scope drift\n")

	_, err := ExecuteCompactSlice(request)
	if err == nil || !strings.Contains(err.Error(), "compact-slice-701-702") || !strings.Contains(err.Error(), "resume") || !strings.Contains(err.Error(), "handoff") || !strings.Contains(err.Error(), "recovery") {
		t.Fatalf("ExecuteCompactSlice() error = %v, want affected slice and actionable scope-drift guidance", err)
	}
	assertCompactSliceFailurePreserved(t, repo, request.ScenarioEvidence)
	if contents, readErr := os.ReadFile(filepath.Join(repo, "unexpected.txt")); readErr != nil || string(contents) != "scope drift\n" {
		t.Fatalf("scope-drift change was not preserved: %q, %v", contents, readErr)
	}
}

func assertCompactSliceFailurePreserved(t *testing.T, repo string, scenarios []CompactSliceScenarioEvidence) {
	t.Helper()
	if commits := gitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "1" {
		t.Fatalf("checkpoint commits = %s, want no partial compact-slice checkpoint", commits)
	}
	if diff := gitOutput(t, repo, "diff", "--", "internal/workflow/slice.go"); !strings.Contains(diff, "failedCompactSlice") {
		t.Fatalf("local slice changes were not preserved: %s", diff)
	}
	for _, evidencePath := range []string{scenarios[0].RedEvidencePath, scenarios[0].GreenEvidencePath, scenarios[0].RefactorEvidencePath, scenarios[0].FocusedTestEvidencePath} {
		if _, err := os.Stat(evidencePath); err != nil {
			t.Fatalf("earlier scenario evidence %q was not preserved: %v", evidencePath, err)
		}
	}
}

func compactSliceFailureRequest(t *testing.T, sliceID string) (string, CompactSliceRequest) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "feature/compact-slice-failure")
	configureTestGitIdentity(t, repo)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n")
	runGit(t, repo, "add", ".gitignore", "internal/workflow/slice.go")
	runGit(t, repo, "commit", "-m", "test: establish compact slice failure baseline")

	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "checkpoint_mode: compact_slice\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "approved_scenarios: [SCN-701, SCN-702]\nnext_scenario: SCN-701\n")
	localInspection := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
	mustWrite(t, localInspection, `{"owners":"resolved","invariants":"resolved"}`+"\n")
	capsuleDecision := filepath.Join(repo, ".rotta", "current", "evidence", "capsule-decision-none-required.json")
	decision, err := json.Marshal(localScopeCapsuleDecision{
		CapsuleDecision:        CapsuleDecisionNoneRequired,
		ScenarioOrSlice:        "SCN-701",
		StatePath:              filepath.Join(repo, ".rotta", "current", "state.yaml"),
		EvidencePath:           localInspection,
		FocusedActions:         1,
		TopLevelComponentCount: 1,
		DirectDependentCount:   0,
	})
	if err != nil {
		t.Fatalf("marshal none-required capsule decision: %v", err)
	}
	mustWrite(t, capsuleDecision, string(decision))

	scenarios := make([]CompactSliceScenarioEvidence, 0, 2)
	for _, scenarioID := range []string{"SCN-701", "SCN-702"} {
		scenario := CompactSliceScenarioEvidence{
			ScenarioID:              scenarioID,
			RedEvidencePath:         filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-red.txt"),
			GreenEvidencePath:       filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-green.txt"),
			RefactorEvidencePath:    filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-refactor.txt"),
			FocusedTestEvidencePath: filepath.Join(repo, ".rotta", "current", "evidence", scenarioID+"-focused-test.txt"),
		}
		for _, evidencePath := range evidencePathsForCompactSlice([]CompactSliceScenarioEvidence{scenario}) {
			if evidencePath != scenarioID {
				mustWrite(t, evidencePath, scenarioID+" evidence\n")
			}
		}
		scenarios = append(scenarios, scenario)
	}
	mustWrite(t, filepath.Join(repo, "internal", "workflow", "slice.go"), "package workflow\n\nfunc failedCompactSlice() {}\n")

	return repo, CompactSliceRequest{
		FeatureWorktree:      repo,
		SliceID:              sliceID,
		ComponentScope:       "internal/workflow",
		ScenarioEvidence:     scenarios,
		ExpectedChangedPaths: []string{"internal/workflow/slice.go"},
		CapsuleDecisionPath:  capsuleDecision,
		RunFullValidation:    func() error { return nil },
	}
}

func evidencePathsForCompactSlice(scenarios []CompactSliceScenarioEvidence) []string {
	var paths []string
	for _, scenario := range scenarios {
		paths = append(paths, scenario.ScenarioID, scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath)
	}
	return paths
}
