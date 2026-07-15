package workflow

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errMalformedFeatureApproval = errors.New("malformed feature approval record")
var errMalformedScenarioReference = errors.New("malformed approved-scenario reference")
var errInvalidScenarioFeaturePath = errors.New("invalid approved-scenario feature path")
var errScenarioIDNotResolvedExactlyOnce = errors.New("approved-scenario ID did not resolve exactly once")
var errScenarioRequirementIDsMismatch = errors.New("approved-scenario requirement IDs do not match feature requirement tags")
var errApprovalBaselineUncommitted = errors.New("approval baseline is not committed")
var errApprovalBaselineUnreachable = errors.New("approval baseline is unreachable")
var errApprovalScopeMismatch = errors.New("approval record has an identity or scenario-scope mismatch")
var errApprovalContractDrift = errors.New("approval record has contract fingerprint drift")

type ContractScope struct {
	SpecPath    string
	FeaturePath string
	ScenarioID  string
}

type ImplementationGateDecision struct {
	Approved bool
	Reason   string
}

func SelectApprovedScenarios(repoRoot string, scope ContractScope, scenarios []string) ([]string, error) {
	selected := make([]string, 0, len(scenarios))
	for _, scenarioID := range scenarios {
		scope.ScenarioID = scenarioID
		decision, err := EvaluateImplementationGate(repoRoot, scope)
		if err != nil {
			return nil, err
		}
		if decision.Approved {
			selected = append(selected, scenarioID)
		}
	}
	return selected, nil
}

func EvaluateImplementationGate(repoRoot string, scope ContractScope) (ImplementationGateDecision, error) {
	approved, err := scopedApprovalContains(repoRoot, scope)
	if err != nil {
		if errors.Is(err, errMalformedFeatureApproval) {
			return ImplementationGateDecision{Reason: "approval record is malformed"}, nil
		}
		if errors.Is(err, errMalformedScenarioReference) {
			return ImplementationGateDecision{Reason: "approved-scenario reference is malformed"}, nil
		}
		if errors.Is(err, errInvalidScenarioFeaturePath) {
			return ImplementationGateDecision{Reason: "approved-scenario feature path is invalid"}, nil
		}
		if errors.Is(err, errScenarioIDNotResolvedExactlyOnce) {
			return ImplementationGateDecision{Reason: "approved-scenario ID did not resolve exactly once"}, nil
		}
		if errors.Is(err, errScenarioRequirementIDsMismatch) {
			return ImplementationGateDecision{Reason: "approved-scenario requirement IDs do not match feature requirement tags"}, nil
		}
		if errors.Is(err, errApprovalBaselineUncommitted) {
			return ImplementationGateDecision{Reason: "approval baseline is not committed"}, nil
		}
		if errors.Is(err, errApprovalBaselineUnreachable) {
			return ImplementationGateDecision{Reason: "approval baseline is unreachable"}, nil
		}
		if errors.Is(err, errApprovalScopeMismatch) {
			return ImplementationGateDecision{Reason: "approval record has an identity or scenario-scope mismatch"}, nil
		}
		if errors.Is(err, errApprovalContractDrift) {
			return ImplementationGateDecision{Reason: "approval record has contract fingerprint drift"}, nil
		}
		return ImplementationGateDecision{}, err
	}
	if approved {
		return ImplementationGateDecision{Approved: true, Reason: "scoped human approval recorded"}, nil
	}
	if scope.SpecPath != "specs/hard_spec.md" {
		return ImplementationGateDecision{
			Approved: false,
			Reason:   fmt.Sprintf("human approval is still required for %s#%s", scope.FeaturePath, scope.ScenarioID),
		}, nil
	}

	return ImplementationGateDecision{
		Approved: false,
		Reason:   "approval record is missing",
	}, nil
}

func scopedApprovalContains(repoRoot string, scope ContractScope) (bool, error) {
	approved, _, err := featureApprovalContains(repoRoot, scope)
	return approved, err
}

func featureApprovalContains(repoRoot string, scope ContractScope) (approved, found bool, err error) {
	file, closeFile, err := openRepositoryFile(repoRoot, featureApprovalPath(scope.FeaturePath))
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, true, err
	}
	defer closeFile()

	inApprovedScenarios := false
	hasFormat := false
	hasContractID := false
	hasStatus := false
	hasFeaturePaths := false
	hasApprovedScenarios := false
	hasFingerprints := false
	hasBaselineConfirmation := false
	hasFeatureIdentity := false
	inFingerprints := false
	fingerprints := map[string]string{}
	baselineCommit := ""
	submissionWorktree := ""
	entryFeaturePath := ""
	entryScenarioID := ""
	entryRequirementIDs := []string(nil)
	approvedRequirementIDs := []string(nil)
	entryFields := map[string]bool{}
	inScenarioEntry := false
	malformedScenarioReference := false
	invalidScenarioFeaturePath := false
	validateScenarioEntry := func() {
		if !inScenarioEntry {
			return
		}
		for _, field := range []string{"feature_path", "scenario_id", "requirement_ids"} {
			if !entryFields[field] {
				malformedScenarioReference = true
			}
		}
		if entryFeaturePath == scope.FeaturePath && entryScenarioID == strings.TrimSpace(scope.ScenarioID) {
			approved = true
			approvedRequirementIDs = entryRequirementIDs
		}
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "format: rotta.feature-approval/v2":
			hasFormat = true
		case "contract_id:", "contract_id: unified-workflow-authority":
			hasContractID = true
		case "status: approved":
			hasStatus = true
		case "feature_paths:":
			hasFeaturePaths = true
		case "approved_scenarios:":
			hasApprovedScenarios = true
		case "contract_fingerprints:":
			hasFingerprints = true
			inFingerprints = true
		case "baseline_confirmation:":
			hasBaselineConfirmation = true
			inFingerprints = false
		}
		if inFingerprints {
			if path, fingerprint, ok := strings.Cut(line, ": "); ok && (path == scope.SpecPath || path == scope.FeaturePath) {
				fingerprints[path] = strings.TrimSpace(fingerprint)
			}
		}
		if value, ok := strings.CutPrefix(line, "baseline_commit: "); ok {
			baselineCommit = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(line, "submission_worktree: "); ok {
			submissionWorktree = strings.TrimSpace(value)
		}
		if line == "approved_scenarios:" {
			inApprovedScenarios = true
			continue
		}
		if value, ok := strings.CutPrefix(line, "- "); ok && !inApprovedScenarios && strings.TrimSpace(value) == scope.FeaturePath {
			hasFeatureIdentity = true
		}
		if !inApprovedScenarios {
			continue
		}
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "-") {
			validateScenarioEntry()
			inApprovedScenarios = false
			continue
		}
		entryLine := line
		if value, ok := strings.CutPrefix(line, "- "); ok {
			validateScenarioEntry()
			inScenarioEntry = true
			entryFields = map[string]bool{}
			entryLine = value
		}
		if field, value, ok := strings.Cut(entryLine, ": "); ok {
			if field != "feature_path" && field != "scenario_id" && field != "requirement_ids" || entryFields[field] || strings.TrimSpace(value) == "" {
				malformedScenarioReference = true
			} else {
				entryFields[field] = true
				if field == "requirement_ids" {
					entryRequirementIDs = parseRequirementIDs(value)
				}
				if field == "feature_path" && !isCanonicalFeaturePath(strings.TrimSpace(value)) {
					invalidScenarioFeaturePath = true
				}
			}
		}
		if value, ok := strings.CutPrefix(line, "- scenario_id: "); ok {
			entryFeaturePath = ""
			entryScenarioID = strings.TrimSpace(value)
			entryRequirementIDs = nil
			continue
		}
		if value, ok := strings.CutPrefix(line, "- feature_path: "); ok {
			entryFeaturePath = strings.TrimSpace(value)
			entryScenarioID = ""
			entryRequirementIDs = nil
			if entryFeaturePath == scope.FeaturePath && entryScenarioID == strings.TrimSpace(scope.ScenarioID) {
				approved = true
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "feature_path: "); ok {
			entryFeaturePath = strings.TrimSpace(value)
			if entryFeaturePath == scope.FeaturePath && entryScenarioID == strings.TrimSpace(scope.ScenarioID) {
				approved = true
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "scenario_id: "); ok {
			entryScenarioID = strings.TrimSpace(value)
			if entryFeaturePath == scope.FeaturePath && entryScenarioID == strings.TrimSpace(scope.ScenarioID) {
				approved = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false, true, err
	}
	validateScenarioEntry()
	if invalidScenarioFeaturePath {
		return false, true, errInvalidScenarioFeaturePath
	}
	if malformedScenarioReference {
		return false, true, errMalformedScenarioReference
	}
	if !hasFormat || !hasContractID || !hasStatus || !hasFeaturePaths || !hasApprovedScenarios || !hasFingerprints || !hasBaselineConfirmation {
		return false, true, errMalformedFeatureApproval
	}
	if !hasFeatureIdentity || !approved {
		return false, true, errApprovalScopeMismatch
	}
	if submissionWorktree != "" && filepath.Clean(submissionWorktree) != filepath.Clean(repoRoot) {
		return false, true, errApprovalScopeMismatch
	}
	for _, path := range []string{scope.SpecPath, scope.FeaturePath} {
		fingerprint, err := contractFileFingerprint(repoRoot, path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, true, errApprovalContractDrift
		}
		if fingerprints[path] != fingerprint {
			return false, true, errApprovalContractDrift
		}
	}
	if !approvalBaselineIsCommitted(repoRoot, baselineCommit) {
		return false, true, errApprovalBaselineUncommitted
	}
	if !approvalBaselineIsReachable(repoRoot, baselineCommit) {
		return false, true, errApprovalBaselineUnreachable
	}
	if _, err := os.Stat(filepath.Join(repoRoot, scope.FeaturePath)); err == nil {
		if !scenarioIDResolvesExactlyOnce(repoRoot, scope.FeaturePath, scope.ScenarioID) {
			return false, true, errScenarioIDNotResolvedExactlyOnce
		}
		if !sameRequirementIDs(approvedRequirementIDs, scenarioRequirementIDs(repoRoot, scope.FeaturePath, scope.ScenarioID)) {
			return false, true, errScenarioRequirementIDsMismatch
		}
	}
	return true, true, nil
}

func parseRequirementIDs(value string) []string {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(value), "["), "]"))
	if value == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(value, " ", ""), ",")
}

func sameRequirementIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]bool, len(left))
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		if !seen[value] {
			return false
		}
	}
	return len(seen) == len(right)
}

func scenarioRequirementIDs(repoRoot, featurePath, scenarioID string) []string {
	file, closeFile, err := openRepositoryFile(repoRoot, featurePath)
	if err != nil {
		return nil
	}
	defer closeFile()

	tag := "@" + scenarioID
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		for _, field := range fields {
			if field == tag {
				requirements := make([]string, 0)
				for _, value := range fields {
					if strings.HasPrefix(value, "@REQ-") {
						requirements = append(requirements, strings.TrimPrefix(value, "@"))
					}
				}
				return requirements
			}
		}
	}
	return nil
}

func scenarioIDResolvesExactlyOnce(repoRoot, featurePath, scenarioID string) bool {
	file, closeFile, err := openRepositoryFile(repoRoot, featurePath)
	if err != nil {
		return false
	}
	defer closeFile()

	tag := "@" + scenarioID
	pendingScenario := false
	resolved := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "@") {
			pendingScenario = false
			for _, value := range strings.Fields(line) {
				if value == tag {
					pendingScenario = true
				}
			}
			continue
		}
		if pendingScenario && strings.HasPrefix(line, "Scenario") {
			resolved++
			pendingScenario = false
		}
	}
	return scanner.Err() == nil && resolved == 1
}

func isCanonicalFeaturePath(path string) bool {
	return path != "" &&
		filepath.ToSlash(path) == path &&
		!filepath.IsAbs(path) &&
		filepath.Clean(path) == path &&
		path != "." &&
		!strings.HasPrefix(path, "../")
}

func approvalBaselineIsCommitted(repoRoot, baselineCommit string) bool {
	if baselineCommit == "" {
		return false
	}
	command := exec.Command("git", "cat-file", "-e", baselineCommit+"^{commit}")
	command.Dir = repoRoot
	return command.Run() == nil
}

func approvalBaselineIsReachable(repoRoot, baselineCommit string) bool {
	command := exec.Command("git", "merge-base", "--is-ancestor", baselineCommit, "HEAD")
	command.Dir = repoRoot
	return command.Run() == nil
}

func featureApprovalPath(featurePath string) string {
	contractID := strings.TrimSuffix(filepath.Base(featurePath), filepath.Ext(featurePath))
	return filepath.Join("specs", "approvals", contractID+".yaml")
}
