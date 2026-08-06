package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	StrictClarificationAction       = "strict-draft-clarification"
	StrictClarificationStopOption   = "Stop / use safe defaults"
	maxStrictClarificationQuestions = 3
)

type StrictClarificationStatus string

const (
	StrictClarificationActive      StrictClarificationStatus = "active"
	StrictClarificationComplete    StrictClarificationStatus = "complete"
	StrictClarificationStopped     StrictClarificationStatus = "stopped"
	StrictClarificationInvalidated StrictClarificationStatus = "invalidated"
)

// StrictClarificationRequest contains only immutable request identity and
// initial request data. Question content is deliberately not caller supplied.
type StrictClarificationRequest struct {
	ContractID     string
	InitialRequest string
	SessionID      string
	Workspace      string
}

type strictClarificationField struct {
	key      string
	question string
	header   string
	options  []QuestionOption
}

// StrictClarificationFlow deliberately retains answers only in memory. It has
// no persistence or delegation path.
type StrictClarificationFlow struct {
	request    StrictClarificationRequest
	answers    map[string]string
	asked      int
	status     StrictClarificationStatus
	displayed  *DisplayedNativeQuestion
	currentKey string
}

type PendingStrictDraft struct {
	WorkflowPolicyArtifacts
	RequiresExplicitApproval bool
	ApprovalScope            ContractScope
	Binding                  StrictDraftBinding
	Publication              WorkflowArtifactPairPublicationStatus
	partial                  *WorkflowArtifactPairIncompleteError
}

// StrictDraftBinding binds a rendered draft to the immutable clarification
// request and exact rendered contents. It is evidence only, never approval.
type StrictDraftBinding struct {
	DraftID     string
	ContractID  string
	SessionID   string
	HardSpecSum string
	FeatureSum  string
}

var validateRetainedStrictDraftArtifact = validateRetainedStrictDraftArtifactAtPath

func (draft PendingStrictDraft) Complete() bool {
	return draft.partial == nil && draft.Publication.Complete()
}

// StrictClarificationSafeStop is the non-durable result of an incomplete or
// cancelled flow. It intentionally exposes no inferred values.
type StrictClarificationSafeStop struct {
	ProposedScope    string
	MissingDecisions []string
}

func NewStrictClarificationFlow(request StrictClarificationRequest) (*StrictClarificationFlow, error) {
	if strings.TrimSpace(request.ContractID) == "" || strings.TrimSpace(request.InitialRequest) == "" || strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Workspace) == "" {
		return nil, fmt.Errorf("strict clarification request is incomplete")
	}
	return &StrictClarificationFlow{request: request, answers: map[string]string{}, status: StrictClarificationActive}, nil
}

// CurrentQuestion produces only one active native single-select question. A
// new prompt ID is supplied by the native interaction boundary, never reused.
func (flow *StrictClarificationFlow) CurrentQuestion(promptID string) *NativeQuestion {
	if flow.status != StrictClarificationActive || flow.displayed != nil || flow.asked >= maxStrictClarificationQuestions {
		return nil
	}
	field := flow.nextRequired()
	if field == nil {
		flow.status = StrictClarificationComplete
		return nil
	}
	options := append([]QuestionOption(nil), field.options...)
	options = append(options, QuestionOption{Label: StrictClarificationStopOption, Description: "Stop clarification and leave unresolved decisions for safe defaults."})
	question := issueNativeQuestion(NativeQuestion{Context: QuestionContext{PromptID: promptID, SessionID: flow.request.SessionID, Workspace: flow.request.Workspace, Action: StrictClarificationAction}, Trigger: MaterialStrictClarificationTrigger, Question: field.question, Header: field.header, Options: options, Custom: false, Purpose: QuestionPurposeDecision}, materialStrictClarificationIssuer)
	displayed, err := NewDisplayedNativeQuestion(cloneSequentialDraftQuestion(question))
	if err != nil {
		flow.stop()
		return nil
	}
	flow.displayed, flow.currentKey = displayed, field.key
	flow.asked++
	returned := cloneSequentialDraftQuestion(question)
	return &returned
}

// ConsumeReply treats all absent, invalid, stale, and mismatched replies as a
// safe stop; such input never becomes answer evidence.
func (flow *StrictClarificationFlow) ConsumeReply(reply OpenCodeQuestionReply, current QuestionContext) {
	if flow.status != StrictClarificationActive || flow.displayed == nil {
		flow.stop()
		return
	}
	result, err := ConsumeOpenCodeQuestionReply(flow.displayed, reply, current)
	if err != nil || result.Selected == StrictClarificationStopOption {
		flow.stop()
		return
	}
	flow.answers[flow.currentKey] = result.Selected
	flow.displayed, flow.currentKey = nil, ""
	if flow.nextRequired() == nil {
		flow.status = StrictClarificationComplete
	} else if flow.asked >= maxStrictClarificationQuestions {
		flow.stop()
	}
}

func (flow *StrictClarificationFlow) Unavailable() { flow.stop() }

// ReplaceCurrentQuestion invalidates the displayed question. A replacement
// cannot inherit prior answer evidence; callers must begin a new flow.
func (flow *StrictClarificationFlow) ReplaceCurrentQuestion() {
	if flow.displayed != nil {
		flow.displayed.Replace()
	}
	flow.stop()
}

// InvalidateForRevision discards all contextual evidence. Callers must start a
// new flow with the materially revised request.
func (flow *StrictClarificationFlow) InvalidateForRevision() {
	flow.answers = map[string]string{}
	flow.displayed, flow.currentKey = nil, ""
	flow.status = StrictClarificationInvalidated
}

func (flow *StrictClarificationFlow) Status() StrictClarificationStatus { return flow.status }

func (flow *StrictClarificationFlow) Answers() map[string]string {
	answers := make(map[string]string, len(flow.answers))
	for key, answer := range flow.answers {
		answers[key] = answer
	}
	return answers
}

func (flow *StrictClarificationFlow) MissingDecisions() []string {
	missing := []string{}
	for _, field := range flow.derivedFields() {
		if _, ok := flow.answers[field.key]; !ok {
			missing = append(missing, field.key)
		}
	}
	return missing
}

func (flow *StrictClarificationFlow) SafeStopProposal() StrictClarificationSafeStop {
	return StrictClarificationSafeStop{
		ProposedScope:    flow.request.InitialRequest,
		MissingDecisions: flow.MissingDecisions(),
	}
}

// GeneratePendingDraft creates namespaced draft artifacts only after every
// required answer was accepted. It records no approval: the exact render must
// pass the existing independent implementation gate later.
func (flow *StrictClarificationFlow) GeneratePendingDraft(repoRoot, hardSpec, feature string) (PendingStrictDraft, error) {
	if flow.status != StrictClarificationComplete {
		return PendingStrictDraft{}, fmt.Errorf("cannot generate draft: clarification is %s; missing decisions: %s", flow.status, strings.Join(flow.MissingDecisions(), ", "))
	}
	binding := newStrictDraftBinding(flow.request, hardSpec, feature)
	artifacts, err := GenerateNamespacedWorkflowPolicyArtifacts(repoRoot, WorkflowPolicyArtifactRequest{ContractID: flow.request.ContractID, HardSpec: hardSpec, Feature: feature})
	if err != nil {
		var incomplete *WorkflowArtifactPairIncompleteError
		if errors.As(err, &incomplete) {
			return pendingStrictDraft(incomplete.Artifacts, binding, incomplete.Status, incomplete), err
		}
		return PendingStrictDraft{}, err
	}
	return pendingStrictDraft(artifacts, binding, WorkflowArtifactPairPublicationStatus{Spec: WorkflowArtifactPublished, Feature: WorkflowArtifactPublished}, nil), nil
}

// RecoverPendingDraft is an explicit, later recovery action. It never retries
// automatically and can publish only the member that the recorded incomplete
// result says this invocation did not publish.
func (flow *StrictClarificationFlow) RecoverPendingDraft(repoRoot string, partial PendingStrictDraft, hardSpec, feature string) (PendingStrictDraft, error) {
	expectedBinding := newStrictDraftBinding(flow.request, hardSpec, feature)
	if flow.status != StrictClarificationComplete || partial.Complete() || partial.partial == nil || partial.Binding != expectedBinding || partial.WorkflowPolicyArtifacts != partial.partial.Artifacts || partial.Publication != partial.partial.Status || !isRecoverableStrictDraftStatus(partial.Publication) || partial.partial.retainedID == nil {
		return partial, fmt.Errorf("strict draft recovery binding or state mismatch")
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return partial, fmt.Errorf("open workflow artifact root for recovery: %w", err)
	}
	defer root.Close()

	retainedPath, retainedContent, missingPath, missingContent := strictDraftRecoveryMembers(partial, hardSpec, feature)
	if err := validateRetainedStrictDraftArtifact(root, retainedPath, retainedContent, partial.partial.retainedID); err != nil {
		return partial, err
	}
	if err := requireMissingWorkflowArtifact(root, missingPath); err != nil {
		return partial, err
	}

	staged, err := stageWorkflowArtifact(root, missingPath, missingContent)
	if err != nil {
		return partial, err
	}
	defer func() {
		if staged != "" {
			_ = removeStagedWorkflowArtifact(root, staged)
		}
	}()
	// Staging can take enough time for another actor to replace the retained
	// pathname. Revalidate immediately before the only recovery publication so
	// an incomplete result never authorizes a counterpart for a replaced file.
	if err := validateRetainedStrictDraftArtifact(root, retainedPath, retainedContent, partial.partial.retainedID); err != nil {
		return partial, err
	}
	if err := publishWorkflowArtifact(root, staged, missingPath); err != nil {
		return partial, &WorkflowArtifactPairIncompleteError{Artifacts: partial.WorkflowPolicyArtifacts, Status: partial.Publication, cause: err, retainedID: partial.partial.retainedID}
	}
	staged = ""
	return pendingStrictDraft(partial.WorkflowPolicyArtifacts, partial.Binding, WorkflowArtifactPairPublicationStatus{Spec: WorkflowArtifactPublished, Feature: WorkflowArtifactPublished}, nil), nil
}

func pendingStrictDraft(artifacts WorkflowPolicyArtifacts, binding StrictDraftBinding, publication WorkflowArtifactPairPublicationStatus, partial *WorkflowArtifactPairIncompleteError) PendingStrictDraft {
	return PendingStrictDraft{WorkflowPolicyArtifacts: artifacts, RequiresExplicitApproval: true, ApprovalScope: ContractScope{SpecPath: artifacts.SpecPath, FeaturePath: artifacts.FeaturePath}, Binding: binding, Publication: publication, partial: partial}
}

func newStrictDraftBinding(request StrictClarificationRequest, hardSpec, feature string) StrictDraftBinding {
	hardSpecSum, featureSum := strictDraftDigest(hardSpec), strictDraftDigest(feature)
	draftID := strictDraftDigest(strings.Join([]string{request.ContractID, request.SessionID, hardSpecSum, featureSum}, "\x00"))
	return StrictDraftBinding{DraftID: draftID, ContractID: request.ContractID, SessionID: request.SessionID, HardSpecSum: hardSpecSum, FeatureSum: featureSum}
}

func strictDraftDigest(content string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
}

func isRecoverableStrictDraftStatus(status WorkflowArtifactPairPublicationStatus) bool {
	return (status.Spec == WorkflowArtifactPublished && status.Feature == WorkflowArtifactNotPublished) || (status.Feature == WorkflowArtifactPublished && status.Spec == WorkflowArtifactNotPublished)
}

func strictDraftRecoveryMembers(partial PendingStrictDraft, hardSpec, feature string) (retainedPath, retainedContent, missingPath, missingContent string) {
	if partial.Publication.Spec == WorkflowArtifactPublished {
		return partial.SpecPath, hardSpec, partial.FeaturePath, feature
	}
	return partial.FeaturePath, feature, partial.SpecPath, hardSpec
}

func validateRetainedStrictDraftArtifactAtPath(root *os.Root, path, expected string, recorded os.FileInfo) error {
	file, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return fmt.Errorf("strict draft recovery retained artifact mismatch: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat retained strict draft artifact: %w", err)
	}
	if !os.SameFile(recorded, info) {
		return fmt.Errorf("strict draft recovery retained artifact identity mismatch")
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read retained strict draft artifact: %w", err)
	}
	if string(content) != expected {
		return fmt.Errorf("strict draft recovery retained artifact content mismatch")
	}
	return nil
}

func requireMissingWorkflowArtifact(root *os.Root, path string) error {
	file, err := root.Open(filepath.FromSlash(path))
	if err == nil {
		_ = file.Close()
		return fmt.Errorf("strict draft recovery collision at %s", path)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("check strict draft recovery target %s: %w", path, err)
	}
	return nil
}

func (flow *StrictClarificationFlow) nextRequired() *strictClarificationField {
	fields := flow.derivedFields()
	for index := range fields {
		field := &fields[index]
		if _, answered := flow.answers[field.key]; !answered {
			return field
		}
	}
	return nil
}

func (flow *StrictClarificationFlow) stop() {
	flow.displayed, flow.currentKey = nil, ""
	if flow.status != StrictClarificationComplete && flow.status != StrictClarificationInvalidated {
		flow.status = StrictClarificationStopped
	}
}

// derivedFields is the sole question plan. It accepts no caller-provided
// fields: the immutable initial request permits the flow, while only accepted
// answers choose the next derived question wording.
func (flow *StrictClarificationFlow) derivedFields() []strictClarificationField {
	scope := strictClarificationField{
		key: "scope", question: fmt.Sprintf("What delivery scope is intended for %q?", flow.request.InitialRequest), header: "Scope",
		options: []QuestionOption{{Label: "Bounded", Description: "Deliver only the smallest stated scope."}, {Label: "Broader", Description: "Include the directly related scope."}},
	}
	fields := []strictClarificationField{scope}
	if scopeAnswer, accepted := flow.answers[scope.key]; accepted {
		risk := strictClarificationField{
			key: "risk", question: fmt.Sprintf("Which risk boundary applies to the selected %q scope?", scopeAnswer), header: "Risk boundary",
			options: []QuestionOption{{Label: "Low", Description: "Use the lowest-risk boundary."}, {Label: "Elevated", Description: "Treat the request as elevated risk."}},
		}
		fields = append(fields, risk)
		if riskAnswer, accepted := flow.answers[risk.key]; accepted {
			fields = append(fields, strictClarificationField{
				key: "priority", question: fmt.Sprintf("What priority applies to the selected %q scope and %q risk boundary?", scopeAnswer, riskAnswer), header: "Priority",
				options: []QuestionOption{{Label: "Correctness", Description: "Prioritize correctness and verification."}, {Label: "Speed", Description: "Prioritize the shortest safe delivery path."}},
			})
		}
	}
	return fields
}

func cloneSequentialDraftQuestion(question NativeQuestion) NativeQuestion {
	question.Options = append([]QuestionOption(nil), question.Options...)
	return question
}
