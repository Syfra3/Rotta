package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources(t *testing.T) {
	// REQ-011 → REQ-012 → SCN-012 → TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources
	// Scenario: Active hard spec and feature files are tracked as the contract source of truth
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")

	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-011 @REQ-012 @SCN-012\nScenario: Active hard spec and feature files are tracked as the contract source of truth\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md", "features/workflow_artifact_lifecycle.feature")
	runGit(t, repo, "commit", "-m", "test: track approved contract artifacts")

	status, err := EvaluateContractSourceOfTruth(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-012",
	})
	if err != nil {
		t.Fatalf("EvaluateContractSourceOfTruth returned error: %v", err)
	}
	if !status.Authoritative {
		t.Fatalf("expected tracked spec and feature to be authoritative, got: %#v", status)
	}
	if !status.SpecTracked || !status.FeatureTracked {
		t.Fatalf("expected both contract files to be tracked, got: %#v", status)
	}
	if status.RequiresAncoraContractText {
		t.Fatalf("expected repository files to recover behavior without full Ancora contract text")
	}
}

func TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract(t *testing.T) {
	// REQ-011 → REQ-020 → SCN-013 → TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract
	// Scenario: Namespaced workflow-policy artifacts do not overwrite an existing active contract
	repo := t.TempDir()
	existingSpec := "# Active installer recovery hard spec\n"
	existingFeature := "Feature: Installer recovery\n"
	newSpec := "# Workflow Artifact Lifecycle\n"
	newFeature := "Feature: Workflow artifact lifecycle\n"
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), existingSpec)
	mustWrite(t, filepath.Join(repo, "features", "installer_recovery.feature"), existingFeature)

	artifacts, err := GenerateNamespacedWorkflowPolicyArtifacts(repo, WorkflowPolicyArtifactRequest{
		ContractID:        "workflow_artifact_lifecycle",
		HardSpec:          newSpec,
		Feature:           newFeature,
		LegacySpecPath:    "specs/hard_spec.md",
		LegacyFeaturePath: "features/installer_recovery.feature",
	})
	if err != nil {
		t.Fatalf("GenerateNamespacedWorkflowPolicyArtifacts returned error: %v", err)
	}

	if artifacts.SpecPath != "specs/workflow_artifact_lifecycle.md" {
		t.Fatalf("expected namespaced hard spec path, got %q", artifacts.SpecPath)
	}
	if artifacts.FeaturePath != "features/workflow_artifact_lifecycle.feature" {
		t.Fatalf("expected namespaced feature path, got %q", artifacts.FeaturePath)
	}
	assertFileContent(t, filepath.Join(repo, artifacts.SpecPath), newSpec)
	assertFileContent(t, filepath.Join(repo, artifacts.FeaturePath), newFeature)
	assertFileContent(t, filepath.Join(repo, "specs", "hard_spec.md"), existingSpec)
	assertFileContent(t, filepath.Join(repo, "features", "installer_recovery.feature"), existingFeature)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("unexpected content for %s: got %q want %q", path, got, want)
	}
}
