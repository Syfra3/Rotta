package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictClarificationFlowSequencesCurrentQuestionsAndCreatesPendingDraft(t *testing.T) {
	flow := newTestClarificationFlow(t)
	first := flow.CurrentQuestion("prompt-1")
	if first == nil || first.Question != `What delivery scope is intended for "Build a Strict-bound capability."?` || first.Custom || len(first.Options) != 3 {
		t.Fatalf("first question = %#v, want one current custom=false question with stop option", first)
	}
	if flow.CurrentQuestion("prompt-blocked") != nil {
		t.Fatal("flow displayed more than one current question")
	}
	answerClarification(t, flow, first, "Bounded", "prompt-1")
	second := flow.CurrentQuestion("prompt-2")
	if second == nil || !strings.Contains(second.Question, `selected "Bounded" scope`) || second.Context.PromptID != "prompt-2" {
		t.Fatalf("second question = %#v, want only request plus accepted-answer progression", second)
	}
	answerClarification(t, flow, second, "Low", "prompt-2")
	third := flow.CurrentQuestion("prompt-3")
	if third == nil || !strings.Contains(third.Question, `selected "Bounded" scope and "Low" risk boundary`) {
		t.Fatalf("third question = %#v, want question derived from accepted active-flow answers", third)
	}
	answerClarification(t, flow, third, "Correctness", "prompt-3")
	if flow.Status() != StrictClarificationComplete || flow.Answers()["scope"] != "Bounded" || flow.Answers()["risk"] != "Low" || flow.Answers()["priority"] != "Correctness" {
		t.Fatalf("flow = status %q answers %#v, want complete active-flow evidence", flow.Status(), flow.Answers())
	}

	repo := t.TempDir()
	draft, err := flow.GeneratePendingDraft(repo, "# Pending\n", "Feature: Pending\n")
	if err != nil {
		t.Fatalf("generate pending draft: %v", err)
	}
	if draft.SpecPath != "specs/clarification-test.md" || draft.FeaturePath != "features/clarification-test.feature" || !draft.RequiresExplicitApproval || draft.ApprovalScope.SpecPath != draft.SpecPath || draft.ApprovalScope.FeaturePath != draft.FeaturePath {
		t.Fatalf("draft = %#v, want namespaced non-authorizing artifacts", draft)
	}
	for _, path := range []string{draft.SpecPath, draft.FeaturePath} {
		if _, err := os.Stat(filepath.Join(repo, path)); err != nil {
			t.Fatalf("pending artifact %s: %v", path, err)
		}
	}
	if decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: draft.SpecPath, FeaturePath: draft.FeaturePath, ScenarioID: "SCN-001"}); err != nil || decision.Approved {
		t.Fatalf("draft generation gate = %#v, %v; want separate explicit approval", decision, err)
	}
}

func TestStrictClarificationFlowStopsWithoutArtifacts(t *testing.T) {
	for _, test := range []struct {
		name string
		stop func(t *testing.T, flow *StrictClarificationFlow, question *NativeQuestion)
	}{
		{name: "explicit stop", stop: func(t *testing.T, flow *StrictClarificationFlow, question *NativeQuestion) {
			answerClarification(t, flow, question, StrictClarificationStopOption, question.Context.PromptID)
		}},
		{name: "dismissal", stop: func(_ *testing.T, flow *StrictClarificationFlow, question *NativeQuestion) {
			flow.ConsumeReply(OpenCodeQuestionReply{}, question.Context)
		}},
		{name: "unavailable", stop: func(_ *testing.T, flow *StrictClarificationFlow, _ *NativeQuestion) { flow.Unavailable() }},
		{name: "invalid", stop: func(_ *testing.T, flow *StrictClarificationFlow, question *NativeQuestion) {
			flow.ConsumeReply(OpenCodeQuestionReply{Answers: [][]string{{"not an option"}}}, question.Context)
		}},
		{name: "stale", stop: func(_ *testing.T, flow *StrictClarificationFlow, question *NativeQuestion) {
			flow.ConsumeReply(OpenCodeQuestionReply{Answers: [][]string{{"Scoped"}}}, QuestionContext{PromptID: "old", SessionID: question.Context.SessionID, Workspace: question.Context.Workspace, Action: question.Context.Action})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			flow := newTestClarificationFlow(t)
			question := flow.CurrentQuestion("prompt-1")
			test.stop(t, flow, question)
			if flow.Status() != StrictClarificationStopped || len(flow.MissingDecisions()) == 0 {
				t.Fatalf("status %q missing %v, want safe stopped flow", flow.Status(), flow.MissingDecisions())
			}
			if proposal := flow.SafeStopProposal(); proposal.ProposedScope == "" || len(proposal.MissingDecisions) == 0 {
				t.Fatalf("safe stop proposal = %#v, want compact scope and missing decisions", proposal)
			}
			if _, err := flow.GeneratePendingDraft(t.TempDir(), "spec", "feature"); err == nil {
				t.Fatal("incomplete flow generated durable artifacts")
			}
		})
	}
}

func TestStrictClarificationFlowCapsAndInvalidatesRevisions(t *testing.T) {
	flow := newTestClarificationFlow(t)
	for _, label := range []string{"Bounded", "Low", "Correctness"} {
		question := flow.CurrentQuestion("prompt-" + label)
		answerClarification(t, flow, question, label, question.Context.PromptID)
	}
	if flow.Status() != StrictClarificationComplete || flow.CurrentQuestion("prompt-four") != nil || len(flow.Answers()) != maxStrictClarificationQuestions {
		t.Fatalf("capped flow = status %q answers %#v, want exactly three questions", flow.Status(), flow.Answers())
	}

	flow = newTestClarificationFlow(t)
	question := flow.CurrentQuestion("prompt-1")
	answerClarification(t, flow, question, "Bounded", "prompt-1")
	flow.InvalidateForRevision()
	if flow.Status() != StrictClarificationInvalidated || len(flow.Answers()) != 0 || flow.CurrentQuestion("prompt-2") != nil {
		t.Fatalf("revised flow = status %q answers %#v, want invalidated flow requiring a new start", flow.Status(), flow.Answers())
	}
}

func TestStrictClarificationFlowRejectsUnsafeContractID(t *testing.T) {
	flow, err := NewStrictClarificationFlow(StrictClarificationRequest{
		ContractID: "../outside", InitialRequest: "Strict request", SessionID: "session", Workspace: "/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	question := flow.CurrentQuestion("prompt")
	answerClarification(t, flow, question, "Bounded", "prompt")
	question = flow.CurrentQuestion("prompt-2")
	answerClarification(t, flow, question, "Low", "prompt-2")
	question = flow.CurrentQuestion("prompt-3")
	answerClarification(t, flow, question, "Correctness", "prompt-3")
	if _, err := flow.GeneratePendingDraft(t.TempDir(), "spec", "feature"); err == nil || !strings.Contains(err.Error(), "invalid namespaced contract id") {
		t.Fatalf("unsafe contract id error = %v", err)
	}
}

func TestStrictClarificationFlowDerivesQuestionsOnlyFromImmutableRequestAndAcceptedAnswers(t *testing.T) {
	request := StrictClarificationRequest{ContractID: "clarification-test", InitialRequest: "Build a Strict-bound capability.", SessionID: "session-1", Workspace: "/workspace"}
	flow, err := NewStrictClarificationFlow(request)
	if err != nil {
		t.Fatal(err)
	}
	request.SessionID, request.Workspace, request.InitialRequest = "replaced", "/replaced", "replaced request"
	first := flow.CurrentQuestion("prompt-1")
	if first.Context.SessionID != "session-1" || first.Context.Workspace != "/workspace" || first.Question != `What delivery scope is intended for "Build a Strict-bound capability."?` || strings.Contains(first.Question, "replaced request") {
		t.Fatalf("first question = %#v, want immutable request-derived question", first)
	}
	answerClarification(t, flow, first, "Bounded", "prompt-1")
	answers := flow.Answers()
	answers["scope"] = "forged caller value"
	second := flow.CurrentQuestion("prompt-2")
	if second == nil || !strings.Contains(second.Question, `selected "Bounded" scope`) || strings.Contains(second.Question, "forged caller value") {
		t.Fatalf("second question = %#v, want only accepted active-flow answer derivation", second)
	}
}

func TestStrictClarificationFlowStopsOnMismatchedOrReplacedQuestion(t *testing.T) {
	for _, test := range []struct {
		name string
		stop func(flow *StrictClarificationFlow, question *NativeQuestion)
	}{
		{name: "mismatched context", stop: func(flow *StrictClarificationFlow, question *NativeQuestion) {
			flow.ConsumeReply(OpenCodeQuestionReply{Answers: [][]string{{"Bounded"}}}, QuestionContext{PromptID: question.Context.PromptID, SessionID: "other-session", Workspace: question.Context.Workspace, Action: question.Context.Action})
		}},
		{name: "replaced question", stop: func(flow *StrictClarificationFlow, question *NativeQuestion) {
			flow.ReplaceCurrentQuestion()
			flow.ConsumeReply(OpenCodeQuestionReply{Answers: [][]string{{"Bounded"}}}, question.Context)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			flow := newTestClarificationFlow(t)
			question := flow.CurrentQuestion("prompt-1")
			test.stop(flow, question)
			if flow.Status() != StrictClarificationStopped || len(flow.Answers()) != 0 || flow.CurrentQuestion("prompt-2") != nil {
				t.Fatalf("flow = status %q answers %#v, want safe stopped flow", flow.Status(), flow.Answers())
			}
		})
	}
}

func TestStrictClarificationFlowIncompletePairIsNonAuthorizingAndRecoverable(t *testing.T) {
	flow, repo, partial, err := newIncompletePendingDraft(t)
	var incomplete *WorkflowArtifactPairIncompleteError
	if !errors.As(err, &incomplete) || incomplete.Artifacts != partial.WorkflowPolicyArtifacts || incomplete.Status != partial.Publication {
		t.Fatalf("expected typed incomplete pair status bound to returned partial, got draft %#v error %v", partial, err)
	}
	if partial.Complete() || !partial.RequiresExplicitApproval || partial.Publication.Complete() {
		t.Fatalf("retained partial must not be a completed or authorizing draft: %#v", partial)
	}
	if decision, gateErr := EvaluateImplementationGate(repo, ContractScope{SpecPath: partial.SpecPath, FeaturePath: partial.FeaturePath, ScenarioID: "SCN-001"}); gateErr != nil || decision.Approved {
		t.Fatalf("retained partial gate = %#v, %v; want no implementation authority", decision, gateErr)
	}
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))

	recovered, recoverErr := flow.RecoverPendingDraft(repo, partial, "# Pending\n", "Feature: Pending\n")
	if recoverErr != nil || !recovered.Complete() || !recovered.Publication.Complete() {
		t.Fatalf("explicit validated recovery = %#v, %v; want completed pair", recovered, recoverErr)
	}
	assertFileContent(t, filepath.Join(repo, recovered.SpecPath), "# Pending\n")
	assertFileContent(t, filepath.Join(repo, recovered.FeaturePath), "Feature: Pending\n")
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))
}

func TestStrictClarificationFlowRecoveryStopsWithoutChangingArtifacts(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(t *testing.T, repo string, partial *PendingStrictDraft)
		spec    string
		feature string
	}{
		{name: "revised content binding", spec: "# Revised\n", feature: "Feature: Pending\n"},
		{name: "stale session binding", spec: "# Pending\n", feature: "Feature: Pending\n", mutate: func(_ *testing.T, _ string, partial *PendingStrictDraft) {
			partial.Binding.SessionID = "stale-session"
		}},
		{name: "invalid completed status", spec: "# Pending\n", feature: "Feature: Pending\n", mutate: func(_ *testing.T, _ string, partial *PendingStrictDraft) {
			partial.Publication = WorkflowArtifactPairPublicationStatus{Spec: WorkflowArtifactPublished, Feature: WorkflowArtifactPublished}
		}},
		{name: "missing target collision", spec: "# Pending\n", feature: "Feature: Pending\n", mutate: func(t *testing.T, repo string, partial *PendingStrictDraft) {
			mustWrite(t, filepath.Join(repo, partial.FeaturePath), "Feature: foreign\n")
		}},
		{name: "foreign retained replacement", spec: "# Pending\n", feature: "Feature: Pending\n", mutate: func(t *testing.T, repo string, partial *PendingStrictDraft) {
			if err := os.Remove(filepath.Join(repo, partial.SpecPath)); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(repo, partial.SpecPath), "# Pending\n")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			flow, repo, partial, _ := newIncompletePendingDraft(t)
			if test.mutate != nil {
				test.mutate(t, repo, &partial)
			}
			beforeSpec, beforeFeature := optionalWorkflowArtifact(t, repo, partial.SpecPath), optionalWorkflowArtifact(t, repo, partial.FeaturePath)
			recovered, err := flow.RecoverPendingDraft(repo, partial, test.spec, test.feature)
			if err == nil || recovered.Complete() {
				t.Fatalf("recovery should safely stop, got %#v, %v", recovered, err)
			}
			if got := optionalWorkflowArtifact(t, repo, partial.SpecPath); got != beforeSpec {
				t.Fatalf("spec changed: got %q want %q", got, beforeSpec)
			}
			if got := optionalWorkflowArtifact(t, repo, partial.FeaturePath); got != beforeFeature {
				t.Fatalf("feature changed: got %q want %q", got, beforeFeature)
			}
			assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
			assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))
		})
	}
}

func TestStrictClarificationFlowRecoveryRejectsTargetCreatedDuringPublication(t *testing.T) {
	flow, repo, partial, _ := newIncompletePendingDraft(t)
	previousPublish := publishWorkflowArtifact
	publishWorkflowArtifact = func(root *os.Root, stagedPath, artifactPath string) error {
		if artifactPath == filepath.FromSlash(partial.FeaturePath) {
			mustWrite(t, filepath.Join(repo, partial.FeaturePath), "Feature: foreign concurrent target\n")
		}
		return previousPublish(root, stagedPath, artifactPath)
	}
	t.Cleanup(func() { publishWorkflowArtifact = previousPublish })

	recovered, err := flow.RecoverPendingDraft(repo, partial, "# Pending\n", "Feature: Pending\n")
	if err == nil || recovered.Complete() || !errors.Is(err, errWorkflowArtifactCollision) {
		t.Fatalf("concurrent recovery collision = %#v, %v; want retained incomplete collision", recovered, err)
	}
	assertFileContent(t, filepath.Join(repo, partial.SpecPath), "# Pending\n")
	assertFileContent(t, filepath.Join(repo, partial.FeaturePath), "Feature: foreign concurrent target\n")
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))
}

func TestStrictClarificationFlowRecoveryStopsWhenRetainedArtifactIsReplacedBeforePublication(t *testing.T) {
	flow, repo, partial, _ := newIncompletePendingDraft(t)
	previousValidate := validateRetainedStrictDraftArtifact
	validations := 0
	validateRetainedStrictDraftArtifact = func(root *os.Root, path, expected string, recorded os.FileInfo) error {
		validations++
		if validations == 2 {
			if err := os.Remove(filepath.Join(repo, partial.SpecPath)); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(repo, partial.SpecPath), "# Foreign replacement\n")
		}
		return previousValidate(root, path, expected, recorded)
	}
	t.Cleanup(func() { validateRetainedStrictDraftArtifact = previousValidate })

	previousPublish := publishWorkflowArtifact
	published := false
	publishWorkflowArtifact = func(root *os.Root, stagedPath, artifactPath string) error {
		published = true
		return previousPublish(root, stagedPath, artifactPath)
	}
	t.Cleanup(func() { publishWorkflowArtifact = previousPublish })

	recovered, err := flow.RecoverPendingDraft(repo, partial, "# Pending\n", "Feature: Pending\n")
	if err == nil || recovered.Complete() || published {
		t.Fatalf("recovery after retained replacement = %#v, %v, published=%t; want safe incomplete stop before publication", recovered, err, published)
	}
	assertFileContent(t, filepath.Join(repo, partial.SpecPath), "# Foreign replacement\n")
	if _, err := os.Stat(filepath.Join(repo, partial.FeaturePath)); !os.IsNotExist(err) {
		t.Fatalf("missing counterpart exists or could not be checked: %v", err)
	}
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))
}

func TestStrictClarificationFlowRecoveryReportsConflictWhenRetainedArtifactIsReplacedDuringPublication(t *testing.T) {
	flow, repo, partial, _ := newIncompletePendingDraft(t)
	previousPublish := publishWorkflowArtifact
	publishWorkflowArtifact = func(root *os.Root, stagedPath, artifactPath string) error {
		if artifactPath == filepath.FromSlash(partial.FeaturePath) {
			if err := os.Remove(filepath.Join(repo, partial.SpecPath)); err != nil {
				t.Fatalf("replace retained artifact: %v", err)
			}
			mustWrite(t, filepath.Join(repo, partial.SpecPath), "# Foreign replacement\n")
		}
		return previousPublish(root, stagedPath, artifactPath)
	}
	t.Cleanup(func() { publishWorkflowArtifact = previousPublish })

	recovered, err := flow.RecoverPendingDraft(repo, partial, "# Pending\n", "Feature: Pending\n")
	var incomplete *WorkflowArtifactPairIncompleteError
	if !errors.As(err, &incomplete) || !errors.Is(err, errWorkflowArtifactPairConflict) || recovered.Complete() {
		t.Fatalf("recovery after post-validation replacement = %#v, %v; want typed incomplete conflict", recovered, err)
	}
	if recovered.Publication != partial.Publication || incomplete.Status != partial.Publication {
		t.Fatalf("conflict status = recovered %#v incomplete %#v, want original incomplete status", recovered.Publication, incomplete.Status)
	}
	assertFileContent(t, filepath.Join(repo, partial.SpecPath), "# Foreign replacement\n")
	assertFileContent(t, filepath.Join(repo, partial.FeaturePath), "Feature: Pending\n")
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "specs"))
	assertNoStagedWorkflowArtifacts(t, filepath.Join(repo, "features"))
}

func newIncompletePendingDraft(t *testing.T) (*StrictClarificationFlow, string, PendingStrictDraft, error) {
	t.Helper()
	flow := newTestClarificationFlow(t)
	for _, label := range []string{"Bounded", "Low", "Correctness"} {
		question := flow.CurrentQuestion("prompt-" + label)
		answerClarification(t, flow, question, label, question.Context.PromptID)
	}
	repo := t.TempDir()
	originalPublish, calls := publishWorkflowArtifact, 0
	publishWorkflowArtifact = func(root *os.Root, stagedPath, artifactPath string) error {
		calls++
		if calls == 2 {
			return os.ErrPermission
		}
		return originalPublish(root, stagedPath, artifactPath)
	}
	t.Cleanup(func() { publishWorkflowArtifact = originalPublish })
	partial, err := flow.GeneratePendingDraft(repo, "# Pending\n", "Feature: Pending\n")
	return flow, repo, partial, err
}

func optionalWorkflowArtifact(t *testing.T, repo, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repo, path))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func newTestClarificationFlow(t *testing.T) *StrictClarificationFlow {
	t.Helper()
	flow, err := NewStrictClarificationFlow(StrictClarificationRequest{ContractID: "clarification-test", InitialRequest: "Build a Strict-bound capability.", SessionID: "session-1", Workspace: "/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func answerClarification(t *testing.T, flow *StrictClarificationFlow, question *NativeQuestion, label, promptID string) {
	t.Helper()
	flow.ConsumeReply(OpenCodeQuestionReply{Answers: [][]string{{label}}}, QuestionContext{PromptID: promptID, SessionID: question.Context.SessionID, Workspace: question.Context.Workspace, Action: question.Context.Action})
}
