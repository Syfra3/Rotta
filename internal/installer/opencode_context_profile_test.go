package installer

import (
	"path/filepath"
	"testing"
)

// REQ-085 → SCN-614 → TestSCN614_OpenCodeContextProfileUsesCurrentSchema
func TestSCN614_OpenCodeContextProfileUsesCurrentSchema(t *testing.T) {
	// Scenario: OpenCode context limits retain complete local failure evidence
	home := t.TempDir()
	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatalf("installOpenCode() error = %v", err)
	}
	config := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	compaction, ok := config["compaction"].(map[string]interface{})
	if !ok {
		t.Fatalf("compaction = %#v, want v2 configuration object", config["compaction"])
	}
	for key, want := range map[string]interface{}{"auto": true, "prune": true, "buffer": float64(10000)} {
		if got := compaction[key]; got != want {
			t.Errorf("compaction.%s = %#v, want %#v", key, got, want)
		}
	}
	toolOutput, ok := config["tool_output"].(map[string]interface{})
	if !ok {
		t.Fatalf("tool_output = %#v, want v2 configuration object", config["tool_output"])
	}
	for key, want := range map[string]interface{}{"max_lines": float64(120), "max_bytes": float64(12288)} {
		if got := toolOutput[key]; got != want {
			t.Errorf("tool_output.%s = %#v, want %#v", key, got, want)
		}
	}
}
