package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ContractSourceStatus struct {
	Authoritative              bool
	SpecTracked                bool
	FeatureTracked             bool
	RequiresAncoraContractText bool
}

type WorkflowPolicyArtifactRequest struct {
	ContractID        string
	HardSpec          string
	Feature           string
	LegacySpecPath    string
	LegacyFeaturePath string
}

type WorkflowPolicyArtifacts struct {
	SpecPath    string
	FeaturePath string
}

func GenerateNamespacedWorkflowPolicyArtifacts(repoRoot string, request WorkflowPolicyArtifactRequest) (WorkflowPolicyArtifacts, error) {
	contractID := strings.TrimSpace(request.ContractID)
	if contractID == "" {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("contract id is required")
	}

	artifacts := WorkflowPolicyArtifacts{
		SpecPath:    filepath.ToSlash(filepath.Join("specs", contractID+".md")),
		FeaturePath: filepath.ToSlash(filepath.Join("features", contractID+".feature")),
	}
	if artifacts.SpecPath == request.LegacySpecPath || artifacts.FeaturePath == request.LegacyFeaturePath {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("namespaced artifact path would overwrite an active contract")
	}

	if err := writeWorkflowArtifact(filepath.Join(repoRoot, artifacts.SpecPath), request.HardSpec); err != nil {
		return WorkflowPolicyArtifacts{}, err
	}
	if err := writeWorkflowArtifact(filepath.Join(repoRoot, artifacts.FeaturePath), request.Feature); err != nil {
		return WorkflowPolicyArtifacts{}, err
	}
	return artifacts, nil
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

func writeWorkflowArtifact(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create workflow artifact parent %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", path, err)
	}
	return nil
}
