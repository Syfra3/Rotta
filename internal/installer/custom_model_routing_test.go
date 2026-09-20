package installer

import "testing"

func TestCustomRoutingUsesSubmittedRoleModels(t *testing.T) {
	custom := defaultOpenCodeRouting()
	custom["rotta-impl"] = "anthropic/claude-sonnet"

	routing, err := resolveOpenCodeRouting(ModelRoutingCustom, custom)
	if err != nil {
		t.Fatalf("resolve custom routing: %v", err)
	}
	if got := routing.models["rotta-impl"]; got != "anthropic/claude-sonnet" {
		t.Fatalf("custom implementation model = %q, want submitted model", got)
	}
}

func TestCustomRoutingRejectsInvalidOrWhitespacePaddedModelIDs(t *testing.T) {
	for _, model := range []string{"not-a-model", " provider/model", "provider/model ", "provider/model/extra", "provider|bad/model", "provider\\bad/model"} {
		t.Run(model, func(t *testing.T) {
			custom := defaultOpenCodeRouting()
			custom["rotta-impl"] = model
			if _, err := resolveOpenCodeRouting(ModelRoutingCustom, custom); err == nil {
				t.Fatalf("invalid custom model %q was accepted", model)
			}
		})
	}
}
