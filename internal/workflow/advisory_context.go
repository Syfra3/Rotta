package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
)

const (
	maxAdvisoryEntries  = 8
	maxVelaExploreCalls = 2
	maxVelaReviewCalls  = 1
)

// AncoraAdvisory is deliberately separate from workflow authority. Its data is
// useful context only; workspace, Git, and Strict records decide continuation.
type AncoraAdvisory interface {
	RecoverRelevant() (AncoraAdvisoryRecovery, error)
	SaveMaterialSummary(AncoraMaterialSummary) error
}

type AncoraAdvisoryRecovery struct {
	Decisions   []string
	Discoveries []string
	Summaries   []string
	References  []AncoraAdvisoryReference
}

// AncoraAdvisoryReference ties advisory context to a current, repository-local
// source. It is evidence only: a missing or changed reference degrades to the
// workspace/Git fallback rather than supplying lifecycle authority.
type AncoraAdvisoryReference struct {
	Path   string
	SHA256 string
}

type AncoraMaterialKind string

const (
	AncoraMaterialDecision  AncoraMaterialKind = "decision"
	AncoraMaterialDiscovery AncoraMaterialKind = "discovery"
	AncoraMaterialFix       AncoraMaterialKind = "fix"
	AncoraMaterialOutcome   AncoraMaterialKind = "outcome"
)

// AncoraMaterialSummary intentionally has no transcript, approval, or status
// fields, so an advisory record cannot become lifecycle authority.
type AncoraMaterialSummary struct {
	Kind    AncoraMaterialKind
	Summary string
}

type AdvisoryRecovery struct {
	Context     AncoraAdvisoryRecovery
	Source      string
	Degraded    bool
	EvidenceGap string
}

type AdvisorySummaryResult struct {
	Stored      bool
	EvidenceGap string
}

type VelaQuestionKind string

const (
	VelaDependency        VelaQuestionKind = "dependency"
	VelaImpact            VelaQuestionKind = "impact"
	VelaOwnership         VelaQuestionKind = "ownership"
	VelaArchitecturalFlow VelaQuestionKind = "architectural_flow"
	VelaUnfamiliarModule  VelaQuestionKind = "unfamiliar_module"
)

type VelaRole string

const (
	VelaExploreRole VelaRole = "exploration"
	VelaReviewRole  VelaRole = "review"
)

// VelaQuestion is a named, bounded structural question. Subject identifies the
// exact module, symbol, or path being queried; Question never carries raw graph
// output or a general exploration request.
type VelaQuestion struct {
	Role     VelaRole
	Kind     VelaQuestionKind
	Subject  string
	Question string
}

// VelaEvidence is already distilled for a capsule. The source interface has no
// raw-output channel by design.
type VelaEvidence struct {
	// Subject must echo the named question subject. This makes stale or
	// conflicting graph packets rejectable before their file hints are used.
	Subject    string
	Symbols    []string
	Files      []string
	Confidence string
	Gaps       []string
	SafeAction string
}

type VelaAdvisory interface {
	AnswerStructuralQuestion(VelaQuestion) (VelaEvidence, error)
}

type VelaAdvisoryResult struct {
	Evidence    VelaEvidence
	Source      string
	EvidenceGap string
}

// AdvisoryContext is task-scoped. It permits one Ancora recovery at start or
// resume and applies the established two-exploration/one-review Vela budgets.
// It never installs, sets up, indexes, re-indexes, or retries either service.
type AdvisoryContext struct {
	ancora          AncoraAdvisory
	vela            VelaAdvisory
	readReference   func(string) ([]byte, error)
	ancoraRecovered bool
	recovery        AdvisoryRecovery
	velaCalls       map[VelaRole]int
}

func NewAdvisoryContext(ancora AncoraAdvisory, vela VelaAdvisory) *AdvisoryContext {
	return &AdvisoryContext{ancora: ancora, vela: vela, velaCalls: map[VelaRole]int{}}
}

// NewWorkflowAdvisoryContext binds advisory references to the current
// repository. It is the constructor used by start/resume/workflow orchestration.
func NewWorkflowAdvisoryContext(repoRoot string, ancora AncoraAdvisory, vela VelaAdvisory) *AdvisoryContext {
	context := NewAdvisoryContext(ancora, vela)
	context.readReference = func(path string) ([]byte, error) {
		return readRepositoryFile(repoRoot, path)
	}
	return context
}

// RecoverAncoraOnce recovers concise relevant context once for a task start or
// resume. Every failure degrades to authoritative workspace/Git evidence.
func (context *AdvisoryContext) RecoverAncoraOnce() AdvisoryRecovery {
	if context.ancoraRecovered {
		return context.recovery
	}
	context.ancoraRecovered = true
	if context.ancora == nil {
		context.recovery = workspaceGitFallback("Ancora advisory context is unavailable")
		return context.recovery
	}
	recovery, err := context.ancora.RecoverRelevant()
	if err != nil {
		context.recovery = workspaceGitFallback("Ancora advisory recovery failed")
		return context.recovery
	}
	if err := context.validateAncoraRecovery(recovery); err != nil {
		context.recovery = workspaceGitFallback("Ancora advisory context was not compact")
		return context.recovery
	}
	context.recovery = AdvisoryRecovery{Context: cloneAncoraRecovery(recovery), Source: "ancora"}
	return context.recovery
}

func (context *AdvisoryContext) SaveMaterialSummary(summary AncoraMaterialSummary) AdvisorySummaryResult {
	if err := validateAncoraMaterialSummary(summary); err != nil {
		return AdvisorySummaryResult{EvidenceGap: "Ancora material summary was not compact"}
	}
	if context.ancora == nil {
		return AdvisorySummaryResult{EvidenceGap: "Ancora advisory context is unavailable; workspace/Git remain authoritative"}
	}
	if err := context.ancora.SaveMaterialSummary(summary); err != nil {
		return AdvisorySummaryResult{EvidenceGap: "Ancora material summary was not saved; workspace/Git remain authoritative"}
	}
	return AdvisorySummaryResult{Stored: true}
}

// AskVela answers only a named structural question. An unavailable, invalid,
// stale, or malformed response falls back to source and never blocks Fast work.
func (context *AdvisoryContext) AskVela(question VelaQuestion) VelaAdvisoryResult {
	if err := validateVelaQuestion(question); err != nil {
		return sourceFallback("Vela requires a named structural question")
	}
	if context.velaCalls[question.Role] >= velaBudget(question.Role) {
		return sourceFallback("Vela call budget is exhausted; use source evidence")
	}
	if context.vela == nil {
		return sourceFallback("Vela is unavailable; use source evidence")
	}
	// Count the single permitted attempt before calling the service: failures do
	// not leave room for an automatic retry.
	context.velaCalls[question.Role]++
	evidence, err := context.vela.AnswerStructuralQuestion(question)
	if err != nil {
		return sourceFallback("Vela did not answer the named question; use source evidence")
	}
	if err := validateVelaEvidence(question, evidence); err != nil {
		return sourceFallback("Vela evidence was stale, ambiguous, or not compact; use source evidence")
	}
	return VelaAdvisoryResult{Evidence: cloneVelaEvidence(evidence), Source: "vela"}
}

func (context *AdvisoryContext) VelaCalls(role VelaRole) int { return context.velaCalls[role] }

func workspaceGitFallback(gap string) AdvisoryRecovery {
	return AdvisoryRecovery{Source: "workspace_git", Degraded: true, EvidenceGap: gap}
}

func sourceFallback(gap string) VelaAdvisoryResult {
	return VelaAdvisoryResult{Source: "source", EvidenceGap: gap, Evidence: VelaEvidence{SafeAction: "inspect authoritative workspace and Git source evidence"}}
}

func (context *AdvisoryContext) validateAncoraRecovery(recovery AncoraAdvisoryRecovery) error {
	entries := append(append(append([]string{}, recovery.Decisions...), recovery.Discoveries...), recovery.Summaries...)
	if len(entries) > maxAdvisoryEntries || len(recovery.References) == 0 || len(recovery.References) > maxAdvisoryEntries {
		return errors.New("too many Ancora entries")
	}
	for _, entry := range entries {
		if err := validateCompactText("Ancora advisory entry", entry); err != nil {
			return err
		}
	}
	if context.readReference == nil {
		return errors.New("Ancora advisory references cannot be checked")
	}
	for _, reference := range recovery.References {
		if !canonicalWorkflowPath(reference.Path) || len(reference.SHA256) != sha256.Size*2 {
			return errors.New("Ancora advisory reference is missing or unsafe")
		}
		if _, err := hex.DecodeString(reference.SHA256); err != nil || strings.ToLower(reference.SHA256) != reference.SHA256 {
			return errors.New("Ancora advisory reference has a non-canonical hash")
		}
		contents, err := context.readReference(reference.Path)
		if err != nil {
			return errors.New("Ancora advisory reference is missing")
		}
		sum := sha256.Sum256(contents)
		if reference.SHA256 != hex.EncodeToString(sum[:]) {
			return errors.New("Ancora advisory reference is stale")
		}
	}
	return nil
}

func validateAncoraMaterialSummary(summary AncoraMaterialSummary) error {
	if summary.Kind != AncoraMaterialDecision && summary.Kind != AncoraMaterialDiscovery && summary.Kind != AncoraMaterialFix && summary.Kind != AncoraMaterialOutcome {
		return errors.New("unknown Ancora material kind")
	}
	if summary.Summary == "" {
		return errors.New("empty Ancora material summary")
	}
	return validateCompactText("Ancora material summary", summary.Summary)
}

func validateVelaQuestion(question VelaQuestion) error {
	if velaBudget(question.Role) == 0 || question.Kind != VelaDependency && question.Kind != VelaImpact && question.Kind != VelaOwnership && question.Kind != VelaArchitecturalFlow && question.Kind != VelaUnfamiliarModule {
		return errors.New("unknown Vela role or question kind")
	}
	if strings.TrimSpace(question.Subject) == "" || strings.TrimSpace(question.Question) == "" {
		return errors.New("unnamed Vela question")
	}
	if err := validateCompactText("Vela subject", question.Subject); err != nil {
		return err
	}
	return validateCompactText("Vela question", question.Question)
}

func validateVelaEvidence(question VelaQuestion, evidence VelaEvidence) error {
	if evidence.Confidence != "high" && evidence.Confidence != "medium" && evidence.Confidence != "low" {
		return errors.New("unknown Vela confidence")
	}
	if evidence.Subject != question.Subject || evidence.SafeAction == "" || len(evidence.Symbols) > maxAdvisoryEntries || len(evidence.Files) == 0 || len(evidence.Files) > maxAdvisoryEntries || len(evidence.Gaps) > maxAdvisoryEntries {
		return errors.New("invalid Vela evidence shape")
	}
	values := append(append(append([]string{}, evidence.Symbols...), evidence.Files...), evidence.Gaps...)
	values = append(values, evidence.SafeAction)
	for _, value := range values {
		if err := validateCompactText("Vela evidence", value); err != nil {
			return err
		}
	}
	for _, path := range evidence.Files {
		if !canonicalWorkflowPath(path) || !velaFileRelevantToSubject(question.Subject, path) {
			return errors.New("Vela file evidence is outside the named subject")
		}
	}
	return nil
}

func velaFileRelevantToSubject(subject, path string) bool {
	if !canonicalWorkflowPath(subject) {
		return false
	}
	if subject == path {
		return true
	}
	return filepath.Ext(subject) == "" && strings.HasPrefix(path, subject+"/")
}

func velaBudget(role VelaRole) int {
	switch role {
	case VelaExploreRole:
		return maxVelaExploreCalls
	case VelaReviewRole:
		return maxVelaReviewCalls
	default:
		return 0
	}
}

func cloneAncoraRecovery(recovery AncoraAdvisoryRecovery) AncoraAdvisoryRecovery {
	recovery.Decisions = append([]string(nil), recovery.Decisions...)
	recovery.Discoveries = append([]string(nil), recovery.Discoveries...)
	recovery.Summaries = append([]string(nil), recovery.Summaries...)
	recovery.References = append([]AncoraAdvisoryReference(nil), recovery.References...)
	return recovery
}

func cloneVelaEvidence(evidence VelaEvidence) VelaEvidence {
	evidence.Symbols = append([]string(nil), evidence.Symbols...)
	evidence.Files = append([]string(nil), evidence.Files...)
	evidence.Gaps = append([]string(nil), evidence.Gaps...)
	return evidence
}
