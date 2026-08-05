package workflow

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ResolveGitRemoteRef resolves one explicit fully-qualified ref using local git.
// It intentionally does not infer a remote, branch, or ancestor relationship.
func ResolveGitRemoteRef(ctx context.Context, repoRoot, remote, ref string) (string, error) {
	if err := validateGitRemoteRef(remote, ref); err != nil {
		return "", err
	}
	output, err := executeGitCLI(ctx, repoRoot, "ls-remote", remote, ref)
	if err != nil {
		return "", fmt.Errorf("resolve remote ref %s/%s: %w", remote, ref, err)
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[1] != ref || !isFullCommitID(fields[0]) {
		return "", fmt.Errorf("remote ref %s/%s did not resolve to exactly one full commit", remote, ref)
	}
	return strings.ToLower(fields[0]), nil
}

func VerifyGitRemoteRef(ctx context.Context, repoRoot, remote, ref, reviewedCommit string) (string, error) {
	if !isFullCommitID(reviewedCommit) {
		return "", fmt.Errorf("invalid reviewed commit %q", reviewedCommit)
	}
	observed, err := ResolveGitRemoteRef(ctx, repoRoot, remote, ref)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(observed, reviewedCommit) {
		return observed, fmt.Errorf("remote ref %s/%s resolved to %s, not reviewed commit %s", remote, ref, observed, strings.ToLower(reviewedCommit))
	}
	return observed, nil
}

// PushAndVerifyGitRemoteRef publishes only to the supplied remote/ref and then
// verifies that the remote ref itself, rather than ancestry, is the review tip.
func PushAndVerifyGitRemoteRef(ctx context.Context, repoRoot, remote, ref, reviewedCommit string) (string, error) {
	if err := validateGitRemoteRef(remote, ref); err != nil {
		return "", err
	}
	if !isFullCommitID(reviewedCommit) {
		return "", fmt.Errorf("invalid reviewed commit %q", reviewedCommit)
	}
	if _, err := executeGitCLI(ctx, repoRoot, "push", remote, reviewedCommit+":"+ref); err != nil {
		return "", fmt.Errorf("push explicit remote ref %s/%s: %w", remote, ref, err)
	}
	return VerifyGitRemoteRef(ctx, repoRoot, remote, ref, reviewedCommit)
}

func validateGitRemoteRef(remote, ref string) error {
	if strings.TrimSpace(remote) == "" || strings.ContainsAny(remote, " \t\r\n") {
		return errors.New("Git remote must be explicit and whitespace-free")
	}
	if !strings.HasPrefix(ref, "refs/") || strings.ContainsAny(ref, " \t\r\n~^:?*[") {
		return errors.New("Git ref must be explicit, fully qualified, and valid")
	}
	return nil
}

func executeGitCLI(ctx context.Context, directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, boundedCommandOutput(output))
	}
	return strings.TrimSpace(string(output)), nil
}
