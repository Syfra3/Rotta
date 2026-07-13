package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSCN218_WorkflowArtifactsRejectEscapingRepositoryPaths(t *testing.T) {
	// REQ-011 → SCN-218 → TestSCN218_WorkflowArtifactsRejectEscapingRepositoryPaths
	// Scenario: Continue from OpenSpec workflow artifacts when Ancora is unavailable
	repo := t.TempDir()
	assertRepositoryPathsRejected(t, repo, "../outside", "/outside")
	assertRepositoryArtifactAccess(t, repo)
}

func assertRepositoryPathsRejected(t *testing.T, repo string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := readRepositoryFile(repo, path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %q to be rejected, got %v", path, err)
		}
		if _, closeFile, err := openRepositoryFile(repo, path); !errors.Is(err, os.ErrNotExist) || closeFile != nil {
			t.Fatalf("expected open of %q to be rejected, got close=%t err=%v", path, closeFile != nil, err)
		}
	}
}

func assertRepositoryArtifactAccess(t *testing.T, repo string) {
	t.Helper()
	path := filepath.Join(repo, "features", "approved.feature")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Feature: approved\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := readRepositoryFile(repo, "features/approved.feature")
	if err != nil || string(data) != "Feature: approved\n" {
		t.Fatalf("expected in-repository artifact read, data=%q err=%v", data, err)
	}
	assertMissingRepositoryArtifacts(t, repo)
}

func assertMissingRepositoryArtifacts(t *testing.T, repo string) {
	t.Helper()
	if _, err := readRepositoryFile(repo, "features/missing.feature"); err == nil {
		t.Fatal("expected missing in-repository artifact to be reported")
	}
	if _, closeFile, err := openRepositoryFile(repo, "features/missing.feature"); err == nil || closeFile != nil {
		t.Fatalf("expected missing artifact open to fail, close=%t err=%v", closeFile != nil, err)
	}
}
