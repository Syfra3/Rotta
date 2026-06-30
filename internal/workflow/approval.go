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
	file, err := os.Open(filepath.Join(repoRoot, scopedApprovalPath(scope.SpecPath)))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()

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

func scopedApprovalPath(specPath string) string {
	contractID := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join("specs", "approvals", contractID+".approved")
}
