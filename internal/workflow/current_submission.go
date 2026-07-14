package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const currentSubmissionStatusInProgress = "in_progress"

type CurrentSubmissionRequest struct {
	ID           string
	SpecPath     string
	FeaturePaths []string
	ScenarioIDs  []string
}

type CurrentSubmissionManifest struct {
	SubmissionID string
	SpecPath     string
	FeaturePaths []string
	ScenarioIDs  []string
	Worktree     string
	Status       string
}

type CurrentSubmissionState struct {
	Phase           string
	CompletedWork   []string
	RemainingWork   []string
	LastAction      string
	SafeResumePoint string
}

type CurrentSubmission struct {
	ManifestPath string
	StatePath    string
	Manifest     CurrentSubmissionManifest
	State        CurrentSubmissionState
}

func InitializeCurrentSubmission(repoRoot string, request CurrentSubmissionRequest) (CurrentSubmission, error) {
	manifest := CurrentSubmissionManifest{
		SubmissionID: request.ID,
		SpecPath:     request.SpecPath,
		FeaturePaths: append([]string(nil), request.FeaturePaths...),
		ScenarioIDs:  append([]string(nil), request.ScenarioIDs...),
		Worktree:     repoRoot,
		Status:       currentSubmissionStatusInProgress,
	}
	state := CurrentSubmissionState{
		Phase:           "implementation",
		CompletedWork:   []string{},
		RemainingWork:   append([]string(nil), request.ScenarioIDs...),
		LastAction:      "initialized current submission",
		SafeResumePoint: "begin implementation",
	}

	currentDirectory := filepath.Join(repoRoot, ".rotta", "current")
	if err := os.MkdirAll(currentDirectory, 0o700); err != nil {
		return CurrentSubmission{}, fmt.Errorf("create current submission directory: %w", err)
	}

	manifestPath := filepath.Join(currentDirectory, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(serializeCurrentSubmissionManifest(manifest)), 0o600); err != nil {
		return CurrentSubmission{}, fmt.Errorf("write current submission manifest: %w", err)
	}

	statePath := filepath.Join(currentDirectory, "state.yaml")
	if err := os.WriteFile(statePath, []byte(serializeCurrentSubmissionState(state)), 0o600); err != nil {
		return CurrentSubmission{}, fmt.Errorf("write current submission state: %w", err)
	}

	return CurrentSubmission{ManifestPath: manifestPath, StatePath: statePath, Manifest: manifest, State: state}, nil
}

func serializeCurrentSubmissionManifest(manifest CurrentSubmissionManifest) string {
	return fmt.Sprintf("submission_id: %s\nspec_path: %s\nfeature_paths:\n%s\nscenario_ids:\n%s\nworktree: %s\nstatus: %s\n",
		manifest.SubmissionID,
		manifest.SpecPath,
		yamlList(manifest.FeaturePaths),
		yamlList(manifest.ScenarioIDs),
		manifest.Worktree,
		manifest.Status,
	)
}

func serializeCurrentSubmissionState(state CurrentSubmissionState) string {
	return fmt.Sprintf("phase: %s\ncompleted_work:\n%s\nremaining_work:\n%s\nlast_action: %s\nsafe_resume_point: %s\n",
		state.Phase,
		yamlList(state.CompletedWork),
		yamlList(state.RemainingWork),
		state.LastAction,
		state.SafeResumePoint,
	)
}

func yamlList(values []string) string {
	if len(values) == 0 {
		return "  []"
	}
	return "  - " + strings.Join(values, "\n  - ")
}
