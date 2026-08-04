package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// REQ-089 → SCN-624 → TestSCN624_OpenCodeMCPStatusSeparatesDirectDiscovery
func TestSCN624_OpenCodeMCPStatusSeparatesDirectDiscovery(t *testing.T) {
	// Scenario: MCP status distinguishes direct discovery from OpenCode resolution
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir)
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   filepath.Join(home, "project"),
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	status := result.MCPStatuses["opencode"]["context7"]
	if status.CommandResolution.Status != MCPObservationCompleted {
		t.Fatalf("command resolution = %#v, want completed", status.CommandResolution)
	}
	if status.FileWrite.Status != MCPObservationCompleted {
		t.Fatalf("file write = %#v, want completed", status.FileWrite)
	}
	if status.SchemaValidity.Status != MCPObservationCompleted {
		t.Fatalf("schema validity = %#v, want completed", status.SchemaValidity)
	}
	if status.OpenCodeServerResolution.Status != MCPObservationNotObservable {
		t.Fatalf("OpenCode server resolution = %#v, want not_observable without opencode CLI", status.OpenCodeServerResolution)
	}
	if status.ToolDiscovery.Status != MCPObservationCompleted || status.ToolDiscovery.Source != MCPDiscoverySourceDirectServer {
		t.Fatalf("tool discovery = %#v, want completed direct-server discovery", status.ToolDiscovery)
	}
	if want := filepath.Join(binDir, "npx"); status.ResolvedCommandPath != want {
		t.Fatalf("resolved command path = %q, want %q", status.ResolvedCommandPath, want)
	}

	evidencePath := filepath.Join(result.BackupDir, "mcp-status.json")
	evidence, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read host-local transaction evidence: %v", err)
	}
	var recorded map[string]map[string]MCPStatusResult
	if err := json.Unmarshal(evidence, &recorded); err != nil {
		t.Fatalf("decode host-local transaction evidence: %v", err)
	}
	if got := recorded["opencode"]["context7"]; !reflect.DeepEqual(got, status) {
		t.Fatalf("recorded status = %#v, want %#v", got, status)
	}
}
