package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	CapsuleDecisionNoneRequired = "none-required"
	maxFocusedLocalActions      = 8
	maxLocalScopeComponents     = 2
	maxLocalScopeDependents     = 5
)

// LocalScopeDelegationRequest contains only the completed local-inspection
// facts needed to decide whether approved work may proceed without a capsule.
type LocalScopeDelegationRequest struct {
	FeatureWorktree    string
	ScenarioOrSlice    string
	FocusedActions     int
	OwnersResolved     bool
	InvariantsResolved bool
	TopLevelComponents []string
	DirectDependents   []string
	EvidencePath       string
	Delegate           func(currentScenarioOrSlice string) error
}

// LocalScopeDelegationReport identifies the direct delegation and the durable
// evidence of its explicit no-capsule decision.
type LocalScopeDelegationReport struct {
	ScenarioOrSlice string
	CapsuleDecision string
	DecisionPath    string
}

type localScopeCapsuleDecision struct {
	CapsuleDecision        string `json:"capsule_decision"`
	ScenarioOrSlice        string `json:"scenario_or_slice"`
	StatePath              string `json:"state_path"`
	EvidencePath           string `json:"evidence_path"`
	FocusedActions         int    `json:"focused_actions"`
	TopLevelComponentCount int    `json:"top_level_component_count"`
	DirectDependentCount   int    `json:"direct_dependent_count"`
}

// DelegateLocalApprovedWork records a feature-local none-required capsule
// decision, then delegates only the current approved scenario or slice.
func DelegateLocalApprovedWork(request LocalScopeDelegationRequest) (LocalScopeDelegationReport, error) {
	if err := validateLocalScope(request); err != nil {
		return LocalScopeDelegationReport{}, err
	}
	if request.Delegate == nil {
		return LocalScopeDelegationReport{}, fmt.Errorf("local scope delegation requires a delegate")
	}

	worktree, err := filepath.Abs(request.FeatureWorktree)
	if err != nil {
		return LocalScopeDelegationReport{}, fmt.Errorf("resolve feature worktree: %w", err)
	}
	statePath := filepath.Join(worktree, ".rotta", "current", "state.yaml")
	currentScenarioOrSlice, err := readCurrentScenarioOrSlice(statePath)
	if err != nil {
		return LocalScopeDelegationReport{}, err
	}
	if currentScenarioOrSlice != request.ScenarioOrSlice {
		return LocalScopeDelegationReport{}, fmt.Errorf("current state scenario or slice %q does not match requested %q", currentScenarioOrSlice, request.ScenarioOrSlice)
	}

	evidencePath, err := localScopeEvidencePath(worktree, request.EvidencePath)
	if err != nil {
		return LocalScopeDelegationReport{}, err
	}
	if err := requireRegularFile(evidencePath); err != nil {
		return LocalScopeDelegationReport{}, fmt.Errorf("local inspection evidence: %w", err)
	}

	decisionPath := filepath.Join(worktree, ".rotta", "current", "evidence", "capsule-decision-none-required.json")
	decision, err := json.Marshal(localScopeCapsuleDecision{
		CapsuleDecision:        CapsuleDecisionNoneRequired,
		ScenarioOrSlice:        request.ScenarioOrSlice,
		StatePath:              statePath,
		EvidencePath:           evidencePath,
		FocusedActions:         request.FocusedActions,
		TopLevelComponentCount: len(request.TopLevelComponents),
		DirectDependentCount:   len(request.DirectDependents),
	})
	if err != nil {
		return LocalScopeDelegationReport{}, fmt.Errorf("serialize local capsule decision: %w", err)
	}
	if err := os.WriteFile(decisionPath, decision, 0o600); err != nil {
		return LocalScopeDelegationReport{}, fmt.Errorf("write local capsule decision: %w", err)
	}
	if err := request.Delegate(request.ScenarioOrSlice); err != nil {
		return LocalScopeDelegationReport{}, err
	}

	return LocalScopeDelegationReport{
		ScenarioOrSlice: request.ScenarioOrSlice,
		CapsuleDecision: CapsuleDecisionNoneRequired,
		DecisionPath:    decisionPath,
	}, nil
}

func validateLocalScope(request LocalScopeDelegationRequest) error {
	if request.ScenarioOrSlice == "" {
		return fmt.Errorf("local scope delegation requires a current scenario or slice")
	}
	if request.FocusedActions > maxFocusedLocalActions {
		return fmt.Errorf("local scope inspection exceeds eight focused actions")
	}
	if !request.OwnersResolved {
		return fmt.Errorf("local scope inspection leaves owners unresolved")
	}
	if !request.InvariantsResolved {
		return fmt.Errorf("local scope inspection leaves invariants unresolved")
	}
	if len(request.TopLevelComponents) > maxLocalScopeComponents {
		return fmt.Errorf("local scope inspection spans more than two top-level components")
	}
	if len(request.DirectDependents) > maxLocalScopeDependents {
		return fmt.Errorf("local scope inspection affects more than five direct dependents")
	}
	return nil
}

func readCurrentScenarioOrSlice(statePath string) (string, error) {
	contents, err := os.ReadFile(statePath)
	if err != nil {
		return "", fmt.Errorf("read current feature-local state: %w", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		key, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(key) == "next_scenario" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", fmt.Errorf("current feature-local state has no next scenario or slice")
}

func localScopeEvidencePath(worktree, evidencePath string) (string, error) {
	evidencePath, err := filepath.Abs(evidencePath)
	if err != nil {
		return "", fmt.Errorf("resolve local inspection evidence: %w", err)
	}
	evidenceRoot := filepath.Join(worktree, ".rotta", "current", "evidence")
	relative, err := filepath.Rel(evidenceRoot, evidencePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("local inspection evidence must remain below %q", evidenceRoot)
	}
	return evidencePath, nil
}
