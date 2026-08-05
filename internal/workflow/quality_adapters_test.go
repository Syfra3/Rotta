package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGoQualityAdaptersRunsRequiredCommandsAtCandidate(t *testing.T) {
	repo := t.TempDir()
	writeQualityAdapterFile(t, filepath.Join(repo, "go.mod"), "module example.com/quality\n\ngo 1.25.0\n")
	writeQualityAdapterFile(t, filepath.Join(repo, "main.go"), "package quality\nfunc Value() int { return 1 }\n")
	runQualityAdapterCommand(t, repo, "git", "init")
	runQualityAdapterCommand(t, repo, "git", "config", "user.email", "quality@example.test")
	runQualityAdapterCommand(t, repo, "git", "config", "user.name", "Quality Test")
	runQualityAdapterCommand(t, repo, "git", "add", ".")
	runQualityAdapterCommand(t, repo, "git", "commit", "-m", "candidate")
	candidate := runQualityAdapterCommand(t, repo, "git", "rev-parse", "HEAD")

	results, err := RunGoQualityAdapters(context.Background(), repo, candidate)
	if err != nil {
		t.Fatalf("RunGoQualityAdapters() error = %v", err)
	}
	if len(results) != 4 || !results[0].Succeeded || !results[1].Succeeded || !results[2].Succeeded || !results[3].Succeeded {
		t.Fatalf("quality results = %#v", results)
	}
	if _, err := RunGoQualityAdapters(context.Background(), repo, strings.Repeat("a", 40)); err == nil {
		t.Fatal("RunGoQualityAdapters() accepted a candidate that is not worktree HEAD")
	}
}

func writeQualityAdapterFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runQualityAdapterCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
