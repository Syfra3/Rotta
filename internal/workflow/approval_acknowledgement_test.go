package workflow

import "testing"

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
