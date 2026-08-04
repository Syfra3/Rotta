package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var explicitCommitSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type FullWorkflowBootstrapRequest struct {
	FeatureID      string
	BaseSHA        string
	SpecPath       string
	FeaturePath    string
	CheckpointMode string
}

type FullWorkflowBootstrap struct {
	WorktreePath  string
	FeatureBranch string
	BaseSHA       string
	ManifestPath  string
}

// BootstrapFullWorkflow creates a new feature worktree from an immutable base
// before creating the feature-local artifacts needed to begin specification.
func BootstrapFullWorkflow(initiatingWorktree string, request FullWorkflowBootstrapRequest) (FullWorkflowBootstrap, error) {
	if !explicitCommitSHAPattern.MatchString(request.BaseSHA) {
		return FullWorkflowBootstrap{}, unsafeWorktreePreparation(fmt.Errorf("base SHA must be an explicit 40-hex commit"))
	}
	if request.CheckpointMode != "strict_per_scenario" {
		return FullWorkflowBootstrap{}, unsafeWorktreePreparation(fmt.Errorf("bootstrap requires strict_per_scenario checkpoint mode"))
	}
	if err := validatePendingContractPaths(request.SpecPath, request.FeaturePath); err != nil {
		return FullWorkflowBootstrap{}, unsafeWorktreePreparation(err)
	}

	submission, err := PrepareNewImplementationSubmission(initiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              request.FeatureID,
		IntegrationBranch: request.BaseSHA,
	})
	if err != nil {
		return FullWorkflowBootstrap{}, err
	}
	resolvedBaseSHA, err := gitSubmissionOutput(submission.WorktreePath, "rev-parse", "HEAD")
	if err != nil || resolvedBaseSHA != request.BaseSHA {
		return FullWorkflowBootstrap{}, fmt.Errorf("verify isolated feature worktree explicit base: got %q, want %q", resolvedBaseSHA, request.BaseSHA)
	}

	policy, err := readRepositoryFile(submission.WorktreePath, ".rotta/quality-gates.yaml")
	if err != nil {
		return FullWorkflowBootstrap{}, fmt.Errorf("bootstrap feature-local policy: %w", err)
	}
	for _, path := range []string{request.SpecPath, request.FeaturePath} {
		if err := writePendingContractArtifact(submission.WorktreePath, path); err != nil {
			return FullWorkflowBootstrap{}, err
		}
	}

	manifestPath := filepath.Join(submission.WorktreePath, ".rotta", "current", "manifest.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		return FullWorkflowBootstrap{}, fmt.Errorf("create feature-local manifest directory: %w", err)
	}
	manifest := fmt.Sprintf("format: rotta.workflow-manifest/v1\nfeature_id: %s\nworktree: %s\nbranch: %s\nbase_sha: %s\npolicy_path: .rotta/quality-gates.yaml\npolicy_fingerprint: %x\nspec_path: %s\nfeature_path: %s\ncheckpoint_mode: %s\n",
		request.FeatureID,
		submission.WorktreePath,
		submission.FeatureBranch,
		request.BaseSHA,
		sha256.Sum256(policy),
		request.SpecPath,
		request.FeaturePath,
		request.CheckpointMode,
	)
	file, err := os.OpenFile(manifestPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return FullWorkflowBootstrap{}, fmt.Errorf("create feature-local manifest: %w", err)
	}
	if _, err := file.WriteString(manifest); err != nil {
		_ = file.Close()
		return FullWorkflowBootstrap{}, fmt.Errorf("write feature-local manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return FullWorkflowBootstrap{}, fmt.Errorf("close feature-local manifest: %w", err)
	}
	return FullWorkflowBootstrap{
		WorktreePath:  submission.WorktreePath,
		FeatureBranch: submission.FeatureBranch,
		BaseSHA:       request.BaseSHA,
		ManifestPath:  manifestPath,
	}, nil
}

func validatePendingContractPaths(specPath, featurePath string) error {
	if !isPendingContractPath(specPath, "specs/", ".md") {
		return fmt.Errorf("invalid pending hard-spec path %q", specPath)
	}
	if !isPendingContractPath(featurePath, "features/", ".feature") {
		return fmt.Errorf("invalid pending feature path %q", featurePath)
	}
	return nil
}

func isPendingContractPath(path, directory, extension string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	return strings.HasPrefix(path, directory) && strings.HasSuffix(path, extension) &&
		clean == filepath.FromSlash(path) && clean != filepath.FromSlash(directory)
}

func writePendingContractArtifact(repoRoot, path string) error {
	filePath, err := repositoryFilePath(repoRoot, path)
	if err != nil {
		return fmt.Errorf("resolve pending contract artifact %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return fmt.Errorf("create pending contract directory for %q: %w", path, err)
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create pending contract artifact %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close pending contract artifact %q: %w", path, err)
	}
	return nil
}
