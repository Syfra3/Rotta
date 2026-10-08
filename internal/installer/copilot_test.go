package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func copilotTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	return home
}

func TestCopilotInstallUsesSharedPolicyAndManagedOwnership(t *testing.T) {
	home := copilotTestHome(t)
	opts := Options{Target: "copilot", ProjectPath: home}
	first, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hosts["copilot"].Status != HostInstallStatusInstalled || len(first.AgentBackupDirs) != 1 {
		t.Fatalf("missing installed host or backup: %+v", first)
	}
	core := filepath.Join(home, ".copilot", "rotta-next", "rotta-core", "SKILL.md")
	stub := filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")
	text, err := os.ReadFile(stub)
	if err != nil || !strings.Contains(string(text), core) || !strings.Contains(string(text), "name: rotta-orchestrator") {
		t.Fatalf("agent cannot load shared core: %s, %v", text, err)
	}
	if data, err := os.ReadFile(core); err != nil || !strings.Contains(string(data), "## Task Capsules") {
		t.Fatalf("shared core not rendered: %v", err)
	}
	role := filepath.Join(home, ".copilot", "rotta-next", "rotta-impl", "SKILL.md")
	if data, err := os.ReadFile(role); err != nil || !strings.Contains(string(data), "Implement and test one coherent slice") {
		t.Fatalf("shared role not rendered: %v", err)
	}
	if _, err := Install(opts); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	if err := os.WriteFile(stub, []byte("user edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(opts); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("edited managed agent should be refused: %v", err)
	}
	if data, _ := os.ReadFile(stub); string(data) != "user edit" {
		t.Fatalf("user edit overwritten: %s", data)
	}
}

func TestCopilotPreservesUnmanagedMCPAndReportsIt(t *testing.T) {
	home := copilotTestHome(t)
	path := copilotMCPPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"mcpServers":{"custom":{"command":"my-server"}}}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Install(Options{Target: "copilot", ProjectPath: home, SetupAncora: true, SetupVela: true, SetupContext7: true})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatalf("unmanaged MCP configuration overwritten: %s", data)
	}
	for _, name := range []string{"ancora", "vela", "context7"} {
		if status := result.MCPStatuses["copilot"][name]; status.Status != MCPStatusPreserved {
			t.Fatalf("%s: want preserved-unvalidated, got %+v", name, status)
		}
	}
}

func TestCopilotRefusesUnownedAndSymlinkedAgents(t *testing.T) {
	for _, kind := range []string{"unowned", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			home := copilotTestHome(t)
			agent := filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")
			if err := os.MkdirAll(filepath.Dir(agent), 0o750); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				other := filepath.Join(home, "user-agent.md")
				if err := os.WriteFile(other, []byte("user content"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, agent); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(agent, []byte("user content"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Install(Options{Target: "copilot", ProjectPath: home}); err == nil {
				t.Fatal("unowned/symlinked agent was accepted")
			}
			if kind == "unowned" {
				if data, _ := os.ReadFile(agent); string(data) != "user content" {
					t.Fatalf("user agent overwritten: %s", data)
				}
			}
		})
	}
}

func TestCopilotRefusesSymlinkedMCPConfig(t *testing.T) {
	home := copilotTestHome(t)
	path := copilotMCPPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(home, "user-mcp.json")
	if err := os.WriteFile(userFile, []byte("user content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(userFile, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Target: "copilot", ProjectPath: home, SetupAncora: true}); err == nil {
		t.Fatalf("symlinked MCP configuration accepted: %v", err)
	}
	if data, _ := os.ReadFile(userFile); string(data) != "user content" {
		t.Fatal("symlink target changed")
	}
}

func TestCopilotManagedMCPSelectionUpdatesAndClearsWithoutRuntimeClaims(t *testing.T) {
	home := copilotTestHome(t)
	opts := Options{Target: "copilot", ProjectPath: home, SetupAncora: true, SetupVela: true, SetupContext7: true}
	result, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ancora", "vela", "context7"} {
		if result.MCPStatuses["copilot"][name].Status != MCPStatusConfiguredPendingHealth {
			t.Fatalf("%s not pending verification: %+v", name, result.MCPStatuses["copilot"][name])
		}
	}
	data, err := os.ReadFile(copilotMCPPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil || len(config.MCPServers) != 3 {
		t.Fatalf("selected MCP config invalid: %s, %v", data, err)
	}
	vela, ok := config.MCPServers["vela"].(map[string]any)
	if !ok || vela["command"] != "vela" || !reflect.DeepEqual(vela["args"], []any{"serve", "--mcp"}) {
		t.Fatalf("Vela MCP entry must launch vela serve --mcp: %s", data)
	}
	if _, err := Install(Options{Target: "copilot", ProjectPath: home}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(copilotMCPPath(home))
	config.MCPServers = nil
	if err := json.Unmarshal(data, &config); err != nil || len(config.MCPServers) != 0 {
		t.Fatalf("deselected servers still enabled: %s, %v", data, err)
	}
}

func TestCopilotAllAndBackupRestore(t *testing.T) {
	home := copilotTestHome(t)
	result, err := Install(Options{Target: "all", ProjectPath: home})
	if err != nil {
		t.Fatal(err)
	}
	if len(selectedHosts("all")) != 5 || len(result.Hosts) != 5 || len(result.AgentBackupDirs) != 5 {
		t.Fatalf("all target must install each host once: hosts=%v backups=%v", result.Hosts, result.AgentBackupDirs)
	}
	stub := filepath.Join(home, ".copilot", "agents", "rotta-impl.agent.md")
	if _, err := os.Stat(stub); err != nil {
		t.Fatal(err)
	}
	backup, err := Backup(Options{Target: "copilot", ProjectPath: home})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stub, []byte("later edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(stub)
	if err != nil || !strings.Contains(string(data), "rotta-impl") {
		t.Fatalf("Copilot agent not restored: %s, %v", data, err)
	}
}
