package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// REQ-001 → SCN-001 → TestSCN001_OmittedRoutingUsesOnlyXDGGlobalConfiguration
func TestSCN001_OmittedRoutingUsesOnlyXDGGlobalConfiguration(t *testing.T) {
	// Scenario: Omitted installer routing request defaults to enabled.
	home := t.TempDir()
	xdg := filepath.Join(home, "custom-xdg")
	project := filepath.Join(home, "project")
	override := filepath.Join(home, "override.json")
	projectConfig := filepath.Join(project, "opencode.json")
	projectBefore := []byte(`{"agent":{"project":{"model":"user/project"}}}`)
	overrideBefore := []byte(`{"agent":{"override":{"model":"user/override"}}}`)
	writeRoutingTestFile(t, projectConfig, projectBefore)
	writeRoutingTestFile(t, override, overrideBefore)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	state := filepath.Join(home, "state")
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("OPENCODE_CONFIG", override)

	if _, err := Install(Options{Target: "opencode", ProjectPath: project}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got, err := os.ReadFile(projectConfig); err != nil || string(got) != string(projectBefore) {
		t.Fatalf("project config changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(override); err != nil || string(got) != string(overrideBefore) {
		t.Fatalf("override config changed: %q, %v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(xdg, "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("read XDG global config: %v", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse XDG global config: %v", err)
	}
	agents := config["agent"].(map[string]interface{})
	wantModels := map[string]string{
		"rotta-orchestrator": "openai/gpt-5.6-sol", "rotta-architect": "openai/gpt-5.6-sol", "rotta-review": "openai/gpt-5.6-sol",
		"rotta-impl": "openai/gpt-5.6-terra", "rotta-ops": "openai/gpt-5.6-luna", "rotta-explore": "openai/gpt-5.6-luna", "rotta-cleaner": "openai/gpt-5.6-luna",
	}
	manifest, err := readManagedArtifactsManifest(filepath.Join(xdg, "rotta", "managed-artifacts.json"))
	if err != nil {
		t.Fatal(err)
	}
	for role, want := range wantModels {
		agent := agents[role].(map[string]interface{})
		if got := agent["model"]; got != want {
			t.Fatalf("%s model = %q, want %q", role, got, want)
		}
		if manifest.Files[openCodeModelOwnershipKey(filepath.Join(xdg, "opencode", "opencode.json"), role)] != contentDigest([]byte(want)) {
			t.Fatalf("missing field ownership for %s.model", role)
		}
		if permission := agent["permission"].(map[string]interface{})["question"]; role != "rotta-orchestrator" && permission != "deny" {
			t.Fatalf("%s question permission = %q, want deny", role, permission)
		}
	}
}

// REQ-004 → SCN-004 → TestSCN004_UnownedModelRefusesBeforeMutation
func TestSCN004_UnownedModelRefusesBeforeMutation(t *testing.T) {
	// Scenario: User-owned configuration is preserved and conflicts refuse safely.
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	target := filepath.Join(xdg, "opencode", "opencode.json")
	before := []byte(`{"agent":{"rotta-architect":{"model":"user/model","description":"keep"}}}`)
	writeRoutingTestFile(t, target, before)
	_, err := Install(Options{Target: "opencode"})
	if err == nil || !containsAll(err.Error(), target, "rotta-architect", "model") {
		t.Fatalf("conflict error = %v, want target, role, and model field", err)
	}
	if got, readErr := os.ReadFile(target); readErr != nil || string(got) != string(before) {
		t.Fatalf("conflicted global config changed: %q, %v", got, readErr)
	}
}

// REQ-005 → SCN-005 → TestSCN005_DisablePreservesOwnedAgentNonModelFields
func TestSCN005_DisablePreservesOwnedAgentNonModelFields(t *testing.T) {
	// Scenario: Disable honors field-level ownership only.
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if _, err := Install(Options{Target: "opencode"}); err != nil {
		t.Fatalf("enabled install: %v", err)
	}
	target := filepath.Join(xdg, "opencode", "opencode.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	architect := config["agent"].(map[string]interface{})["rotta-architect"].(map[string]interface{})
	architect["user_setting"] = "keep"
	updated, _ := json.Marshal(config)
	writeRoutingTestFile(t, target, updated)
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("disabled install: %v", err)
	}
	data, _ = os.ReadFile(target)
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	architect = config["agent"].(map[string]interface{})["rotta-architect"].(map[string]interface{})
	if _, exists := architect["model"]; exists || architect["user_setting"] != "keep" {
		t.Fatalf("disable did not preserve the agent object and user field: %#v", architect)
	}
}

// REQ-006 → SCN-006 → TestSCN006_CustomXDGFailureRestoresConfigAndState
func TestSCN006_CustomXDGFailureRestoresConfigAndState(t *testing.T) {
	// Scenario: Custom-XDG transaction failure restores exact state.
	home := t.TempDir()
	xdg := filepath.Join(home, "custom-xdg")
	target := filepath.Join(xdg, "opencode", "opencode.json")
	before := []byte(`{"theme":"keep","agent":{}}`)
	writeRoutingTestFile(t, target, before)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	state := filepath.Join(home, "state")
	t.Setenv("XDG_STATE_HOME", state)
	routingFailureHook = func(stage string) error {
		if stage == "after-managed-state" {
			return errors.New("injected state failure")
		}
		return nil
	}
	t.Cleanup(func() { routingFailureHook = nil })
	_, err := Install(Options{Target: "opencode"})
	if err == nil {
		t.Fatal("Install() error = nil, want injected failure")
	}
	if got, readErr := os.ReadFile(target); readErr != nil || string(got) != string(before) {
		t.Fatalf("custom XDG config was not restored: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(xdg, "rotta", "managed-artifacts.json")); !os.IsNotExist(statErr) {
		t.Fatalf("managed state remains after rollback: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(xdg, "opencode", "skills", "rotta-next", "rotta-architect", "SKILL.md")); !os.IsNotExist(statErr) {
		t.Fatalf("routing asset remains after rollback: %v", statErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(state, "rotta", "installer-transactions"))
	if readErr != nil || len(entries) != 1 {
		t.Fatalf("read routing backup: %v, %#v", readErr, entries)
	}
	manifest, err := loadBackupManifest(filepath.Join(state, "rotta", "installer-transactions", entries[0].Name(), "manifest.json"))
	if err != nil || !containsString(manifest.BackedUpPaths, target) {
		t.Fatalf("backup did not use the custom XDG target: %#v, %v", manifest.BackedUpPaths, err)
	}
}

func TestRoutingConflictPreflightCreatesNoBackupOrState(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	state := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", state)
	target := filepath.Join(xdg, "opencode", "opencode.json")
	before := []byte(`{"agent":{"rotta-impl":{"model":"user/model"}}}`)
	writeRoutingTestFile(t, target, before)

	if _, err := Install(Options{Target: "opencode"}); err == nil {
		t.Fatal("conflicting routing install succeeded")
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != string(before) {
		t.Fatalf("refused config = %q, %v", got, err)
	}
	if got := installerTransactionCount(t, state); got != 0 {
		t.Fatalf("refused routing created %d backup(s)", got)
	}
	if _, err := os.Stat(filepath.Join(xdg, "rotta", "managed-artifacts.json")); !os.IsNotExist(err) {
		t.Fatalf("refused routing created managed state: %v", err)
	}
}

func TestRoutingReplaysDoNotCreateBackupOrStateEntries(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	state := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", state)

	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingEnabled}); err != nil {
		t.Fatalf("initial enabled install: %v", err)
	}
	if got := installerTransactionCount(t, state); got != 1 {
		t.Fatalf("initial enabled backup count = %d, want 1", got)
	}
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingEnabled}); err != nil {
		t.Fatalf("repeated enabled install: %v", err)
	}
	if got := installerTransactionCount(t, state); got != 1 {
		t.Fatalf("repeated enabled backup count = %d, want 1", got)
	}
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("initial disabled install: %v", err)
	}
	if got := installerTransactionCount(t, state); got != 2 {
		t.Fatalf("initial disabled backup count = %d, want 2", got)
	}
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("repeated disabled install: %v", err)
	}
	if got := installerTransactionCount(t, state); got != 2 {
		t.Fatalf("repeated disabled backup count = %d, want 2", got)
	}
}

func TestRoutingNoOpThroughMultiHostTargetsSkipsRoutingTransactionEntries(t *testing.T) {
	for _, targetName := range []string{"both", "all"} {
		for _, routing := range []ModelRoutingRequest{ModelRoutingEnabled, ModelRoutingDisabled} {
			t.Run(targetName+"/"+string(routing), func(t *testing.T) {
				home := t.TempDir()
				xdg := filepath.Join(home, "xdg")
				state := filepath.Join(home, "state")
				t.Setenv("HOME", home)
				t.Setenv("XDG_CONFIG_HOME", xdg)
				t.Setenv("XDG_STATE_HOME", state)

				if _, err := Install(Options{Target: "opencode", ModelRouting: routing}); err != nil {
					t.Fatalf("initial %s routing install: %v", routing, err)
				}
				manifestPath := filepath.Join(xdg, "rotta", "managed-artifacts.json")
				stateBefore, err := readManagedArtifactsManifest(manifestPath)
				if err != nil {
					t.Fatal(err)
				}

				result, err := Install(Options{Target: targetName, ModelRouting: routing})
				if err != nil {
					t.Fatalf("reconciled routing through %s: %v", targetName, err)
				}
				if result.BackupDir == "" {
					t.Fatalf("%s host work did not receive its own transaction", targetName)
				}
				assertNoRoutingBackupEntries(t, result.BackupDir, xdg)
				stateAfter, err := readManagedArtifactsManifest(manifestPath)
				if err != nil || !reflect.DeepEqual(routingStateEntries(stateAfter, xdg), routingStateEntries(stateBefore, xdg)) {
					t.Fatalf("reconciled routing state changed: %#v, %v", routingStateEntries(stateAfter, xdg), err)
				}
			})
		}
	}
}

func TestRoutingPreservesUnrelatedGlobalConfiguration(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	target := filepath.Join(xdg, "opencode", "opencode.json")
	before := []byte(`{"compaction":{"auto":false,"buffer":77,"user":"keep"},"tool_output":{"max_lines":9,"user":"keep"},"arbitrary":{"nested":true}}`)
	writeRoutingTestFile(t, target, before)

	if _, err := Install(Options{Target: "opencode"}); err != nil {
		t.Fatalf("enabled install: %v", err)
	}
	assertRoutingGlobalFields(t, target)
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("disabled install: %v", err)
	}
	assertRoutingGlobalFields(t, target)
}

func TestRoutingPreservesLegacyAgentsAndDefaultAgent(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	target := filepath.Join(xdg, "opencode", "opencode.json")
	legacy := map[string]interface{}{
		"bob-orchestrator":   map[string]interface{}{"description": "legacy Bob", "model": "user/bob"},
		"clean-orchestrator": map[string]interface{}{"description": "legacy Clean", "tools": map[string]interface{}{"read": true}},
	}
	before := map[string]interface{}{"default_agent": "bob-orchestrator", "agent": legacy}
	data, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	writeRoutingTestFile(t, target, data)

	for _, routing := range []ModelRoutingRequest{ModelRoutingEnabled, ModelRoutingDisabled} {
		if _, err := Install(Options{Target: "opencode", ModelRouting: routing}); err != nil {
			t.Fatalf("%s routing install: %v", routing, err)
		}
		got := readRoutingConfig(t, target)
		if got["default_agent"] != before["default_agent"] || !reflect.DeepEqual(got["agent"].(map[string]interface{})["bob-orchestrator"], legacy["bob-orchestrator"]) || !reflect.DeepEqual(got["agent"].(map[string]interface{})["clean-orchestrator"], legacy["clean-orchestrator"]) {
			t.Fatalf("%s routing changed legacy configuration: %#v", routing, got)
		}
	}
}

func TestOpenCodeInstallerPathsUseCustomXDGHome(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "custom-xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatalf("install OpenCode routing assets: %v", err)
	}
	if _, err := ConfigureContext7(Options{SetupContext7: true}, home); err != nil {
		t.Fatalf("configure Context7: %v", err)
	}
	writeRoutingTestFile(t, filepath.Join(xdg, "opencode", "opencode.jsonc"), []byte(`{"mcp":{"ancora":{"command":"/tmp/ancora","args":["mcp"]}}}`))
	if err := serializeAncoraMCPCommand(home, "opencode"); err != nil {
		t.Fatalf("serialize Ancora command: %v", err)
	}
	if _, err := installOpenCodeVelaFreshnessGuard(home); err != nil {
		t.Fatalf("install Vela guard: %v", err)
	}
	for _, path := range []string{
		filepath.Join(xdg, "opencode", "opencode.json"),
		filepath.Join(xdg, "opencode", "opencode.jsonc"),
		filepath.Join(xdg, "opencode", "plugin", openCodeVelaFreshnessPluginFile),
		filepath.Join(xdg, "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md"),
		filepath.Join(xdg, "rotta", "managed-artifacts.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("custom XDG path %s was not used: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); !os.IsNotExist(err) {
		t.Fatalf("OpenCode installer wrote outside custom XDG home: %v", err)
	}
	instruction := filepath.Join(xdg, "opencode", "instructions.md")
	writeRoutingTestFile(t, instruction, []byte("managed integration instruction"))
	if err := cleanVelaArtifacts("opencode", home, filepath.Join(home, "project")); err != nil {
		t.Fatalf("clean Vela artifacts: %v", err)
	}
	if _, err := os.Stat(instruction); !os.IsNotExist(err) {
		t.Fatalf("Vela cleanup did not use custom XDG instructions path: %v", err)
	}
}

func TestRoutingAllowsExternalXDGConfigRoot(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatalf("install OpenCode routing into external XDG root: %v", err)
	}
	for _, path := range []string{
		filepath.Join(xdg, "opencode", "opencode.json"),
		filepath.Join(xdg, "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md"),
		filepath.Join(xdg, "rotta", "managed-artifacts.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("routing asset outside HOME %s was not created: %v", path, err)
		}
	}
	escape := t.TempDir()
	link := filepath.Join(xdg, "escape")
	if err := os.Symlink(escape, link); err != nil {
		t.Fatal(err)
	}
	if err := validateManagedParents(home, filepath.Join(link, "SKILL.md")); err == nil {
		t.Fatal("external XDG symlink escape was accepted")
	}
	if err := validateManagedParents(home, filepath.Join(xdg, "..", "outside", "SKILL.md")); err == nil {
		t.Fatal("external XDG traversal was accepted")
	}
}

func installerTransactionCount(t *testing.T, state string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(state, "rotta", "installer-transactions"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func assertRoutingGlobalFields(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	compaction := config["compaction"].(map[string]interface{})
	if compaction["auto"] != false || compaction["buffer"] != float64(77) || compaction["user"] != "keep" {
		t.Fatalf("compaction changed: %#v", compaction)
	}
	toolOutput := config["tool_output"].(map[string]interface{})
	if toolOutput["max_lines"] != float64(9) || toolOutput["user"] != "keep" {
		t.Fatalf("tool output changed: %#v", toolOutput)
	}
	if arbitrary := config["arbitrary"].(map[string]interface{}); arbitrary["nested"] != true {
		t.Fatalf("arbitrary config changed: %#v", arbitrary)
	}
}

func readRoutingConfig(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func assertNoRoutingBackupEntries(t *testing.T, backupDir, xdg string) {
	t.Helper()
	manifest, err := loadBackupManifest(filepath.Join(backupDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append([]string{
		filepath.Join(xdg, "opencode", "opencode.json"),
		filepath.Join(xdg, "rotta", "managed-artifacts.json"),
	}, filepath.Join(xdg, "opencode", "skills", "rotta-next")) {
		if containsString(manifest.BackedUpPaths, path) || containsString(manifest.MissingPaths, path) {
			t.Fatalf("reconciled routing created backup entry for %s: %#v", path, manifest)
		}
	}
	if _, err := os.Stat(filepath.Join(backupDir, "agents", "opencode")); !os.IsNotExist(err) {
		t.Fatalf("reconciled routing created an OpenCode agent backup: %v", err)
	}
}

func routingStateEntries(manifest managedArtifactsManifest, xdg string) map[string]string {
	entries := map[string]string{}
	for path, digest := range manifest.Files {
		if isWithin(path, xdg) {
			entries[path] = digest
		}
	}
	return entries
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAll(value string, terms ...string) bool {
	for _, term := range terms {
		if !strings.Contains(value, term) {
			return false
		}
	}
	return true
}

func writeRoutingTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
