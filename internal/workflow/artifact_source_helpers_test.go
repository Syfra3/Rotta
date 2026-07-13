package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

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

func assertContractAction(t *testing.T, plan []ContractCleanupAction, path string, want ContractCleanupActionKind) {
	t.Helper()
	for _, action := range plan {
		if action.Path == path {
			if action.Kind != want {
				t.Fatalf("expected %s action for %s, got %s", want, path, action.Kind)
			}
			return
		}
	}
	t.Fatalf("expected action for %s in %#v", path, plan)
}

func assertArchivePlanKeepsPath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, keptPath := range plan.KeptActivePaths {
		if keptPath == path {
			return
		}
	}
	t.Fatalf("expected archive preparation to keep %s active under features, got %#v", path, plan)
}

func assertArchiveMove(t *testing.T, plan CompletedChangeArchivePlan, sourcePath, destinationPath, reason string) {
	t.Helper()
	for _, move := range plan.ArchiveMoves {
		if move.SourcePath == sourcePath {
			if move.DestinationPath != destinationPath || move.Reason != reason {
				t.Fatalf("unexpected archive move for %s: got %#v", sourcePath, move)
			}
			return
		}
	}
	t.Fatalf("expected archive move for %s in %#v", sourcePath, plan)
}

func assertArchivePlanDoesNotMovePath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, move := range plan.ArchiveMoves {
		if move.SourcePath == path {
			t.Fatalf("expected %s to stay out of archive moves, got %#v", path, plan)
		}
	}
}

func assertArchivePlanDoesNotKeepPath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, keptPath := range plan.KeptActivePaths {
		if keptPath == path {
			t.Fatalf("expected %s not to be kept as an active feature contract, got %#v", path, plan)
		}
	}
}

func assertFileDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to exist", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertReviewSetIncludesPath(t *testing.T, plan WorkflowArtifactReviewSetPlan, path string) {
	t.Helper()
	for _, includedPath := range plan.IncludedPaths {
		if includedPath == path {
			return
		}
	}
	t.Fatalf("expected review set to include %s, got %#v", path, plan)
}

func assertReviewSetExcludesPath(t *testing.T, plan WorkflowArtifactReviewSetPlan, path string) {
	t.Helper()
	for _, excludedPath := range plan.ExcludedPaths {
		if excludedPath == path {
			return
		}
	}
	t.Fatalf("expected review set to exclude %s, got %#v", path, plan)
}

func assertCleanupGuidanceAction(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path string, want WorkflowArtifactCleanupActionKind) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path {
			if item.Action != want {
				t.Fatalf("expected cleanup action %q for %s, got %#v", want, path, item)
			}
			if item.Reason == "" {
				t.Fatalf("expected cleanup guidance reason for %s, got %#v", path, item)
			}
			return
		}
	}
	t.Fatalf("expected cleanup guidance for %s in %#v", path, report)
}

func assertCleanupGuidanceDoesNotUseAction(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path string, forbidden WorkflowArtifactCleanupActionKind) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path && item.Action == forbidden {
			t.Fatalf("expected cleanup guidance for %s not to use %q, got %#v", path, forbidden, item)
		}
	}
}

func assertCleanupGuidanceReason(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path, want string) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path {
			if item.Reason != want {
				t.Fatalf("expected cleanup reason %q for %s, got %#v", want, path, item)
			}
			return
		}
	}
	t.Fatalf("expected cleanup guidance reason for %s in %#v", path, report)
}

func assertPointerIssue(t *testing.T, report WorkflowPointerValidationReport, path string, want PointerIssueKind) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Path == path {
			if issue.Kind != want {
				t.Fatalf("expected %s issue for %s, got %s", want, path, issue.Kind)
			}
			return
		}
	}
	t.Fatalf("expected issue for %s in %#v", path, report.Issues)
}

func checksumFor(content string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
}
