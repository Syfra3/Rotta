package installer

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/assets"
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
		"non-blocking background graph refresh",
		"cached graph may be used while refresh runs",
		"vela update",
		"vela build",
		"foreground refresh",
		"provenance",
		"confidence",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
	assertNotContains(t, got, "visible start/end/fallback feedback")
	assertNotContains(t, got, "Ancora remains the primary MCP surface")
}

func TestIntegrationInstructionsWhenAncoraAndVelaEnabled(t *testing.T) {
	got := integrationInstructions(Options{SetupAncora: true, SetupVela: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"### Vela Graph Intelligence Enabled",
		"Ancora remains the primary MCP surface",
		"non-blocking background graph refresh",
		"cached graph may be used while refresh runs",
		"foreground refresh",
		"provenance",
		"confidence",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
}

func TestReadRenderedAssetAppendsDisabledIntegrationInstructions(t *testing.T) {
	data, err := readRenderedAsset("agents/rotta-orchestrator.md", Options{})
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
	data, err := readRenderedAsset("agents/rotta-orchestrator.md", Options{SetupAncora: true, SetupVela: true})
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
		"non-blocking background graph refresh",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
}

func TestCanonicalWorkflowInstructionsEnforceCleanTDDTaskBoundaries(t *testing.T) {
	got := canonicalWorkflowInstructions()

	assertContainsAll(t, got, []string{
		"Every TDD scenario task starts clean",
		"git status --short",
		"update the task checklist with completed, remaining, and next work",
		"checkpoint or clean the task diff before starting another scenario",
		"Approved spec/feature contracts are tracked durable artifacts",
	})
}

func TestVelaInstructionsEnforceExactSubjectStructuralQueryWorkflow(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
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

func velaStructuralQueryEnforcementStrings() []string {
	return []string{
		"For structural dependency, reverse-dependency, impact, path, ownership, or architecture questions, run `vela_status` first",
		"For ranking or hotspot structural questions",
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"Do not manually rank candidates by repeatedly dumping full edges",
		"at most 5 graph calls total for one ranking/hotspot question",
		"call `vela_module_summary` or `vela_explain` only for top candidates",
		"Full edge dumps require an explicit user request",
		"If compact tools are unavailable, use a bounded fallback",
		"Use `vela_lookup` to resolve concrete subjects before specialized graph calls",
		"Prefer exact file, module, controller, use-case, service, DTO, route handler, endpoint, or API-client subjects",
		"If symbol-level `vela_dependencies` or `vela_reverse_dependencies` returns `(none)` or an empty result and a containing file node exists, retry at file level",
		"`vela_explore` is routing/discovery only, not final proof when ambiguous",
		"Launch an exploration subagent for structural questions only after the exact Vela workflow fails",
		"Before launching that subagent, state the specific Vela insufficiency or gap",
		"Final answers must report Vela confidence and gaps",
		"graph-call budget use",
		"Vela is advisory only",
	}
}

func TestVelaInstructionsEnforceCompactRankingBudget(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, []string{
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"limit 10 candidates",
		"3 examples per candidate",
		"5 examples for `vela_module_summary`",
		"at most 5 graph calls total",
		"bounded fallback",
	})
}

func TestRottaOrchestratorAssetEnforcesCompactRankingWorkflow(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"Vela compact ranking enforcement",
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"at most 5 graph calls total",
		"Do not manually rank candidates by repeatedly dumping full edges",
		"Vela is advisory graph intelligence only",
	})
}

func TestRottaOrchestratorAssetEnforcesCleanTDDTaskBoundaries(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"TDD task boundary rule",
		"every scenario task MUST start from a clean",
		"update the task checklist with",
		"Do not launch the next `rotta-impl` call",
		"until the worktree is clean again",
		"`rotta-impl` reports changed files; it does",
		"not decide how to persist or discard them",
	})
}

// REQ-003 → SCN-330 → TestSCN330_OnlyOrchestratorPersistsLifecycleDecisions
func TestSCN330_OnlyOrchestratorPersistsLifecycleDecisions(t *testing.T) {
	// Scenario: Only the orchestrator persists lifecycle decisions
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Exclusive Lifecycle Authority",
		"Only the Rotta-Orchestrator may persist lifecycle decisions",
		"approval, phase transition, scenario acceptance, checkpoint, or lifecycle archive",
		"Phase-role output alone is never lifecycle authority",
	})
}

// REQ-003 → SCN-331 → TestSCN331_SpecWorkProducesOnlyAssignedContractArtifacts
func TestSCN331_SpecWorkProducesOnlyAssignedContractArtifacts(t *testing.T) {
	// Scenario: Spec work produces only its contract artifacts
	data, err := assets.FS.ReadFile("agents/rotta-spec.md")
	if err != nil {
		t.Fatalf("read spec asset: %v", err)
	}

	got := string(data)
	assertContainsAll(t, got, []string{
		"MAY ONLY write the assigned hard spec and Gherkin contract artifacts",
		"MUST NOT create an approval record, baseline, current state, lifecycle state, or commit",
	})
	for _, forbidden := range []string{
		"Maintain the workflow state index",
		"Save a STATE INDEX",
		"Update the state index",
		"ancora_save",
	} {
		assertNotContains(t, got, forbidden)
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
