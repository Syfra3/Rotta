package workflow

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushAndVerifyGitRemoteRefRequiresExactTip(t *testing.T) {
	repo := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	runQualityAdapterCommand(t, repo, "git", "init")
	runQualityAdapterCommand(t, repo, "git", "config", "user.email", "git@example.test")
	runQualityAdapterCommand(t, repo, "git", "config", "user.name", "Git Test")
	writeQualityAdapterFile(t, filepath.Join(repo, "file.txt"), "first\n")
	runQualityAdapterCommand(t, repo, "git", "add", ".")
	runQualityAdapterCommand(t, repo, "git", "commit", "-m", "first")
	commit := runQualityAdapterCommand(t, repo, "git", "rev-parse", "HEAD")
	runQualityAdapterCommand(t, t.TempDir(), "git", "init", "--bare", remote)
	runQualityAdapterCommand(t, repo, "git", "remote", "add", "review", remote)

	observed, err := PushAndVerifyGitRemoteRef(context.Background(), repo, "review", "refs/heads/review", commit)
	if err != nil || observed != commit {
		t.Fatalf("PushAndVerifyGitRemoteRef() = %q, %v", observed, err)
	}
	if _, err := ResolveGitRemoteRef(context.Background(), repo, "review", "review"); err == nil {
		t.Fatal("ResolveGitRemoteRef() accepted an unqualified ref")
	}
	writeQualityAdapterFile(t, filepath.Join(repo, "file.txt"), "second\n")
	runQualityAdapterCommand(t, repo, "git", "commit", "-am", "second")
	second := runQualityAdapterCommand(t, repo, "git", "rev-parse", "HEAD")
	runQualityAdapterCommand(t, repo, "git", "push", "review", second+":refs/heads/review")
	if observed, err := VerifyGitRemoteRef(context.Background(), repo, "review", "refs/heads/review", commit); err == nil || !strings.EqualFold(observed, second) {
		t.Fatalf("VerifyGitRemoteRef() = %q, %v; want moved ref evidence", observed, err)
	}
}
