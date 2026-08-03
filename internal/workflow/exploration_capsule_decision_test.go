package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-086 → SCN-616 → TestSCN616_LocalScopeDelegatesWithoutExplorationCapsule
func TestSCN616_LocalScopeDelegatesWithoutExplorationCapsule(t *testing.T) {
	// Scenario: Local scope below the uncertainty threshold does not create an exploration capsule
	for _, testCase := range []struct {
		name          string
		request       LocalScopeDelegationRequest
		wantError     string
		wantDelegated bool
	}{
		{
			name:          "resolved local scope",
			request:       LocalScopeDelegationRequest{FocusedActions: 8, OwnersResolved: true, InvariantsResolved: true, TopLevelComponents: []string{"internal/workflow", "cmd/rotta"}, DirectDependents: []string{"cmd/rotta", "internal/tui", "internal/installer", "internal/workflow_test", "docs"}},
			wantDelegated: true,
		},
		{
			name:      "unresolved owner",
			request:   LocalScopeDelegationRequest{FocusedActions: 8, InvariantsResolved: true},
			wantError: "owners",
		},
		{
			name:      "unresolved invariant",
			request:   LocalScopeDelegationRequest{FocusedActions: 8, OwnersResolved: true},
			wantError: "invariants",
		},
		{
			name:      "more than eight focused actions",
			request:   LocalScopeDelegationRequest{FocusedActions: 9, OwnersResolved: true, InvariantsResolved: true},
			wantError: "eight",
		},
		{
			name:      "more than two top level components",
			request:   LocalScopeDelegationRequest{FocusedActions: 8, OwnersResolved: true, InvariantsResolved: true, TopLevelComponents: []string{"one", "two", "three"}},
			wantError: "components",
		},
		{
			name:      "more than five direct dependents",
			request:   LocalScopeDelegationRequest{FocusedActions: 8, OwnersResolved: true, InvariantsResolved: true, DirectDependents: []string{"one", "two", "three", "four", "five", "six"}},
			wantError: "dependents",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := t.TempDir()
			statePath := filepath.Join(repo, ".rotta", "current", "state.yaml")
			evidencePath := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
			mustWrite(t, statePath, "next_scenario: SCN-616\n")
			mustWrite(t, evidencePath, "{\"owners\":\"resolved\",\"invariants\":\"resolved\"}\n")

			delegated := []string{}
			request := testCase.request
			request.FeatureWorktree = repo
			request.ScenarioOrSlice = "SCN-616"
			request.EvidencePath = evidencePath
			request.Delegate = func(currentScenarioOrSlice string) error {
				delegated = append(delegated, currentScenarioOrSlice)
				return nil
			}

			report, err := DelegateLocalApprovedWork(request)
			if testCase.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
					t.Fatalf("DelegateLocalApprovedWork() error = %v, want %q", err, testCase.wantError)
				}
				if len(delegated) != 0 {
					t.Fatalf("delegated %v despite unsafe local scope", delegated)
				}
				return
			}
			if err != nil {
				t.Fatalf("DelegateLocalApprovedWork() error = %v", err)
			}
			if !testCase.wantDelegated || len(delegated) != 1 || delegated[0] != "SCN-616" {
				t.Fatalf("delegated = %v, want only current SCN-616", delegated)
			}
			if report.ScenarioOrSlice != "SCN-616" || report.CapsuleDecision != CapsuleDecisionNoneRequired {
				t.Fatalf("report = %#v, want SCN-616 with an explicit none-required decision", report)
			}
			if strings.Contains(report.DecisionPath, string(filepath.Separator)+"capsules"+string(filepath.Separator)) {
				t.Fatalf("decision path = %q, must not create a capsule", report.DecisionPath)
			}
			decision, err := os.ReadFile(report.DecisionPath)
			if err != nil {
				t.Fatalf("read capsule decision: %v", err)
			}
			if !strings.Contains(string(decision), `"capsule_decision":"none-required"`) || strings.Contains(string(decision), "capsule_path") {
				t.Fatalf("capsule decision = %s, want explicit none-required without a capsule reference", decision)
			}
			if !strings.Contains(string(decision), statePath) || !strings.Contains(string(decision), evidencePath) {
				t.Fatalf("capsule decision = %s, want current state and evidence references", decision)
			}
		})
	}
}
