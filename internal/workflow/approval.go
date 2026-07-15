package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		return ImplementationGateDecision{}, err
	}
	if approved {
		return ImplementationGateDecision{Approved: true, Reason: "scoped human approval recorded"}, nil
	}

	return ImplementationGateDecision{
		Approved: false,
		Reason:   fmt.Sprintf("human approval is still required for %s#%s", scope.FeaturePath, scope.ScenarioID),
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
	entryFeaturePath := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "approved_scenarios:" {
			inApprovedScenarios = true
			continue
		}
		if !inApprovedScenarios {
			continue
		}
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "-") {
			break
		}
		if value, ok := strings.CutPrefix(line, "- feature_path: "); ok {
			entryFeaturePath = strings.TrimSpace(value)
			continue
		}
		if value, ok := strings.CutPrefix(line, "scenario_id: "); ok && entryFeaturePath == scope.FeaturePath && strings.TrimSpace(value) == strings.TrimSpace(scope.ScenarioID) {
			return true, true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, true, err
	}
	return false, true, nil
}

func scopedApprovalPath(specPath string) string {
	contractID := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join("specs", "approvals", contractID+".approved")
}

func featureApprovalPath(featurePath string) string {
	contractID := strings.TrimSuffix(filepath.Base(featurePath), filepath.Ext(featurePath))
	return filepath.Join("specs", "approvals", contractID+".yaml")
}
