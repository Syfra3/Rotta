package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityWorkflowArtifactsAreOwnerOnly(t *testing.T) {
	repo := t.TempDir()
	artifacts, err := GenerateNamespacedWorkflowPolicyArtifacts(repo, WorkflowPolicyArtifactRequest{
		ContractID: "secure-workflow",
		HardSpec:   "# spec\n",
		Feature:    "Feature: secure\n",
	})
	if err != nil {
		t.Fatalf("GenerateNamespacedWorkflowPolicyArtifacts: %v", err)
	}
	assertWorkflowMode(t, filepath.Join(repo, artifacts.SpecPath), 0o600)
	assertWorkflowMode(t, filepath.Join(repo, artifacts.FeaturePath), 0o600)
	assertWorkflowMode(t, filepath.Join(repo, "specs"), 0o750)
	assertWorkflowMode(t, filepath.Join(repo, "features"), 0o750)
}

func assertWorkflowMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %o, want %o", path, got, want)
	}
}
