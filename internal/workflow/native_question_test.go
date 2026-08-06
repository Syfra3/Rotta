package workflow

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarshalOpenCodeQuestionRequestUsesNativeSingleSelectSchema(t *testing.T) {
	question, err := NewVelaStaleGraphQuestion(QuestionContext{
		PromptID: "prompt-vela", SessionID: "session-vela", Workspace: "/canonical/project", Action: VelaStaleGraphAction,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := MarshalOpenCodeQuestionRequest(question)
	if err != nil {
		t.Fatalf("marshal OpenCode question request: %v", err)
	}
	var request OpenCodeQuestionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("decode emitted request: %v", err)
	}
	if len(request.Questions) != 1 {
		t.Fatalf("questions = %#v, want exactly one native question", request.Questions)
	}
	emitted := request.Questions[0]
	if emitted.Question != question.Question || emitted.Header != question.Header || emitted.Multiple || emitted.Custom {
		t.Fatalf("emitted question = %#v, want matching single-select custom=false question", emitted)
	}
	if len(emitted.Options) != len(question.Options) || emitted.Options[1] != question.Options[1] {
		t.Fatalf("emitted options = %#v, want %#v", emitted.Options, question.Options)
	}
	if strings.Contains(string(payload), "PromptID") || strings.Contains(string(payload), "Purpose") {
		t.Fatalf("OpenCode payload leaked local context: %s", payload)
	}
}

func TestConsumeOpenCodeQuestionReplyBindsNestedAnswerAndRejectsUnsafeReplies(t *testing.T) {
	question := testMaterialPolicyQuestion(t, "prompt-1", "Approve")

	var reply OpenCodeQuestionReply
	if err := json.Unmarshal([]byte(`{"answers":[["Approve"]]}`), &reply); err != nil {
		t.Fatal(err)
	}
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ConsumeOpenCodeQuestionReply(displayed, reply, question.Context)
	if err != nil || result.Selected != "Approve" {
		t.Fatalf("consume nested answer = %#v, %v; want bound Approve selection", result, err)
	}

	for _, test := range []struct {
		name    string
		reply   OpenCodeQuestionReply
		current QuestionContext
		want    string
	}{
		{name: "no answer field", reply: OpenCodeQuestionReply{}, current: question.Context, want: "no single"},
		{name: "missing answer", reply: OpenCodeQuestionReply{Answers: [][]string{}}, current: question.Context, want: "no single"},
		{name: "dismissed answer", reply: OpenCodeQuestionReply{Answers: [][]string{{}}}, current: question.Context, want: "no single"},
		{name: "multiple selections", reply: OpenCodeQuestionReply{Answers: [][]string{{"Approve", MaterialPolicyStopOption}}}, current: question.Context, want: "no single"},
		{name: "custom answer", reply: OpenCodeQuestionReply{Answers: [][]string{{"Other"}}}, current: question.Context, want: "option"},
		{name: "current context mismatch", reply: OpenCodeQuestionReply{Answers: [][]string{{"Approve"}}}, current: QuestionContext{PromptID: "prompt-1", SessionID: "other", Workspace: "/workspace", Action: MaterialPolicyDecisionAction}, want: "session"},
	} {
		t.Run(test.name, func(t *testing.T) {
			displayed, err := NewDisplayedNativeQuestion(question)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ConsumeOpenCodeQuestionReply(displayed, test.reply, test.current); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ConsumeOpenCodeQuestionReply() error = %v, want %q rejection", err, test.want)
			}
		})
	}
}

func TestNativeQuestionRequiresBoundCurrentSelectableAnswer(t *testing.T) {
	question := testMaterialPolicyQuestion(t, "prompt-1", "Approve")
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatalf("display question: %v", err)
	}

	result, err := displayed.Consume(NativeQuestionAnswer{
		Context: question.Context,
		Label:   "Approve",
	}, question.Context)
	if err != nil {
		t.Fatalf("consume current selected answer: %v", err)
	}
	if result.Selected != "Approve" || result.PendingAction != nil {
		t.Fatalf("result = %#v, want decision evidence without an action", result)
	}
	if _, err := displayed.Consume(NativeQuestionAnswer{Context: question.Context, Label: "Approve"}, question.Context); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("reused answer error = %v, want closed question rejection", err)
	}
}

func TestNativeQuestionRejectsStaleReplacedMismatchedAndInvalidAnswers(t *testing.T) {
	question := testMaterialPolicyQuestion(t, "prompt-1", "Keep")
	for _, test := range []struct {
		name    string
		answer  NativeQuestionAnswer
		current QuestionContext
		replace bool
		want    string
	}{
		{name: "stale prompt", answer: NativeQuestionAnswer{Context: QuestionContext{PromptID: "old", SessionID: "session-1", Workspace: "/workspace", Action: MaterialPolicyDecisionAction}, Label: "Keep"}, current: question.Context, want: "stale"},
		{name: "mismatched session", answer: NativeQuestionAnswer{Context: QuestionContext{PromptID: "prompt-1", SessionID: "other", Workspace: "/workspace", Action: MaterialPolicyDecisionAction}, Label: "Keep"}, current: question.Context, want: "session"},
		{name: "mismatched workspace", answer: NativeQuestionAnswer{Context: QuestionContext{PromptID: "prompt-1", SessionID: "session-1", Workspace: "/other", Action: MaterialPolicyDecisionAction}, Label: "Keep"}, current: question.Context, want: "workspace"},
		{name: "mismatched action", answer: NativeQuestionAnswer{Context: QuestionContext{PromptID: "prompt-1", SessionID: "session-1", Workspace: "/workspace", Action: "other"}, Label: "Keep"}, current: question.Context, want: "action"},
		{name: "custom answer", answer: NativeQuestionAnswer{Context: question.Context, Label: "Other"}, current: question.Context, want: "option"},
		{name: "replaced prompt", answer: NativeQuestionAnswer{Context: question.Context, Label: "Keep"}, current: question.Context, replace: true, want: "replaced"},
	} {
		t.Run(test.name, func(t *testing.T) {
			displayed, err := NewDisplayedNativeQuestion(question)
			if err != nil {
				t.Fatal(err)
			}
			if test.replace {
				displayed.Replace()
			}
			if _, err := displayed.Consume(test.answer, test.current); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Consume() error = %v, want %q rejection", err, test.want)
			}
		})
	}
}

func TestVelaStaleGraphQuestionCreatesOnlyPendingReindexReview(t *testing.T) {
	root := filepath.Clean("/canonical/project")
	question, err := NewVelaStaleGraphQuestion(QuestionContext{PromptID: "prompt-vela", SessionID: "session-vela", Workspace: root, Action: VelaStaleGraphAction})
	if err != nil {
		t.Fatalf("new Vela question: %v", err)
	}
	if question.Header == "" || len([]rune(question.Header)) > 30 || question.Custom {
		t.Fatalf("Vela consent schema = %#v, want short header and explicit custom=false", question)
	}
	if got := question.Options; len(got) != 3 || got[0].Label != VelaUseSourceFallback || got[1].Label != VelaRequestReindexReview || got[2].Label != VelaStopAndRevisit {
		t.Fatalf("Vela options = %#v", got)
	}
	for _, option := range question.Options {
		if option.Label == "" || option.Description == "" {
			t.Fatalf("invalid option shape: %#v", option)
		}
	}

	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ConsumeVelaStaleGraphAnswer(displayed, NativeQuestionAnswer{Context: question.Context, Label: VelaRequestReindexReview}, question.Context)
	if err != nil {
		t.Fatalf("select re-index review: %v", err)
	}
	if result.PendingAction == nil || result.PendingAction.Kind != VelaReindexReviewAction || result.PendingAction.ProjectRoot != root || !result.PendingAction.RequiresExplicitAuthorization {
		t.Fatalf("pending result = %#v", result)
	}
	if result.PendingAction.Executed {
		t.Fatalf("pending action executed unexpectedly: %#v", result.PendingAction)
	}

	fallback, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ConsumeVelaStaleGraphAnswer(fallback, NativeQuestionAnswer{Context: question.Context, Label: VelaUseSourceFallback}, question.Context)
	if err != nil {
		t.Fatalf("select source fallback: %v", err)
	}
	if result.PendingAction != nil {
		t.Fatalf("source fallback created an action: %#v", result)
	}

	stopped, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ConsumeVelaStaleGraphAnswer(stopped, NativeQuestionAnswer{Context: question.Context, Label: VelaStopAndRevisit}, question.Context)
	if err != nil {
		t.Fatalf("select stop: %v", err)
	}
	if result.PendingAction != nil {
		t.Fatalf("stop created an action: %#v", result)
	}
}

func TestNativeQuestionRejectsInvalidOpenCodeSchemaShape(t *testing.T) {
	_, err := NewDisplayedNativeQuestion(NativeQuestion{
		Context: QuestionContext{PromptID: "p", SessionID: "s", Workspace: "/w", Action: VelaStaleGraphAction}, Trigger: VelaStaleEvidenceTrigger,
		Question: "q", Header: strings.Repeat("x", 31),
		Options: []QuestionOption{{Label: "", Description: "missing label"}},
	})
	if err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("invalid question error = %v, want header validation", err)
	}
}

func TestNativeQuestionRejectsCustomConsent(t *testing.T) {
	_, err := NewDisplayedNativeQuestion(NativeQuestion{
		Context: QuestionContext{PromptID: "p", SessionID: "s", Workspace: "/w", Action: VelaStaleGraphAction}, Trigger: VelaStaleEvidenceTrigger,
		Question: "Consent?", Header: "Consent",
		Options: []QuestionOption{{Label: "No", Description: "Do not consent."}},
		Custom:  true,
		Purpose: QuestionPurposeConsent,
	})
	if err == nil || !strings.Contains(err.Error(), "approved factory") {
		t.Fatalf("custom consent error = %v, want factory rejection", err)
	}
}

func TestExactStrictApprovalBindsRenderedContractAndProducesDecisionEvidenceOnly(t *testing.T) {
	approval := ExactStrictApprovalContext{
		ContractPath: ".rotta/strict/live-native-question-flow.md", ContentDigest: strings.Repeat("a", 64), RenderedRevision: "rev-7",
		SessionID: "session-1", Workspace: "/workspace", Action: ExactStrictApprovalAction,
	}
	context := QuestionContext{PromptID: "approval-1", SessionID: approval.SessionID, Workspace: approval.Workspace, Action: approval.Action}
	question, err := NewExactStrictApprovalQuestion(context, approval)
	if err != nil {
		t.Fatal(err)
	}
	if question.Custom || question.Approval == nil || len(question.Options) != 2 || question.Options[0].Label != ExactStrictApproveOption || question.Options[1].Label != ExactStrictStopOption {
		t.Fatalf("exact approval question = %#v, want explicit choices and no custom answer", question)
	}
	for _, want := range []string{approval.ContractPath, approval.ContentDigest, approval.RenderedRevision} {
		if !strings.Contains(question.Question+question.Options[0].Description, want) {
			t.Fatalf("exact approval display omits %q: %#v", want, question)
		}
	}
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ConsumeExactStrictApprovalReply(displayed, OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, approval)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected != ExactStrictApproveOption || result.PendingAction != nil || result.ExactApproval == nil || result.ExactApproval.Context != approval {
		t.Fatalf("exact approval result = %#v, want bound decision evidence without an action", result)
	}
}

func TestNativeQuestionRejectsUnapprovedOrGenericTriggerConstruction(t *testing.T) {
	base := NativeQuestion{
		Context: QuestionContext{PromptID: "p", SessionID: "s", Workspace: "/workspace", Action: MaterialPolicyDecisionAction},
		Trigger: MaterialPolicyDecisionTrigger, Question: "Choose the bounded policy.", Header: "Policy",
		Options: []QuestionOption{{Label: "Keep", Description: "Keep the safe default."}, {Label: MaterialPolicyStopOption, Description: "Stop safely."}},
		Purpose: QuestionPurposeDecision, SafeDefault: "Keep",
	}
	for _, test := range []struct {
		name string
		edit func(*NativeQuestion)
	}{
		{name: "missing trigger", edit: func(value *NativeQuestion) { value.Trigger = ""; value.Context.Action = "arbitrary" }},
		{name: "unknown trigger", edit: func(value *NativeQuestion) { value.Trigger = "generic-continuation" }},
		{name: "wrong action", edit: func(value *NativeQuestion) { value.Context.Action = "arbitrary" }},
		{name: "generic continuation", edit: func(value *NativeQuestion) { value.Question = "Continue after review?" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			question := base
			test.edit(&question)
			if _, err := NewDisplayedNativeQuestion(question); err == nil {
				t.Fatal("NewDisplayedNativeQuestion accepted an unapproved construction")
			}
			if _, err := MarshalOpenCodeQuestionRequest(question); err == nil {
				t.Fatal("MarshalOpenCodeQuestionRequest accepted an unapproved construction")
			}
		})
	}
}

func TestExactStrictApprovalRejectsUnsafeOrMismatchedAnswers(t *testing.T) {
	approval := ExactStrictApprovalContext{ContractPath: ".rotta/strict/contract.md", ContentDigest: strings.Repeat("b", 64), RenderedRevision: "rev-1", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}
	context := QuestionContext{PromptID: "approval", SessionID: approval.SessionID, Workspace: approval.Workspace, Action: approval.Action}
	for _, test := range []struct {
		name    string
		reply   OpenCodeQuestionReply
		current ExactStrictApprovalContext
		replace bool
		want    string
	}{
		{name: "absent", current: approval, want: "no single"},
		{name: "dismissed", reply: OpenCodeQuestionReply{Answers: [][]string{{}}}, current: approval, want: "no single"},
		{name: "invalid", reply: OpenCodeQuestionReply{Answers: [][]string{{"Other"}}}, current: approval, want: "option"},
		{name: "custom", reply: OpenCodeQuestionReply{Answers: [][]string{{"custom response"}}}, current: approval, want: "option"},
		{name: "replaced", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: approval, replace: true, want: "replaced"},
		{name: "stale revision", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: ExactStrictApprovalContext{ContractPath: approval.ContractPath, ContentDigest: approval.ContentDigest, RenderedRevision: "rev-2", SessionID: approval.SessionID, Workspace: approval.Workspace, Action: approval.Action}, want: "binding"},
		{name: "mismatched digest", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: ExactStrictApprovalContext{ContractPath: approval.ContractPath, ContentDigest: strings.Repeat("c", 64), RenderedRevision: approval.RenderedRevision, SessionID: approval.SessionID, Workspace: approval.Workspace, Action: approval.Action}, want: "binding"},
		{name: "mismatched session", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: ExactStrictApprovalContext{ContractPath: approval.ContractPath, ContentDigest: approval.ContentDigest, RenderedRevision: approval.RenderedRevision, SessionID: "other", Workspace: approval.Workspace, Action: approval.Action}, want: "binding"},
		{name: "mismatched workspace", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: ExactStrictApprovalContext{ContractPath: approval.ContractPath, ContentDigest: approval.ContentDigest, RenderedRevision: approval.RenderedRevision, SessionID: approval.SessionID, Workspace: "/other", Action: approval.Action}, want: "binding"},
		{name: "mismatched action", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactStrictApproveOption}}}, current: ExactStrictApprovalContext{ContractPath: approval.ContractPath, ContentDigest: approval.ContentDigest, RenderedRevision: approval.RenderedRevision, SessionID: approval.SessionID, Workspace: approval.Workspace, Action: "other"}, want: "incomplete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			question, err := NewExactStrictApprovalQuestion(context, approval)
			if err != nil {
				t.Fatal(err)
			}
			displayed, err := NewDisplayedNativeQuestion(question)
			if err != nil {
				t.Fatal(err)
			}
			if test.replace {
				displayed.Replace()
			}
			if _, err := ConsumeExactStrictApprovalReply(displayed, test.reply, test.current); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ConsumeExactStrictApprovalReply() error = %v, want %q rejection", err, test.want)
			}
		})
	}
}

func TestExactStrictApprovalRejectsAbsentOrNonCanonicalBindings(t *testing.T) {
	base := ExactStrictApprovalContext{ContractPath: ".rotta/strict/contract.md", ContentDigest: strings.Repeat("d", 64), RenderedRevision: "rev-1", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}
	context := QuestionContext{PromptID: "approval", SessionID: base.SessionID, Workspace: base.Workspace, Action: base.Action}
	for _, test := range []struct {
		name string
		edit func(*ExactStrictApprovalContext)
	}{
		{name: "absent path", edit: func(value *ExactStrictApprovalContext) { value.ContractPath = "" }},
		{name: "stale path", edit: func(value *ExactStrictApprovalContext) { value.ContractPath = ".rotta/strict/../other.md" }},
		{name: "absent digest", edit: func(value *ExactStrictApprovalContext) { value.ContentDigest = "" }},
		{name: "invalid digest", edit: func(value *ExactStrictApprovalContext) { value.ContentDigest = strings.Repeat("G", 64) }},
		{name: "absent revision", edit: func(value *ExactStrictApprovalContext) { value.RenderedRevision = "" }},
		{name: "absent session", edit: func(value *ExactStrictApprovalContext) { value.SessionID = "" }},
		{name: "absent workspace", edit: func(value *ExactStrictApprovalContext) { value.Workspace = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := base
			test.edit(&value)
			if _, err := NewExactStrictApprovalQuestion(context, value); err == nil {
				t.Fatal("invalid exact approval binding was accepted")
			}
		})
	}
}

func TestMaterialPolicyQuestionHasChoicesSafeDefaultAndNoContinuation(t *testing.T) {
	context := QuestionContext{PromptID: "policy-1", SessionID: "session", Workspace: "/workspace", Action: MaterialPolicyDecisionAction}
	question, err := NewMaterialPolicyDecisionQuestion(context, MaterialPolicyDecision{
		Question: "Which conflict policy applies to this bounded case?", Header: "Conflict policy",
		Alternatives: []QuestionOption{{Label: "Preserve", Description: "Keep the current policy; this is the safe default."}, {Label: "Replace", Description: "Use the replacement policy."}}, SafeDefault: "Preserve",
	})
	if err != nil || question.Custom || len(question.Options) != 3 || question.Options[2].Label != MaterialPolicyStopOption {
		t.Fatalf("policy question = %#v, %v; want explicit choices and safe stop", question, err)
	}
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err := displayed.Consume(NativeQuestionAnswer{Context: context, Label: MaterialPolicyStopOption}, context)
	if err != nil || result.PendingAction != nil {
		t.Fatalf("policy safe stop = %#v, %v; want decision evidence without an action", result, err)
	}
	if _, err := NewMaterialPolicyDecisionQuestion(context, MaterialPolicyDecision{Question: "Continue after review?", Header: "Continue", Alternatives: []QuestionOption{{Label: "Yes", Description: "continue"}}, SafeDefault: "Yes"}); err == nil || !strings.Contains(err.Error(), "continuation") {
		t.Fatalf("generic continuation error = %v", err)
	}
}

func TestExactOperationConsentBindsRenderedContextAndCreatesOnlyPendingAuthorization(t *testing.T) {
	consent := ExactOperationConsentContext{
		Action: "delete-external-preview", OperationClass: ExternalOperationClass, CanonicalTarget: "https://example.test/previews/42", SessionID: "session-1", Workspace: "/canonical/project",
		MaterialEffect: "Permanently deletes preview 42 from the external service.", ApprovalScope: "This one rendered deletion only.",
		ContentDigest: strings.Repeat("e", 64), RenderedRevision: "operation-rev-3",
	}
	context := QuestionContext{PromptID: "consent-1", SessionID: "session-1", Workspace: consent.Workspace, Action: consent.Action}
	question, err := NewExactOperationConsentQuestion(context, consent)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{consent.Action, consent.CanonicalTarget, consent.Workspace, consent.MaterialEffect, consent.ApprovalScope, consent.ContentDigest, consent.RenderedRevision, ExactOperationConsentOption} {
		if !strings.Contains(question.Question+question.Options[0].Label, want) {
			t.Fatalf("consent prompt does not render %q: %#v", want, question)
		}
	}
	if question.Custom || len(question.Options) != 2 || question.Options[0].Label != ExactOperationConsentOption {
		t.Fatalf("consent question = %#v, want bounded single-select consent", question)
	}
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ConsumeExactOperationConsentReply(displayed, OpenCodeQuestionReply{Answers: [][]string{{ExactOperationConsentOption}}}, consent)
	if err != nil {
		t.Fatal(err)
	}
	if result.PendingAction == nil || result.PendingAction.Consent == nil || *result.PendingAction.Consent != consent || result.PendingAction.Executed || !result.PendingAction.RequiresExplicitAuthorization {
		t.Fatalf("consent result = %#v, want unexecuted bounded pending authorization", result)
	}
	stopped, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ConsumeExactOperationConsentReply(stopped, OpenCodeQuestionReply{Answers: [][]string{{OperationConsentStopOption}}}, consent)
	if err != nil || result.PendingAction != nil {
		t.Fatalf("consent stop = %#v, %v; want safe stop without a record", result, err)
	}
}

func TestExactOperationConsentRejectsUnsafeOrStaleReplies(t *testing.T) {
	consent := ExactOperationConsentContext{Action: "send-external-notice", OperationClass: ExternalOperationClass, CanonicalTarget: "https://example.test/notices/42", SessionID: "session-1", Workspace: "/canonical/project", MaterialEffect: "Sends one external notice.", ApprovalScope: "This one rendered notice only.", ContentDigest: strings.Repeat("f", 64), RenderedRevision: "operation-rev-1"}
	context := QuestionContext{PromptID: "consent-1", SessionID: "session-1", Workspace: consent.Workspace, Action: consent.Action}
	for _, test := range []struct {
		name    string
		reply   OpenCodeQuestionReply
		current ExactOperationConsentContext
		replace bool
	}{
		{name: "missing", current: consent},
		{name: "dismissed", reply: OpenCodeQuestionReply{Answers: [][]string{{}}}, current: consent},
		{name: "custom", reply: OpenCodeQuestionReply{Answers: [][]string{{"approve once"}}}, current: consent},
		{name: "replaced", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactOperationConsentOption}}}, current: consent, replace: true},
		{name: "stale revision", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactOperationConsentOption}}}, current: ExactOperationConsentContext{Action: consent.Action, OperationClass: consent.OperationClass, CanonicalTarget: consent.CanonicalTarget, SessionID: consent.SessionID, Workspace: consent.Workspace, MaterialEffect: consent.MaterialEffect, ApprovalScope: consent.ApprovalScope, ContentDigest: consent.ContentDigest, RenderedRevision: "operation-rev-2"}},
		{name: "mismatched session", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactOperationConsentOption}}}, current: ExactOperationConsentContext{Action: consent.Action, OperationClass: consent.OperationClass, CanonicalTarget: consent.CanonicalTarget, SessionID: "other", Workspace: consent.Workspace, MaterialEffect: consent.MaterialEffect, ApprovalScope: consent.ApprovalScope, ContentDigest: consent.ContentDigest, RenderedRevision: consent.RenderedRevision}},
		{name: "mismatched workspace", reply: OpenCodeQuestionReply{Answers: [][]string{{ExactOperationConsentOption}}}, current: ExactOperationConsentContext{Action: consent.Action, OperationClass: consent.OperationClass, CanonicalTarget: consent.CanonicalTarget, SessionID: consent.SessionID, Workspace: "/other", MaterialEffect: consent.MaterialEffect, ApprovalScope: consent.ApprovalScope, ContentDigest: consent.ContentDigest, RenderedRevision: consent.RenderedRevision}},
	} {
		t.Run(test.name, func(t *testing.T) {
			question, err := NewExactOperationConsentQuestion(context, consent)
			if err != nil {
				t.Fatal(err)
			}
			displayed, err := NewDisplayedNativeQuestion(question)
			if err != nil {
				t.Fatal(err)
			}
			if test.replace {
				displayed.Replace()
			}
			if _, err := ConsumeExactOperationConsentReply(displayed, test.reply, test.current); err == nil {
				t.Fatal("unsafe operation consent reply was accepted")
			}
		})
	}
}

func TestExactOperationConsentRejectsUnclassifiedOrNonCanonicalTargets(t *testing.T) {
	base := ExactOperationConsentContext{
		Action: "delete-preview", OperationClass: DestructiveOperationClass, CanonicalTarget: "/canonical/project/previews/42",
		SessionID: "session", Workspace: "/canonical/project", MaterialEffect: "Deletes one preview.", ApprovalScope: "This one rendered deletion.",
		ContentDigest: strings.Repeat("a", 64), RenderedRevision: "revision-1",
	}
	context := QuestionContext{PromptID: "consent", SessionID: base.SessionID, Workspace: base.Workspace, Action: base.Action}
	for _, test := range []struct {
		name string
		edit func(*ExactOperationConsentContext)
	}{
		{name: "unclassified operation", edit: func(value *ExactOperationConsentContext) { value.OperationClass = "" }},
		{name: "relative destructive target", edit: func(value *ExactOperationConsentContext) { value.CanonicalTarget = "previews/42" }},
		{name: "unclean destructive target", edit: func(value *ExactOperationConsentContext) { value.CanonicalTarget = "/canonical/project/previews/../42" }},
		{name: "out of scope destructive target", edit: func(value *ExactOperationConsentContext) { value.CanonicalTarget = "/other/project/42" }},
		{name: "ambiguous external target", edit: func(value *ExactOperationConsentContext) {
			value.OperationClass = ExternalOperationClass
			value.CanonicalTarget = "https://example.test/notices/42#fragment"
		}},
		{name: "unclean external target", edit: func(value *ExactOperationConsentContext) {
			value.OperationClass = ExternalOperationClass
			value.CanonicalTarget = "https://example.test/notices/../42"
		}},
		{name: "relative external target", edit: func(value *ExactOperationConsentContext) {
			value.OperationClass = ExternalOperationClass
			value.CanonicalTarget = "/notices/42"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			consent := base
			test.edit(&consent)
			if _, err := NewExactOperationConsentQuestion(context, consent); err == nil {
				t.Fatal("invalid operation consent was displayed")
			}
		})
	}
}

func TestNativeQuestionRejectsForgedApprovedTriggerFactories(t *testing.T) {
	approval := ExactStrictApprovalContext{ContractPath: ".rotta/strict/contract.md", ContentDigest: strings.Repeat("a", 64), RenderedRevision: "rev-1", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}
	consent := ExactOperationConsentContext{Action: "delete-preview", OperationClass: DestructiveOperationClass, CanonicalTarget: "/workspace/preview", SessionID: "session", Workspace: "/workspace", MaterialEffect: "Deletes one preview.", ApprovalScope: "This one rendered deletion.", ContentDigest: strings.Repeat("b", 64), RenderedRevision: "rev-1"}
	for _, test := range []struct {
		name     string
		question NativeQuestion
	}{
		{name: "Vela", question: NativeQuestion{Context: QuestionContext{PromptID: "vela", SessionID: "session", Workspace: "/workspace", Action: VelaStaleGraphAction}, Trigger: VelaStaleEvidenceTrigger, Question: "Graph evidence is stale. What should happen?", Header: "Graph evidence", Options: []QuestionOption{{Label: VelaUseSourceFallback, Description: "Continue with source evidence; do not change graph state."}, {Label: VelaRequestReindexReview, Description: "Record a bounded re-index review for this project root."}, {Label: VelaStopAndRevisit, Description: "Stop this path and revisit the decision."}}, Purpose: QuestionPurposeConsent}},
		{name: "clarification", question: NativeQuestion{Context: QuestionContext{PromptID: "clarify", SessionID: "session", Workspace: "/workspace", Action: StrictClarificationAction}, Trigger: MaterialStrictClarificationTrigger, Question: "What delivery scope is intended?", Header: "Scope", Options: []QuestionOption{{Label: "Bounded", Description: "Deliver only the smallest stated scope."}, {Label: StrictClarificationStopOption, Description: "Stop clarification and leave unresolved decisions for safe defaults."}}, Purpose: QuestionPurposeDecision}},
		{name: "approval", question: newExactStrictApprovalQuestion(QuestionContext{PromptID: "approval", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}, approval)},
		{name: "operation", question: newExactOperationConsentQuestion(QuestionContext{PromptID: "operation", SessionID: "session", Workspace: "/workspace", Action: consent.Action}, consent)},
		{name: "policy", question: NativeQuestion{Context: QuestionContext{PromptID: "policy", SessionID: "session", Workspace: "/workspace", Action: MaterialPolicyDecisionAction}, Trigger: MaterialPolicyDecisionTrigger, Question: "Which bounded policy applies?", Header: "Policy", Options: []QuestionOption{{Label: "Keep", Description: "Keep the safe default."}, {Label: MaterialPolicyStopOption, Description: "Stop safely."}}, Purpose: QuestionPurposeDecision, SafeDefault: "Keep"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewDisplayedNativeQuestion(test.question); err == nil || !strings.Contains(err.Error(), "approved factory") {
				t.Fatalf("NewDisplayedNativeQuestion() error = %v, want forged-question rejection", err)
			}
			if _, err := MarshalOpenCodeQuestionRequest(test.question); err == nil || !strings.Contains(err.Error(), "approved factory") {
				t.Fatalf("MarshalOpenCodeQuestionRequest() error = %v, want forged-question rejection", err)
			}
		})
	}
}

func TestNativeQuestionRejectsMutationOfEveryFactoryOutput(t *testing.T) {
	approval := ExactStrictApprovalContext{ContractPath: ".rotta/strict/contract.md", ContentDigest: strings.Repeat("c", 64), RenderedRevision: "rev-1", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}
	consent := ExactOperationConsentContext{Action: "delete-preview", OperationClass: DestructiveOperationClass, CanonicalTarget: "/workspace/preview", SessionID: "session", Workspace: "/workspace", MaterialEffect: "Deletes one preview.", ApprovalScope: "This one rendered deletion.", ContentDigest: strings.Repeat("d", 64), RenderedRevision: "rev-1"}
	flow, err := NewStrictClarificationFlow(StrictClarificationRequest{ContractID: "contract", InitialRequest: "deliver a bounded change", SessionID: "session", Workspace: "/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	clarification := flow.CurrentQuestion("clarification")
	if clarification == nil {
		t.Fatal("expected clarification factory output")
	}
	policy := testMaterialPolicyQuestion(t, "policy", "Keep")
	operation, err := NewExactOperationConsentQuestion(QuestionContext{PromptID: "operation", SessionID: "session", Workspace: "/workspace", Action: consent.Action}, consent)
	if err != nil {
		t.Fatal(err)
	}
	exactApproval, err := NewExactStrictApprovalQuestion(QuestionContext{PromptID: "approval", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}, approval)
	if err != nil {
		t.Fatal(err)
	}
	vela, err := NewVelaStaleGraphQuestion(QuestionContext{PromptID: "vela", SessionID: "session", Workspace: "/workspace", Action: VelaStaleGraphAction})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		question NativeQuestion
	}{
		{name: "Vela", question: vela},
		{name: "clarification", question: *clarification},
		{name: "approval", question: exactApproval},
		{name: "operation", question: operation},
		{name: "policy", question: policy},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.question.Question = "Forged canonical prompt"
			if _, err := NewDisplayedNativeQuestion(test.question); err == nil || !strings.Contains(err.Error(), "approved factory") {
				t.Fatalf("NewDisplayedNativeQuestion() error = %v, want mutated-factory rejection", err)
			}
			if _, err := MarshalOpenCodeQuestionRequest(test.question); err == nil || !strings.Contains(err.Error(), "approved factory") {
				t.Fatalf("MarshalOpenCodeQuestionRequest() error = %v, want mutated-factory rejection", err)
			}
		})
	}
}

func TestDisplayedNativeQuestionRejectsPostDisplayCallerMutationForEveryFactory(t *testing.T) {
	approval := ExactStrictApprovalContext{ContractPath: ".rotta/strict/contract.md", ContentDigest: strings.Repeat("c", 64), RenderedRevision: "rev-1", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}
	consent := ExactOperationConsentContext{Action: "delete-preview", OperationClass: DestructiveOperationClass, CanonicalTarget: "/workspace/preview", SessionID: "session", Workspace: "/workspace", MaterialEffect: "Deletes one preview.", ApprovalScope: "This one rendered deletion.", ContentDigest: strings.Repeat("d", 64), RenderedRevision: "rev-1"}
	flow, err := NewStrictClarificationFlow(StrictClarificationRequest{ContractID: "contract", InitialRequest: "deliver a bounded change", SessionID: "session", Workspace: "/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	clarification := flow.CurrentQuestion("clarification")
	if clarification == nil {
		t.Fatal("expected clarification factory output")
	}
	exactApproval, err := NewExactStrictApprovalQuestion(QuestionContext{PromptID: "approval", SessionID: "session", Workspace: "/workspace", Action: ExactStrictApprovalAction}, approval)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := NewExactOperationConsentQuestion(QuestionContext{PromptID: "operation", SessionID: "session", Workspace: "/workspace", Action: consent.Action}, consent)
	if err != nil {
		t.Fatal(err)
	}
	vela, err := NewVelaStaleGraphQuestion(QuestionContext{PromptID: "vela", SessionID: "session", Workspace: "/workspace", Action: VelaStaleGraphAction})
	if err != nil {
		t.Fatal(err)
	}
	policy := testMaterialPolicyQuestion(t, "policy", "Keep")

	for _, test := range []struct {
		name     string
		question NativeQuestion
		consume  func(*DisplayedNativeQuestion, NativeQuestion) (NativeQuestionResult, error)
	}{
		{name: "clarification", question: *clarification, consume: func(displayed *DisplayedNativeQuestion, mutated NativeQuestion) (NativeQuestionResult, error) {
			return displayed.Consume(NativeQuestionAnswer{Context: mutated.Context, Label: "forged post-display option"}, mutated.Context)
		}},
		{name: "strict approval", question: exactApproval, consume: func(displayed *DisplayedNativeQuestion, mutated NativeQuestion) (NativeQuestionResult, error) {
			return ConsumeExactStrictApprovalReply(displayed, OpenCodeQuestionReply{Answers: [][]string{{"forged post-display option"}}}, approval)
		}},
		{name: "policy decision", question: policy, consume: func(displayed *DisplayedNativeQuestion, mutated NativeQuestion) (NativeQuestionResult, error) {
			return displayed.Consume(NativeQuestionAnswer{Context: mutated.Context, Label: "forged post-display option"}, mutated.Context)
		}},
		{name: "Vela stale", question: vela, consume: func(displayed *DisplayedNativeQuestion, mutated NativeQuestion) (NativeQuestionResult, error) {
			return ConsumeVelaStaleGraphAnswer(displayed, NativeQuestionAnswer{Context: mutated.Context, Label: "forged post-display option"}, mutated.Context)
		}},
		{name: "operation consent", question: operation, consume: func(displayed *DisplayedNativeQuestion, mutated NativeQuestion) (NativeQuestionResult, error) {
			return ConsumeExactOperationConsentReply(displayed, OpenCodeQuestionReply{Answers: [][]string{{"forged post-display option"}}}, consent)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			displayed, err := NewDisplayedNativeQuestion(test.question)
			if err != nil {
				t.Fatal(err)
			}
			// Mutate the caller-held slice after display. A shallow display copy
			// would accept this forged option and could emit decision evidence.
			test.question.Options[0].Label = "forged post-display option"
			result, err := test.consume(displayed, test.question)
			if err == nil || !strings.Contains(err.Error(), "option") {
				t.Fatalf("post-display mutation result = %#v, error = %v; want forged option rejection", result, err)
			}
			if result.Selected != "" || result.ExactApproval != nil || result.PendingAction != nil {
				t.Fatalf("post-display mutation emitted decision or pending authorization: %#v", result)
			}
		})
	}
}

func TestDisplayedNativeQuestionRevalidatesItsSealedIssuedQuestion(t *testing.T) {
	question := testMaterialPolicyQuestion(t, "sealed", "Keep")
	displayed, err := NewDisplayedNativeQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	displayed.question.Header = "forged rendered identity"

	result, err := displayed.Consume(NativeQuestionAnswer{Context: question.Context, Label: "Keep"}, question.Context)
	if err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("Consume() result = %#v, error = %v; want sealed-question rejection", result, err)
	}
	if result.Selected != "" || result.ExactApproval != nil || result.PendingAction != nil {
		t.Fatalf("modified stored question emitted decision or pending authorization: %#v", result)
	}
}

func testMaterialPolicyQuestion(t *testing.T, promptID, safeDefault string) NativeQuestion {
	t.Helper()
	question, err := NewMaterialPolicyDecisionQuestion(
		QuestionContext{PromptID: promptID, SessionID: "session-1", Workspace: "/workspace", Action: MaterialPolicyDecisionAction},
		MaterialPolicyDecision{Question: "Which bounded policy applies?", Header: "Policy", Alternatives: []QuestionOption{{Label: safeDefault, Description: "Keep the safe default."}}, SafeDefault: safeDefault},
	)
	if err != nil {
		t.Fatalf("new material policy question: %v", err)
	}
	return question
}
