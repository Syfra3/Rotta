package workflow

import (
	"encoding/json"
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

func evidencePathsForCompactSlice(scenarios []CompactSliceScenarioEvidence) []string {
	var paths []string
	for _, scenario := range scenarios {
		paths = append(paths, scenario.ScenarioID, scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath)
	}
	return paths
}
