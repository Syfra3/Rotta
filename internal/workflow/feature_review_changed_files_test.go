package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// REQ-082 → SCN-605 → TestSCN605_ResolvesCheckpointChangesFromBaselineToHEAD
func TestSCN605_ResolvesCheckpointChangesFromBaselineToHEAD(t *testing.T) {
	// Scenario: Review sees baseline-to-HEAD changes after checkpoint commits
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "changed.txt"), "before\n")
	mustWrite(t, filepath.Join(repo, "renamed-before.txt"), "rename me\n")
	mustWrite(t, filepath.Join(repo, "deleted.txt"), "delete me\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "test: confirm approved baseline")
	baseline := runGitOutput(t, repo, "rev-parse", "HEAD")

	mustWrite(t, filepath.Join(repo, "changed.txt"), "after\n")
	runGit(t, repo, "mv", "renamed-before.txt", "renamed-after.txt")
	if err := os.Remove(filepath.Join(repo, "deleted.txt")); err != nil {
		t.Fatalf("delete checkpoint file: %v", err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "checkpoint: SCN-605 changes")
	if status := runGitOutput(t, repo, "status", "--short"); status != "" {
		t.Fatalf("checkpoint worktree status = %q, want clean", status)
	}

	resolved, err := ResolveFeatureReviewChangedFiles(repo, baseline)
	if err != nil {
		t.Fatalf("ResolveFeatureReviewChangedFiles returned error: %v", err)
	}
	if resolved.BaselineSHA != baseline {
		t.Errorf("baseline SHA = %q, want %q", resolved.BaselineSHA, baseline)
	}
	if want := runGitOutput(t, repo, "rev-parse", "HEAD"); resolved.HEADSHA != want {
		t.Errorf("HEAD SHA = %q, want %q", resolved.HEADSHA, want)
	}
	if want := runGitOutput(t, repo, "diff", "--name-status", baseline+"...HEAD"); resolved.NameStatus != want {
		t.Errorf("name-status = %q, want %q", resolved.NameStatus, want)
	}
	if want := []string{"changed.txt"}; !reflect.DeepEqual(resolved.ChangedPaths, want) {
		t.Errorf("changed paths = %#v, want %#v", resolved.ChangedPaths, want)
	}
	if want := []FeatureReviewRename{{From: "renamed-before.txt", To: "renamed-after.txt"}}; !reflect.DeepEqual(resolved.RenamedPaths, want) {
		t.Errorf("renamed paths = %#v, want %#v", resolved.RenamedPaths, want)
	}
	if want := []string{"deleted.txt"}; !reflect.DeepEqual(resolved.DeletedPaths, want) {
		t.Errorf("deleted paths = %#v, want %#v", resolved.DeletedPaths, want)
	}
}
