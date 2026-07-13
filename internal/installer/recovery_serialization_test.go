package installer

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// REQ-028 → SCN-231 → TestSCN231_SerializesRecoveredManagedMCPCommandAsBareCommand
func TestSCN231_SerializesRecoveredManagedMCPCommandAsBareCommand(t *testing.T) {
	// Scenario: Serialize a recovered managed MCP executable as a canonical bare command
	configPath := filepath.Join(t.TempDir(), "ancora.json")
	resolvedLocation := "/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora"
	writeTestFile(t, configPath, []byte(`{"command":"/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora","args":["mcp"]}`))

	if err := serializeRecoveredManagedMCPCommand(configPath, "", "ancora"); err != nil {
		t.Fatalf("serialize recovered managed MCP command: %v", err)
	}

	var config struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(mustReadFile(t, configPath), &config); err != nil {
		t.Fatalf("decode recovered managed MCP configuration: %v", err)
	}
	if config.Command != "ancora" {
		t.Fatalf("expected canonical bare command %q, got %q", "ancora", config.Command)
	}
	if config.Command == resolvedLocation {
		t.Fatalf("serialized command retained resolved executable location %q", resolvedLocation)
	}
}
