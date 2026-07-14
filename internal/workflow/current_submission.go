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
	BlockedWork     []string
	LastAction      string
	SafeResumePoint string
}

type CurrentSubmission struct {
	ManifestPath string
	StatePath    string
	Manifest     CurrentSubmissionManifest
	State        CurrentSubmissionState
}

// CurrentSubmissionAncoraPointer is the compact state that can be saved to
// Ancora. It deliberately contains references and progress, not contracts.
type CurrentSubmissionAncoraPointer struct {
	SubmissionID   string
	Phase          string
	Status         string
	CompletedWork  []string
	RemainingWork  []string
	BlockedWork    []string
	LastAction     string
	LocalStatePath string
	EvidencePaths  []string
}

type CurrentSubmissionAncoraPointerReport struct {
	Unavailable bool
	Stale       bool
	Repaired    CurrentSubmissionAncoraPointer
}

type CurrentSubmissionResume struct {
	CurrentSubmission
	CompletedWork []string
	RemainingWork []string
	BlockedWork   []string
	AncoraPointer CurrentSubmissionAncoraPointerReport
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

// ResumeCurrentSubmission derives progress only from current local workflow
// files. A supplied Ancora pointer is checked and returned in repaired form for
// the caller to save when memory is available; it is never used as contract
// content or a recovery source.
func ResumeCurrentSubmission(repoRoot string, pointer *CurrentSubmissionAncoraPointer) (CurrentSubmissionResume, error) {
	submission, err := LoadCurrentSubmission(repoRoot)
	if err != nil {
		return CurrentSubmissionResume{}, err
	}

	stateContents, err := os.ReadFile(submission.StatePath)
	if err != nil {
		return CurrentSubmissionResume{}, unusableCurrentSubmissionState(err)
	}
	state, err := parseCurrentSubmissionState(string(stateContents))
	if err != nil {
		return CurrentSubmissionResume{}, unusableCurrentSubmissionState(err)
	}
	submission.State = state

	repaired := currentSubmissionAncoraPointer(submission)
	report := CurrentSubmissionAncoraPointerReport{Repaired: repaired}
	if pointer == nil {
		report.Unavailable = true
	} else if !sameCurrentSubmissionAncoraPointer(*pointer, repaired) {
		report.Stale = true
	}

	return CurrentSubmissionResume{
		CurrentSubmission: submission,
		CompletedWork:     append([]string(nil), state.CompletedWork...),
		RemainingWork:     append([]string(nil), state.RemainingWork...),
		BlockedWork:       append([]string(nil), state.BlockedWork...),
		AncoraPointer:     report,
	}, nil
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
		if line == "  []" && list != nil {
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

func parseCurrentSubmissionState(contents string) (CurrentSubmissionState, error) {
	var state CurrentSubmissionState
	var list *[]string
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		if strings.HasPrefix(line, "  - ") && list != nil {
			*list = append(*list, strings.TrimPrefix(line, "  - "))
			continue
		}
		if line == "  []" && list != nil {
			continue
		}

		list = nil
		switch {
		case strings.HasPrefix(line, "phase: "):
			state.Phase = strings.TrimPrefix(line, "phase: ")
		case line == "completed_work:":
			list = &state.CompletedWork
		case line == "remaining_work:":
			list = &state.RemainingWork
		case line == "blocked_work:":
			list = &state.BlockedWork
		case strings.HasPrefix(line, "last_action: "):
			state.LastAction = strings.TrimPrefix(line, "last_action: ")
		case strings.HasPrefix(line, "safe_resume_point: "):
			state.SafeResumePoint = strings.TrimPrefix(line, "safe_resume_point: ")
		default:
			return CurrentSubmissionState{}, fmt.Errorf("invalid state line %q", line)
		}
	}
	if state.Phase == "" || state.LastAction == "" || state.SafeResumePoint == "" {
		return CurrentSubmissionState{}, fmt.Errorf("state is missing required fields")
	}
	return state, nil
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
	return fmt.Sprintf("phase: %s\ncompleted_work:\n%s\nremaining_work:\n%s\nblocked_work:\n%s\nlast_action: %s\nsafe_resume_point: %s\n",
		state.Phase,
		yamlList(state.CompletedWork),
		yamlList(state.RemainingWork),
		yamlList(state.BlockedWork),
		state.LastAction,
		state.SafeResumePoint,
	)
}

func currentSubmissionAncoraPointer(submission CurrentSubmission) CurrentSubmissionAncoraPointer {
	return CurrentSubmissionAncoraPointer{
		SubmissionID:   submission.Manifest.SubmissionID,
		Phase:          submission.State.Phase,
		Status:         submission.Manifest.Status,
		CompletedWork:  append([]string(nil), submission.State.CompletedWork...),
		RemainingWork:  append([]string(nil), submission.State.RemainingWork...),
		BlockedWork:    append([]string(nil), submission.State.BlockedWork...),
		LastAction:     submission.State.LastAction,
		LocalStatePath: ".rotta/current/state.yaml",
		EvidencePaths:  []string{".rotta/current/tdd-log.md"},
	}
}

func sameCurrentSubmissionAncoraPointer(left, right CurrentSubmissionAncoraPointer) bool {
	return left.SubmissionID == right.SubmissionID &&
		left.Phase == right.Phase &&
		left.Status == right.Status &&
		left.LastAction == right.LastAction &&
		left.LocalStatePath == right.LocalStatePath &&
		strings.Join(left.CompletedWork, "\x00") == strings.Join(right.CompletedWork, "\x00") &&
		strings.Join(left.RemainingWork, "\x00") == strings.Join(right.RemainingWork, "\x00") &&
		strings.Join(left.BlockedWork, "\x00") == strings.Join(right.BlockedWork, "\x00") &&
		strings.Join(left.EvidencePaths, "\x00") == strings.Join(right.EvidencePaths, "\x00")
}

func yamlList(values []string) string {
	if len(values) == 0 {
		return "  []"
	}
	return "  - " + strings.Join(values, "\n  - ")
}
