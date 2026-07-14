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

func LoadCurrentSubmission(repoRoot string) (CurrentSubmission, error) {
	manifestPath := filepath.Join(repoRoot, ".rotta", "current", "manifest.yaml")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return CurrentSubmission{}, unusableCurrentSubmissionState(err)
	}

	manifest, err := parseCurrentSubmissionManifest(string(contents))
	if err != nil {
		return CurrentSubmission{}, unusableCurrentSubmissionState(err)
	}
	for _, featurePath := range manifest.FeaturePaths {
		if _, err := os.Stat(filepath.Join(repoRoot, featurePath)); err != nil {
			return CurrentSubmission{}, unusableCurrentSubmissionState(fmt.Errorf("feature file %q: %w", featurePath, err))
		}
	}

	return CurrentSubmission{ManifestPath: manifestPath, StatePath: filepath.Join(repoRoot, ".rotta", "current", "state.yaml"), Manifest: manifest}, nil
}

func unusableCurrentSubmissionState(cause error) error {
	return fmt.Errorf("current submission state cannot be safely used: %w", cause)
}

func parseCurrentSubmissionManifest(contents string) (CurrentSubmissionManifest, error) {
	var manifest CurrentSubmissionManifest
	var list *[]string
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		if strings.HasPrefix(line, "  - ") && list != nil {
			*list = append(*list, strings.TrimPrefix(line, "  - "))
			continue
		}

		list = nil
		switch {
		case strings.HasPrefix(line, "submission_id: "):
			manifest.SubmissionID = strings.TrimPrefix(line, "submission_id: ")
		case strings.HasPrefix(line, "spec_path: "):
			manifest.SpecPath = strings.TrimPrefix(line, "spec_path: ")
		case line == "feature_paths:":
			list = &manifest.FeaturePaths
		case line == "scenario_ids:":
			list = &manifest.ScenarioIDs
		case strings.HasPrefix(line, "worktree: "):
			manifest.Worktree = strings.TrimPrefix(line, "worktree: ")
		case strings.HasPrefix(line, "status: "):
			manifest.Status = strings.TrimPrefix(line, "status: ")
		default:
			return CurrentSubmissionManifest{}, fmt.Errorf("invalid manifest line %q", line)
		}
	}

	if manifest.SubmissionID == "" || manifest.SpecPath == "" || len(manifest.FeaturePaths) == 0 || len(manifest.ScenarioIDs) == 0 || manifest.Worktree == "" || manifest.Status == "" {
		return CurrentSubmissionManifest{}, fmt.Errorf("manifest is missing required fields")
	}
	return manifest, nil
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
