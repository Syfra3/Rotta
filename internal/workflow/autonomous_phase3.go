package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AutonomousPhase3WorktreeRequest struct {
	Scope        ContractScope
	Branch       string
	WorktreePath string
}

type AutonomousPhase3WorktreePreparation struct {
	Branch       string
	WorktreePath string
}

func PrepareAutonomousPhase3Worktree(repoRoot string, request AutonomousPhase3WorktreeRequest) (AutonomousPhase3WorktreePreparation, error) {
	command := exec.Command("git", "worktree", "add", "-b", request.Branch, request.WorktreePath, "HEAD")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("create isolated Phase 3 worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}

	for _, path := range []string{request.Scope.SpecPath, request.Scope.FeaturePath} {
		if _, err := os.Stat(filepath.Join(request.WorktreePath, filepath.FromSlash(path))); err != nil {
			return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("verify approved contract artifact %s: %w", path, err)
		}
	}

	status := exec.Command("git", "status", "--short")
	status.Dir = request.WorktreePath
	output, err := status.CombinedOutput()
	if err != nil {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("check isolated Phase 3 worktree status: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "" {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("isolated Phase 3 worktree has non-ignored changes: %s", strings.TrimSpace(string(output)))
	}

	return AutonomousPhase3WorktreePreparation{Branch: request.Branch, WorktreePath: request.WorktreePath}, nil
}
