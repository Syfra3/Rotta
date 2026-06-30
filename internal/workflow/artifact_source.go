package workflow

import (
	"fmt"
	"os/exec"
)

type ContractSourceStatus struct {
	Authoritative              bool
	SpecTracked                bool
	FeatureTracked             bool
	RequiresAncoraContractText bool
}

func EvaluateContractSourceOfTruth(repoRoot string, scope ContractScope) (ContractSourceStatus, error) {
	specTracked, err := gitTracksPath(repoRoot, scope.SpecPath)
	if err != nil {
		return ContractSourceStatus{}, err
	}
	featureTracked, err := gitTracksPath(repoRoot, scope.FeaturePath)
	if err != nil {
		return ContractSourceStatus{}, err
	}

	authoritative := specTracked && featureTracked
	return ContractSourceStatus{
		Authoritative:              authoritative,
		SpecTracked:                specTracked,
		FeatureTracked:             featureTracked,
		RequiresAncoraContractText: false,
	}, nil
}

func gitTracksPath(repoRoot, path string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("check tracked path %s: %w: %s", path, err, output)
	}
	return true, nil
}
