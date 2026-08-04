package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-089 → SCN-625 → TestSCN625_VelaFailureRestoresOnlyScopedOpenCodeManagedChanges
func TestSCN625_VelaFailureRestoresOnlyScopedOpenCodeManagedChanges(t *testing.T) {
	// Scenario: A Vela failure rolls back only its own selected managed changes
	home := t.TempDir()
	backupDir := filepath.Join(home, ".local", "state", "rotta", "installer-transactions", "transaction")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	baseline := []byte(`{"mcp":{"ancora":{"command":["ancora","mcp"],"enabled":true},"user-owned":{"command":["user-mcp"]}},"theme":"dark"}`)
	writeTestFile(t, configPath, baseline)

	t.Run("safe restore retains successful integration", func(t *testing.T) {
		transaction, err := beginOpenCodeVelaTransaction(backupDir, configPath)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, configPath, openCodeConfigWithVela(t, baseline, false))

		rollback, err := transaction.recover(errors.New("vela configuration failed"))
		if err != nil {
			t.Fatal(err)
		}
		if !rollback.Restored || rollback.ConcurrentModification {
			t.Fatalf("rollback = %#v, want safe scoped restore", rollback)
		}
		if got := string(mustReadFile(t, configPath)); got != string(baseline) {
			t.Fatalf("OpenCode config = %s, want only Vela change restored to %s", got, baseline)
		}
		if _, err := os.Stat(filepath.Join(backupDir, "vela-rollback.json")); err != nil {
			t.Fatalf("scoped rollback evidence missing from host-local transaction: %v", err)
		}
	})

	t.Run("concurrent modification refuses restore without broad rollback", func(t *testing.T) {
		writeTestFile(t, configPath, baseline)
		transaction, err := beginOpenCodeVelaTransaction(backupDir, configPath)
		if err != nil {
			t.Fatal(err)
		}
		concurrent := openCodeConfigWithVela(t, baseline, true)
		writeTestFile(t, configPath, concurrent)

		rollback, err := transaction.recover(errors.New("vela configuration failed"))
		if err == nil || !rollback.ConcurrentModification || rollback.Restored || !strings.Contains(err.Error(), "concurrent modification") {
			t.Fatalf("rollback = %#v, error = %v, want concurrent-modification refusal", rollback, err)
		}
		if got := string(mustReadFile(t, configPath)); got != string(concurrent) {
			t.Fatalf("OpenCode config was broadly rolled back: got %s, want %s", got, concurrent)
		}
		evidence, err := os.ReadFile(filepath.Join(backupDir, "vela-rollback.json"))
		if err != nil || !strings.Contains(string(evidence), `"concurrent_modification": true`) {
			t.Fatalf("concurrent scoped rollback evidence = %s, error = %v", evidence, err)
		}
	})

	t.Run("user-owned Vela entry is not treated as a managed change", func(t *testing.T) {
		writeTestFile(t, configPath, baseline)
		transaction, err := beginOpenCodeVelaTransaction(backupDir, configPath)
		if err != nil {
			t.Fatal(err)
		}
		userOwned := []byte(`{"mcp":{"ancora":{"command":["ancora","mcp"],"enabled":true},"user-owned":{"command":["user-mcp"]},"vela":{"command":["user-vela"]}},"theme":"dark"}`)
		writeTestFile(t, configPath, userOwned)

		rollback, err := transaction.recover(errors.New("vela configuration failed"))
		if err == nil || !rollback.ConcurrentModification || rollback.Restored {
			t.Fatalf("rollback = %#v, error = %v, want user-owned Vela refusal", rollback, err)
		}
		if got := string(mustReadFile(t, configPath)); got != string(userOwned) {
			t.Fatalf("user-owned Vela entry was overwritten: got %s, want %s", got, userOwned)
		}
	})
}

// REQ-089 → SCN-625 → TestSCN625_InstallerRetainsSuccessfulOpenCodeIntegrationAfterVelaFailure
func TestSCN625_InstallerRetainsSuccessfulOpenCodeIntegrationAfterVelaFailure(t *testing.T) {
	// Scenario: A Vela failure rolls back only its own selected managed changes
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/bin")
	writeTestFile(t, configPath, []byte(`{"mcp":{"ancora":{"command":["ancora","mcp"],"enabled":true}}}`))
	writeExecutable(t, filepath.Join(binDir, "ancora"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "vela"), "#!/bin/sh\nexit 42\n")

	result, err := Install(Options{
		Target:      "opencode",
		ProjectPath: filepath.Join(home, "project"),
		SetupAncora: true,
		SetupVela:   true,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want Vela failure")
	}
	if result == nil {
		t.Fatal("Install() result = nil, want retained successful OpenCode integration state")
	}
	if result.AncoraBin != filepath.Join(binDir, "ancora") {
		t.Fatalf("Ancora bin = %q, want successful setup path %q", result.AncoraBin, filepath.Join(binDir, "ancora"))
	}
	if got := string(mustReadFile(t, configPath)); !strings.Contains(got, `"ancora"`) {
		t.Fatalf("successful Ancora config was not retained: %s", got)
	}
	if status := result.MCPStatuses["opencode"]["vela"]; status.Status != MCPStatusFailed {
		t.Fatalf("Vela status = %#v, want failed scoped-recovery status", status)
	}
	if _, err := os.Stat(filepath.Join(result.BackupDir, "vela-rollback.json")); err != nil {
		t.Fatalf("scoped rollback result is not in host-local transaction evidence: %v", err)
	}
}

// REQ-089 → SCN-625 → TestSCN625_UnavailableVelaDoesNotBroadlyRestoreOpenCodeConfig
func TestSCN625_UnavailableVelaDoesNotBroadlyRestoreOpenCodeConfig(t *testing.T) {
	// Scenario: A Vela failure rolls back only its own selected managed changes
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/bin")
	writeTestFile(t, configPath, []byte(`{"theme":"dark"}`))
	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
if [ "$1" = setup ]; then
  mkdir -p "$HOME/.config/opencode"
  printf '{"theme":"dark","mcp":{"ancora":{"command":["ancora","mcp"],"enabled":true}}}' > "$HOME/.config/opencode/opencode.json"
fi
`)

	result, err := Install(Options{
		Target:      "opencode",
		ProjectPath: filepath.Join(home, "project"),
		SetupAncora: true,
		SetupVela:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadFile(t, configPath)); !strings.Contains(got, `"ancora"`) {
		t.Fatalf("successful Ancora config was broadly restored away: %s", got)
	}
	if status := result.MCPStatuses["opencode"]["vela"]; status.Status != MCPStatusDegraded {
		t.Fatalf("Vela status = %#v, want degraded without rolling back Ancora", status)
	}
}

// REQ-089 → SCN-625 → TestSCN625_Context7SuccessSurvivesScopedVelaRecovery
func TestSCN625_Context7SuccessSurvivesScopedVelaRecovery(t *testing.T) {
	// Scenario: A Vela failure rolls back only its own selected managed changes
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/bin")
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
/bin/sed -i 's/"mcp": {/"mcp": {"vela":{"command":["vela","mcp"],"args":["mcp"],"enabled":true},/' "$HOME/.config/opencode/opencode.json"
exit 42
`)

	result, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   filepath.Join(home, "project"),
		SetupContext7: true,
		SetupVela:     true,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want Vela failure")
	}
	if result == nil {
		t.Fatal("Install() result = nil, want retained Context7 result")
	}
	if result.Context7.Status != Context7StatusConfigured || !result.Context7.HealthRan {
		t.Fatalf("Context7 result = %#v, want successful configured status before Vela failure", result.Context7)
	}
	config := string(mustReadFile(t, configPath))
	if !strings.Contains(config, `"context7"`) || strings.Contains(config, `"vela"`) {
		t.Fatalf("OpenCode config = %s, want retained Context7 and rolled-back Vela only", config)
	}
	if status := result.MCPStatuses["opencode"]["context7"]; status.Status != MCPStatusConfigured {
		t.Fatalf("Context7 status = %#v, want retained configured status", status)
	}
	if _, err := os.Stat(filepath.Join(result.BackupDir, "vela-rollback.json")); err != nil {
		t.Fatalf("scoped Vela rollback evidence missing: %v", err)
	}
}

func openCodeConfigWithVela(t *testing.T, baseline []byte, concurrent bool) []byte {
	t.Helper()
	var config map[string]interface{}
	if err := json.Unmarshal(baseline, &config); err != nil {
		t.Fatal(err)
	}
	mcp := config["mcp"].(map[string]interface{})
	mcp["vela"] = map[string]interface{}{"command": []string{"vela", "mcp"}, "args": []string{"mcp"}, "enabled": true}
	if concurrent {
		config["theme"] = "light"
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
