package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const currentSubmissionStatusInProgress = "in_progress"

const defaultArchiveRetentionPeriod = 30 * 24 * time.Hour

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
	Phase                     string
	CompletedWork             []string
	RemainingWork             []string
	BlockedWork               []string
	LastAction                string
	SafeResumePoint           string
	BaselineCheckpoint        string
	ApprovalRecordPath        string
	ApprovalRecordFingerprint string
	Evidence                  []string
	Checkpoint                string
	NextScenario              string
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
	contents, err := readRepositoryFile(repoRoot, ".rotta/current/manifest.yaml")
	if err != nil {
		return CurrentSubmission{}, unusableCurrentSubmissionState(err)
	}

	manifest, err := parseCurrentSubmissionManifest(string(contents))
	if err != nil {
		return CurrentSubmission{}, unusableCurrentSubmissionState(err)
	}
	for _, featurePath := range manifest.FeaturePaths {
		if err := repositoryFileExists(repoRoot, featurePath); err != nil {
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

	stateContents, err := readRepositoryFile(repoRoot, ".rotta/current/state.yaml")
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

// RecordCurrentSubmissionAncoraState models the compact payload sent to
// Ancora. It reads local execution state and serializes references only, so
// unavailable memory never prevents a local lifecycle resume.
func RecordCurrentSubmissionAncoraState(repoRoot string) ([]byte, error) {
	resumed, err := ResumeCurrentSubmission(repoRoot, nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resumed.AncoraPointer.Repaired)
}

// ArchiveTerminalCurrentSubmission moves only the local execution directory
// after the submission reaches a terminal status and its feature changes are
// safely committed. Durable contracts remain at their manifest paths.
func ArchiveTerminalCurrentSubmission(repoRoot string, featureChangesCommitted bool) error {
	if !featureChangesCommitted {
		return fmt.Errorf("cannot archive current submission before feature changes are safely committed")
	}

	submission, err := LoadCurrentSubmission(repoRoot)
	if err != nil {
		return err
	}
	if !isTerminalCurrentSubmissionStatus(submission.Manifest.Status) {
		return fmt.Errorf("cannot archive non-terminal current submission status %q", submission.Manifest.Status)
	}

	archiveDirectory := filepath.Join(repoRoot, ".rotta", "archive")
	if err := os.MkdirAll(archiveDirectory, 0o700); err != nil {
		return fmt.Errorf("create current submission archive directory: %w", err)
	}
	if err := os.Rename(filepath.Join(repoRoot, ".rotta", "current"), filepath.Join(archiveDirectory, submission.Manifest.SubmissionID)); err != nil {
		return fmt.Errorf("archive current submission: %w", err)
	}
	return nil
}

func isTerminalCurrentSubmissionStatus(status string) bool {
	return status == "completed" || status == "abandoned" || status == "cancelled"
}

// CleanupExpiredArchivedSubmissions removes only archive directories whose
// local execution state has reached the default 30-day retention limit.
func CleanupExpiredArchivedSubmissions(repoRoot string, now time.Time) ([]string, error) {
	archiveRoot := filepath.Join(repoRoot, ".rotta", "archive")
	entries, err := os.ReadDir(archiveRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read submission archives: %w", err)
	}

	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect submission archive %q: %w", entry.Name(), err)
		}
		if now.Sub(info.ModTime()) < defaultArchiveRetentionPeriod {
			continue
		}
		if err := os.RemoveAll(filepath.Join(archiveRoot, entry.Name())); err != nil {
			return nil, fmt.Errorf("remove expired submission archive %q: %w", entry.Name(), err)
		}
		removed = append(removed, entry.Name())
	}
	return removed, nil
}

// RemoveArchivedSubmission explicitly removes one local execution archive.
// Archive IDs are constrained to direct children so cleanup cannot reach
// durable repository contracts.
func RemoveArchivedSubmission(repoRoot, submissionID string) error {
	if submissionID == "" || filepath.Base(submissionID) != submissionID || submissionID == "." {
		return fmt.Errorf("invalid submission archive ID %q", submissionID)
	}
	if err := os.RemoveAll(filepath.Join(repoRoot, ".rotta", "archive", submissionID)); err != nil {
		return fmt.Errorf("remove submission archive %q: %w", submissionID, err)
	}
	return nil
}

func unusableCurrentSubmissionState(cause error) error {
	return fmt.Errorf("current submission state cannot be safely used: %w", cause)
}

func parseCurrentSubmissionManifest(contents string) (CurrentSubmissionManifest, error) {
	var manifest CurrentSubmissionManifest
	if err := parseCurrentSubmissionDocument(contents,
		map[string]*string{
			"submission_id": &manifest.SubmissionID,
			"spec_path":     &manifest.SpecPath,
			"worktree":      &manifest.Worktree,
			"status":        &manifest.Status,
		},
		map[string]*[]string{
			"feature_paths:": &manifest.FeaturePaths,
			"scenario_ids:":  &manifest.ScenarioIDs,
		}); err != nil {
		return CurrentSubmissionManifest{}, fmt.Errorf("invalid manifest: %w", err)
	}

	if manifest.SubmissionID == "" || manifest.SpecPath == "" || len(manifest.FeaturePaths) == 0 || len(manifest.ScenarioIDs) == 0 || manifest.Worktree == "" || manifest.Status == "" {
		return CurrentSubmissionManifest{}, fmt.Errorf("manifest is missing required fields")
	}
	return manifest, nil
}

func parseCurrentSubmissionState(contents string) (CurrentSubmissionState, error) {
	var state CurrentSubmissionState
	if err := parseCurrentSubmissionDocument(contents,
		map[string]*string{
			"phase":                       &state.Phase,
			"last_action":                 &state.LastAction,
			"safe_resume_point":           &state.SafeResumePoint,
			"baseline_checkpoint":         &state.BaselineCheckpoint,
			"approval_record_path":        &state.ApprovalRecordPath,
			"approval_record_fingerprint": &state.ApprovalRecordFingerprint,
			"checkpoint":                  &state.Checkpoint,
			"next_scenario":               &state.NextScenario,
		},
		map[string]*[]string{
			"completed_work:": &state.CompletedWork,
			"remaining_work:": &state.RemainingWork,
			"blocked_work:":   &state.BlockedWork,
			"evidence:":       &state.Evidence,
		}); err != nil {
		return CurrentSubmissionState{}, fmt.Errorf("invalid state: %w", err)
	}
	if state.Phase == "" || state.LastAction == "" || state.SafeResumePoint == "" {
		return CurrentSubmissionState{}, fmt.Errorf("state is missing required fields")
	}
	return state, nil
}

func parseCurrentSubmissionDocument(contents string, scalarFields map[string]*string, listFields map[string]*[]string) error {
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
		if target, ok := listFields[line]; ok {
			list = target
			continue
		}
		name, value, ok := strings.Cut(line, ": ")
		if !ok {
			return fmt.Errorf("invalid line %q", line)
		}
		target, ok := scalarFields[name]
		if !ok {
			return fmt.Errorf("invalid line %q", line)
		}
		*target = value
	}
	return nil
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
	return fmt.Sprintf("phase: %s\ncompleted_work:\n%s\nremaining_work:\n%s\nblocked_work:\n%s\nlast_action: %s\nsafe_resume_point: %s\nbaseline_checkpoint: %s\napproval_record_path: %s\napproval_record_fingerprint: %s\nevidence:\n%s\ncheckpoint: %s\nnext_scenario: %s\n",
		state.Phase,
		yamlList(state.CompletedWork),
		yamlList(state.RemainingWork),
		yamlList(state.BlockedWork),
		state.LastAction,
		state.SafeResumePoint,
		state.BaselineCheckpoint,
		state.ApprovalRecordPath,
		state.ApprovalRecordFingerprint,
		yamlList(state.Evidence),
		state.Checkpoint,
		state.NextScenario,
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
