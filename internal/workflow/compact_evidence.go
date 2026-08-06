package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Syfra3/Rotta/internal/rtkexec"
)

const (
	maxCompactCapsuleTextBytes = 512
	maxCompactCapsulePaths     = 32
	maxCompactCapsuleScope     = 8
	maxCompactEvidenceRefs     = 8
	maxCompactDiagnostics      = 8
	maxCompactCapsuleBytes     = 8 * 1024
	maxObservedTokenCount      = uint64(1<<53 - 1)
	maxRTKStateBytes           = 4 * 1024
	maxRTKFilterOutputBytes    = 8 * 1024
)

const (
	compactEvidenceRisk       = "durable_evidence_authoritative"
	compactDiagnosticsOmitted = "diagnostics omitted; consult durable evidence"
)

var compactCommands = map[string]bool{
	"handoff-validate":    true,
	"preflight":           true,
	"publication-plan":    true,
	"scoped-verify":       true,
	"tool-output-pruning": true,
}

// These are the complete, authored workflow messages that a compact capsule
// may carry. Dynamic errors belong exclusively in the durable evidence record.
var compactRemediations = map[string]bool{
	"continue from the validated handoff evidence":                                       true,
	"continue only with the validated local mirror; restore Ancora separately":           true,
	"continue to independent review; publication remains planning only":                  true,
	"continue with the scoped deterministic command before review":                       true,
	"correct the in-scope failure and rerun scoped verification":                         true,
	"obtain separate exact authority before any commit, push, or pull-request operation": true,
	"provide matching current command evidence or omit the evidence reference":           true,
	"repair the canonical handoff mirror and workspace state before continuing":          true,
	"restore a clean recorded worktree before continuing":                                true,
	"restore a clean recorded worktree before publication planning":                      true,
	"restore a clean recorded worktree before scoped verification":                       true,
}

var compactDiagnostics = map[string]bool{
	"read-only plan: separate approval is required for every publication operation": true,
	"recovery source: ancora+mirror":                                                true,
	"recovery source: mirror":                                                       true,
	"recovery source: mirror+ancora":                                                true,
	"scoped Go verification failed; inspect durable evidence":                       true,
	compactDiagnosticsOmitted:                                                       true,
}

type TokenSizeStatus string

const (
	TokenSizeObserved      TokenSizeStatus = "observed"
	TokenSizeNotObservable TokenSizeStatus = "not_observable"
)

// ExposedTokenSize never estimates a token count: it is observed only when
// the host exposed it, and otherwise explicitly not_observable.
type ExposedTokenSize struct {
	Status TokenSizeStatus `json:"status"`
	Tokens *uint64         `json:"tokens,omitempty"`
}

// CompactEvidenceResult is an opaque, agent-facing compact capsule. Complete
// command output remains in the referenced durable evidence, not in this
// capsule. Its unexported sealing method prevents external implementations;
// NewCompactEvidenceResult is its only public construction path.
type CompactEvidenceResult interface {
	json.Marshaler
	compactEvidenceResult()
}

// compactEvidenceResult is deliberately private so external callers cannot
// use a struct literal or field assignment to bypass semantic validation.
type compactEvidenceResult struct {
	format             string
	canonicalOutcome   WorkflowCommandResult
	evidence           DurableEvidenceReference
	changedPaths       []string
	scope              []string
	risk               string
	remediation        string
	compactionEvidence []DurableEvidenceReference
	promptTokens       ExposedTokenSize
	capsuleTokens      ExposedTokenSize
}

type compactEvidenceJSON struct {
	Format             string                     `json:"format"`
	CanonicalOutcome   WorkflowCommandResult      `json:"canonical_outcome"`
	Evidence           DurableEvidenceReference   `json:"evidence"`
	ChangedPaths       []string                   `json:"changed_paths"`
	Scope              []string                   `json:"scope"`
	Risk               string                     `json:"risk"`
	Remediation        string                     `json:"remediation"`
	CompactionEvidence []DurableEvidenceReference `json:"compaction_evidence"`
	PromptTokens       ExposedTokenSize           `json:"prompt_tokens"`
	CapsuleTokens      ExposedTokenSize           `json:"capsule_tokens"`
}

type CompactEvidenceInput struct {
	CanonicalOutcome   WorkflowCommandResult
	Evidence           DurableEvidenceReference
	ChangedPaths       []string
	Scope              []string
	Risk               string
	Remediation        string
	CompactionEvidence []DurableEvidenceReference
	PromptTokens       *uint64
	CapsuleTokens      *uint64
}

// NewCompactEvidenceResult rejects, rather than silently truncating, values
// that cannot safely be a compact capsule. Each field has a semantic schema
// or a finite canonical allowlist; this API has no raw-output field by design.
func NewCompactEvidenceResult(input CompactEvidenceInput) (CompactEvidenceResult, error) {
	input.CanonicalOutcome = cloneWorkflowCommandResult(input.CanonicalOutcome)
	input.Evidence = cloneDurableEvidenceReference(input.Evidence)
	input.ChangedPaths = append([]string(nil), input.ChangedPaths...)
	input.Scope = append([]string(nil), input.Scope...)
	input.CompactionEvidence = cloneDurableEvidenceReferences(input.CompactionEvidence)
	if err := validateCompactEvidenceInput(input); err != nil {
		return nil, err
	}
	prompt, _ := exposedTokenSize(input.PromptTokens)
	capsule, _ := exposedTokenSize(input.CapsuleTokens)
	result := compactEvidenceResult{
		format:             "rotta.compact-evidence/v1",
		canonicalOutcome:   input.CanonicalOutcome,
		evidence:           input.Evidence,
		changedPaths:       input.ChangedPaths,
		scope:              input.Scope,
		risk:               input.Risk,
		remediation:        input.Remediation,
		compactionEvidence: input.CompactionEvidence,
		promptTokens:       prompt,
		capsuleTokens:      capsule,
	}
	if _, err := result.MarshalJSON(); err != nil {
		return nil, err
	}
	return result, nil
}

func validateCompactEvidenceInput(input CompactEvidenceInput) error {
	if err := validateCompactOutcome(input.CanonicalOutcome); err != nil {
		return err
	}
	if input.CanonicalOutcome.EvidencePath == "" || input.CanonicalOutcome.EvidenceHash == "" {
		return fmt.Errorf("compact evidence requires a canonical outcome evidence reference")
	}
	if input.Evidence.Path != input.CanonicalOutcome.EvidencePath || input.Evidence.Hash != input.CanonicalOutcome.EvidenceHash {
		return fmt.Errorf("compact evidence authority must match the canonical outcome evidence reference")
	}
	if err := validateCompactEvidenceReference(input.Evidence); err != nil {
		return err
	}
	if err := validateCompactPaths("changed paths", input.ChangedPaths, maxCompactCapsulePaths); err != nil {
		return err
	}
	if err := validateCompactPaths("scope", input.Scope, maxCompactCapsuleScope); err != nil {
		return err
	}
	if input.Risk != compactEvidenceRisk {
		return errors.New("compact capsule risk is not a canonical value")
	}
	if !compactRemediations[input.Remediation] {
		return errors.New("compact capsule remediation is not an allowlisted workflow message")
	}
	if err := validateCompactText("remediation", input.Remediation); err != nil {
		return err
	}
	if len(input.CompactionEvidence) > maxCompactEvidenceRefs {
		return fmt.Errorf("compaction evidence exceeds %d references", maxCompactEvidenceRefs)
	}
	for _, reference := range input.CompactionEvidence {
		if err := validateCompactEvidenceReference(reference); err != nil {
			return err
		}
	}
	if _, err := exposedTokenSize(input.PromptTokens); err != nil {
		return err
	}
	if _, err := exposedTokenSize(input.CapsuleTokens); err != nil {
		return err
	}
	return nil
}

func (result compactEvidenceResult) compactEvidenceResult() {}

// MarshalJSON emits only the approved compact representation. The private
// concrete representation means callers cannot create, decode, or mutate it.
func (result compactEvidenceResult) MarshalJSON() ([]byte, error) {
	if result.format != "rotta.compact-evidence/v1" {
		return nil, errors.New("serialize compact capsule: invalid format")
	}
	input := CompactEvidenceInput{
		CanonicalOutcome:   cloneWorkflowCommandResult(result.canonicalOutcome),
		Evidence:           cloneDurableEvidenceReference(result.evidence),
		ChangedPaths:       append([]string(nil), result.changedPaths...),
		Scope:              append([]string(nil), result.scope...),
		Risk:               result.risk,
		Remediation:        result.remediation,
		CompactionEvidence: cloneDurableEvidenceReferences(result.compactionEvidence),
	}
	if result.promptTokens.Status == TokenSizeObserved {
		input.PromptTokens = result.promptTokens.Tokens
	}
	if result.capsuleTokens.Status == TokenSizeObserved {
		input.CapsuleTokens = result.capsuleTokens.Tokens
	}
	if err := validateCompactEvidenceInput(input); err != nil || !validExposedTokenSize(result.promptTokens) || !validExposedTokenSize(result.capsuleTokens) {
		if err != nil {
			return nil, fmt.Errorf("serialize compact capsule: %w", err)
		}
		return nil, errors.New("serialize compact capsule: invalid observed token size")
	}
	serialized, err := json.Marshal(compactEvidenceJSON{
		Format:             result.format,
		CanonicalOutcome:   input.CanonicalOutcome,
		Evidence:           input.Evidence,
		ChangedPaths:       input.ChangedPaths,
		Scope:              input.Scope,
		Risk:               input.Risk,
		Remediation:        input.Remediation,
		CompactionEvidence: input.CompactionEvidence,
		PromptTokens:       result.promptTokens,
		CapsuleTokens:      result.capsuleTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("serialize compact capsule: %w", err)
	}
	if len(serialized) > maxCompactCapsuleBytes {
		return nil, fmt.Errorf("compact capsule exceeds canonical %d-byte aggregate limit", maxCompactCapsuleBytes)
	}
	return serialized, nil
}

func validExposedTokenSize(value ExposedTokenSize) bool {
	if value.Status == TokenSizeNotObservable {
		return value.Tokens == nil
	}
	return value.Status == TokenSizeObserved && value.Tokens != nil && *value.Tokens <= maxObservedTokenCount
}

func cloneWorkflowCommandResult(result WorkflowCommandResult) WorkflowCommandResult {
	result.CanonicalInputs.Scope = append([]string(nil), result.CanonicalInputs.Scope...)
	result.Diagnostics = append([]string(nil), result.Diagnostics...)
	return result
}

func cloneDurableEvidenceReference(reference DurableEvidenceReference) DurableEvidenceReference {
	return reference
}

func cloneDurableEvidenceReferences(references []DurableEvidenceReference) []DurableEvidenceReference {
	return append([]DurableEvidenceReference(nil), references...)
}

func validateCompactOutcome(outcome WorkflowCommandResult) error {
	if outcome.compactCapsule != nil {
		return errors.New("canonical outcome must not embed another capsule")
	}
	if outcome.Format != WorkflowCommandFormat || !compactCommands[outcome.Command] || (outcome.Status != string(OutcomePassed) && outcome.Status != string(OutcomeFailed)) {
		return errors.New("canonical outcome has an unknown format, command, or status")
	}
	if outcome.CanonicalInputs.Worktree != "." || !workflowFeatureID.MatchString(outcome.CanonicalInputs.Feature) || !canonicalCompactRepositoryPath(outcome.CanonicalInputs.ContractPath) || !canonicalCompactCommit(outcome.CanonicalInputs.Baseline) || (outcome.CanonicalInputs.HandoffID != "" && !canonicalWorkflowHandoffTaskID(outcome.CanonicalInputs.HandoffID)) {
		return errors.New("canonical outcome has invalid structured inputs")
	}
	if err := validateCompactPaths("canonical outcome scope", outcome.CanonicalInputs.Scope, maxCompactCapsuleScope); err != nil {
		return err
	}
	if !canonicalCompactEvidencePath(outcome.EvidencePath) || !canonicalCompactHash(outcome.EvidenceHash) {
		return errors.New("canonical outcome has invalid durable evidence reference")
	}
	if !compactRemediations[outcome.Remediation] {
		return errors.New("canonical outcome remediation is not an allowlisted workflow message")
	}
	if err := validateCompactText("canonical outcome remediation", outcome.Remediation); err != nil {
		return err
	}
	if len(outcome.Diagnostics) > maxCompactDiagnostics {
		return fmt.Errorf("canonical outcome diagnostics exceeds %d entries", maxCompactDiagnostics)
	}
	for _, diagnostic := range outcome.Diagnostics {
		if !compactDiagnostics[diagnostic] {
			return errors.New("canonical outcome diagnostic is not an allowlisted workflow message")
		}
	}
	return nil
}

func compactCapsuleDiagnostics(diagnostics []string) []string {
	result := make([]string, 0, len(diagnostics)+1)
	omitted := false
	for _, diagnostic := range diagnostics {
		if compactDiagnostics[diagnostic] {
			result = append(result, diagnostic)
		} else {
			omitted = true
		}
	}
	if omitted {
		result = append(result, compactDiagnosticsOmitted)
	}
	return result
}

func validateCompactEvidenceReference(reference DurableEvidenceReference) error {
	if !compactCommands[reference.Check] || !canonicalCompactEvidencePath(reference.Path) || !canonicalCompactHash(reference.Hash) || (reference.Status != OutcomePassed && reference.Status != OutcomeFailed) {
		return errors.New("compact evidence reference is not a canonical structured reference")
	}
	return nil
}

func validateCompactPaths(name string, values []string, limit int) error {
	if len(values) > limit {
		return fmt.Errorf("%s exceeds %d entries", name, limit)
	}
	for _, value := range values {
		if !canonicalCompactRepositoryPath(value) {
			return fmt.Errorf("%s contains a non-canonical repository path", name)
		}
	}
	return nil
}

func canonicalCompactEvidencePath(path string) bool {
	return canonicalCompactRepositoryPath(path) && strings.HasPrefix(path, ".rotta/current/evidence/")
}

func canonicalCompactRepositoryPath(path string) bool {
	if !canonicalWorkflowPath(path) {
		return false
	}
	for _, value := range path {
		if (value < 'a' || value > 'z') && (value < 'A' || value > 'Z') && (value < '0' || value > '9') && value != '/' && value != '.' && value != '_' && value != '-' {
			return false
		}
	}
	return true
}

func canonicalCompactHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func canonicalCompactCommit(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 20 && strings.ToLower(value) == value
}

func validateCompactStrings(name string, values []string, limit int) error {
	if len(values) > limit {
		return fmt.Errorf("%s exceeds %d entries", name, limit)
	}
	for _, value := range values {
		if err := validateCompactText(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateCompactText(name, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", name)
	}
	if len(value) > maxCompactCapsuleTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxCompactCapsuleTextBytes)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must be single-line compact metadata, not raw output", name)
	}
	for _, byteValue := range []byte(value) {
		if byteValue < 0x20 || byteValue == 0x7f {
			return fmt.Errorf("%s contains control data and is not compact metadata", name)
		}
	}
	return nil
}

func exposedTokenSize(tokens *uint64) (ExposedTokenSize, error) {
	if tokens == nil {
		return ExposedTokenSize{Status: TokenSizeNotObservable}, nil
	}
	if *tokens > maxObservedTokenCount {
		return ExposedTokenSize{}, fmt.Errorf("observed token count exceeds %d", maxObservedTokenCount)
	}
	value := *tokens
	return ExposedTokenSize{Status: TokenSizeObserved, Tokens: &value}, nil
}

type RTKInstallStatus string

const (
	RTKStatusSuccess RTKInstallStatus = "success"
	RTKStatusSkipped RTKInstallStatus = "skipped"
	RTKStatusFailure RTKInstallStatus = "failure"
)

// RTKExecutableRecord comes from host-local installer transaction evidence.
// Runtime use must revalidate all three executable facts.
type RTKExecutableRecord struct {
	Status         RTKInstallStatus `json:"status"`
	ExecutablePath string           `json:"executable_path,omitempty"`
	Version        string           `json:"version,omitempty"`
	ExecutableHash string           `json:"executable_hash,omitempty"`
	FailureReason  string           `json:"failure_reason,omitempty"`
}

// RTKExecutable is an already-open executable object. Version, fingerprint,
// and filtering all operate on that one object rather than re-resolving its
// pathname after validation.
type RTKExecutable interface {
	Path() string
	Version() (string, error)
	Fingerprint() (string, error)
	Run(args []string, stdin string, maxOutput int) (string, error)
	Close() error
}

type RTKResolver interface {
	Resolve(recordedPath string) (RTKExecutable, error)
}

type RTKFilter interface {
	Filter(executable RTKExecutable, output string) (string, error)
}

// SystemRTKResolver accepts only exact Homebrew Cellar paths recorded by the
// installer. It delegates to the Linux descriptor-and-sealed-snapshot policy.
type SystemRTKResolver struct{}

func (SystemRTKResolver) Resolve(recordedPath string) (RTKExecutable, error) {
	return rtkexec.OpenRTK(recordedPath)
}

// SystemRTKFilter uses RTK's documented generic error view over a fixed
// /bin/cat input stream. It never re-runs the lifecycle command: the input is
// already-bounded presentation text, while the complete original result stays
// in durable evidence.
type SystemRTKFilter struct{}

func (SystemRTKFilter) Filter(executable RTKExecutable, output string) (string, error) {
	return executable.Run([]string{"err", "/bin/cat"}, output, maxRTKFilterOutputBytes)
}

// RTKOutputPresentation is a non-authoritative view. Its evidence reference
// remains the full durable command result whether RTK ran or not.
type RTKOutputPresentation struct {
	Output       string
	UsedRTK      bool
	EvidencePath string
	EvidenceHash string
}

// LoadRTKExecutableRecord reads exactly one bounded host-local JSON state
// record. Malformed, oversized, and missing records are optional-runtime
// failures; callers preserve their unfiltered output.
func LoadRTKExecutableRecord(path string) (RTKExecutableRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return RTKExecutableRecord{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRTKStateBytes+1))
	if err != nil {
		return RTKExecutableRecord{}, err
	}
	if len(data) > maxRTKStateBytes {
		return RTKExecutableRecord{}, fmt.Errorf("RTK state exceeds %d bytes", maxRTKStateBytes)
	}
	var record RTKExecutableRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return RTKExecutableRecord{}, fmt.Errorf("decode RTK state: %w", err)
	}
	return record, nil
}

// DefaultRTKStatePath is the host-local state written by the installer, not a
// workspace artifact. Its absence is an intentional unfiltered fallback.
func DefaultRTKStatePath() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "rotta", "rtk.json"), nil
}

// RTKPresentation is optional runtime configuration for a command's
// chat-facing view. Nil configuration means RTK is not consulted at all.
type RTKPresentation struct {
	Disabled  bool
	StatePath string
	Loader    func(string) (RTKExecutableRecord, error)
	Resolver  RTKResolver
	Filter    RTKFilter
}

func DefaultRTKPresentation() *RTKPresentation {
	statePath, err := DefaultRTKStatePath()
	if err != nil {
		// State discovery is optional presentation infrastructure. A blank path
		// makes the loader fail closed and leaves the bounded summary untouched.
		return &RTKPresentation{Resolver: SystemRTKResolver{}, Filter: SystemRTKFilter{}}
	}
	return &RTKPresentation{StatePath: statePath, Resolver: SystemRTKResolver{}, Filter: SystemRTKFilter{}}
}

// DisabledRTKPresentation is the explicit test-safe presenter. It never reads
// host state, resolves a pathname, or starts an RTK process.
func DisabledRTKPresentation() *RTKPresentation { return &RTKPresentation{Disabled: true} }

func (runtime RTKPresentation) Present(report LifecycleCommandReport, underlyingOutput string) RTKOutputPresentation {
	if runtime.Disabled {
		return RTKOutputPresentation{Output: underlyingOutput, EvidencePath: report.EvidencePath, EvidenceHash: report.EvidenceHash}
	}
	return PresentRecordedRTKOutput(report, underlyingOutput, runtime.StatePath, runtime.Loader, runtime.Resolver, runtime.Filter)
}

// PresentRecordedRTKOutput is the production runtime seam: it loads the
// installer-written state, then invokes RTK only after validation succeeds.
// Fakes can inject its loader, resolver, and filter through RTKPresentation.
func PresentRecordedRTKOutput(report LifecycleCommandReport, underlyingOutput, statePath string, loader func(string) (RTKExecutableRecord, error), resolver RTKResolver, filter RTKFilter) RTKOutputPresentation {
	presentation := RTKOutputPresentation{Output: underlyingOutput, EvidencePath: report.EvidencePath, EvidenceHash: report.EvidenceHash}
	if loader == nil {
		loader = LoadRTKExecutableRecord
	}
	record, err := loader(statePath)
	if err != nil {
		return presentation
	}
	return PresentRTKOutput(report, underlyingOutput, record, resolver, filter)
}

// PresentRTKOutput does not write or mutate durable command evidence. A stale,
// missing, replaced, or failing executable returns the exact unfiltered input.
func PresentRTKOutput(report LifecycleCommandReport, underlyingOutput string, record RTKExecutableRecord, resolver RTKResolver, filter RTKFilter) RTKOutputPresentation {
	presentation := RTKOutputPresentation{Output: underlyingOutput, EvidencePath: report.EvidencePath, EvidenceHash: report.EvidenceHash}
	if report.EvidencePath == "" || report.EvidenceHash == "" || resolver == nil || filter == nil {
		return presentation
	}
	executable, err := usableRTKRecord(record, resolver)
	if err != nil {
		return presentation
	}
	defer executable.Close()
	filtered, err := filter.Filter(executable, underlyingOutput)
	if err != nil {
		return presentation
	}
	presentation.Output, presentation.UsedRTK = filtered, true
	return presentation
}

func usableRTKRecord(record RTKExecutableRecord, resolver RTKResolver) (RTKExecutable, error) {
	if record.Status != RTKStatusSuccess || record.ExecutablePath == "" || record.Version == "" || record.ExecutableHash == "" {
		return nil, errors.New("RTK state is not a verified success")
	}
	executable, err := resolver.Resolve(record.ExecutablePath)
	if err != nil {
		return nil, err
	}
	version, err := executable.Version()
	if err != nil || version != record.Version {
		_ = executable.Close()
		return nil, errors.New("RTK version no longer matches recorded state")
	}
	hash, err := executable.Fingerprint()
	if err != nil || hash != record.ExecutableHash {
		_ = executable.Close()
		return nil, errors.New("RTK fingerprint no longer matches recorded state")
	}
	return executable, nil
}
