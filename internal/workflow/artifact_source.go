package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errStopFeatureSearch = fmt.Errorf("stop feature search")

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

func GenerateNamespacedWorkflowPolicyArtifacts(repoRoot string, request WorkflowPolicyArtifactRequest) (WorkflowPolicyArtifacts, error) {
	contractID := strings.TrimSpace(request.ContractID)
	if contractID == "" {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("contract id is required")
	}

	artifacts := WorkflowPolicyArtifacts{
		SpecPath:    filepath.ToSlash(filepath.Join("specs", contractID+".md")),
		FeaturePath: filepath.ToSlash(filepath.Join("features", contractID+".feature")),
	}
	if artifacts.SpecPath == request.LegacySpecPath || artifacts.FeaturePath == request.LegacyFeaturePath {
		return WorkflowPolicyArtifacts{}, fmt.Errorf("namespaced artifact path would overwrite an active contract")
	}

	if err := writeWorkflowArtifact(filepath.Join(repoRoot, artifacts.SpecPath), request.HardSpec); err != nil {
		return WorkflowPolicyArtifacts{}, err
	}
	if err := writeWorkflowArtifact(filepath.Join(repoRoot, artifacts.FeaturePath), request.Feature); err != nil {
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
		classification.Kind = WorkflowArtifactLocalGeneratedCache
		classification.ReviewCandidate = input.IntentionallyTrackedGeneratedArtifact && input.ProjectArtifactDecision
		classification.RequiresProjectArtifactDecision = input.IntentionallyTrackedGeneratedArtifact && !input.ProjectArtifactDecision
		return classification
	}
	if isSensitiveWorkflowArtifact(input) {
		classification.Kind = WorkflowArtifactRejectedSensitive
		classification.ReviewCandidate = false
		classification.RequiresSanitizedReplacement = true
		return classification
	}
	retirementReason := strings.TrimSpace(input.RetirementReason)
	if input.Retired {
		classification.Kind = WorkflowArtifactRetired
		classification.ArchiveCandidate = retirementReason != ""
		classification.ArchiveReason = retirementReason
		return classification
	}
	if input.Superseded {
		classification.Kind = WorkflowArtifactSuperseded
		classification.ArchiveCandidate = retirementReason != ""
		classification.ArchiveReason = retirementReason
		return classification
	}
	if input.ProcessOnly {
		classification.Kind = WorkflowArtifactProcessOnly
		classification.ArchiveCandidate = retirementReason != ""
		classification.ArchiveReason = retirementReason
		return classification
	}
	if strings.HasPrefix(filepath.ToSlash(input.Path), "features/") && strings.HasSuffix(input.Path, ".feature") && input.Approved {
		classification.Kind = WorkflowArtifactActiveRegressionContract
		classification.ArchiveCandidate = false
	}
	return classification
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
	err := filepath.WalkDir(featuresRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".feature" {
			return nil
		}

		featurePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return fmt.Errorf("make feature path relative %s: %w", path, err)
		}
		featurePath = filepath.ToSlash(featurePath)
		approved, err := scopedApprovalContains(repoRoot, ContractScope{
			SpecPath:    specPathForFeature(featurePath),
			FeaturePath: featurePath,
			ScenarioID:  scenarioID,
		})
		if err != nil {
			return err
		}
		if !approved {
			return nil
		}

		file, closeFile, err := openRepositoryFile(repoRoot, featurePath)
		if err != nil {
			return fmt.Errorf("open feature file %s: %w", featurePath, err)
		}
		defer closeFile()
		scenarios, err := ParseFeatureScenarioTags(featurePath, file)
		if err != nil {
			return err
		}
		for _, scenario := range scenarios {
			if scenario.ScenarioID == scenarioID {
				found = featurePath
				return errStopFeatureSearch
			}
		}
		return nil
	})
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

func writeWorkflowArtifact(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create workflow artifact parent %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", path, err)
	}
	return nil
}
