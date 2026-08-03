package workflow

import (
	"strings"
	"testing"
)

// REQ-083 → SCN-608 → TestSCN608_SoleDisplayedAcknowledgementAdvancesBoundActionOnce
func TestSCN608_SoleDisplayedAcknowledgementAdvancesBoundActionOnce(t *testing.T) {
	// Scenario: A sole displayed acknowledgement authorizes its bound action
	for _, acknowledgement := range []string{"x", "yes", "agree", "approved", "approve"} {
		t.Run(acknowledgement, func(t *testing.T) {
			display := NewDisplayedApprovalAction("workflow-ergonomics", "unchanged-contract-fingerprint", "start implementation")
			var advanced []string

			err := display.ConsumeAcknowledgement(acknowledgement, "workflow-ergonomics", "unchanged-contract-fingerprint", func(action string) error {
				advanced = append(advanced, action)
				return nil
			})
			if err != nil {
				t.Fatalf("ConsumeAcknowledgement(%q) returned error: %v", acknowledgement, err)
			}
			if len(advanced) != 1 || advanced[0] != "start implementation" {
				t.Fatalf("advanced actions = %q, want only the displayed action", advanced)
			}
			if err := display.ConsumeAcknowledgement(acknowledgement, "workflow-ergonomics", "unchanged-contract-fingerprint", func(string) error {
				t.Fatal("a consumed displayed action advanced again")
				return nil
			}); err == nil {
				t.Fatal("a displayed action was consumed more than once")
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
		{name: "replaced prompt", acknowledgement: "approve", current: replacePrompt(bound), wantReason: "prompt"},
		{name: "restarted session", acknowledgement: "approve", current: replaceSession(bound), wantReason: "session"},
		{name: "feature drift", acknowledgement: "approve", current: replaceFeature(bound), wantReason: "feature"},
		{name: "contract drift", acknowledgement: "approve", current: replaceContract(bound), wantReason: "contract"},
		{name: "policy drift", acknowledgement: "approve", current: replacePolicy(bound), wantReason: "policy"},
		{name: "final snapshot drift", acknowledgement: "approve", current: replaceFinalSnapshot(bound), wantReason: "final snapshot"},
		{name: "multiple intents", acknowledgement: "approve and archive", current: bound, wantReason: "multiple intents"},
		{name: "multiple pending actions", acknowledgement: "approve", current: multiplePendingActions(bound), wantReason: "more than one"},
		{name: "unsupported token", acknowledgement: "certainly", current: bound, wantReason: "not an exact approval token"},
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
