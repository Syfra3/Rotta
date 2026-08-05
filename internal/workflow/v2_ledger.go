package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	V2DraftStatus    = "Draft"
	V2ContractStatus = "Contract"
	V2TDDStatus      = "TDD"
	V2ReviewStatus   = "Review"
	V2ArchiveStatus  = "Archive"
	V2ArchivedStatus = "archived"

	v2OrchestratorAuthorizer = "orchestrator"
)

var v2SubmissionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// V2NewSubmissionRequest is the only request that may create initial v2 state.
type V2NewSubmissionRequest struct {
	SubmissionID string
	Draft        string
	BaseCommit   string
}

// V2TransitionRequest carries the complete authorization context required for
// a lifecycle write. Workers may provide evidence, but cannot be authorizers.
type V2TransitionRequest struct {
	SubmissionID    string
	ExpectedStatus  string
	TargetStatus    string
	LedgerVersion   uint64
	Authorizer      string
	AuthorizedScope []string
	EvidenceRefs    []string
}

type V2Transition struct {
	FromStatus      string   `json:"from_status"`
	ToStatus        string   `json:"to_status"`
	LedgerVersion   uint64   `json:"ledger_version"`
	Authorizer      string   `json:"authorizer"`
	AuthorizedScope []string `json:"authorized_scope"`
	EvidenceRefs    []string `json:"evidence_refs"`
}

// V2SubmissionLedger is operational state only; it does not contain approved
// contract scope until a separately approved Contract-to-TDD transition.
type V2SubmissionLedger struct {
	SubmissionID             string                     `json:"submission_id"`
	Draft                    string                     `json:"draft"`
	Status                   string                     `json:"status"`
	LedgerVersion            uint64                     `json:"ledger_version"`
	BaseCommit               string                     `json:"base_commit"`
	ContractFingerprint      string                     `json:"contract_fingerprint,omitempty"`
	ContractCommit           string                     `json:"contract_commit,omitempty"`
	ApprovedScenarioIDs      []string                   `json:"approved_scenario_ids,omitempty"`
	AcceptedScenarioIDs      []string                   `json:"accepted_scenario_ids,omitempty"`
	TDDEvidence              []V2ScenarioRGREvidence    `json:"tdd_evidence,omitempty"`
	TargetedVelaEvidence     []V2TargetedVelaQuestion   `json:"targeted_vela_evidence,omitempty"`
	ImplementationCommit     string                     `json:"implementation_commit,omitempty"`
	ReviewedCommit           string                     `json:"reviewed_commit,omitempty"`
	QualityPolicyFingerprint string                     `json:"quality_policy_fingerprint,omitempty"`
	QualityEvidence          []V2QualityDimensionResult `json:"quality_evidence,omitempty"`
	Publication              *V2Publication             `json:"publication,omitempty"`
	Worktree                 *V2WorktreeIdentity        `json:"worktree,omitempty"`
	Transitions              []V2Transition             `json:"transitions,omitempty"`
}

// V2ContractArtifact identifies the approved behavioral contract without
// placing its content or approval authority in the operational ledger.
type V2ContractArtifact struct {
	SubmissionID string   `json:"submission_id"`
	Fingerprint  string   `json:"fingerprint"`
	SpecPath     string   `json:"spec_path"`
	FeaturePaths []string `json:"feature_paths"`
}

// V2AncoraPointer is advisory only. It is compared with durable state but is
// never used to choose, repair, or advance lifecycle state.
type V2AncoraPointer struct {
	SubmissionID        string
	LedgerVersion       uint64
	Status              string
	ContractFingerprint string
}

type V2AncoraPointerState string

const (
	V2AncoraUnavailable V2AncoraPointerState = "unavailable"
	V2AncoraConsistent  V2AncoraPointerState = "consistent"
	V2AncoraStale       V2AncoraPointerState = "stale_or_conflicting"
)

type V2ResumeResult struct {
	Ledger             V2SubmissionLedger
	Contract           V2ContractArtifact
	AncoraPointerState V2AncoraPointerState
}

func InitializeV2NewSubmission(repoRoot string, request V2NewSubmissionRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if strings.TrimSpace(request.Draft) == "" {
		return V2SubmissionLedger{}, errors.New("NEW submission requires a supplied draft")
	}
	if !isFullCommitID(request.BaseCommit) {
		return V2SubmissionLedger{}, errors.New("NEW submission requires a full immutable base commit identifier")
	}
	if err := rejectExistingV2Identity(repoRoot, request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}

	ledger := V2SubmissionLedger{
		SubmissionID:  request.SubmissionID,
		Draft:         request.Draft,
		Status:        V2DraftStatus,
		LedgerVersion: 1,
		BaseCommit:    strings.ToLower(request.BaseCommit),
	}
	contents, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("serialize initial v2 submission ledger: %w", err)
	}

	directory := filepath.Join(repoRoot, ".rotta", "v2", "submissions")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("create v2 submission directory: %w", err)
	}
	path := v2LedgerPath(repoRoot, request.SubmissionID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return V2SubmissionLedger{}, fmt.Errorf("v2 submission identity %q is not fresh; choose a different identity or resolve the existing durable artifact outside lifecycle initialization", request.SubmissionID)
		}
		return V2SubmissionLedger{}, fmt.Errorf("create initial v2 submission ledger: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return V2SubmissionLedger{}, fmt.Errorf("write initial v2 submission ledger: %w", err)
	}
	if err := file.Close(); err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("close initial v2 submission ledger: %w", err)
	}
	return ledger, nil
}

func LoadV2SubmissionLedger(repoRoot, submissionID string) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(submissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	contents, err := os.ReadFile(v2LedgerPath(repoRoot, submissionID))
	if err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("read v2 submission ledger: %w", err)
	}
	var ledger V2SubmissionLedger
	if err := json.Unmarshal(contents, &ledger); err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("parse v2 submission ledger: %w", err)
	}
	if err := validateV2Ledger(ledger, submissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

// RecordV2ContractArtifact persists the contract identity produced in Contract.
// Approval and lifecycle state remain separate orchestrator-authorized actions.
func RecordV2ContractArtifact(repoRoot string, artifact V2ContractArtifact) error {
	if err := validateV2SubmissionID(artifact.SubmissionID); err != nil {
		return err
	}
	if strings.TrimSpace(artifact.Fingerprint) == "" || !isRepositoryPath(artifact.SpecPath) || len(artifact.FeaturePaths) == 0 {
		return errors.New("v2 contract artifact is incomplete")
	}
	for _, path := range artifact.FeaturePaths {
		if !isRepositoryPath(path) {
			return errors.New("v2 contract artifact contains an invalid feature path")
		}
	}
	contents, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize v2 contract artifact: %w", err)
	}
	directory := filepath.Join(repoRoot, ".rotta", "v2", "contracts")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create v2 contract directory: %w", err)
	}
	file, err := os.OpenFile(v2ContractPath(repoRoot, artifact.SubmissionID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("v2 contract artifact already exists for %q; changed contract content requires a new explicit approval", artifact.SubmissionID)
		}
		return fmt.Errorf("create v2 contract artifact: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write v2 contract artifact: %w", err)
	}
	return nil
}

// ResumeV2Submission validates only phase-appropriate durable v2 records.
// A missing or conflicting artifact fails closed and starts no lifecycle work.
func ResumeV2Submission(repoRoot, submissionID string, pointer *V2AncoraPointer) (V2ResumeResult, error) {
	ledger, err := LoadV2SubmissionLedger(repoRoot, submissionID)
	if err != nil {
		return V2ResumeResult{}, fmt.Errorf("cannot establish durable v2 lifecycle state for %q: %w; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID, err)
	}
	if ledger.Status == V2DraftStatus {
		if strings.TrimSpace(ledger.Draft) == "" {
			return V2ResumeResult{}, fmt.Errorf("cannot establish durable v2 lifecycle state for %q: initial Draft record is incomplete; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID)
		}
		return V2ResumeResult{Ledger: ledger, AncoraPointerState: assessV2AncoraPointer(ledger, V2ContractArtifact{}, pointer)}, nil
	}
	if !isV2LifecycleStatus(ledger.Status) {
		return V2ResumeResult{}, fmt.Errorf("cannot establish durable v2 lifecycle state for %q: invalid status %q; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID, ledger.Status)
	}
	contract, err := loadV2ContractArtifact(repoRoot, submissionID)
	if err != nil {
		return V2ResumeResult{}, fmt.Errorf("cannot establish durable v2 lifecycle state for %q: %w; safe next action: restore or resolve matching durable contract artifacts outside lifecycle recovery", submissionID, err)
	}
	if ledger.Status != V2ContractStatus && (ledger.ContractFingerprint != contract.Fingerprint || !isFullCommitID(ledger.ContractCommit) || len(ledger.ApprovedScenarioIDs) == 0 || ledger.Worktree == nil || validateV2WorktreeIdentity(*ledger.Worktree, ledger.ContractCommit) != nil) {
		return V2ResumeResult{}, fmt.Errorf("cannot establish durable v2 lifecycle state for %q: TDD-or-later durable facts are incomplete or conflicting; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID)
	}
	return V2ResumeResult{
		Ledger:             ledger,
		Contract:           contract,
		AncoraPointerState: assessV2AncoraPointer(ledger, contract, pointer),
	}, nil
}

func PersistV2Transition(repoRoot string, request V2TransitionRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer {
		return V2SubmissionLedger{}, errors.New("v2 transition is unauthorized: expected orchestrator authorizer; return bounded evidence to the orchestrator")
	}
	if len(request.AuthorizedScope) == 0 || len(request.EvidenceRefs) == 0 {
		return V2SubmissionLedger{}, errors.New("v2 transition is incomplete: authorized scope and evidence references are required")
	}

	unlock, err := lockV2Ledger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	defer unlock()

	ledger, err := LoadV2SubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	if ledger.LedgerVersion != request.LedgerVersion {
		return V2SubmissionLedger{}, fmt.Errorf("v2 transition rejected: stale ledger version (expected %d, observed %d); reload durable state and resubmit bounded evidence", request.LedgerVersion, ledger.LedgerVersion)
	}
	if ledger.Status != request.ExpectedStatus || !isLegalV2Transition(ledger.Status, request.TargetStatus) {
		return V2SubmissionLedger{}, fmt.Errorf("v2 transition rejected: expected status %q, observed %q, target %q; reload durable state and request a legal orchestrator authorization", request.ExpectedStatus, ledger.Status, request.TargetStatus)
	}
	if ledger.Status == V2TDDStatus && request.TargetStatus == V2ReviewStatus && !allV2ScenariosAccepted(ledger) {
		return V2SubmissionLedger{}, errors.New("v2 transition rejected: every approved scenario requires accepted Red-Green-Refactor evidence before Review")
	}

	ledger.Transitions = append(ledger.Transitions, V2Transition{
		FromStatus:      ledger.Status,
		ToStatus:        request.TargetStatus,
		LedgerVersion:   ledger.LedgerVersion,
		Authorizer:      request.Authorizer,
		AuthorizedScope: append([]string(nil), request.AuthorizedScope...),
		EvidenceRefs:    append([]string(nil), request.EvidenceRefs...),
	})
	ledger.Status = request.TargetStatus
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

func v2LedgerPath(repoRoot, submissionID string) string {
	return filepath.Join(repoRoot, ".rotta", "v2", "submissions", submissionID+".yaml")
}

func v2ContractPath(repoRoot, submissionID string) string {
	return filepath.Join(repoRoot, ".rotta", "v2", "contracts", submissionID+".yaml")
}

func rejectExistingV2Identity(repoRoot, submissionID string) error {
	for _, path := range []string{
		v2LedgerPath(repoRoot, submissionID),
		v2ContractPath(repoRoot, submissionID),
	} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("v2 submission identity %q is not fresh; choose a different identity or resolve the existing durable artifact outside lifecycle initialization", submissionID)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check v2 submission identity freshness: %w", err)
		}
	}
	return nil
}

func lockV2Ledger(repoRoot, submissionID string) (func(), error) {
	path := v2LedgerPath(repoRoot, submissionID) + ".lock"
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("v2 transition is already in progress; retry after the current persistence operation completes")
		}
		return nil, fmt.Errorf("lock v2 submission ledger: %w", err)
	}
	_ = file.Close()
	return func() { _ = os.Remove(path) }, nil
}

func writeV2LedgerAtomically(path string, ledger V2SubmissionLedger) error {
	contents, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize v2 submission ledger: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ledger-*")
	if err != nil {
		return fmt.Errorf("create v2 ledger temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set v2 ledger temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write v2 ledger temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close v2 ledger temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("atomically persist v2 transition: %w", err)
	}
	return nil
}

func validateV2SubmissionID(submissionID string) error {
	if !v2SubmissionIDPattern.MatchString(submissionID) {
		return fmt.Errorf("invalid v2 submission ID %q", submissionID)
	}
	return nil
}

func validateV2Ledger(ledger V2SubmissionLedger, submissionID string) error {
	if ledger.SubmissionID != submissionID || ledger.LedgerVersion == 0 || !isFullCommitID(ledger.BaseCommit) {
		return errors.New("v2 submission ledger is invalid and cannot establish lifecycle state")
	}
	return nil
}

func loadV2ContractArtifact(repoRoot, submissionID string) (V2ContractArtifact, error) {
	contents, err := os.ReadFile(v2ContractPath(repoRoot, submissionID))
	if err != nil {
		return V2ContractArtifact{}, fmt.Errorf("read v2 contract artifact: %w", err)
	}
	var artifact V2ContractArtifact
	if err := json.Unmarshal(contents, &artifact); err != nil {
		return V2ContractArtifact{}, fmt.Errorf("parse v2 contract artifact: %w", err)
	}
	if artifact.SubmissionID != submissionID || strings.TrimSpace(artifact.Fingerprint) == "" || !isRepositoryPath(artifact.SpecPath) || len(artifact.FeaturePaths) == 0 {
		return V2ContractArtifact{}, errors.New("v2 contract artifact is invalid")
	}
	for _, path := range artifact.FeaturePaths {
		if !isRepositoryPath(path) {
			return V2ContractArtifact{}, errors.New("v2 contract artifact contains an invalid feature path")
		}
	}
	return artifact, nil
}

func assessV2AncoraPointer(ledger V2SubmissionLedger, contract V2ContractArtifact, pointer *V2AncoraPointer) V2AncoraPointerState {
	if pointer == nil {
		return V2AncoraUnavailable
	}
	if pointer.SubmissionID != ledger.SubmissionID || pointer.LedgerVersion != ledger.LedgerVersion || pointer.Status != ledger.Status || (contract.Fingerprint != "" && pointer.ContractFingerprint != contract.Fingerprint) {
		return V2AncoraStale
	}
	return V2AncoraConsistent
}

func isRepositoryPath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	return path != "" && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func isFullCommitID(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') && !(character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}

func isLegalV2Transition(from, to string) bool {
	return (from == V2DraftStatus && to == V2ContractStatus) ||
		(from == V2ContractStatus && to == V2TDDStatus) ||
		(from == V2TDDStatus && to == V2ReviewStatus) ||
		(from == V2ReviewStatus && (to == V2TDDStatus || to == V2ArchiveStatus)) ||
		(from == V2ArchiveStatus && to == V2ArchivedStatus)
}

func isV2LifecycleStatus(status string) bool {
	return status == V2DraftStatus || status == V2ContractStatus || status == V2TDDStatus || status == V2ReviewStatus || status == V2ArchiveStatus || status == V2ArchivedStatus
}

func allV2ScenariosAccepted(ledger V2SubmissionLedger) bool {
	if len(ledger.ApprovedScenarioIDs) == 0 || len(ledger.ApprovedScenarioIDs) != len(ledger.AcceptedScenarioIDs) || len(ledger.AcceptedScenarioIDs) != len(ledger.TDDEvidence) {
		return false
	}
	accepted := make(map[string]bool, len(ledger.AcceptedScenarioIDs))
	for _, scenario := range ledger.AcceptedScenarioIDs {
		accepted[scenario] = true
	}
	for _, scenario := range ledger.ApprovedScenarioIDs {
		if !accepted[scenario] {
			return false
		}
	}
	return true
}
