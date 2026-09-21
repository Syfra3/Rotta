package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomRoutingWritesExactModelsAndTransitionsToDisabled(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	custom := DefaultOpenCodeRouting()
	custom["rotta-impl"] = "anthropic/claude-sonnet"

	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingCustom, ModelRoutingModels: custom}); err != nil {
		t.Fatalf("custom install: %v", err)
	}
	path := filepath.Join(xdg, "opencode", "opencode.json")
	config := readRoutingConfig(t, path)
	if got := config["agent"].(map[string]interface{})["rotta-impl"].(map[string]interface{})["model"]; got != custom["rotta-impl"] {
		t.Fatalf("custom model = %q, want %q", got, custom["rotta-impl"])
	}
	manifest, err := readManagedArtifactsManifest(filepath.Join(xdg, "rotta", "managed-artifacts.json"))
	if err != nil {
		t.Fatalf("read ownership state: %v", err)
	}
	if got := manifest.Files[openCodeModelOwnershipKey(path, "rotta-impl")]; got != contentDigest([]byte(custom["rotta-impl"])) {
		t.Fatalf("custom ownership digest = %q", got)
	}
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("disable custom routing: %v", err)
	}
	config = readRoutingConfig(t, path)
	for _, role := range routingRolesForTest() {
		if _, exists := config["agent"].(map[string]interface{})[role].(map[string]interface{})["model"]; exists {
			t.Fatalf("%s model remains after disabling custom routing", role)
		}
	}
	if _, err := os.Stat(filepath.Join(xdg, "rotta", "managed-artifacts.json")); err != nil {
		t.Fatalf("read ownership state: %v", err)
	}
}

func TestRoutingTransitionsBetweenDefaultCustomAndDisabledWithoutReplayBackup(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	state := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", state)
	custom := DefaultOpenCodeRouting()
	custom["rotta-review"] = "anthropic/claude-opus"

	requests := []Options{
		{Target: "opencode", ModelRouting: ModelRoutingEnabled},
		{Target: "opencode", ModelRouting: ModelRoutingCustom, ModelRoutingModels: custom},
		{Target: "opencode", ModelRouting: ModelRoutingEnabled},
		{Target: "opencode", ModelRouting: ModelRoutingCustom, ModelRoutingModels: custom},
		{Target: "opencode", ModelRouting: ModelRoutingDisabled},
	}
	for index, request := range requests {
		if _, err := Install(request); err != nil {
			t.Fatalf("transition %d: %v", index, err)
		}
	}
	if got := installerTransactionCount(t, state); got != len(requests) {
		t.Fatalf("transition backups = %d, want %d", got, len(requests))
	}
	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatalf("repeat disabled install: %v", err)
	}
	if got := installerTransactionCount(t, state); got != len(requests) {
		t.Fatalf("disabled replay created backup: %d", got)
	}
}

func TestCustomRoutingExactReplayDoesNotMutateConfigurationOrOwnership(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	state := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", state)
	custom := DefaultOpenCodeRouting()
	custom["rotta-impl"] = "anthropic/claude-sonnet"
	request := Options{Target: "opencode", ModelRouting: ModelRoutingCustom, ModelRoutingModels: custom}
	if _, err := Install(request); err != nil {
		t.Fatalf("initial custom install: %v", err)
	}
	configPath := filepath.Join(xdg, "opencode", "opencode.json")
	ownershipPath := filepath.Join(xdg, "rotta", "managed-artifacts.json")
	configBefore, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	ownershipBefore, err := os.ReadFile(ownershipPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(request); err != nil {
		t.Fatalf("custom replay: %v", err)
	}
	configAfter, _ := os.ReadFile(configPath)
	ownershipAfter, _ := os.ReadFile(ownershipPath)
	if string(configAfter) != string(configBefore) || string(ownershipAfter) != string(ownershipBefore) {
		t.Fatal("exact custom replay mutated configuration or ownership")
	}
	if got := installerTransactionCount(t, state); got != 1 {
		t.Fatalf("exact custom replay created backup count %d, want 1", got)
	}
}

func TestCustomRoutingRollbackRestoresExactConfigurationAndOwnership(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	path := filepath.Join(xdg, "opencode", "opencode.json")
	before := []byte(`{"theme":"keep","agent":{}}`)
	writeRoutingTestFile(t, path, before)
	custom := DefaultOpenCodeRouting()
	custom["rotta-ops"] = "anthropic/claude-haiku"
	routingFailureHook = func(stage string) error {
		if stage == "after-managed-state" {
			return errors.New("injected custom routing failure")
		}
		return nil
	}
	t.Cleanup(func() { routingFailureHook = nil })

	if _, err := Install(Options{Target: "opencode", ModelRouting: ModelRoutingCustom, ModelRoutingModels: custom}); err == nil {
		t.Fatal("custom routing succeeded despite injected failure")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(before) {
		t.Fatalf("custom rollback config = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(xdg, "rotta", "managed-artifacts.json")); !os.IsNotExist(err) {
		t.Fatalf("custom rollback left ownership state: %v", err)
	}
}

func routingRolesForTest() []string {
	return []string{"rotta-orchestrator", "rotta-architect", "rotta-review", "rotta-impl", "rotta-ops", "rotta-explore", "rotta-cleaner"}
}
