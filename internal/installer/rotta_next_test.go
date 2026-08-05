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
	for _, name := range []string{"rotta-orchestrator", "rotta-explore", "rotta-impl", "rotta-review", "rotta-ops"} {
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
