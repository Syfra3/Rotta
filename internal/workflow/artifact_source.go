package workflow

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errStopFeatureSearch = fmt.Errorf("stop feature search")

var errWorkflowArtifactCollision = errors.New("workflow artifact target already exists")

var errWorkflowArtifactPairIncomplete = errors.New("workflow artifact pair publication incomplete")

var errWorkflowArtifactCleanupIncomplete = errors.New("workflow artifact cleanup incomplete")

var publishWorkflowArtifact = publishWorkflowArtifactNoReplace

var copyWorkflowArtifact = io.Copy

var closeWorkflowArtifact = func(file *os.File) error {
	return file.Close()
}

type ContractSourceStatus struct {
	Authoritative              bool
	SpecTracked                bool
	FeatureTracked             bool
	RequiresAncoraContractText bool
}

type ContractCleanupActionKind string

const ContractCleanupTrack ContractCleanupActionKind = "track"

type ContractCleanupAction struct {
	Path string
	Kind ContractCleanupActionKind
}

type WorkflowArtifactCleanupActionKind string

const (
	WorkflowArtifactCleanupTrack       WorkflowArtifactCleanupActionKind = "track"
	WorkflowArtifactCleanupKeepPending WorkflowArtifactCleanupActionKind = "keep pending"
	WorkflowArtifactCleanupArchive     WorkflowArtifactCleanupActionKind = "archive"
	WorkflowArtifactCleanupIgnore      WorkflowArtifactCleanupActionKind = "ignore"
	WorkflowArtifactCleanupDelete      WorkflowArtifactCleanupActionKind = "delete"
)

type WorkflowArtifactCleanupGuidanceItem struct {
	Path   string
	Action WorkflowArtifactCleanupActionKind
	Reason string
}

type WorkflowArtifactLifecycleKind string

const (
	WorkflowArtifactActiveRegressionContract WorkflowArtifactLifecycleKind = "active_regression_contract"
	WorkflowArtifactLocalGeneratedCache      WorkflowArtifactLifecycleKind = "local_generated_cache"
	WorkflowArtifactRejectedSensitive        WorkflowArtifactLifecycleKind = "rejected_sensitive"
	WorkflowArtifactRetired                  WorkflowArtifactLifecycleKind = "retired"
	WorkflowArtifactSuperseded               WorkflowArtifactLifecycleKind = "superseded"
	WorkflowArtifactProcessOnly              WorkflowArtifactLifecycleKind = "process_only"
)

type WorkflowArtifactLifecycleInput struct {
	Path                                  string
	Implemented                           bool
	Approved                              bool
	Retired                               bool
	Superseded                            bool
	ProcessOnly                           bool
	IntentionallyTrackedGeneratedArtifact bool
	ProjectArtifactDecision               bool
	RetirementReason                      string
	Content                               string
}

type WorkflowArtifactLifecycleClassification struct {
	Path                            string
	Kind                            WorkflowArtifactLifecycleKind
	ArchiveCandidate                bool
	ArchiveReason                   string
	ReviewCandidate                 bool
	RequiresProjectArtifactDecision bool
	RequiresSanitizedReplacement    bool
}

type CompletedChangeArchivePlan struct {
	KeptActivePaths []string
	ArchiveMoves    []WorkflowArtifactArchiveMove
}

type WorkflowArtifactReviewSetPlan struct {
	IncludedPaths []string
	ExcludedPaths []string
}

type WorkflowArtifactCleanupGuidanceReport struct {
	Items []WorkflowArtifactCleanupGuidanceItem
}

type WorkflowArtifactArchiveMove struct {
	SourcePath      string
	DestinationPath string
	Reason          string
}

type WorkflowPolicyArtifactRequest struct {
	ContractID        string
	HardSpec          string
	Feature           string
	LegacySpecPath    string
	LegacyFeaturePath string
}

type WorkflowPolicyArtifacts struct {
	SpecPath    string
	FeaturePath string
}

// WorkflowArtifactPublicationStatus records whether this invocation published
// an individual member of a draft pair. It is deliberately about publication,
// not whether a pathname currently exists (a failed target may be foreign).
type WorkflowArtifactPublicationStatus string

const (
	WorkflowArtifactNotPublished WorkflowArtifactPublicationStatus = "not_published"
	WorkflowArtifactPublished    WorkflowArtifactPublicationStatus = "published"
)

type WorkflowArtifactPairPublicationStatus struct {
	Spec    WorkflowArtifactPublicationStatus
	Feature WorkflowArtifactPublicationStatus
}

func (status WorkflowArtifactPairPublicationStatus) Complete() bool {
	return status.Spec == WorkflowArtifactPublished && status.Feature == WorkflowArtifactPublished
}

// WorkflowArtifactPairIncompleteError is the explicit recoverable result of a
// failed sequential pair publication. The canonical paths and per-member
// publication status remain available to a later explicit recovery attempt.
type WorkflowArtifactPairIncompleteError struct {
	Artifacts  WorkflowPolicyArtifacts
	Status     WorkflowArtifactPairPublicationStatus
	cause      error
	retainedID fs.FileInfo
}

func (err *WorkflowArtifactPairIncompleteError) Error() string {
	return fmt.Sprintf("%s: spec %s (%s), feature %s (%s)", errWorkflowArtifactPairIncomplete, err.Artifacts.SpecPath, err.Status.Spec, err.Artifacts.FeaturePath, err.Status.Feature)
}

func (err *WorkflowArtifactPairIncompleteError) Unwrap() error {
	return errors.Join(errWorkflowArtifactPairIncomplete, err.cause)
}

func GenerateNamespacedWorkflowPolicyArtifacts(repoRoot string, request WorkflowPolicyArtifactRequest) (WorkflowPolicyArtifacts, error) {
	contractID := strings.TrimSpace(request.ContractID)
	if contractID == "" {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("contract id is required")
	}

	artifacts := WorkflowPolicyArtifacts{
		SpecPath:    filepath.ToSlash(filepath.Join("specs", contractID+".md")),
		FeaturePath: filepath.ToSlash(filepath.Join("features", contractID+".feature")),
	}
	if err := validatePendingContractPaths(artifacts.SpecPath, artifacts.FeaturePath); err != nil {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("invalid namespaced contract id %q: %w", contractID, err)
	}
	if artifacts.SpecPath == request.LegacySpecPath || artifacts.FeaturePath == request.LegacyFeaturePath {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("namespaced artifact path would overwrite an active contract")
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("open workflow artifact root: %w", err)
	}
	defer root.Close()
	for _, path := range []string{artifacts.SpecPath, artifacts.FeaturePath} {
		if err := prepareWorkflowArtifactParent(root, path); err != nil {
			return WorkflowPolicyArtifacts{}, err
		}
	}
	if err := writeWorkflowArtifactPair(root, artifacts, request.HardSpec, request.Feature); err != nil {
		return WorkflowPolicyArtifacts{}, err
	}
	return artifacts, nil
}

func EvaluateContractSourceOfTruth(repoRoot string, scope ContractScope) (ContractSourceStatus, error) {
	specTracked, err := gitTracksPath(repoRoot, scope.SpecPath)
	if err != nil {
		return ContractSourceStatus{}, err
	}
	featureTracked, err := gitTracksPath(repoRoot, scope.FeaturePath)
	if err != nil {
		return ContractSourceStatus{}, err
	}

	authoritative := specTracked && featureTracked
	return ContractSourceStatus{
		Authoritative:              authoritative,
		SpecTracked:                specTracked,
		FeatureTracked:             featureTracked,
		RequiresAncoraContractText: false,
	}, nil
}

func PlanCleanTreeContractActions(repoRoot string, scope ContractScope) ([]ContractCleanupAction, error) {
	decision, err := EvaluateImplementationGate(repoRoot, scope)
	if err != nil {
		return nil, err
	}
	if !decision.Approved {
		return nil, fmt.Errorf("cannot plan active contract cleanup: %s", decision.Reason)
	}

	var actions []ContractCleanupAction
	for _, path := range []string{scope.SpecPath, scope.FeaturePath} {
		tracked, err := gitTracksPath(repoRoot, path)
		if err != nil {
			return nil, err
		}
		if !tracked {
			actions = append(actions, ContractCleanupAction{Path: path, Kind: ContractCleanupTrack})
		}
	}
	return actions, nil
}

func ClassifyWorkflowArtifactLifecycle(input WorkflowArtifactLifecycleInput) WorkflowArtifactLifecycleClassification {
	classification := WorkflowArtifactLifecycleClassification{Path: input.Path, ReviewCandidate: true}
	if isLocalGeneratedGraphOrCachePath(input.Path) {
		return localGeneratedArtifactClassification(classification, input)
	}
	if isSensitiveWorkflowArtifact(input) {
		return sensitiveArtifactClassification(classification)
	}
	if classification, classified := classifyArtifactRetirement(classification, input); classified {
		return classification
	}
	if strings.HasPrefix(filepath.ToSlash(input.Path), "features/") && strings.HasSuffix(input.Path, ".feature") && input.Approved {
		classification.Kind = WorkflowArtifactActiveRegressionContract
		classification.ArchiveCandidate = false
	}
	return classification
}

func localGeneratedArtifactClassification(classification WorkflowArtifactLifecycleClassification, input WorkflowArtifactLifecycleInput) WorkflowArtifactLifecycleClassification {
	classification.Kind = WorkflowArtifactLocalGeneratedCache
	classification.ReviewCandidate = input.IntentionallyTrackedGeneratedArtifact && input.ProjectArtifactDecision
	classification.RequiresProjectArtifactDecision = input.IntentionallyTrackedGeneratedArtifact && !input.ProjectArtifactDecision
	return classification
}

func sensitiveArtifactClassification(classification WorkflowArtifactLifecycleClassification) WorkflowArtifactLifecycleClassification {
	classification.Kind = WorkflowArtifactRejectedSensitive
	classification.ReviewCandidate = false
	classification.RequiresSanitizedReplacement = true
	return classification
}

func classifyArtifactRetirement(classification WorkflowArtifactLifecycleClassification, input WorkflowArtifactLifecycleInput) (WorkflowArtifactLifecycleClassification, bool) {
	kind, retired := artifactRetirementKind(input)
	if !retired {
		return classification, false
	}
	classification.Kind = kind
	classification.ArchiveReason = strings.TrimSpace(input.RetirementReason)
	classification.ArchiveCandidate = classification.ArchiveReason != ""
	return classification, true
}

func artifactRetirementKind(input WorkflowArtifactLifecycleInput) (WorkflowArtifactLifecycleKind, bool) {
	if input.Retired {
		return WorkflowArtifactRetired, true
	}
	if input.Superseded {
		return WorkflowArtifactSuperseded, true
	}
	if input.ProcessOnly {
		return WorkflowArtifactProcessOnly, true
	}
	return "", false
}

func isSensitiveWorkflowArtifact(input WorkflowArtifactLifecycleInput) bool {
	path := filepath.ToSlash(strings.ToLower(input.Path))
	if hasSensitiveWorkflowPath(path) {
		return true
	}
	if isSanitizedAuthoredExample(path, input.Content) {
		return false
	}
	return hasSensitiveContentMarker(input.Content)
}

func isSanitizedAuthoredExample(path, content string) bool {
	return strings.Contains(path, "example") && strings.Contains(strings.ToLower(content), "redacted")
}

func hasSensitiveWorkflowPath(path string) bool {
	for _, part := range strings.Split(path, "/") {
		switch part {
		case "backup", "backups", "restore", "restores", "snapshot", "snapshots", "captures", "machine-state", ".ssh", "ssh":
			return true
		}
	}
	for _, marker := range []string{"token", "secret", "api_key", "apikey", "private_key"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func hasSensitiveContentMarker(content string) bool {
	normalized := strings.ToLower(content)
	if strings.Contains(normalized, "redacted") {
		return false
	}
	for _, marker := range []string{"api_token", "token", "api_key", "apikey", "secret", "authorization:", "bearer ", "identityfile", "private key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func PrepareWorkflowArtifactReviewSet(inputs []WorkflowArtifactLifecycleInput) WorkflowArtifactReviewSetPlan {
	var plan WorkflowArtifactReviewSetPlan
	for _, input := range inputs {
		classification := ClassifyWorkflowArtifactLifecycle(input)
		if classification.ReviewCandidate {
			plan.IncludedPaths = append(plan.IncludedPaths, classification.Path)
			continue
		}
		plan.ExcludedPaths = append(plan.ExcludedPaths, classification.Path)
	}
	return plan
}

func PrepareCompletedChangeArchive(repoRoot string) (CompletedChangeArchivePlan, error) {
	completed, err := completedScenarioIDs(repoRoot)
	if err != nil {
		return CompletedChangeArchivePlan{}, err
	}

	var plan CompletedChangeArchivePlan
	for scenarioID := range completed {
		featurePath, err := approvedFeaturePathForScenario(repoRoot, scenarioID)
		if err != nil {
			return CompletedChangeArchivePlan{}, err
		}
		if featurePath == "" {
			continue
		}
		classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{
			Path:        featurePath,
			Implemented: true,
			Approved:    true,
		})
		if classification.Kind == WorkflowArtifactActiveRegressionContract && !classification.ArchiveCandidate {
			plan.KeptActivePaths = append(plan.KeptActivePaths, featurePath)
		}
	}
	return plan, nil
}

func PlanWorkflowArtifactArchive(inputs []WorkflowArtifactLifecycleInput) CompletedChangeArchivePlan {
	var plan CompletedChangeArchivePlan
	for _, input := range inputs {
		classification := ClassifyWorkflowArtifactLifecycle(input)
		if classification.Kind == WorkflowArtifactActiveRegressionContract && !classification.ArchiveCandidate {
			plan.KeptActivePaths = append(plan.KeptActivePaths, classification.Path)
			continue
		}
		if !classification.ArchiveCandidate {
			continue
		}
		plan.ArchiveMoves = append(plan.ArchiveMoves, WorkflowArtifactArchiveMove{
			SourcePath:      classification.Path,
			DestinationPath: filepath.ToSlash(filepath.Join("archive", classification.Path)),
			Reason:          classification.ArchiveReason,
		})
	}
	return plan
}

func PrepareWorkflowArtifactCleanupGuidance(inputs []WorkflowArtifactLifecycleInput) WorkflowArtifactCleanupGuidanceReport {
	var report WorkflowArtifactCleanupGuidanceReport
	for _, input := range inputs {
		classification := ClassifyWorkflowArtifactLifecycle(input)
		item := WorkflowArtifactCleanupGuidanceItem{Path: input.Path}
		switch {
		case classification.Kind == WorkflowArtifactRejectedSensitive:
			item.Action = WorkflowArtifactCleanupDelete
			item.Reason = "sensitive workflow output must be deleted, ignored, or replaced with a sanitized authored example"
		case classification.Kind == WorkflowArtifactLocalGeneratedCache:
			item.Action = WorkflowArtifactCleanupIgnore
			item.Reason = "local generated graph or cache artifact stays ignored unless intentionally promoted"
		case isWorkflowContractPath(input.Path) && !input.Approved:
			item.Action = WorkflowArtifactCleanupKeepPending
			item.Reason = "pending contract remains pending until human approval"
		case classification.ArchiveCandidate:
			item.Action = WorkflowArtifactCleanupArchive
			item.Reason = classification.ArchiveReason
		case isWorkflowContractPath(input.Path) && input.Approved:
			item.Action = WorkflowArtifactCleanupTrack
			item.Reason = "active behavior contract remains tracked"
		default:
			item.Action = WorkflowArtifactCleanupTrack
			item.Reason = "project artifact remains tracked for review"
		}
		report.Items = append(report.Items, item)
	}
	return report
}

func isWorkflowContractPath(path string) bool {
	normalized := filepath.ToSlash(path)
	return (strings.HasPrefix(normalized, "features/") && strings.HasSuffix(normalized, ".feature")) ||
		(strings.HasPrefix(normalized, "specs/") && strings.HasSuffix(normalized, ".md"))
}

func isLocalGeneratedGraphOrCachePath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case ".vela", ".cache", "cache", "caches":
			return true
		}
	}
	return false
}

func completedScenarioIDs(repoRoot string) (map[string]bool, error) {
	content, err := readRepositoryFile(repoRoot, "specs/.implementation-complete")
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read implementation completion marker: %w", err)
	}

	completed := map[string]bool{}
	for _, line := range strings.Split(string(content), "\n") {
		id := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if strings.HasPrefix(id, "SCN-") {
			completed[id] = true
		}
	}
	return completed, nil
}

func approvedFeaturePathForScenario(repoRoot, scenarioID string) (string, error) {
	featuresRoot := filepath.Join(repoRoot, "features")
	var found string
	err := filepath.WalkDir(featuresRoot, approvedFeatureFinder(repoRoot, scenarioID, &found))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		if err == errStopFeatureSearch {
			return found, nil
		}
		return "", fmt.Errorf("find approved feature for completed scenario: %w", err)
	}
	return "", nil
}

func approvedFeatureFinder(repoRoot, scenarioID string, found *string) fs.WalkDirFunc {
	return func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".feature" {
			return walkErr
		}
		featurePath, err := relativeFeaturePath(repoRoot, path)
		if err != nil {
			return err
		}
		matches, err := approvedFeatureContainsScenario(repoRoot, featurePath, scenarioID)
		if err != nil || !matches {
			return err
		}
		*found = featurePath
		return errStopFeatureSearch
	}
}

func relativeFeaturePath(repoRoot, path string) (string, error) {
	featurePath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return "", fmt.Errorf("make feature path relative %s: %w", path, err)
	}
	return filepath.ToSlash(featurePath), nil
}

func approvedFeatureContainsScenario(repoRoot, featurePath, scenarioID string) (bool, error) {
	approved, err := scopedApprovalContains(repoRoot, ContractScope{SpecPath: specPathForFeature(featurePath), FeaturePath: featurePath, ScenarioID: scenarioID})
	if err != nil || !approved {
		return false, err
	}
	file, closeFile, err := openRepositoryFile(repoRoot, featurePath)
	if err != nil {
		return false, fmt.Errorf("open feature file %s: %w", featurePath, err)
	}
	defer closeFile()
	scenarios, err := ParseFeatureScenarioTags(featurePath, file)
	if err != nil {
		return false, err
	}
	for _, scenario := range scenarios {
		if scenario.ScenarioID == scenarioID {
			return true, nil
		}
	}
	return false, nil
}

func gitTracksPath(repoRoot, path string) (bool, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("check tracked path %s: %w: %s", path, err, output)
	}
	wanted := filepath.ToSlash(filepath.Clean(path))
	for _, tracked := range strings.Fields(string(output)) {
		if filepath.ToSlash(tracked) == wanted {
			return true, nil
		}
	}
	return false, nil
}

func prepareWorkflowArtifactParent(root *os.Root, path string) error {
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid workflow artifact path %q", path)
	}
	if err := root.MkdirAll(filepath.Dir(clean), 0o750); err != nil {
		return fmt.Errorf("create workflow artifact parent %s: %w", filepath.Dir(clean), err)
	}
	return nil
}

func writeWorkflowArtifact(root *os.Root, path, content string) error {
	clean := filepath.Clean(filepath.FromSlash(path))
	file, err := root.OpenFile(clean, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", clean, err)
	}
	defer file.Close()
	if _, err := file.Write([]byte(content)); err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", clean, err)
	}
	return nil
}

func writeWorkflowArtifactPair(root *os.Root, artifacts WorkflowPolicyArtifacts, hardSpec, feature string) (err error) {
	stagedSpec, err := stageWorkflowArtifact(root, artifacts.SpecPath, hardSpec)
	if err != nil {
		return err
	}
	defer func() {
		if stagedSpec != "" {
			err = errors.Join(err, removeStagedWorkflowArtifact(root, stagedSpec))
		}
	}()
	stagedFeature, err := stageWorkflowArtifact(root, artifacts.FeaturePath, feature)
	if err != nil {
		return err
	}
	defer func() {
		if stagedFeature != "" {
			err = errors.Join(err, removeStagedWorkflowArtifact(root, stagedFeature))
		}
	}()

	if err := publishWorkflowArtifact(root, stagedSpec, artifacts.SpecPath); err != nil {
		return err
	}
	stagedSpec = ""
	retainedID, err := root.Stat(filepath.FromSlash(artifacts.SpecPath))
	if err != nil {
		return incompleteWorkflowArtifactPairError(artifacts, retainedID, err)
	}
	if err := publishWorkflowArtifact(root, stagedFeature, artifacts.FeaturePath); err != nil {
		// Do not roll back the published spec by pathname. Another actor may have
		// replaced it after publication, and os.Root.Remove cannot prove ownership
		// at removal time. Retain it and report the incomplete pair instead.
		return incompleteWorkflowArtifactPairError(artifacts, retainedID, err)
	}
	stagedFeature = ""
	return nil
}

func incompleteWorkflowArtifactPairError(artifacts WorkflowPolicyArtifacts, retainedID fs.FileInfo, cause error) *WorkflowArtifactPairIncompleteError {
	return &WorkflowArtifactPairIncompleteError{
		Artifacts:  artifacts,
		Status:     WorkflowArtifactPairPublicationStatus{Spec: WorkflowArtifactPublished, Feature: WorkflowArtifactNotPublished},
		cause:      cause,
		retainedID: retainedID,
	}
}

func publishWorkflowArtifactNoReplace(root *os.Root, stagedPath, artifactPath string) error {
	stagedPath = filepath.Clean(filepath.FromSlash(stagedPath))
	artifactPath = filepath.Clean(filepath.FromSlash(artifactPath))
	staged, err := root.Open(stagedPath)
	if err != nil {
		return fmt.Errorf("open staged workflow artifact %s: %w", stagedPath, err)
	}
	defer staged.Close()

	artifact, err := root.OpenFile(artifactPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", errWorkflowArtifactCollision, artifactPath)
		}
		return fmt.Errorf("reserve workflow artifact %s: %w", artifactPath, err)
	}
	if _, err := copyWorkflowArtifact(artifact, staged); err != nil {
		closeErr := closeWorkflowArtifact(artifact)
		return errors.Join(
			fmt.Errorf("publish workflow artifact %s: %w", artifactPath, err),
			closeErr,
			incompleteWorkflowArtifactCleanupError(artifactPath),
		)
	}
	if err := closeWorkflowArtifact(artifact); err != nil {
		return errors.Join(
			fmt.Errorf("close published workflow artifact %s: %w", artifactPath, err),
			incompleteWorkflowArtifactCleanupError(artifactPath),
		)
	}
	if err := removeStagedWorkflowArtifact(root, stagedPath); err != nil {
		return err
	}
	return nil
}

func incompleteWorkflowArtifactCleanupError(artifactPath string) error {
	// O_EXCL proves only that this invocation created the original directory
	// entry. There is no compare-and-remove operation for os.Root, so a later
	// pathname removal could delete a foreign replacement. Preserve the target
	// and report the partial publication for explicit recovery.
	return fmt.Errorf("%w: retained %s after failed publication", errWorkflowArtifactCleanupIncomplete, artifactPath)
}

func stageWorkflowArtifact(root *os.Root, path, content string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	stagedPath, err := workflowArtifactTemporaryPath(clean)
	if err != nil {
		return "", err
	}
	if err := writeWorkflowArtifact(root, stagedPath, content); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", err
		}
		return "", errors.Join(err, removeStagedWorkflowArtifact(root, stagedPath))
	}
	return stagedPath, nil
}

func workflowArtifactTemporaryPath(path string) (string, error) {
	randomSuffix := make([]byte, 12)
	if _, err := rand.Read(randomSuffix); err != nil {
		return "", fmt.Errorf("generate workflow artifact temporary path: %w", err)
	}
	return fmt.Sprintf("%s.tmp-%x", path, randomSuffix), nil
}

func removeStagedWorkflowArtifact(root *os.Root, path string) error {
	if err := root.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove staged workflow artifact %s: %w", path, err)
	}
	return nil
}
