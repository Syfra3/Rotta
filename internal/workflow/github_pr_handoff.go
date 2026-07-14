package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ManualGitHubPRHandoffRequest contains the already-reviewed submission data
// needed to print, but never execute, human publication commands.
type ManualGitHubPRHandoffRequest struct {
	Submission     NewImplementationSubmission
	ReviewedPaths  []string
	HostDisclaimer string
}

// PresentManualGitHubPRHandoff prints a manual-only GitHub publication handoff.
func PresentManualGitHubPRHandoff(request ManualGitHubPRHandoffRequest) (string, error) {
	if request.Submission.WorktreePath == "" || !filepath.IsAbs(request.Submission.WorktreePath) || !isSafeGitBranchName(request.Submission.BaseBranch) || !isSafeGitBranchName(request.Submission.FeatureBranch) {
		return "", fmt.Errorf("manual GitHub PR handoff requires the recorded feature worktree and branches")
	}
	remote, webURL, err := resolveGitHubPushRemote(request.Submission.WorktreePath)
	if err != nil {
		return "", err
	}

	var handoff strings.Builder
	fmt.Fprintf(&handoff, "Phase 4 passed. Run these commands yourself from the recorded worktree:\n\n  cd %q\n  git status --short\n", request.Submission.WorktreePath)
	if len(request.ReviewedPaths) > 0 {
		handoff.WriteString("\nIf the status contains only your reviewed outstanding changes, optionally commit only those paths:\n  git add --")
		for _, path := range request.ReviewedPaths {
			fmt.Fprintf(&handoff, " %q", path)
		}
		handoff.WriteString("\n  git commit -m \"reviewed changes\"\n")
	}
	fmt.Fprintf(&handoff, "\n  git push %s %s\n  gh pr create --base %s --head %s\n\nOr open the GitHub web UI: %s/compare/%s\n\n%s\n", remote, request.Submission.FeatureBranch, request.Submission.BaseBranch, request.Submission.FeatureBranch, webURL, request.Submission.FeatureBranch, request.HostDisclaimer)
	return handoff.String(), nil
}

func resolveGitHubPushRemote(worktreePath string) (string, string, error) {
	remotes, err := gitSubmissionOutput(worktreePath, "remote")
	if err != nil {
		return "", "", fmt.Errorf("resolve GitHub push remote: %w", err)
	}
	var names []string
	var webURL string
	for _, remote := range strings.Fields(remotes) {
		url, err := gitSubmissionOutput(worktreePath, "remote", "get-url", "--push", remote)
		if err != nil {
			return "", "", fmt.Errorf("resolve push URL for remote %q: %w", remote, err)
		}
		if githubURL, ok := githubWebURL(url); ok {
			names = append(names, remote)
			webURL = githubURL
		}
	}
	if len(names) != 1 {
		return "", "", fmt.Errorf("manual GitHub PR handoff requires exactly one GitHub-capable push remote")
	}
	return names[0], webURL, nil
}

func githubWebURL(remoteURL string) (string, bool) {
	path := ""
	switch {
	case strings.HasPrefix(remoteURL, "git@github.com:"):
		path = strings.TrimPrefix(remoteURL, "git@github.com:")
	case strings.HasPrefix(remoteURL, "ssh://git@github.com/"):
		path = strings.TrimPrefix(remoteURL, "ssh://git@github.com/")
	case strings.HasPrefix(remoteURL, "https://github.com/"):
		path = strings.TrimPrefix(remoteURL, "https://github.com/")
	case strings.HasPrefix(remoteURL, "http://github.com/"):
		path = strings.TrimPrefix(remoteURL, "http://github.com/")
	default:
		return "", false
	}
	path = strings.TrimSuffix(path, ".git")
	if path == "" || strings.Contains(path, "?") || strings.Contains(path, "#") {
		return "", false
	}
	return "https://github.com/" + path, true
}
