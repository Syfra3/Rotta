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

type WorkflowArtifactLifecycleKind string

const (
	WorkflowArtifactActiveRegressionContract WorkflowArtifactLifecycleKind = "active_regression_contract"
	WorkflowArtifactLocalGeneratedCache      WorkflowArtifactLifecycleKind = "local_generated_cache"
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
}

type WorkflowArtifactLifecycleClassification struct {
	Path                            string
	Kind                            WorkflowArtifactLifecycleKind
	ArchiveCandidate                bool
	ArchiveReason                   string
	ReviewCandidate                 bool
	RequiresProjectArtifactDecision bool
}

type CompletedChangeArchivePlan struct {
	KeptActivePaths []string
	ArchiveMoves    []WorkflowArtifactArchiveMove
}

type WorkflowArtifactReviewSetPlan struct {
	IncludedPaths []string
	ExcludedPaths []string
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
	content, err := os.ReadFile(filepath.Join(repoRoot, "specs", ".implementation-complete"))
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

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open feature file %s: %w", featurePath, err)
		}
		defer file.Close()
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
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("check tracked path %s: %w: %s", path, err, output)
	}
	return true, nil
}

func writeWorkflowArtifact(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create workflow artifact parent %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", path, err)
	}
	return nil
}
