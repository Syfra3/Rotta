package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPiRoutingDefaultCustomDisabledAndOwnership(t *testing.T) {
	home := t.TempDir()
	defaultOpts := Options{PiModelRouting: ModelRoutingEnabled}
	if _, err := installPi(defaultOpts, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, DefaultPiRouting())
	custom := DefaultPiRouting()
	custom["reviewer"] = "anthropic/claude-sonnet-4-5"
	if _, err := installPi(Options{PiModelRouting: ModelRoutingCustom, PiModelRoutingModels: custom}, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, custom)
	if _, err := installPi(Options{PiModelRouting: ModelRoutingDisabled}, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, map[string]string{})
	if err := os.WriteFile(piModelRoutingPath(home), []byte("user owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(defaultOpts, home); err == nil {
		t.Fatal("modified Pi routing was overwritten")
	}
}

func TestPiRoutingRollbackAndBackupIncludeProfile(t *testing.T) {
	home := t.TempDir()
	if _, err := installPi(Options{}, home); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(piModelRoutingPath(home))
	if err != nil {
		t.Fatal(err)
	}
	piInstallFailureHook = func(stage string) error {
		if stage == "after-assets" {
			return os.ErrInvalid
		}
		return nil
	}
	t.Cleanup(func() { piInstallFailureHook = nil })
	if _, err := installPi(Options{PiModelRouting: ModelRoutingDisabled}, home); err == nil {
		t.Fatal("failure succeeded")
	}
	after, err := os.ReadFile(piModelRoutingPath(home))
	if err != nil || string(after) != string(before) {
		t.Fatalf("rollback = %q, %v", after, err)
	}
	found := false
	for _, path := range piBackupPaths(home) {
		found = found || path == piModelRoutingPath(home)
	}
	if !found {
		t.Fatal("backup omitted Pi routing profile")
	}
}

func TestPiBackupRestoreRestoresModelRoutingProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	custom := DefaultPiRouting()
	custom["operations"] = "anthropic/claude-haiku"
	if _, err := installPi(Options{PiModelRouting: ModelRoutingCustom, PiModelRoutingModels: custom}, home); err != nil {
		t.Fatal(err)
	}
	backup, err := Backup(Options{Target: "pi", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piModelRoutingPath(home), []byte(`{"version":1,"roles":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, custom)
}

func TestPiCustomRoutingRequiresAllAndOnlyRoles(t *testing.T) {
	custom := DefaultPiRouting()
	delete(custom, "operations")
	if _, err := resolvePiRouting(ModelRoutingCustom, custom); err == nil {
		t.Fatal("incomplete custom profile accepted")
	}
	custom = DefaultPiRouting()
	custom["unknown"] = "x/y"
	if _, err := resolvePiRouting(ModelRoutingCustom, custom); err == nil {
		t.Fatal("unknown custom role accepted")
	}
}

func TestAllKeepsPiAndOpenCodeRoutingIndependent(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	if _, err := Install(Options{Target: "all", ModelRouting: ModelRoutingEnabled, PiModelRouting: ModelRoutingDisabled}); err != nil {
		t.Fatal(err)
	}
	config := readRoutingConfig(t, filepath.Join(xdg, "opencode", "opencode.json"))
	if got := config["agent"].(map[string]interface{})["rotta-impl"].(map[string]interface{})["model"]; got != "openai/gpt-5.6-terra" {
		t.Fatalf("OpenCode model = %q", got)
	}
	assertPiRouting(t, home, map[string]string{})
}

func assertPiRouting(t *testing.T, home string, want map[string]string) {
	t.Helper()
	data, err := os.ReadFile(piModelRoutingPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version int               `json:"version"`
		Roles   map[string]string `json:"roles"`
	}
	if err := json.Unmarshal(data, &got); err != nil || got.Version != 1 {
		t.Fatalf("parse profile: %v %s", err, data)
	}
	if len(got.Roles) != len(want) {
		t.Fatalf("roles = %#v, want %#v", got.Roles, want)
	}
	for role, model := range want {
		if got.Roles[role] != model {
			t.Fatalf("%s = %q, want %q", role, got.Roles[role], model)
		}
	}
}
