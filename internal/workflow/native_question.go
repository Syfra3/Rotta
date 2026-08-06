package workflow

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// QuestionPurpose distinguishes a bounded decision from a consent request.
type QuestionPurpose string

// NativeQuestionTrigger is the closed set of human-question reasons approved
// by the Strict contract. A question cannot be displayed or marshalled unless
// it declares one of these reasons.
type NativeQuestionTrigger string

const (
	MaterialStrictClarificationTrigger NativeQuestionTrigger = "material-strict-clarification"
	ExactStrictApprovalTrigger         NativeQuestionTrigger = "exact-strict-approval"
	MaterialPolicyDecisionTrigger      NativeQuestionTrigger = "material-policy-decision"
	ExactOperationConsentTrigger       NativeQuestionTrigger = "exact-operation-consent"
	VelaStaleEvidenceTrigger           NativeQuestionTrigger = "vela-stale-evidence"
)

// OperationClass is the closed set of operation kinds that may request
// one-time consent. It is input classification only and never executes work.
type OperationClass string

const (
	DestructiveOperationClass OperationClass = "destructive"
	ExternalOperationClass    OperationClass = "external"
)

const (
	QuestionPurposeDecision QuestionPurpose = "decision"
	QuestionPurposeConsent  QuestionPurpose = "consent"

	VelaStaleGraphAction     = "vela-stale-graph"
	VelaReindexReviewAction  = "vela-reindex-review"
	VelaUseSourceFallback    = "Use source fallback"
	VelaRequestReindexReview = "Request bounded re-index"
	VelaStopAndRevisit       = "Stop and revisit"

	ExactStrictApprovalAction = "strict-exact-contract-approval"
	ExactStrictApproveOption  = "Approve exact rendered contract"
	ExactStrictStopOption     = "Do not approve / stop"

	MaterialPolicyDecisionAction = "material-policy-decision"
	MaterialPolicyStopOption     = "Stop and revisit policy"

	ExactOperationConsentOption = "Approve the exact rendered operation once"
	OperationConsentStopOption  = "Do not approve / stop"
)

// QuestionContext binds one displayed question and its answer to the active
// session, workspace, and named consumer action.
type QuestionContext struct {
	PromptID  string
	SessionID string
	Workspace string
	Action    string
}

// ExactStrictApprovalContext identifies the only rendered contract for which
// an approval decision may be collected. ContentDigest is lowercase SHA-256.
type ExactStrictApprovalContext struct {
	ContractPath     string
	ContentDigest    string
	RenderedRevision string
	SessionID        string
	Workspace        string
	Action           string
}

// ExactOperationConsentContext identifies one rendered destructive or external
// operation. It is decision binding only; it cannot execute the operation.
type ExactOperationConsentContext struct {
	Action           string
	OperationClass   OperationClass
	CanonicalTarget  string
	SessionID        string
	Workspace        string
	MaterialEffect   string
	ApprovalScope    string
	ContentDigest    string
	RenderedRevision string
}

// MaterialPolicyDecision is a named, bounded, non-operational decision.
// SafeDefault must name one listed alternative; stop is always also available.
type MaterialPolicyDecision struct {
	Question     string
	Header       string
	Alternatives []QuestionOption
	SafeDefault  string
}

// QuestionOption is the OpenCode-native selectable option shape.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// NativeQuestion is a bounded, single-select OpenCode question request.
// Custom is always explicit; consent questions use false so only a displayed
// choice can supply decision evidence.
type NativeQuestion struct {
	Context     QuestionContext
	Trigger     NativeQuestionTrigger
	Question    string
	Header      string
	Options     []QuestionOption
	Custom      bool
	Purpose     QuestionPurpose
	Approval    *ExactStrictApprovalContext
	Consent     *ExactOperationConsentContext
	SafeDefault string

	issuer      nativeQuestionIssuer
	fingerprint string
}

// nativeQuestionIssuer is deliberately private: a NativeQuestion can only be
// made displayable by one of the approved producers in this package.
type nativeQuestionIssuer uint8

const (
	noNativeQuestionIssuer nativeQuestionIssuer = iota
	materialStrictClarificationIssuer
	exactStrictApprovalIssuer
	materialPolicyDecisionIssuer
	exactOperationConsentIssuer
	velaStaleEvidenceIssuer
)

// NativeQuestionAnswer is the selected label returned by the native question
// tool with its originating interaction binding.
type NativeQuestionAnswer struct {
	Context QuestionContext
	Label   string
}

// OpenCodeQuestionRequest is the native question tool input. It deliberately
// contains one question because this bridge collects one bounded decision at a
// time.
type OpenCodeQuestionRequest struct {
	Questions []OpenCodeQuestion `json:"questions"`
}

// OpenCodeQuestion is the question shape accepted in an OpenCode questions
// request. Multiple is explicit so the wire contract remains single-select.
type OpenCodeQuestion struct {
	Question string           `json:"question"`
	Header   string           `json:"header"`
	Options  []QuestionOption `json:"options"`
	Multiple bool             `json:"multiple"`
	Custom   bool             `json:"custom"`
}

// OpenCodeQuestionReply is the native question tool reply. Each nested answer
// list corresponds to one request question.
type OpenCodeQuestionReply struct {
	Answers [][]string `json:"answers"`
}

// PendingExplicitAction describes a possible next step. It is not an
// authorization and it is never executed by question handling.
type PendingExplicitAction struct {
	Kind                          string
	ProjectRoot                   string
	RequiresExplicitAuthorization bool
	Executed                      bool
	Consent                       *ExactOperationConsentContext
}

// NativeQuestionResult contains only ephemeral decision evidence and, for a
// named illustrative flow, an unexecuted next action requiring fresh consent.
type NativeQuestionResult struct {
	Selected      string
	PendingAction *PendingExplicitAction
	ExactApproval *ExactStrictApprovalEvidence
}

// ExactStrictApprovalEvidence is ephemeral decision evidence only. It never
// authorizes delegation, an operation, Vela activity, or Git activity.
type ExactStrictApprovalEvidence struct {
	Context  ExactStrictApprovalContext
	Selected string
}

// DisplayedNativeQuestion tracks the sole current display of one question.
type DisplayedNativeQuestion struct {
	question          NativeQuestion
	sealedFingerprint string
	closed            bool
	replaced          bool
}

func NewDisplayedNativeQuestion(question NativeQuestion) (*DisplayedNativeQuestion, error) {
	if err := validateNativeQuestion(question); err != nil {
		return nil, err
	}
	issued := sealedNativeQuestionCopy(question)
	return &DisplayedNativeQuestion{
		question:          issued,
		sealedFingerprint: nativeQuestionFingerprint(issued),
	}, nil
}

// MarshalOpenCodeQuestionRequest validates and serializes a single native
// question for the OpenCode question tool. Context and purpose intentionally
// remain local: they bind answer consumption but are not OpenCode fields.
func MarshalOpenCodeQuestionRequest(question NativeQuestion) ([]byte, error) {
	if err := validateNativeQuestion(question); err != nil {
		return nil, err
	}
	return json.Marshal(OpenCodeQuestionRequest{Questions: []OpenCodeQuestion{{
		Question: question.Question,
		Header:   question.Header,
		Options:  question.Options,
		Multiple: false,
		Custom:   question.Custom,
	}}})
}

// ConsumeOpenCodeQuestionReply extracts the sole selected answer from an
// OpenCode reply, binds it to the displayed request, and consumes it against
// the current context. Missing, dismissed, multi-select, custom, stale, and
// mismatched replies return no decision evidence.
func ConsumeOpenCodeQuestionReply(displayed *DisplayedNativeQuestion, reply OpenCodeQuestionReply, current QuestionContext) (NativeQuestionResult, error) {
	if displayed == nil {
		return NativeQuestionResult{}, fmt.Errorf("native question is not displayed")
	}
	if len(reply.Answers) != 1 || len(reply.Answers[0]) != 1 || strings.TrimSpace(reply.Answers[0][0]) == "" {
		return NativeQuestionResult{}, fmt.Errorf("native question reply has no single selected answer")
	}
	return displayed.Consume(NativeQuestionAnswer{
		Context: displayed.question.Context,
		Label:   reply.Answers[0][0],
	}, current)
}

// Replace invalidates the displayed request before any prior answer can be
// consumed. The replacement itself must be displayed as a new question.
func (question *DisplayedNativeQuestion) Replace() {
	question.replaced = true
}

// Consume accepts exactly one current listed option. It validates every
// binding before returning decision evidence and performs no action.
func (question *DisplayedNativeQuestion) Consume(answer NativeQuestionAnswer, current QuestionContext) (NativeQuestionResult, error) {
	if question.closed {
		return NativeQuestionResult{}, fmt.Errorf("native question is closed")
	}
	if question.replaced {
		return NativeQuestionResult{}, fmt.Errorf("native question was replaced")
	}
	if err := question.validateIssuedQuestion(); err != nil {
		return NativeQuestionResult{}, err
	}
	if answer.Context.PromptID != question.question.Context.PromptID || current.PromptID != question.question.Context.PromptID {
		return NativeQuestionResult{}, fmt.Errorf("native question answer is stale")
	}
	if answer.Context.SessionID != question.question.Context.SessionID || current.SessionID != question.question.Context.SessionID {
		return NativeQuestionResult{}, fmt.Errorf("native question answer session does not match")
	}
	if answer.Context.Workspace != question.question.Context.Workspace || current.Workspace != question.question.Context.Workspace {
		return NativeQuestionResult{}, fmt.Errorf("native question answer workspace does not match")
	}
	if answer.Context.Action != question.question.Context.Action || current.Action != question.question.Context.Action {
		return NativeQuestionResult{}, fmt.Errorf("native question answer action does not match")
	}
	if !question.hasOption(answer.Label) {
		return NativeQuestionResult{}, fmt.Errorf("native question answer is not a current option")
	}
	question.closed = true
	return NativeQuestionResult{Selected: answer.Label}, nil
}

// validateIssuedQuestion verifies that the sealed displayed request remains an
// approved factory output. It is intentionally repeated at consume time so no
// decision can be emitted from a changed stored request.
func (question *DisplayedNativeQuestion) validateIssuedQuestion() error {
	if question.sealedFingerprint == "" || question.sealedFingerprint != nativeQuestionFingerprint(question.question) {
		return fmt.Errorf("displayed native question was modified")
	}
	if err := validateNativeQuestion(question.question); err != nil {
		return fmt.Errorf("displayed native question is invalid: %w", err)
	}
	return nil
}

func sealedNativeQuestionCopy(question NativeQuestion) NativeQuestion {
	clone := question
	clone.Options = append([]QuestionOption(nil), question.Options...)
	if question.Approval != nil {
		approval := *question.Approval
		clone.Approval = &approval
	}
	if question.Consent != nil {
		consent := *question.Consent
		clone.Consent = &consent
	}
	return clone
}

func (question *DisplayedNativeQuestion) hasOption(label string) bool {
	for _, option := range question.question.Options {
		if option.Label == label {
			return true
		}
	}
	return false
}

func hasQuestionOption(options []QuestionOption, label string) bool {
	for _, option := range options {
		if option.Label == label {
			return true
		}
	}
	return false
}

// NewVelaStaleGraphQuestion provides the sole illustrative stale-graph flow.
// Its re-index option requests later review only; it cannot invoke Vela.
func NewVelaStaleGraphQuestion(context QuestionContext) (NativeQuestion, error) {
	if context.Action != VelaStaleGraphAction {
		return NativeQuestion{}, fmt.Errorf("Vela stale-graph question requires action %q", VelaStaleGraphAction)
	}
	if !filepath.IsAbs(context.Workspace) || filepath.Clean(context.Workspace) != context.Workspace {
		return NativeQuestion{}, fmt.Errorf("Vela stale-graph question requires a canonical project root")
	}
	question := NativeQuestion{
		Context:  context,
		Trigger:  VelaStaleEvidenceTrigger,
		Question: "Graph evidence is stale. What should happen?",
		Header:   "Graph evidence",
		Options: []QuestionOption{
			{Label: VelaUseSourceFallback, Description: "Continue with source evidence; do not change graph state."},
			{Label: VelaRequestReindexReview, Description: "Record a bounded re-index review for this project root."},
			{Label: VelaStopAndRevisit, Description: "Stop this path and revisit the decision."},
		},
		Custom:  false,
		Purpose: QuestionPurposeConsent,
	}
	question = issueNativeQuestion(question, velaStaleEvidenceIssuer)
	if err := validateNativeQuestion(question); err != nil {
		return NativeQuestion{}, err
	}
	return question, nil
}

// ConsumeVelaStaleGraphAnswer converts only the displayed re-index-review
// selection into a pending next action. The pending record remains explicitly
// unauthorized and this function has no operational side effects.
func ConsumeVelaStaleGraphAnswer(displayed *DisplayedNativeQuestion, answer NativeQuestionAnswer, current QuestionContext) (NativeQuestionResult, error) {
	result, err := displayed.Consume(answer, current)
	if err != nil {
		return NativeQuestionResult{}, err
	}
	if result.Selected == VelaRequestReindexReview {
		result.PendingAction = &PendingExplicitAction{
			Kind:                          VelaReindexReviewAction,
			ProjectRoot:                   current.Workspace,
			RequiresExplicitAuthorization: true,
		}
	}
	return result, nil
}

// NewExactStrictApprovalQuestion creates the only exact-contract approval
// question. Approval binding remains local and is omitted from tool payloads.
func NewExactStrictApprovalQuestion(context QuestionContext, approval ExactStrictApprovalContext) (NativeQuestion, error) {
	if context.Action != ExactStrictApprovalAction || approval.Action != ExactStrictApprovalAction {
		return NativeQuestion{}, fmt.Errorf("exact Strict approval requires action %q", ExactStrictApprovalAction)
	}
	if context.SessionID != approval.SessionID || context.Workspace != approval.Workspace {
		return NativeQuestion{}, fmt.Errorf("exact Strict approval context does not match question context")
	}
	if err := validateExactStrictApprovalContext(approval); err != nil {
		return NativeQuestion{}, err
	}
	question := NativeQuestion{
		Context: context, Trigger: ExactStrictApprovalTrigger,
		Question: fmt.Sprintf("Approve this exact rendered Strict contract at canonical path %q (SHA-256 digest: %s; rendered revision: %s)?", approval.ContractPath, approval.ContentDigest, approval.RenderedRevision), Header: "Strict approval",
		Options: []QuestionOption{{Label: ExactStrictApproveOption, Description: fmt.Sprintf("Record a decision only for contract %s, SHA-256 %s, revision %s.", approval.ContractPath, approval.ContentDigest, approval.RenderedRevision)}, {Label: ExactStrictStopOption, Description: "Do not approve; stop this Strict path safely."}},
		Custom:  false, Purpose: QuestionPurposeConsent, Approval: &approval,
	}
	question = issueNativeQuestion(question, exactStrictApprovalIssuer)
	return question, validateNativeQuestion(question)
}

// ConsumeExactStrictApprovalReply rejects absent, stale, changed, replaced,
// invalid, custom, and binding-mismatched answers before emitting evidence.
func ConsumeExactStrictApprovalReply(displayed *DisplayedNativeQuestion, reply OpenCodeQuestionReply, current ExactStrictApprovalContext) (NativeQuestionResult, error) {
	if displayed == nil || displayed.question.Approval == nil {
		return NativeQuestionResult{}, fmt.Errorf("exact Strict approval is not displayed")
	}
	if err := validateExactStrictApprovalContext(current); err != nil {
		return NativeQuestionResult{}, err
	}
	if *displayed.question.Approval != current {
		return NativeQuestionResult{}, fmt.Errorf("exact Strict approval binding does not match current rendered contract")
	}
	result, err := ConsumeOpenCodeQuestionReply(displayed, reply, QuestionContext{PromptID: displayed.question.Context.PromptID, SessionID: current.SessionID, Workspace: current.Workspace, Action: current.Action})
	if err != nil {
		return NativeQuestionResult{}, err
	}
	result.ExactApproval = &ExactStrictApprovalEvidence{Context: current, Selected: result.Selected}
	return result, nil
}

// NewExactOperationConsentQuestion creates the sole destructive/external
// one-time-consent question. The rendered text exposes the exact action,
// canonical target/workspace, material effect, and approval scope.
func NewExactOperationConsentQuestion(context QuestionContext, consent ExactOperationConsentContext) (NativeQuestion, error) {
	if context.Action != consent.Action || context.SessionID != consent.SessionID || context.Workspace != consent.Workspace {
		return NativeQuestion{}, fmt.Errorf("operation consent context does not match question context")
	}
	if err := validateExactOperationConsentContext(consent); err != nil {
		return NativeQuestion{}, err
	}
	question := NativeQuestion{
		Context:  context,
		Trigger:  ExactOperationConsentTrigger,
		Question: fmt.Sprintf("One-time consent for action %q on canonical target %q in workspace %q. Material effect: %s Approval scope: %s Rendered revision: %s Content digest: %s", consent.Action, consent.CanonicalTarget, consent.Workspace, consent.MaterialEffect, consent.ApprovalScope, consent.RenderedRevision, consent.ContentDigest),
		Header:   "One-time operation consent",
		Options: []QuestionOption{
			{Label: ExactOperationConsentOption, Description: "Create only a bounded pending authorization record for this exact rendered operation."},
			{Label: OperationConsentStopOption, Description: "Do not approve; stop this operation path safely."},
		},
		Custom: false, Purpose: QuestionPurposeConsent, Consent: &consent,
	}
	question = issueNativeQuestion(question, exactOperationConsentIssuer)
	return question, validateNativeQuestion(question)
}

// ConsumeExactOperationConsentReply accepts only the exact one-time option and
// creates an unexecuted pending authorization record. Any other valid choice
// safely stops with no record; unsafe replies are rejected by the wire adapter.
func ConsumeExactOperationConsentReply(displayed *DisplayedNativeQuestion, reply OpenCodeQuestionReply, current ExactOperationConsentContext) (NativeQuestionResult, error) {
	if displayed == nil {
		return NativeQuestionResult{}, fmt.Errorf("operation consent is not displayed")
	}
	if err := validateExactOperationConsentContext(current); err != nil {
		return NativeQuestionResult{}, err
	}
	if displayed.question.Consent == nil || *displayed.question.Consent != current || displayed.question.Context.Action != current.Action || displayed.question.Context.SessionID != current.SessionID || displayed.question.Context.Workspace != current.Workspace {
		return NativeQuestionResult{}, fmt.Errorf("operation consent binding does not match current rendered operation")
	}
	result, err := ConsumeOpenCodeQuestionReply(displayed, reply, displayed.question.Context)
	if err != nil {
		return NativeQuestionResult{}, err
	}
	if result.Selected == ExactOperationConsentOption {
		copy := current
		result.PendingAction = &PendingExplicitAction{
			Kind:                          current.Action,
			ProjectRoot:                   current.Workspace,
			RequiresExplicitAuthorization: true,
			Consent:                       &copy,
		}
	}
	return result, nil
}

// NewMaterialPolicyDecisionQuestion rejects generic continuation prompts and
// supplies a stop outcome. It cannot name an operational consumer action.
func NewMaterialPolicyDecisionQuestion(context QuestionContext, decision MaterialPolicyDecision) (NativeQuestion, error) {
	if context.Action != MaterialPolicyDecisionAction {
		return NativeQuestion{}, fmt.Errorf("material policy question requires action %q", MaterialPolicyDecisionAction)
	}
	lowerQuestion := strings.ToLower(decision.Question)
	if strings.Contains(lowerQuestion, "continue") || strings.Contains(lowerQuestion, "proceed") || strings.Contains(lowerQuestion, "approve work") {
		return NativeQuestion{}, fmt.Errorf("material policy question cannot be a generic continuation")
	}
	if !hasQuestionOption(decision.Alternatives, decision.SafeDefault) {
		return NativeQuestion{}, fmt.Errorf("material policy safe default must be a listed alternative")
	}
	options := append([]QuestionOption(nil), decision.Alternatives...)
	options = append(options, QuestionOption{Label: MaterialPolicyStopOption, Description: "Stop and retain the stated safe default without making a policy decision."})
	question := NativeQuestion{Context: context, Trigger: MaterialPolicyDecisionTrigger, Question: decision.Question, Header: decision.Header, Options: options, Custom: false, Purpose: QuestionPurposeDecision, SafeDefault: decision.SafeDefault}
	question = issueNativeQuestion(question, materialPolicyDecisionIssuer)
	return question, validateNativeQuestion(question)
}

func validateExactStrictApprovalContext(context ExactStrictApprovalContext) error {
	if context.SessionID == "" || context.Workspace == "" || context.Action != ExactStrictApprovalAction || context.RenderedRevision == "" {
		return fmt.Errorf("exact Strict approval context is incomplete")
	}
	path := filepath.ToSlash(context.ContractPath)
	if filepath.IsAbs(context.ContractPath) || path != filepath.ToSlash(filepath.Clean(context.ContractPath)) || !strings.HasPrefix(path, ".rotta/strict/") || !strings.HasSuffix(path, ".md") {
		return fmt.Errorf("exact Strict approval contract path is not canonical")
	}
	if len(context.ContentDigest) != 64 {
		return fmt.Errorf("exact Strict approval digest is invalid")
	}
	for _, character := range context.ContentDigest {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return fmt.Errorf("exact Strict approval digest is invalid")
		}
	}
	return nil
}

func validateExactOperationConsentContext(context ExactOperationConsentContext) error {
	if strings.TrimSpace(context.Action) == "" || strings.TrimSpace(context.SessionID) == "" || !filepath.IsAbs(context.Workspace) || filepath.Clean(context.Workspace) != context.Workspace || strings.TrimSpace(context.CanonicalTarget) == "" || strings.TrimSpace(context.MaterialEffect) == "" || strings.TrimSpace(context.ApprovalScope) == "" || strings.TrimSpace(context.RenderedRevision) == "" {
		return fmt.Errorf("operation consent context is incomplete or not canonical")
	}
	if context.OperationClass != DestructiveOperationClass && context.OperationClass != ExternalOperationClass {
		return fmt.Errorf("operation consent class is not permitted")
	}
	if err := validateOperationTarget(context); err != nil {
		return err
	}
	if len(context.ContentDigest) != 64 {
		return fmt.Errorf("operation consent digest is invalid")
	}
	for _, character := range context.ContentDigest {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return fmt.Errorf("operation consent digest is invalid")
		}
	}
	return nil
}

func validateOperationTarget(context ExactOperationConsentContext) error {
	target := context.CanonicalTarget
	if strings.TrimSpace(target) != target {
		return fmt.Errorf("operation consent target is not canonical")
	}
	switch context.OperationClass {
	case DestructiveOperationClass:
		if !filepath.IsAbs(target) || filepath.Clean(target) != target || (target != context.Workspace && !strings.HasPrefix(target, context.Workspace+string(filepath.Separator))) {
			return fmt.Errorf("destructive operation target is not a canonical workspace target")
		}
	case ExternalOperationClass:
		parsed, err := url.ParseRequestURI(target)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Path == "" || parsed.Path == "/" || filepath.ToSlash(filepath.Clean(parsed.Path)) != parsed.Path || parsed.String() != target {
			return fmt.Errorf("external operation target is not a canonical HTTPS target")
		}
	}
	return nil
}

func validateNativeQuestion(question NativeQuestion) error {
	if question.Context.PromptID == "" || question.Context.SessionID == "" || question.Context.Workspace == "" || question.Context.Action == "" {
		return fmt.Errorf("native question context is incomplete")
	}
	if strings.TrimSpace(question.Question) == "" {
		return fmt.Errorf("native question text is required")
	}
	if utf8.RuneCountInString(question.Header) > 30 {
		return fmt.Errorf("native question header exceeds 30 characters")
	}
	if strings.TrimSpace(question.Header) == "" {
		return fmt.Errorf("native question header is required")
	}
	if question.Purpose != QuestionPurposeDecision && question.Purpose != QuestionPurposeConsent {
		return fmt.Errorf("native question purpose is invalid")
	}
	if err := validateNativeQuestionTrigger(question); err != nil {
		return err
	}
	if question.Purpose == QuestionPurposeConsent && question.Custom {
		return fmt.Errorf("native question consent must disable custom answers")
	}
	if len(question.Options) == 0 {
		return fmt.Errorf("native question requires options")
	}
	labels := make(map[string]struct{}, len(question.Options))
	for _, option := range question.Options {
		if strings.TrimSpace(option.Label) == "" || strings.TrimSpace(option.Description) == "" {
			return fmt.Errorf("native question option requires label and description")
		}
		if _, exists := labels[option.Label]; exists {
			return fmt.Errorf("native question option labels must be unique")
		}
		labels[option.Label] = struct{}{}
	}
	return nil
}

// issueNativeQuestion records the private provenance and complete canonical
// shape at the factory boundary. Public fields are intentional wire data, so
// validation must also detect mutation after a factory returns.
func issueNativeQuestion(question NativeQuestion, issuer nativeQuestionIssuer) NativeQuestion {
	question.issuer = issuer
	question.fingerprint = nativeQuestionFingerprint(question)
	return question
}

func nativeQuestionFingerprint(question NativeQuestion) string {
	type canonicalNativeQuestion struct {
		Context     QuestionContext
		Trigger     NativeQuestionTrigger
		Question    string
		Header      string
		Options     []QuestionOption
		Custom      bool
		Purpose     QuestionPurpose
		Approval    *ExactStrictApprovalContext
		Consent     *ExactOperationConsentContext
		SafeDefault string
	}
	encoded, err := json.Marshal(canonicalNativeQuestion{
		Context: question.Context, Trigger: question.Trigger, Question: question.Question,
		Header: question.Header, Options: question.Options, Custom: question.Custom,
		Purpose: question.Purpose, Approval: question.Approval, Consent: question.Consent,
		SafeDefault: question.SafeDefault,
	})
	if err != nil {
		return ""
	}
	return string(encoded)
}

func validateNativeQuestionTrigger(question NativeQuestion) error {
	if question.issuer == noNativeQuestionIssuer || question.fingerprint == "" || question.fingerprint != nativeQuestionFingerprint(question) {
		return fmt.Errorf("native question was not issued by an approved factory")
	}
	switch question.Trigger {
	case MaterialStrictClarificationTrigger:
		if question.issuer != materialStrictClarificationIssuer || question.Context.Action != StrictClarificationAction || question.Purpose != QuestionPurposeDecision || question.Custom || question.Approval != nil || question.Consent != nil || question.SafeDefault != "" {
			return fmt.Errorf("material Strict clarification trigger binding is invalid")
		}
	case ExactStrictApprovalTrigger:
		if question.issuer != exactStrictApprovalIssuer || question.Context.Action != ExactStrictApprovalAction || question.Approval == nil || question.Consent != nil || question.Purpose != QuestionPurposeConsent || question.Context.SessionID != question.Approval.SessionID || question.Context.Workspace != question.Approval.Workspace || question.fingerprint != nativeQuestionFingerprint(newExactStrictApprovalQuestion(question.Context, *question.Approval)) {
			return fmt.Errorf("exact Strict approval trigger binding is invalid")
		}
	case MaterialPolicyDecisionTrigger:
		if question.issuer != materialPolicyDecisionIssuer || question.Context.Action != MaterialPolicyDecisionAction || question.Purpose != QuestionPurposeDecision || question.Custom || question.Approval != nil || question.Consent != nil || !hasQuestionOption(question.Options, question.SafeDefault) || !hasQuestionOption(question.Options, MaterialPolicyStopOption) {
			return fmt.Errorf("material policy trigger binding is invalid")
		}
	case ExactOperationConsentTrigger:
		if question.issuer != exactOperationConsentIssuer || question.Consent == nil || question.Approval != nil || question.Purpose != QuestionPurposeConsent || question.Context.Action != question.Consent.Action || question.Context.SessionID != question.Consent.SessionID || question.Context.Workspace != question.Consent.Workspace || question.fingerprint != nativeQuestionFingerprint(newExactOperationConsentQuestion(question.Context, *question.Consent)) {
			return fmt.Errorf("operation consent trigger binding is invalid")
		}
	case VelaStaleEvidenceTrigger:
		if question.issuer != velaStaleEvidenceIssuer || question.Context.Action != VelaStaleGraphAction || !filepath.IsAbs(question.Context.Workspace) || filepath.Clean(question.Context.Workspace) != question.Context.Workspace || question.Purpose != QuestionPurposeConsent || question.Approval != nil || question.Consent != nil || question.SafeDefault != "" || question.fingerprint != nativeQuestionFingerprint(newVelaStaleGraphQuestion(question.Context)) {
			return fmt.Errorf("Vela stale-evidence trigger binding is invalid")
		}
	default:
		return fmt.Errorf("native question trigger is not approved")
	}
	if strings.Contains(strings.ToLower(question.Question), "continue") || strings.Contains(strings.ToLower(question.Question), "proceed") || strings.Contains(strings.ToLower(question.Question), "approve work") {
		return fmt.Errorf("native question cannot be a generic continuation")
	}
	return nil
}

func newVelaStaleGraphQuestion(context QuestionContext) NativeQuestion {
	return NativeQuestion{Context: context, Trigger: VelaStaleEvidenceTrigger, Question: "Graph evidence is stale. What should happen?", Header: "Graph evidence", Options: []QuestionOption{
		{Label: VelaUseSourceFallback, Description: "Continue with source evidence; do not change graph state."},
		{Label: VelaRequestReindexReview, Description: "Record a bounded re-index review for this project root."},
		{Label: VelaStopAndRevisit, Description: "Stop this path and revisit the decision."},
	}, Custom: false, Purpose: QuestionPurposeConsent}
}

func newExactStrictApprovalQuestion(context QuestionContext, approval ExactStrictApprovalContext) NativeQuestion {
	return NativeQuestion{Context: context, Trigger: ExactStrictApprovalTrigger,
		Question: fmt.Sprintf("Approve this exact rendered Strict contract at canonical path %q (SHA-256 digest: %s; rendered revision: %s)?", approval.ContractPath, approval.ContentDigest, approval.RenderedRevision), Header: "Strict approval",
		Options: []QuestionOption{{Label: ExactStrictApproveOption, Description: fmt.Sprintf("Record a decision only for contract %s, SHA-256 %s, revision %s.", approval.ContractPath, approval.ContentDigest, approval.RenderedRevision)}, {Label: ExactStrictStopOption, Description: "Do not approve; stop this Strict path safely."}},
		Custom:  false, Purpose: QuestionPurposeConsent, Approval: &approval}
}

func newExactOperationConsentQuestion(context QuestionContext, consent ExactOperationConsentContext) NativeQuestion {
	return NativeQuestion{Context: context, Trigger: ExactOperationConsentTrigger,
		Question: fmt.Sprintf("One-time consent for action %q on canonical target %q in workspace %q. Material effect: %s Approval scope: %s Rendered revision: %s Content digest: %s", consent.Action, consent.CanonicalTarget, consent.Workspace, consent.MaterialEffect, consent.ApprovalScope, consent.RenderedRevision, consent.ContentDigest), Header: "One-time operation consent",
		Options: []QuestionOption{{Label: ExactOperationConsentOption, Description: "Create only a bounded pending authorization record for this exact rendered operation."}, {Label: OperationConsentStopOption, Description: "Do not approve; stop this operation path safely."}},
		Custom:  false, Purpose: QuestionPurposeConsent, Consent: &consent}
}
