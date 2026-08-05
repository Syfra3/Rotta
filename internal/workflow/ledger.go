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
	DraftStatus    = "Draft"
	ContractStatus = "Contract"
	TDDStatus      = "TDD"
	ReviewStatus   = "Review"
	ArchiveStatus  = "Archive"
	ArchivedStatus = "archived"

	orchestratorAuthorizer = "orchestrator"
)

var submissionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// NewSubmissionRequest is the only request that may create initial workflow state.
type NewSubmissionRequest struct {
	SubmissionID string
	Draft        string
	BaseCommit   string
}

// TransitionRequest carries the complete authorization context required for
// a lifecycle write. Workers may provide evidence, but cannot be authorizers.
type TransitionRequest struct {
	SubmissionID    string
	ExpectedStatus  string
	TargetStatus    string
	LedgerVersion   uint64
	Authorizer      string
	AuthorizedScope []string
	EvidenceRefs    []string
}

type Transition struct {
	FromStatus      string   `json:"from_status"`
	ToStatus        string   `json:"to_status"`
	LedgerVersion   uint64   `json:"ledger_version"`
	Authorizer      string   `json:"authorizer"`
	AuthorizedScope []string `json:"authorized_scope"`
	EvidenceRefs    []string `json:"evidence_refs"`
}

// SubmissionLedger is operational state only; it does not contain approved
// contract scope until a separately approved Contract-to-TDD transition.
type SubmissionLedger struct {
	SubmissionID             string                   `json:"submission_id"`
	Draft                    string                   `json:"draft"`
	Status                   string                   `json:"status"`
	LedgerVersion            uint64                   `json:"ledger_version"`
	BaseCommit               string                   `json:"base_commit"`
	ContractFingerprint      string                   `json:"contract_fingerprint,omitempty"`
	ContractCommit           string                   `json:"contract_commit,omitempty"`
	ApprovedScenarioIDs      []string                 `json:"approved_scenario_ids,omitempty"`
	AcceptedScenarioIDs      []string                 `json:"accepted_scenario_ids,omitempty"`
	TDDEvidence              []ScenarioRGREvidence    `json:"tdd_evidence,omitempty"`
	TargetedVelaEvidence     []TargetedVelaQuestion   `json:"targeted_vela_evidence,omitempty"`
	ImplementationCommit     string                   `json:"implementation_commit,omitempty"`
	ReviewedCommit           string                   `json:"reviewed_commit,omitempty"`
	QualityPolicyFingerprint string                   `json:"quality_policy_fingerprint,omitempty"`
	QualityEvidence          []QualityDimensionResult `json:"quality_evidence,omitempty"`
	Publication              *Publication             `json:"publication,omitempty"`
	Worktree                 *WorktreeIdentity        `json:"worktree,omitempty"`
	Transitions              []Transition             `json:"transitions,omitempty"`
}

// ContractArtifact identifies the approved behavioral contract without
// placing its content or approval authority in the operational ledger.
type ContractArtifact struct {
	SubmissionID string   `json:"submission_id"`
	Fingerprint  string   `json:"fingerprint"`
	SpecPath     string   `json:"spec_path"`
	FeaturePaths []string `json:"feature_paths"`
}

// AncoraPointer is advisory only. It is compared with durable state but is
// never used to choose, repair, or advance lifecycle state.
type AncoraPointer struct {
	SubmissionID        string
	LedgerVersion       uint64
	Status              string
	ContractFingerprint string
}

type AncoraPointerState string

const (
	AncoraUnavailable AncoraPointerState = "unavailable"
	AncoraConsistent  AncoraPointerState = "consistent"
	AncoraStale       AncoraPointerState = "stale_or_conflicting"
)

type ResumeResult struct {
	Ledger             SubmissionLedger
	Contract           ContractArtifact
	AncoraPointerState AncoraPointerState
}

func InitializeNewSubmission(repoRoot string, request NewSubmissionRequest) (SubmissionLedger, error) {
	if err := validateSubmissionID(request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}
	if strings.TrimSpace(request.Draft) == "" {
		return SubmissionLedger{}, errors.New("NEW submission requires a supplied draft")
	}
	if !isFullCommitID(request.BaseCommit) {
		return SubmissionLedger{}, errors.New("NEW submission requires a full immutable base commit identifier")
	}
	if err := rejectExistingIdentity(repoRoot, request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}

	ledger := SubmissionLedger{
		SubmissionID:  request.SubmissionID,
		Draft:         request.Draft,
		Status:        DraftStatus,
		LedgerVersion: 1,
		BaseCommit:    strings.ToLower(request.BaseCommit),
	}
	contents, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return SubmissionLedger{}, fmt.Errorf("serialize initial submission ledger: %w", err)
	}

	directory := filepath.Join(repoRoot, ".rotta", "workflow", "submissions")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return SubmissionLedger{}, fmt.Errorf("create workflow submission directory: %w", err)
	}
	path := ledgerPath(repoRoot, request.SubmissionID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return SubmissionLedger{}, fmt.Errorf("submission identity %q is not fresh; choose a different identity or resolve the existing durable artifact outside lifecycle initialization", request.SubmissionID)
		}
		return SubmissionLedger{}, fmt.Errorf("create initial submission ledger: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return SubmissionLedger{}, fmt.Errorf("write initial submission ledger: %w", err)
	}
	if err := file.Close(); err != nil {
		return SubmissionLedger{}, fmt.Errorf("close initial submission ledger: %w", err)
	}
	return ledger, nil
}

func LoadSubmissionLedger(repoRoot, submissionID string) (SubmissionLedger, error) {
	if err := validateSubmissionID(submissionID); err != nil {
		return SubmissionLedger{}, err
	}
	contents, err := os.ReadFile(ledgerPath(repoRoot, submissionID))
	if err != nil {
		return SubmissionLedger{}, fmt.Errorf("read submission ledger: %w", err)
	}
	var ledger SubmissionLedger
	if err := json.Unmarshal(contents, &ledger); err != nil {
		return SubmissionLedger{}, fmt.Errorf("parse submission ledger: %w", err)
	}
	if err := validateLedger(ledger, submissionID); err != nil {
		return SubmissionLedger{}, err
	}
	return ledger, nil
}

// RecordContractArtifact persists the contract identity produced in Contract.
// Approval and lifecycle state remain separate orchestrator-authorized actions.
func RecordContractArtifact(repoRoot string, artifact ContractArtifact) error {
	if err := validateSubmissionID(artifact.SubmissionID); err != nil {
		return err
	}
	if strings.TrimSpace(artifact.Fingerprint) == "" || !isRepositoryPath(artifact.SpecPath) || len(artifact.FeaturePaths) == 0 {
		return errors.New("contract artifact is incomplete")
	}
	for _, path := range artifact.FeaturePaths {
		if !isRepositoryPath(path) {
			return errors.New("contract artifact contains an invalid feature path")
		}
	}
	contents, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize contract artifact: %w", err)
	}
	directory := filepath.Join(repoRoot, ".rotta", "workflow", "contracts")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create workflow contract directory: %w", err)
	}
	file, err := os.OpenFile(contractPath(repoRoot, artifact.SubmissionID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("contract artifact already exists for %q; changed contract content requires a new explicit approval", artifact.SubmissionID)
		}
		return fmt.Errorf("create contract artifact: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write contract artifact: %w", err)
	}
	return nil
}

// ResumeSubmission validates only phase-appropriate durable records.
// A missing or conflicting artifact fails closed and starts no lifecycle work.
func ResumeSubmission(repoRoot, submissionID string, pointer *AncoraPointer) (ResumeResult, error) {
	ledger, err := LoadSubmissionLedger(repoRoot, submissionID)
	if err != nil {
		return ResumeResult{}, fmt.Errorf("cannot establish durable lifecycle state for %q: %w; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID, err)
	}
	if ledger.Status == DraftStatus {
		if strings.TrimSpace(ledger.Draft) == "" {
			return ResumeResult{}, fmt.Errorf("cannot establish durable lifecycle state for %q: initial Draft record is incomplete; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID)
		}
		return ResumeResult{Ledger: ledger, AncoraPointerState: assessAncoraPointer(ledger, ContractArtifact{}, pointer)}, nil
	}
	if !isLifecycleStatus(ledger.Status) {
		return ResumeResult{}, fmt.Errorf("cannot establish durable lifecycle state for %q: invalid status %q; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID, ledger.Status)
	}
	contract, err := loadContractArtifact(repoRoot, submissionID)
	if err != nil {
		return ResumeResult{}, fmt.Errorf("cannot establish durable lifecycle state for %q: %w; safe next action: restore or resolve matching durable contract artifacts outside lifecycle recovery", submissionID, err)
	}
	if ledger.Status != ContractStatus && (ledger.ContractFingerprint != contract.Fingerprint || !isFullCommitID(ledger.ContractCommit) || len(ledger.ApprovedScenarioIDs) == 0 || ledger.Worktree == nil || validateWorktreeIdentity(*ledger.Worktree, ledger.ContractCommit) != nil) {
		return ResumeResult{}, fmt.Errorf("cannot establish durable lifecycle state for %q: TDD-or-later durable facts are incomplete or conflicting; safe next action: resolve durable artifacts outside lifecycle recovery", submissionID)
	}
	return ResumeResult{
		Ledger:             ledger,
		Contract:           contract,
		AncoraPointerState: assessAncoraPointer(ledger, contract, pointer),
	}, nil
}

func PersistTransition(repoRoot string, request TransitionRequest) (SubmissionLedger, error) {
	if err := validateSubmissionID(request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}
	if request.Authorizer != orchestratorAuthorizer {
		return SubmissionLedger{}, errors.New("transition is unauthorized: expected orchestrator authorizer; return bounded evidence to the orchestrator")
	}
	if len(request.AuthorizedScope) == 0 || len(request.EvidenceRefs) == 0 {
		return SubmissionLedger{}, errors.New("transition is incomplete: authorized scope and evidence references are required")
	}

	unlock, err := lockLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	defer unlock()

	ledger, err := LoadSubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	if ledger.LedgerVersion != request.LedgerVersion {
		return SubmissionLedger{}, fmt.Errorf("transition rejected: stale ledger version (expected %d, observed %d); reload durable state and resubmit bounded evidence", request.LedgerVersion, ledger.LedgerVersion)
	}
	if ledger.Status != request.ExpectedStatus || !isLegalTransition(ledger.Status, request.TargetStatus) {
		return SubmissionLedger{}, fmt.Errorf("transition rejected: expected status %q, observed %q, target %q; reload durable state and request a legal orchestrator authorization", request.ExpectedStatus, ledger.Status, request.TargetStatus)
	}
	if ledger.Status == TDDStatus && request.TargetStatus == ReviewStatus && !allScenariosAccepted(ledger) {
		return SubmissionLedger{}, errors.New("transition rejected: every approved scenario requires accepted Red-Green-Refactor evidence before Review")
	}

	ledger.Transitions = append(ledger.Transitions, Transition{
		FromStatus:      ledger.Status,
		ToStatus:        request.TargetStatus,
		LedgerVersion:   ledger.LedgerVersion,
		Authorizer:      request.Authorizer,
		AuthorizedScope: append([]string(nil), request.AuthorizedScope...),
		EvidenceRefs:    append([]string(nil), request.EvidenceRefs...),
	})
	ledger.Status = request.TargetStatus
	ledger.LedgerVersion++
	if err := writeLedgerAtomically(ledgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return SubmissionLedger{}, err
	}
	return ledger, nil
}

func ledgerPath(repoRoot, submissionID string) string {
	return filepath.Join(repoRoot, ".rotta", "workflow", "submissions", submissionID+".yaml")
}

func contractPath(repoRoot, submissionID string) string {
	return filepath.Join(repoRoot, ".rotta", "workflow", "contracts", submissionID+".yaml")
}

func rejectExistingIdentity(repoRoot, submissionID string) error {
	for _, path := range []string{
		ledgerPath(repoRoot, submissionID),
		contractPath(repoRoot, submissionID),
	} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("submission identity %q is not fresh; choose a different identity or resolve the existing durable artifact outside lifecycle initialization", submissionID)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check submission identity freshness: %w", err)
		}
	}
	return nil
}

func lockLedger(repoRoot, submissionID string) (func(), error) {
	path := ledgerPath(repoRoot, submissionID) + ".lock"
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("transition is already in progress; retry after the current persistence operation completes")
		}
		return nil, fmt.Errorf("lock submission ledger: %w", err)
	}
	_ = file.Close()
	return func() { _ = os.Remove(path) }, nil
}

func writeLedgerAtomically(path string, ledger SubmissionLedger) error {
	contents, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize submission ledger: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ledger-*")
	if err != nil {
		return fmt.Errorf("create ledger temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set ledger temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write ledger temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close ledger temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("atomically persist transition: %w", err)
	}
	return nil
}

func validateSubmissionID(submissionID string) error {
	if !submissionIDPattern.MatchString(submissionID) {
		return fmt.Errorf("invalid submission ID %q", submissionID)
	}
	return nil
}

func validateLedger(ledger SubmissionLedger, submissionID string) error {
	if ledger.SubmissionID != submissionID || ledger.LedgerVersion == 0 || !isFullCommitID(ledger.BaseCommit) {
		return errors.New("submission ledger is invalid and cannot establish lifecycle state")
	}
	return nil
}

func loadContractArtifact(repoRoot, submissionID string) (ContractArtifact, error) {
	contents, err := os.ReadFile(contractPath(repoRoot, submissionID))
	if err != nil {
		return ContractArtifact{}, fmt.Errorf("read contract artifact: %w", err)
	}
	var artifact ContractArtifact
	if err := json.Unmarshal(contents, &artifact); err != nil {
		return ContractArtifact{}, fmt.Errorf("parse contract artifact: %w", err)
	}
	if artifact.SubmissionID != submissionID || strings.TrimSpace(artifact.Fingerprint) == "" || !isRepositoryPath(artifact.SpecPath) || len(artifact.FeaturePaths) == 0 {
		return ContractArtifact{}, errors.New("contract artifact is invalid")
	}
	for _, path := range artifact.FeaturePaths {
		if !isRepositoryPath(path) {
			return ContractArtifact{}, errors.New("contract artifact contains an invalid feature path")
		}
	}
	return artifact, nil
}

func assessAncoraPointer(ledger SubmissionLedger, contract ContractArtifact, pointer *AncoraPointer) AncoraPointerState {
	if pointer == nil {
		return AncoraUnavailable
	}
	if pointer.SubmissionID != ledger.SubmissionID || pointer.LedgerVersion != ledger.LedgerVersion || pointer.Status != ledger.Status || (contract.Fingerprint != "" && pointer.ContractFingerprint != contract.Fingerprint) {
		return AncoraStale
	}
	return AncoraConsistent
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

func isLegalTransition(from, to string) bool {
	return (from == DraftStatus && to == ContractStatus) ||
		(from == ContractStatus && to == TDDStatus) ||
		(from == TDDStatus && to == ReviewStatus) ||
		(from == ReviewStatus && (to == TDDStatus || to == ArchiveStatus)) ||
		(from == ArchiveStatus && to == ArchivedStatus)
}

func isLifecycleStatus(status string) bool {
	return status == DraftStatus || status == ContractStatus || status == TDDStatus || status == ReviewStatus || status == ArchiveStatus || status == ArchivedStatus
}

func allScenariosAccepted(ledger SubmissionLedger) bool {
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
