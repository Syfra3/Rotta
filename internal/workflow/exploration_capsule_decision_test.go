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

// REQ-086 → SCN-617 → TestSCN617_CrossComponentUncertaintyCreatesBoundedTraceableCapsule
func TestSCN617_CrossComponentUncertaintyCreatesBoundedTraceableCapsule(t *testing.T) {
	// Scenario: Cross-component uncertainty produces a bounded traceable capsule
	repo := t.TempDir()
	statePath := filepath.Join(repo, ".rotta", "current", "state.yaml")
	evidencePath := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
	mustWrite(t, statePath, "next_scenario: SCN-617\n")
	mustWrite(t, evidencePath, "{\"owner\":\"unresolved\"}\n")

	var implementationInput ImplementationCapsuleInput
	report, err := CreateExplorationCapsule(ExplorationCapsuleRequest{
		FeatureWorktree:       repo,
		CapsuleID:             "scn-617-cross-component",
		ScenarioOrSlice:       "SCN-617",
		FocusedActions:        8,
		OwnerResolved:         false,
		InvariantResolved:     true,
		TopLevelComponents:    []string{"internal/workflow", "cmd/rotta", "internal/tui"},
		Objective:             "resolve ownership across the workflow boundary",
		InScope:               []string{"capsule decision", "delegation input"},
		OutOfScope:            []string{"lifecycle transitions"},
		Files:                 []string{"internal/workflow/exploration_capsule_decision.go"},
		Symbols:               []string{"CreateExplorationCapsule"},
		Invariants:            []string{"implementation receives no transcript"},
		TestCommands:          []string{"go test ./internal/workflow -run '^TestSCN617_'"},
		Risks:                 []string{"owner may remain unresolved"},
		UnresolvedBlockers:    []string{"confirm cross-component owner"},
		ManifestFingerprint:   "manifest-fingerprint",
		ContractFingerprint:   "contract-fingerprint",
		PolicyFingerprint:     "policy-fingerprint",
		RequiredEvidencePaths: []string{evidencePath},
		Delegate: func(input ImplementationCapsuleInput) error {
			implementationInput = input
			return nil
		},
	})
	if err != nil {
		t.Fatalf("CreateExplorationCapsule() error = %v", err)
	}
	if report.CapsuleID != "scn-617-cross-component" || report.ScenarioOrSlice != "SCN-617" || report.CapsuleFingerprint == "" {
		t.Fatalf("report = %#v, want traceable SCN-617 capsule", report)
	}
	wantCapsulePath := filepath.Join(repo, ".rotta", "current", "capsules", "scn-617-cross-component.md")
	if report.CapsulePath != wantCapsulePath {
		t.Fatalf("capsule path = %q, want %q", report.CapsulePath, wantCapsulePath)
	}
	if implementationInput.CapsulePath != wantCapsulePath || implementationInput.ScenarioOrSlice != "SCN-617" || len(implementationInput.RequiredEvidencePaths) != 1 || implementationInput.RequiredEvidencePaths[0] != evidencePath {
		t.Fatalf("implementation input = %#v, want only capsule path, current scenario, and required evidence", implementationInput)
	}

	capsule, err := os.ReadFile(report.CapsulePath)
	if err != nil {
		t.Fatalf("read capsule: %v", err)
	}
	if lines := len(strings.Split(strings.TrimSuffix(string(capsule), "\n"), "\n")); lines > 120 {
		t.Fatalf("capsule has %d lines, want at most 120", lines)
	}
	if len(capsule) > 12*1024 {
		t.Fatalf("capsule has %d bytes, want at most %d", len(capsule), 12*1024)
	}
	for _, required := range []string{"## Objective", "## In scope", "## Out of scope", "## Files", "## Symbols", "## Invariants", "## Test commands", "## Risks", "## Unresolved blockers", "manifest-fingerprint", report.CapsuleFingerprint} {
		if !strings.Contains(string(capsule), required) {
			t.Fatalf("capsule = %s, missing %q", capsule, required)
		}
	}
}

// REQ-086 → SCN-617 → TestSCN617_BoundExhaustedOrStaleCapsuleBlocksDelegation
func TestSCN617_BoundExhaustedOrStaleCapsuleBlocksDelegation(t *testing.T) {
	// Scenario: Cross-component uncertainty produces a bounded traceable capsule
	newRequest := func(t *testing.T, capsuleID string) ExplorationCapsuleRequest {
		t.Helper()
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "next_scenario: SCN-617\n")
		evidencePath := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
		mustWrite(t, evidencePath, "{\"owner\":\"unresolved\"}\n")
		return ExplorationCapsuleRequest{
			FeatureWorktree:       repo,
			CapsuleID:             capsuleID,
			ScenarioOrSlice:       "SCN-617",
			FocusedActions:        8,
			OwnerResolved:         false,
			InvariantResolved:     true,
			Objective:             "resolve the required owner",
			InScope:               []string{"owner lookup"},
			OutOfScope:            []string{"lifecycle state"},
			Invariants:            []string{"capsule remains bounded"},
			Risks:                 []string{"owner stays unresolved"},
			UnresolvedBlockers:    []string{"owner unresolved"},
			ManifestFingerprint:   "manifest-fingerprint",
			ContractFingerprint:   "contract-fingerprint",
			PolicyFingerprint:     "policy-fingerprint",
			RequiredEvidencePaths: []string{evidencePath},
		}
	}

	t.Run("bound exhausted", func(t *testing.T) {
		request := newRequest(t, "bound-exhausted")
		delegated := false
		request.BoundExhausted = true
		request.Delegate = func(ImplementationCapsuleInput) error {
			delegated = true
			return nil
		}
		report, err := CreateExplorationCapsule(request)
		if err == nil || !strings.Contains(err.Error(), "bound exhausted") {
			t.Fatalf("CreateExplorationCapsule() error = %v, want bound-exhausted blocker", err)
		}
		if delegated {
			t.Fatal("bound-exhausted capsule delegated implementation")
		}
		capsule, readErr := os.ReadFile(report.CapsulePath)
		if readErr != nil || !strings.Contains(string(capsule), "- Bound exhausted: true") || !strings.Contains(string(capsule), "owner unresolved") {
			t.Fatalf("bound-exhausted capsule = %q, read error = %v, want recorded blocker", capsule, readErr)
		}
	})

	t.Run("stale fingerprint", func(t *testing.T) {
		request := newRequest(t, "stale-capsule")
		request.Delegate = func(ImplementationCapsuleInput) error { return nil }
		report, err := CreateExplorationCapsule(request)
		if err != nil {
			t.Fatalf("CreateExplorationCapsule() error = %v", err)
		}

		delegated := false
		err = ResumeExplorationCapsule(ExplorationCapsuleResumeRequest{
			CapsulePath:           report.CapsulePath,
			ScenarioOrSlice:       "SCN-617",
			ManifestFingerprint:   "changed-manifest-fingerprint",
			ContractFingerprint:   "contract-fingerprint",
			PolicyFingerprint:     "policy-fingerprint",
			RequiredEvidencePaths: request.RequiredEvidencePaths,
			Delegate: func(ImplementationCapsuleInput) error {
				delegated = true
				return nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("ResumeExplorationCapsule() error = %v, want stale blocker", err)
		}
		if delegated {
			t.Fatal("stale capsule delegated implementation")
		}
	})

	t.Run("matching fingerprint reuses the original capsule only", func(t *testing.T) {
		request := newRequest(t, "tampered-capsule")
		request.Delegate = func(ImplementationCapsuleInput) error { return nil }
		report, err := CreateExplorationCapsule(request)
		if err != nil {
			t.Fatalf("CreateExplorationCapsule() error = %v", err)
		}

		var resumed ImplementationCapsuleInput
		err = ResumeExplorationCapsule(ExplorationCapsuleResumeRequest{
			CapsulePath:           report.CapsulePath,
			ScenarioOrSlice:       "SCN-617",
			ManifestFingerprint:   request.ManifestFingerprint,
			ContractFingerprint:   request.ContractFingerprint,
			PolicyFingerprint:     request.PolicyFingerprint,
			RequiredEvidencePaths: request.RequiredEvidencePaths,
			Delegate: func(input ImplementationCapsuleInput) error {
				resumed = input
				return nil
			},
		})
		if err != nil || resumed.CapsulePath != report.CapsulePath || resumed.ScenarioOrSlice != "SCN-617" {
			t.Fatalf("ResumeExplorationCapsule() error = %v, input = %#v, want matching capsule delegation", err, resumed)
		}

		capsule, err := os.ReadFile(report.CapsulePath)
		if err != nil {
			t.Fatalf("read capsule: %v", err)
		}
		if err := os.WriteFile(report.CapsulePath, append(capsule, []byte("unbounded transcript detail\n")...), 0o600); err != nil {
			t.Fatalf("tamper capsule: %v", err)
		}
		delegated := false
		err = ResumeExplorationCapsule(ExplorationCapsuleResumeRequest{
			CapsulePath:           report.CapsulePath,
			ScenarioOrSlice:       "SCN-617",
			ManifestFingerprint:   request.ManifestFingerprint,
			ContractFingerprint:   request.ContractFingerprint,
			PolicyFingerprint:     request.PolicyFingerprint,
			RequiredEvidencePaths: request.RequiredEvidencePaths,
			Delegate: func(ImplementationCapsuleInput) error {
				delegated = true
				return nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("ResumeExplorationCapsule() error = %v, want stale tamper blocker", err)
		}
		if delegated {
			t.Fatal("tampered capsule delegated implementation")
		}
	})
}

// REQ-086 → SCN-617 → TestSCN617_StateScenarioDriftBlocksCapsuleResume
func TestSCN617_StateScenarioDriftBlocksCapsuleResume(t *testing.T) {
	// Scenario: Cross-component uncertainty produces a bounded traceable capsule
	repo := t.TempDir()
	statePath := filepath.Join(repo, ".rotta", "current", "state.yaml")
	evidencePath := filepath.Join(repo, ".rotta", "current", "evidence", "local-inspection.json")
	mustWrite(t, statePath, "next_scenario: SCN-617\n")
	mustWrite(t, evidencePath, "{\"owner\":\"unresolved\"}\n")

	request := ExplorationCapsuleRequest{
		FeatureWorktree:       repo,
		CapsuleID:             "state-drift-capsule",
		ScenarioOrSlice:       "SCN-617",
		FocusedActions:        8,
		OwnerResolved:         false,
		InvariantResolved:     true,
		Objective:             "resolve the required owner",
		InScope:               []string{"owner lookup"},
		OutOfScope:            []string{"lifecycle state"},
		Invariants:            []string{"capsule remains bounded"},
		Risks:                 []string{"owner stays unresolved"},
		ManifestFingerprint:   "manifest-fingerprint",
		ContractFingerprint:   "contract-fingerprint",
		PolicyFingerprint:     "policy-fingerprint",
		RequiredEvidencePaths: []string{evidencePath},
		Delegate:              func(ImplementationCapsuleInput) error { return nil },
	}
	report, err := CreateExplorationCapsule(request)
	if err != nil {
		t.Fatalf("CreateExplorationCapsule() error = %v", err)
	}
	mustWrite(t, statePath, "next_scenario: SCN-618\n")

	delegated := false
	err = ResumeExplorationCapsule(ExplorationCapsuleResumeRequest{
		CapsulePath:           report.CapsulePath,
		ScenarioOrSlice:       "SCN-617",
		ManifestFingerprint:   request.ManifestFingerprint,
		ContractFingerprint:   request.ContractFingerprint,
		PolicyFingerprint:     request.PolicyFingerprint,
		RequiredEvidencePaths: request.RequiredEvidencePaths,
		Delegate: func(ImplementationCapsuleInput) error {
			delegated = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "stale") || !strings.Contains(err.Error(), "scenario") {
		t.Fatalf("ResumeExplorationCapsule() error = %v, want stale scenario blocker", err)
	}
	if delegated {
		t.Fatal("state-drifted capsule delegated implementation")
	}
}
