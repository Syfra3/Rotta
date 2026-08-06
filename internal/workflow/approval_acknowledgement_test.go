package workflow

import (
	"reflect"
	"strings"
	"testing"
)

const scn610ReviewedCommit = "3d8d4cc6f8ed0e40a11d6aa2b6dc3e2f4f85d06b"

// REQ-083 → SCN-608 → TestSCN608_SoleDisplayedAcknowledgementAdvancesBoundActionOnce
func TestSCN608_SoleDisplayedAcknowledgementAdvancesBoundActionOnce(t *testing.T) {
	// REQ-095 supersedes the old acknowledgement path: only native Questions
	// may advance workflow state.
	for _, acknowledgement := range []string{"x", "yes", "agree", "approved", "approve"} {
		t.Run(acknowledgement, func(t *testing.T) {
			display := NewDisplayedApprovalAction("workflow-ergonomics", "unchanged-contract-fingerprint", "start implementation")
			var advanced []string

			err := display.ConsumeAcknowledgement(acknowledgement, "workflow-ergonomics", "unchanged-contract-fingerprint", func(action string) error {
				advanced = append(advanced, action)
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "native Question") {
				t.Fatalf("ConsumeAcknowledgement(%q) = %v, want legacy rejection", acknowledgement, err)
			}
			if len(advanced) != 0 {
				t.Fatalf("legacy acknowledgement advanced actions = %q", advanced)
			}
		})
	}
}

// REQ-083 → SCN-609 → TestSCN609_StaleOrAmbiguousAcknowledgementDoesNotAdvance
func TestSCN609_StaleOrAmbiguousAcknowledgementDoesNotAdvance(t *testing.T) {
	// Scenario: A stale or ambiguous acknowledgement cannot advance the workflow
	bound := ApprovalAcknowledgementContext{
		PromptID:            "prompt-1",
		SessionID:           "session-1",
		FeatureID:           "workflow-ergonomics",
		ContractFingerprint: "contract-1",
		PolicyFingerprint:   "policy-1",
		FinalSnapshot:       "reviewed-1",
		PendingActions:      1,
	}

	for _, testCase := range []struct {
		name            string
		acknowledgement string
		current         ApprovalAcknowledgementContext
		wantReason      string
	}{
		{name: "replaced prompt", acknowledgement: "approve", current: replacePrompt(bound), wantReason: "native Question"},
		{name: "restarted session", acknowledgement: "approve", current: replaceSession(bound), wantReason: "native Question"},
		{name: "feature drift", acknowledgement: "approve", current: replaceFeature(bound), wantReason: "native Question"},
		{name: "contract drift", acknowledgement: "approve", current: replaceContract(bound), wantReason: "native Question"},
		{name: "policy drift", acknowledgement: "approve", current: replacePolicy(bound), wantReason: "native Question"},
		{name: "final snapshot drift", acknowledgement: "approve", current: replaceFinalSnapshot(bound), wantReason: "native Question"},
		{name: "multiple intents", acknowledgement: "approve and archive", current: bound, wantReason: "native Question"},
		{name: "multiple pending actions", acknowledgement: "approve", current: multiplePendingActions(bound), wantReason: "native Question"},
		{name: "unsupported token", acknowledgement: "certainly", current: bound, wantReason: "native Question"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			display := NewContextualDisplayedApprovalAction(bound, "advance approval, checkpoint, review, archive, and completion")
			advanced := false

			err := display.ConsumeContextualAcknowledgement(testCase.acknowledgement, testCase.current, func(string) error {
				advanced = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantReason) {
				t.Fatalf("ConsumeContextualAcknowledgement() error = %v, want rejection reason containing %q", err, testCase.wantReason)
			}
			if advanced {
				t.Fatal("a rejected acknowledgement changed lifecycle state")
			}
		})
	}
}

// REQ-095 → free-form final review acknowledgements must never complete state.
func TestFinalApprovalRejectsEveryFreeFormCompletionPath(t *testing.T) {
	for _, acknowledgement := range []string{"approve", "Approve", "yes", "x", "anything"} {
		t.Run(acknowledgement, func(t *testing.T) {
			review := newSCN610FinalHumanReview()
			before := *review
			err := NewDisplayedFinalApprovalAction("workflow-ergonomics", scn610ReviewedCommit).Approve(acknowledgement, review)
			if err == nil || !strings.Contains(err.Error(), "native Question") {
				t.Fatalf("Approve(%q) = %v, want native Question rejection", acknowledgement, err)
			}
			if !reflect.DeepEqual(*review, before) {
				t.Fatalf("rejected free-form approval mutated final review: got %#v, want %#v", *review, before)
			}
		})
	}
}

func newSCN610FinalHumanReview() *FinalHumanReview {
	return &FinalHumanReview{
		FeatureID:                   "workflow-ergonomics",
		Phase:                       "final_human_review",
		CurrentCommit:               scn610ReviewedCommit,
		ReviewedCommit:              scn610ReviewedCommit,
		CurrentEvidenceFingerprint:  "evidence-1",
		ReviewedEvidenceFingerprint: "evidence-1",
		CurrentPolicyFingerprint:    "policy-1",
		ReviewedPolicyFingerprint:   "policy-1",
		PendingActions:              1,
	}
}

func replacePrompt(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.PromptID = "prompt-2"
	return context
}

func replaceSession(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.SessionID = "session-2"
	return context
}

func replaceFeature(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.FeatureID = "other-feature"
	return context
}

func replaceContract(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.ContractFingerprint = "contract-2"
	return context
}

func replacePolicy(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.PolicyFingerprint = "policy-2"
	return context
}

func replaceFinalSnapshot(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.FinalSnapshot = "reviewed-2"
	return context
}

func multiplePendingActions(context ApprovalAcknowledgementContext) ApprovalAcknowledgementContext {
	context.PendingActions = 2
	return context
}
