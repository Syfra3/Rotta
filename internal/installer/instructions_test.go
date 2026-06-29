package installer

import (
	"strings"
	"testing"
)

func TestIntegrationInstructionsWhenAncoraAndVelaDisabled(t *testing.T) {
	got := integrationInstructions(Options{})

	for _, want := range []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("integration instructions missing %q:\n%s", want, got)
		}
	}
}

func TestIntegrationInstructionsWhenAncoraEnabledAndVelaDisabled(t *testing.T) {
	got := integrationInstructions(Options{SetupAncora: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"ancora_context",
		"ancora_save",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
	})
	assertNotContains(t, got, "### Vela Graph Intelligence Enabled")
}

func TestIntegrationInstructionsWhenAncoraDisabledAndVelaEnabled(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Enabled",
		"Vela may be available as standalone `vela_*` MCP tools",
		"trigger extraction/indexing first",
		"provenance",
		"confidence",
	})
	assertNotContains(t, got, "Ancora remains the primary MCP surface")
}

func TestIntegrationInstructionsWhenAncoraAndVelaEnabled(t *testing.T) {
	got := integrationInstructions(Options{SetupAncora: true, SetupVela: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"### Vela Graph Intelligence Enabled",
		"Ancora remains the primary MCP surface",
		"trigger extraction/indexing first",
		"provenance",
		"confidence",
	})
}

func TestReadRenderedAssetAppendsDisabledIntegrationInstructions(t *testing.T) {
	data, err := readRenderedAsset("agents/clean-orchestrator.md", Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
		"state_ref: \"specs/hard_spec.md + features/*.feature\"",
	})
	assertNotContains(t, got, "ancora_topic")
	assertNotContains(t, got, "ancora_context")
	assertNotContains(t, got, "ancora_save:")
}

func TestReadRenderedAssetAppendsEnabledIntegrationInstructions(t *testing.T) {
	data, err := readRenderedAsset("agents/clean-orchestrator.md", Options{SetupAncora: true, SetupVela: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"ancora_context",
		"ancora_save",
		"### Vela Graph Intelligence Enabled",
		"Ancora remains the primary MCP surface",
	})
}

func TestVelaBinCandidatesIncludesLinuxbrew(t *testing.T) {
	got := strings.Join(velaBinCandidates(), "\n")
	assertContainsAll(t, got, []string{
		"/opt/homebrew/bin/vela",
		"/home/linuxbrew/.linuxbrew/bin/vela",
		"/usr/local/bin/vela",
	})
}

func TestVelaResultAddFileDeduplicatesGraphDB(t *testing.T) {
	result := &VelaResult{}
	result.addFiles("/project/.vela/graph.db", "/home/.claude/vela-mcp.json")
	result.addFiles("/project/.vela/graph.db", "/home/.config/opencode/opencode.json")

	if countOccurrences(result.Files, "/project/.vela/graph.db") != 1 {
		t.Fatalf("expected graph db once, got %#v", result.Files)
	}
}

func assertContainsAll(t *testing.T, got string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func assertNotContains(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("unexpected %q:\n%s", unwanted, got)
	}
}

func countOccurrences(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}
