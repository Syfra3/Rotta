package workflow

import (
	"strings"
	"testing"
)

// REQ-088 → SCN-621 → TestSCN621_LifecycleModelIsRuntimeAuthority
func TestSCN621_LifecycleModelIsRuntimeAuthority(t *testing.T) {
	// Scenario: Generated roles share one lifecycle authority without contradictory ownership
	got := LifecycleModelInstructions()
	for _, want := range []string{
		LifecycleModelID,
		".rotta/current/manifest.yaml",
		".rotta/current/state.yaml",
		".rotta/current/evidence/",
		"Only the Rotta-Orchestrator may approve, checkpoint, archive, recover, or complete a feature.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("lifecycle model missing %q:\n%s", want, got)
		}
	}
}
