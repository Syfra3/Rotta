package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// QualityAdapterResult records bounded evidence for a required local Go check.
type QualityAdapterResult struct {
	Name      string
	Arguments []string
	Succeeded bool
	Output    string
}

// RunGoQualityAdapters executes the required Go review commands in the exact
// candidate worktree. A failed adapter is reported, not converted to success.
func RunGoQualityAdapters(ctx context.Context, repoRoot, candidateCommit string) ([]QualityAdapterResult, error) {
	if !isFullCommitID(candidateCommit) {
		return nil, fmt.Errorf("invalid candidate commit %q", candidateCommit)
	}
	head, err := executeGitCLI(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve candidate worktree HEAD: %w", err)
	}
	if !strings.EqualFold(head, candidateCommit) {
		return nil, fmt.Errorf("candidate commit %s does not match worktree HEAD %s", candidateCommit, head)
	}
	coverage, err := os.CreateTemp(repoRoot, ".rotta-coverage-*.out")
	if err != nil {
		return nil, fmt.Errorf("create local coverage profile: %w", err)
	}
	coveragePath := coverage.Name()
	if err := coverage.Close(); err != nil {
		_ = os.Remove(coveragePath)
		return nil, fmt.Errorf("close local coverage profile: %w", err)
	}
	defer os.Remove(coveragePath)

	commands := []struct {
		name string
		args []string
	}{
		{"go test", []string{"test", "./..."}},
		{"go vet", []string{"vet", "./..."}},
		{"go coverage", []string{"test", "-coverprofile=" + coveragePath, "./..."}},
		{"go tool cover", []string{"tool", "cover", "-func=" + coveragePath}},
	}
	results := make([]QualityAdapterResult, 0, len(commands))
	for _, check := range commands {
		command := exec.CommandContext(ctx, "go", check.args...)
		command.Dir = repoRoot
		output, runErr := command.CombinedOutput()
		result := QualityAdapterResult{Name: check.name, Arguments: append([]string(nil), check.args...), Succeeded: runErr == nil, Output: boundedCommandOutput(output)}
		results = append(results, result)
		if runErr != nil {
			return results, fmt.Errorf("%s: %w", check.name, runErr)
		}
	}
	return results, nil
}
