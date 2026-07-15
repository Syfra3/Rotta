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

func EvaluateImplementationGate(repoRoot string, scope ContractScope) (ImplementationGateDecision, error) {
	approved, err := scopedApprovalContains(repoRoot, scope)
	if err != nil {
		if errors.Is(err, errMalformedFeatureApproval) {
			return ImplementationGateDecision{Reason: "approval record is malformed"}, nil
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
	approved, found, err := featureApprovalContains(repoRoot, scope)
	if err != nil || found {
		return approved, err
	}

	file, closeFile, err := openRepositoryFile(repoRoot, scopedApprovalPath(scope.SpecPath))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer closeFile()

	wantedScenario := strings.TrimSpace(scope.ScenarioID)
	wantedReference := scope.FeaturePath + "#" + wantedScenario
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == wantedScenario || line == wantedReference {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
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
	entryFeaturePath := ""
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
			inApprovedScenarios = false
			continue
		}
		if value, ok := strings.CutPrefix(line, "- feature_path: "); ok {
			entryFeaturePath = strings.TrimSpace(value)
			continue
		}
		if value, ok := strings.CutPrefix(line, "scenario_id: "); ok && entryFeaturePath == scope.FeaturePath && strings.TrimSpace(value) == strings.TrimSpace(scope.ScenarioID) {
			approved = true
		}
	}
	if err := scanner.Err(); err != nil {
		return false, true, err
	}
	if !hasFormat || !hasContractID || !hasStatus || !hasFeaturePaths || !hasApprovedScenarios || !hasFingerprints || !hasBaselineConfirmation {
		return false, true, errMalformedFeatureApproval
	}
	if !hasFeatureIdentity || !approved {
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
	return true, true, nil
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

func scopedApprovalPath(specPath string) string {
	contractID := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join("specs", "approvals", contractID+".approved")
}

func featureApprovalPath(featurePath string) string {
	contractID := strings.TrimSuffix(filepath.Base(featurePath), filepath.Ext(featurePath))
	return filepath.Join("specs", "approvals", contractID+".yaml")
}
