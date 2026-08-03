package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	CapsuleDecisionNoneRequired = "none-required"
	CapsuleDecisionCreated      = "created"
	maxFocusedLocalActions      = 8
	maxLocalScopeComponents     = 2
	maxLocalScopeDependents     = 5
	maxCapsuleLines             = 120
	maxCapsuleBytes             = 12 * 1024
	maxCapsuleFiles             = 12
	maxCapsuleSymbols           = 20
	maxCapsuleTestCommands      = 5
	capsuleFingerprintPrefix    = "- Capsule fingerprint: "
	capsuleMetadataPrefix       = "<!-- rotta-capsule/v1 "
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

// ExplorationCapsuleRequest contains the bounded local-inspection facts and
// the selected capsule content needed to delegate cross-component work.
type ExplorationCapsuleRequest struct {
	FeatureWorktree       string
	CapsuleID             string
	ScenarioOrSlice       string
	FocusedActions        int
	OwnerResolved         bool
	InvariantResolved     bool
	TopLevelComponents    []string
	DirectDependents      []string
	Objective             string
	InScope               []string
	OutOfScope            []string
	Files                 []string
	Symbols               []string
	Invariants            []string
	TestCommands          []string
	Risks                 []string
	UnresolvedBlockers    []string
	ManifestFingerprint   string
	ContractFingerprint   string
	PolicyFingerprint     string
	RequiredEvidencePaths []string
	BoundExhausted        bool
	Delegate              func(ImplementationCapsuleInput) error
}

// ImplementationCapsuleInput is the complete exploration context an
// implementation delegate may receive.
type ImplementationCapsuleInput struct {
	CapsulePath           string
	ScenarioOrSlice       string
	RequiredEvidencePaths []string
}

// ExplorationCapsuleReport identifies a persisted, fingerprint-bound capsule.
type ExplorationCapsuleReport struct {
	CapsuleID          string
	CapsulePath        string
	CapsuleFingerprint string
	ScenarioOrSlice    string
	DecisionPath       string
}

// ExplorationCapsuleResumeRequest supplies only the current bindings and
// required evidence needed to safely reuse a persisted capsule.
type ExplorationCapsuleResumeRequest struct {
	CapsulePath           string
	ScenarioOrSlice       string
	ManifestFingerprint   string
	ContractFingerprint   string
	PolicyFingerprint     string
	RequiredEvidencePaths []string
	Delegate              func(ImplementationCapsuleInput) error
}

type explorationCapsuleDecision struct {
	CapsuleDecision       string   `json:"capsule_decision"`
	CapsuleID             string   `json:"capsule_id"`
	CapsuleFingerprint    string   `json:"capsule_fingerprint"`
	ScenarioOrSlice       string   `json:"scenario_or_slice"`
	StatePath             string   `json:"state_path"`
	RequiredEvidencePaths []string `json:"required_evidence_paths"`
}

type explorationCapsuleMetadata struct {
	ScenarioOrSlice string `json:"scenario_or_slice"`
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

// CreateExplorationCapsule records a bounded capsule, links it to the current
// execution evidence, and delegates only its path, scenario, and evidence.
func CreateExplorationCapsule(request ExplorationCapsuleRequest) (ExplorationCapsuleReport, error) {
	if err := validateExplorationCapsuleRequest(request); err != nil {
		return ExplorationCapsuleReport{}, err
	}
	worktree, err := filepath.Abs(request.FeatureWorktree)
	if err != nil {
		return ExplorationCapsuleReport{}, fmt.Errorf("resolve feature worktree: %w", err)
	}
	statePath := filepath.Join(worktree, ".rotta", "current", "state.yaml")
	currentScenarioOrSlice, err := readCurrentScenarioOrSlice(statePath)
	if err != nil {
		return ExplorationCapsuleReport{}, err
	}
	if currentScenarioOrSlice != request.ScenarioOrSlice {
		return ExplorationCapsuleReport{}, fmt.Errorf("current state scenario or slice %q does not match requested %q", currentScenarioOrSlice, request.ScenarioOrSlice)
	}
	evidencePaths, err := requiredCapsuleEvidencePaths(worktree, request.RequiredEvidencePaths)
	if err != nil {
		return ExplorationCapsuleReport{}, err
	}

	capsuleContents, fingerprint, err := renderExplorationCapsule(request)
	if err != nil {
		return ExplorationCapsuleReport{}, err
	}
	capsulePath := filepath.Join(worktree, ".rotta", "current", "capsules", request.CapsuleID+".md")
	if err := os.MkdirAll(filepath.Dir(capsulePath), 0o700); err != nil {
		return ExplorationCapsuleReport{}, fmt.Errorf("create capsule directory: %w", err)
	}
	if err := os.WriteFile(capsulePath, []byte(capsuleContents), 0o600); err != nil {
		return ExplorationCapsuleReport{}, fmt.Errorf("write exploration capsule: %w", err)
	}

	decisionPath := filepath.Join(worktree, ".rotta", "current", "evidence", "capsule-decision-"+request.CapsuleID+".json")
	decision, err := json.Marshal(explorationCapsuleDecision{
		CapsuleDecision:       CapsuleDecisionCreated,
		CapsuleID:             request.CapsuleID,
		CapsuleFingerprint:    fingerprint,
		ScenarioOrSlice:       request.ScenarioOrSlice,
		StatePath:             statePath,
		RequiredEvidencePaths: evidencePaths,
	})
	if err != nil {
		return ExplorationCapsuleReport{}, fmt.Errorf("serialize exploration capsule decision: %w", err)
	}
	if err := os.WriteFile(decisionPath, decision, 0o600); err != nil {
		return ExplorationCapsuleReport{}, fmt.Errorf("write exploration capsule decision: %w", err)
	}

	report := ExplorationCapsuleReport{
		CapsuleID:          request.CapsuleID,
		CapsulePath:        capsulePath,
		CapsuleFingerprint: fingerprint,
		ScenarioOrSlice:    request.ScenarioOrSlice,
		DecisionPath:       decisionPath,
	}
	if request.BoundExhausted {
		return report, fmt.Errorf("exploration capsule bound exhausted: %s", strings.Join(request.UnresolvedBlockers, "; "))
	}
	if err := request.Delegate(ImplementationCapsuleInput{
		CapsulePath:           capsulePath,
		ScenarioOrSlice:       request.ScenarioOrSlice,
		RequiredEvidencePaths: evidencePaths,
	}); err != nil {
		return ExplorationCapsuleReport{}, err
	}
	return report, nil
}

func validateExplorationCapsuleRequest(request ExplorationCapsuleRequest) error {
	if request.CapsuleID == "" || filepath.Base(request.CapsuleID) != request.CapsuleID {
		return fmt.Errorf("exploration capsule requires a safe capsule ID")
	}
	if request.ScenarioOrSlice == "" {
		return fmt.Errorf("exploration capsule requires a current scenario or slice")
	}
	if request.FocusedActions < 0 || request.FocusedActions > maxFocusedLocalActions {
		return fmt.Errorf("exploration capsule requires at most eight focused actions")
	}
	if request.OwnerResolved && request.InvariantResolved && len(request.TopLevelComponents) <= maxLocalScopeComponents && len(request.DirectDependents) <= maxLocalScopeDependents {
		return fmt.Errorf("exploration capsule requires unresolved ownership or invariants, more than two components, or more than five direct dependents")
	}
	if request.Objective == "" || len(request.InScope) == 0 || len(request.OutOfScope) == 0 || len(request.Invariants) == 0 || len(request.Risks) == 0 || request.ManifestFingerprint == "" || request.ContractFingerprint == "" || request.PolicyFingerprint == "" {
		return fmt.Errorf("exploration capsule requires objective, scope, invariants, risks, and fingerprints")
	}
	if len(request.Files) > maxCapsuleFiles || len(request.Symbols) > maxCapsuleSymbols || len(request.TestCommands) > maxCapsuleTestCommands {
		return fmt.Errorf("exploration capsule exceeds bounded files, symbols, or test commands")
	}
	if len(request.RequiredEvidencePaths) == 0 {
		return fmt.Errorf("exploration capsule requires local inspection evidence")
	}
	if request.BoundExhausted && len(request.UnresolvedBlockers) == 0 {
		return fmt.Errorf("bound-exhausted exploration capsule requires an unresolved blocker")
	}
	if request.Delegate == nil {
		return fmt.Errorf("exploration capsule requires an implementation delegate")
	}
	return nil
}

func requiredCapsuleEvidencePaths(worktree string, requestedPaths []string) ([]string, error) {
	paths := make([]string, 0, len(requestedPaths))
	for _, requestedPath := range requestedPaths {
		path, err := localScopeEvidencePath(worktree, requestedPath)
		if err != nil {
			return nil, err
		}
		if err := requireRegularFile(path); err != nil {
			return nil, fmt.Errorf("exploration capsule evidence: %w", err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func renderExplorationCapsule(request ExplorationCapsuleRequest) (string, string, error) {
	metadata, err := json.Marshal(explorationCapsuleMetadata{ScenarioOrSlice: request.ScenarioOrSlice})
	if err != nil {
		return "", "", fmt.Errorf("serialize exploration capsule metadata: %w", err)
	}
	var capsule strings.Builder
	capsule.WriteString("# Exploration capsule: ")
	capsule.WriteString(request.CapsuleID)
	capsule.WriteString("\n")
	capsule.WriteString(capsuleMetadataPrefix)
	capsule.Write(metadata)
	capsule.WriteString(" -->\n\n## Objective\n")
	capsule.WriteString(request.Objective)
	capsule.WriteString("\n\n## In scope\n")
	writeCapsuleList(&capsule, request.InScope)
	capsule.WriteString("\n## Out of scope\n")
	writeCapsuleList(&capsule, request.OutOfScope)
	capsule.WriteString("\n## Files\n")
	writeCapsuleList(&capsule, request.Files)
	capsule.WriteString("\n## Symbols\n")
	writeCapsuleList(&capsule, request.Symbols)
	capsule.WriteString("\n## Invariants\n")
	writeCapsuleList(&capsule, request.Invariants)
	capsule.WriteString("\n## Test commands\n")
	writeCapsuleList(&capsule, request.TestCommands)
	capsule.WriteString("\n## Risks\n")
	writeCapsuleList(&capsule, request.Risks)
	capsule.WriteString("\n## Unresolved blockers\n")
	writeCapsuleList(&capsule, request.UnresolvedBlockers)
	if request.BoundExhausted {
		capsule.WriteString("- Bound exhausted: true\n")
	}
	capsule.WriteString("\n## Bindings\n")
	fmt.Fprintf(&capsule, "- Manifest fingerprint: %s\n- Contract fingerprint: %s\n- Policy fingerprint: %s\n", request.ManifestFingerprint, request.ContractFingerprint, request.PolicyFingerprint)

	contents := capsule.String()
	fingerprintBytes := sha256.Sum256([]byte(contents))
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	contents += capsuleFingerprintPrefix + fingerprint + "\n"
	if len(contents) > maxCapsuleBytes || len(strings.Split(strings.TrimSuffix(contents, "\n"), "\n")) > maxCapsuleLines {
		return "", "", fmt.Errorf("exploration capsule exceeds %d lines or %d bytes", maxCapsuleLines, maxCapsuleBytes)
	}
	return contents, fingerprint, nil
}

// ResumeExplorationCapsule reuses a capsule only when its persisted bindings
// remain current; stale and bound-exhausted capsules stop before delegation.
func ResumeExplorationCapsule(request ExplorationCapsuleResumeRequest) error {
	if request.CapsulePath == "" || request.ScenarioOrSlice == "" || request.ManifestFingerprint == "" || request.ContractFingerprint == "" || request.PolicyFingerprint == "" || request.Delegate == nil {
		return fmt.Errorf("resume exploration capsule requires path, current bindings, scenario or slice, and delegate")
	}
	capsulePath, worktree, err := featureWorktreeForCapsulePath(request.CapsulePath)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(capsulePath)
	if err != nil {
		return fmt.Errorf("read exploration capsule: %w", err)
	}
	if err := verifyCapsuleFingerprint(string(contents)); err != nil {
		return err
	}
	metadata, err := readCapsuleMetadata(string(contents))
	if err != nil {
		return err
	}
	currentScenarioOrSlice, err := readCurrentScenarioOrSlice(filepath.Join(worktree, ".rotta", "current", "state.yaml"))
	if err != nil {
		return err
	}
	if metadata.ScenarioOrSlice != request.ScenarioOrSlice || currentScenarioOrSlice != metadata.ScenarioOrSlice {
		return fmt.Errorf("exploration capsule is stale: scenario or slice does not match current feature-local state")
	}
	if strings.Contains(string(contents), "- Bound exhausted: true\n") {
		return fmt.Errorf("exploration capsule bound exhausted; unresolved blocker recorded")
	}
	if err := verifyCapsuleBindings(string(contents), request); err != nil {
		return err
	}
	for _, evidencePath := range request.RequiredEvidencePaths {
		if err := requireRegularFile(evidencePath); err != nil {
			return fmt.Errorf("resume exploration capsule evidence: %w", err)
		}
	}
	return request.Delegate(ImplementationCapsuleInput{
		CapsulePath:           capsulePath,
		ScenarioOrSlice:       request.ScenarioOrSlice,
		RequiredEvidencePaths: append([]string(nil), request.RequiredEvidencePaths...),
	})
}

func featureWorktreeForCapsulePath(capsulePath string) (string, string, error) {
	absCapsulePath, err := filepath.Abs(capsulePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve exploration capsule path: %w", err)
	}
	capsulesDirectory := filepath.Dir(absCapsulePath)
	currentDirectory := filepath.Dir(capsulesDirectory)
	rottaDirectory := filepath.Dir(currentDirectory)
	if filepath.Base(capsulesDirectory) != "capsules" || filepath.Base(currentDirectory) != "current" || filepath.Base(rottaDirectory) != ".rotta" {
		return "", "", fmt.Errorf("exploration capsule is stale: path is outside feature-local capsules")
	}
	return absCapsulePath, filepath.Dir(rottaDirectory), nil
}

func readCapsuleMetadata(contents string) (explorationCapsuleMetadata, error) {
	for _, line := range strings.Split(contents, "\n") {
		if !strings.HasPrefix(line, capsuleMetadataPrefix) {
			continue
		}
		metadataJSON, found := strings.CutSuffix(strings.TrimPrefix(line, capsuleMetadataPrefix), " -->")
		if !found {
			break
		}
		var metadata explorationCapsuleMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil || metadata.ScenarioOrSlice == "" {
			break
		}
		return metadata, nil
	}
	return explorationCapsuleMetadata{}, fmt.Errorf("exploration capsule is stale: scenario or slice binding is missing")
}

func verifyCapsuleFingerprint(contents string) error {
	fingerprintOffset := strings.LastIndex(contents, capsuleFingerprintPrefix)
	if fingerprintOffset < 0 {
		return fmt.Errorf("exploration capsule is stale: capsule fingerprint is missing")
	}
	recordedFingerprint := strings.TrimPrefix(contents[fingerprintOffset:], capsuleFingerprintPrefix)
	if !strings.HasSuffix(recordedFingerprint, "\n") {
		return fmt.Errorf("exploration capsule is stale: capsule fingerprint does not match")
	}
	recordedFingerprint = strings.TrimSuffix(recordedFingerprint, "\n")
	actualFingerprint := sha256.Sum256([]byte(contents[:fingerprintOffset]))
	if recordedFingerprint != hex.EncodeToString(actualFingerprint[:]) {
		return fmt.Errorf("exploration capsule is stale: capsule fingerprint does not match")
	}
	return nil
}

func verifyCapsuleBindings(contents string, request ExplorationCapsuleResumeRequest) error {
	for _, binding := range []struct {
		name  string
		value string
	}{
		{"Manifest", request.ManifestFingerprint},
		{"Contract", request.ContractFingerprint},
		{"Policy", request.PolicyFingerprint},
	} {
		if !strings.Contains(contents, "- "+binding.name+" fingerprint: "+binding.value+"\n") {
			return fmt.Errorf("exploration capsule is stale: %s fingerprint does not match", strings.ToLower(binding.name))
		}
	}
	return nil
}

func writeCapsuleList(capsule *strings.Builder, values []string) {
	if len(values) == 0 {
		capsule.WriteString("- none\n")
		return
	}
	for _, value := range values {
		capsule.WriteString("- ")
		capsule.WriteString(value)
		capsule.WriteByte('\n')
	}
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
