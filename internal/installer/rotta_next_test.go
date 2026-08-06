package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/assets"
)

func TestRottaNextCorePolicyUsesCoherentSlices(t *testing.T) {
	data, err := assets.FS.ReadFile("core/rotta-core.md")
	if err != nil {
		t.Fatal(err)
	}
	policy := string(data)
	for _, want := range []string{"one coherent slice", "independent review", "Do not require a worktree", "2,000 tokens"} {
		if !strings.Contains(policy, want) {
			t.Fatalf("core policy missing %q", want)
		}
	}
	if strings.Contains(policy, "one approved scenario at a time") {
		t.Fatal("core policy retained scenario boundary")
	}
}

func TestRottaNextHandoffGuidancePreservesOrchestratorBoundary(t *testing.T) {
	for _, asset := range []struct {
		path string
		want []string
	}{
		{"core/rotta-core.md", []string{"`rotta.handoff/v1`", "never authority", "newest valid matching mirror by sequence"}},
		{"agents/rotta-orchestrator.md", []string{"Only the orchestrator may record `rotta.handoff/v1` status", "Ancora/mirror agreement", "degraded recovery"}},
		{"agents/rotta-impl.md", []string{"Return handoff evidence only", "Do not create, accept, block, complete"}},
		{"agents/rotta-cleaner.md", []string{"Return handoff evidence only", "Do not create, accept, block, complete"}},
		{"agents/rotta-architect.md", []string{"Return handoff evidence only", "Do not create, accept, block, complete"}},
		{"agents/rotta-review.md", []string{"Return handoff evidence only", "Do not create, accept, block, complete"}},
	} {
		data, err := assets.FS.ReadFile(asset.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range asset.want {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s missing handoff guidance %q", asset.path, want)
			}
		}
	}
	if got := memoryInstructions(true); !strings.Contains(got, "atomic matching `.rotta/handoffs/` mirror") || !strings.Contains(got, "never timestamp") {
		t.Fatalf("enabled Ancora instructions omit handoff recovery guidance: %s", got)
	}
}

func TestRottaNextInstallsCoreAndAllRoles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatalf("install OpenCode Rotta Next: %v", err)
	}

	configData, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	agents := config["agent"].(map[string]interface{})
	for _, name := range []string{"rotta-orchestrator", "rotta-explore", "rotta-impl", "rotta-review", "rotta-ops", "rotta-cleaner", "rotta-architect"} {
		if _, ok := agents[name]; !ok {
			t.Fatalf("missing Rotta Next agent %q", name)
		}
		assertRottaNextFileContains(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-next", name, "SKILL.md"), "Load the `rotta-core` skill")
	}
	assertRottaNextFileContains(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md"), "Fast mode is the default")
	if _, ok := agents["rotta-spec"]; ok {
		t.Fatal("retired rotta-spec agent was installed")
	}
	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatalf("reinstall managed OpenCode roles: %v", err)
	}
	orchestrator := agents["rotta-orchestrator"].(map[string]interface{})
	orchestrator["prompt"] = "user modification"
	configData, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "opencode", "opencode.json"), configData, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installOpenCode(Options{}, home); err == nil {
		t.Fatal("modified managed OpenCode agent was overwritten")
	}
}

func TestRottaNextInstallsConditionalQualityRolesAcrossHosts(t *testing.T) {
	core, err := assets.FS.ReadFile("core/rotta-core.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(core), "Cleaner and architect are conditional deep-review roles, never a standard Fast-slice requirement") {
		t.Fatal("core policy no longer makes quality roles conditional in Fast mode")
	}
	orchestrator, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(orchestrator), "Cleaner and architect are conditional deep-review roles, never a standard Fast-slice requirement") {
		t.Fatal("orchestrator no longer defines the standard Fast slice")
	}

	for _, role := range []struct {
		name  string
		asset string
		wants []string
	}{
		{
			name:  "rotta-cleaner",
			asset: "agents/rotta-cleaner.md",
			wants: []string{"Load the `rotta-core` skill", "conditional deep-review role", "approved behavior-preserving cleanup", "Stop and return to the orchestrator", "Do not add product behavior"},
		},
		{
			name:  "rotta-architect",
			asset: "agents/rotta-architect.md",
			wants: []string{"Load the `rotta-core` skill", "conditional deep-review role", "read-only by default", "Stop and return to the orchestrator", "Do not implement, edit, operate"},
		},
	} {
		t.Run(role.name+" asset constraints", func(t *testing.T) {
			data, err := assets.FS.ReadFile(role.asset)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range role.wants {
				if !strings.Contains(string(data), want) {
					t.Fatalf("%s missing role constraint %q", role.asset, want)
				}
			}
		})
	}
	for _, role := range rottaAgents {
		switch role.key {
		case "rotta-cleaner":
			if !role.tools["bash"] || !role.tools["edit"] || !role.tools["write"] {
				t.Fatal("cleaner lacks the tools required for approved cleanup and targeted evidence")
			}
		case "rotta-architect":
			if role.tools["bash"] || role.tools["edit"] || role.tools["write"] {
				t.Fatal("architect received implementation or operation tools")
			}
		}
	}

	claudeHome := t.TempDir()
	if _, err := installClaudeCode(Options{}, claudeHome); err != nil {
		t.Fatalf("install Claude Code Rotta Next: %v", err)
	}
	for _, name := range []string{"rotta-cleaner", "rotta-architect"} {
		assertRottaNextFileContains(t, filepath.Join(claudeHome, ".claude", "skills", "rotta-next", name, "SKILL.md"), "Load the `rotta-core` skill")
		assertRottaNextFileContains(t, filepath.Join(claudeHome, ".claude", "agents", name+".md"), "conditional deep-review role")
	}

	codexHome := t.TempDir()
	if _, err := installCodex(Options{}, codexHome); err != nil {
		t.Fatalf("install Codex Rotta Next: %v", err)
	}
	for _, heading := range []string{"# Rotta Cleaner", "# Rotta Architect"} {
		assertRottaNextFileContains(t, filepath.Join(codexHome, ".codex", "AGENTS.md"), heading)
	}
}

func TestRottaCleanerEnforcesConditionalEvidencePolicy(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-cleaner.md")
	if err != nil {
		t.Fatal(err)
	}
	policy := string(data)
	for _, want := range []string{
		"Fast mode does not require coverage, complexity/CRAP, duplication, or mutation evidence",
		"all** of these conditions hold",
		"the task is Strict, deep review was selected, or review identified a concrete changed-code risk",
		"declared by project metadata, repository configuration, or explicit user instruction",
		"confirms the pre-existing declared command or tool is runnable; static declaration alone does not establish availability",
		"fit the capsule's stated verification budget",
		"Do not install tools, guess commands, substitute a different tool or command, automatically execute a declared command, or silently skip",
		"visible evidence gap: it is neither a passing quality result nor an automatic block for Fast work",
		"changed-code delta evidence only",
		"newly introduced or worsened complexity or coverage risk relative to the slice as advisory evidence, never a universal threshold",
		"bounded, non-default, and targeted only to changed, weakly-covered, risk-sensitive behavior",
		"do not use a full-repository mutation run by default",
		"Any cleaner edit must remain behavior-preserving, requires relevant verification, invalidates prior review evidence, and requires a fresh independent review",
	} {
		if !strings.Contains(policy, want) {
			t.Fatalf("cleaner policy missing %q", want)
		}
	}
}

func TestRottaNextEnforcesBoundedDeepReviewPolicy(t *testing.T) {
	assetsToCheck := []struct {
		path  string
		wants []string
	}{
		{"core/rotta-core.md", []string{
			"Fast mode is the default: `orchestrator → impl → review → outcome`; it spawns neither cleaner nor architect by default",
			"Strict classification, an explicit user request, repository policy, or concrete review evidence",
			"A deep capsule and handoff record the selected trigger and expected evidence",
			"There is no direct `review → cleaner` or `review → architect` route",
			"exactly one fresh independent final review",
			"whether the route was Fast or deep",
		}},
		{"agents/rotta-orchestrator.md", []string{
			"`orchestrator → impl → review → outcome`; it spawns neither cleaner nor architect by default",
			"at most one cleaner and one optional architect per coherent slice",
			"A reviewer returns to the orchestrator; only the orchestrator may initiate one bounded escalation",
			"Never route `review → cleaner` or `review → architect`",
			"isolated `architect → impl` remediation capsule",
		}},
		{"agents/rotta-review.md", []string{
			"requires exactly one fresh independent final review",
			"Never route directly to cleaner or architect",
		}},
		{"agents/rotta-cleaner.md", []string{
			"initiate a recursive quality route",
			"isolated `architect → impl` remediation capsule",
		}},
		{"agents/rotta-architect.md", []string{
			"Do not self-schedule, schedule cleaner, recursively escalate quality review, or self-approve",
			"stop for non-isolated or broader findings",
		}},
	}
	for _, asset := range assetsToCheck {
		data, err := assets.FS.ReadFile(asset.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range asset.wants {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s missing bounded deep-review policy %q", asset.path, want)
			}
		}
	}
}

func TestManagedArtifactsRefuseUnownedModifiedAndSymlinkTargets(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "managed", "SKILL.md")
	data := []byte("first")
	if _, err := installManagedFiles(home, map[string][]byte{path: data}); err != nil {
		t.Fatalf("initial managed install: %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installManagedFiles(home, map[string][]byte{path: []byte("next")}); err == nil {
		t.Fatal("modified managed file was overwritten")
	}

	unowned := filepath.Join(home, "unowned", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(unowned), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unowned, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installManagedFiles(home, map[string][]byte{unowned: []byte("next")}); err == nil {
		t.Fatal("unowned file was overwritten")
	}

	target := filepath.Join(home, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(home, "symlink", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(symlink), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := installManagedFiles(home, map[string][]byte{symlink: []byte("next")}); err == nil {
		t.Fatal("managed symlink target was followed")
	}

	parentTarget := filepath.Join(home, "parent-target")
	if err := os.MkdirAll(parentTarget, 0o750); err != nil {
		t.Fatal(err)
	}
	parentLink := filepath.Join(home, "parent-link")
	if err := os.Symlink(parentTarget, parentLink); err != nil {
		t.Fatal(err)
	}
	if _, err := installManagedFiles(home, map[string][]byte{filepath.Join(parentLink, "SKILL.md"): []byte("next")}); err == nil {
		t.Fatal("managed parent symlink was followed")
	}
}

func TestRottaNextRejectedOpenCodeInstallPreservesAgentConfig(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	original := []byte(`{"agent":{"rotta-orchestrator":{"prompt":"existing"}},"theme":"user"}`)
	writeRottaNextTestFile(t, configPath, original)
	writeRottaNextTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md"), []byte("unmanaged"))

	if _, err := installOpenCode(Options{}, home); err == nil {
		t.Fatal("unmanaged role file did not reject installation")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("rejected install changed OpenCode config:\n%s", data)
	}
}

func assertRottaNextFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s missing %q", path, want)
	}
}

func writeRottaNextTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSetupVelaDoesNotIndexDuringInstallation(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "project")
	result, err := SetupVela(Options{Target: "opencode", SetupVela: true}, home, project)
	if err != nil {
		t.Fatalf("record Vela availability: %v", err)
	}
	if status := result.MCPAvailability["opencode"]["vela"]; status.Status != MCPStatusSkipped {
		t.Fatalf("Vela setup status = %#v, want skipped", status)
	}
	if _, err := os.Stat(filepath.Join(project, ".vela", "graph.db")); !os.IsNotExist(err) {
		t.Fatalf("installation created graph state: %v", err)
	}
}

func TestRottaNextVelaSetupPreservesExistingConfigurationAndRoles(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeRottaNextTestFile(t, filepath.Join(home, ".claude", "vela-mcp.json"), []byte(`{"mcp":"existing"}`))

	if _, err := Install(Options{Target: "claude-code", ProjectPath: project, SetupVela: true}); err != nil {
		t.Fatalf("install with advisory Vela: %v", err)
	}
	assertRottaNextFileContains(t, filepath.Join(home, ".claude", "skills", "rotta-next", "rotta-core", "SKILL.md"), "Fast mode is the default")
	assertRottaNextFileContains(t, filepath.Join(home, ".claude", "vela-mcp.json"), "existing")
}

func TestManagedArtifactsRejectMalformedManifest(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "managed", "SKILL.md")
	if _, err := installManagedFiles(home, map[string][]byte{path: []byte("first")}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(home, ".config", "rotta", "managed-artifacts.json")
	writeRottaNextTestFile(t, manifest, []byte(`{"version":1,"files":{"/tmp/not-a-real-path":"invalid"}}`))
	if _, err := installManagedFiles(home, map[string][]byte{path: []byte("next")}); err == nil {
		t.Fatal("malformed ownership manifest was accepted")
	}
}
