package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiRoutingDefaultCustomDisabledAndOwnership(t *testing.T) {
	home := t.TempDir()
	defaultOpts := Options{PiModelRouting: ModelRoutingEnabled}
	if _, err := installPi(defaultOpts, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, DefaultPiRouting(), DefaultPiRoutingEfforts())
	assertPiSettings(t, home, DefaultPiOrchestratorModel(), DefaultPiOrchestratorEffort())
	if err := os.WriteFile(piSettingsPath(home), []byte(`{"large":9007199254740993,"nested":{"keep":true},"defaultModel":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	custom := DefaultPiRouting()
	efforts := DefaultPiRoutingEfforts()
	custom["reviewer"] = "anthropic/claude-sonnet-4-5"
	efforts["reviewer"] = "medium"
	if _, err := installPi(Options{PiModelRouting: ModelRoutingCustom, PiModelRoutingModels: custom, PiModelRoutingEfforts: efforts, PiOrchestratorModel: "anthropic/claude-sonnet-4-5", PiOrchestratorEffort: "high"}, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, custom, efforts)
	assertPiSettings(t, home, "anthropic/claude-sonnet-4-5", "high")
	settingsData, err := os.ReadFile(piSettingsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settingsData), "9007199254740993") || !strings.Contains(string(settingsData), `"nested"`) {
		t.Fatalf("settings merge did not preserve raw user keys: %s", settingsData)
	}
	if _, err := installPi(Options{PiModelRouting: ModelRoutingDisabled}, home); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, map[string]string{}, nil)
	assertPiSettings(t, home, "anthropic/claude-sonnet-4-5", "high")
	if err := os.WriteFile(piModelRoutingPath(home), []byte("user owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(defaultOpts, home); err == nil {
		t.Fatal("modified Pi routing was overwritten")
	}
}

func TestPiSolContextWindowDefaultPreservesUserModelsAndDisabledLeavesItUntouched(t *testing.T) {
	home := t.TempDir()
	modelsPath := piModelsPath(home)
	if err := os.MkdirAll(filepath.Dir(modelsPath), 0o750); err != nil {
		t.Fatal(err)
	}
	original := `{"large":9007199254740993,"providers":{"openai-codex":{"keep":true,"modelOverrides":{"other":{"contextWindow":42},"gpt-6-sol":{"keep":"value","contextWindow":123}}}},"top":{"keep":true}}`
	if err := os.WriteFile(modelsPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(Options{PiModelRouting: ModelRoutingEnabled}, home); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "9007199254740993") || !strings.Contains(string(data), `"keep": "value"`) || !strings.Contains(string(data), `"other"`) || !strings.Contains(string(data), `"top"`) {
		t.Fatalf("models merge did not preserve user data: %s", data)
	}
	assertPiSolContextWindow(t, home, 1000000)

	beforeDisabled := string(data)
	if _, err := installPi(Options{PiModelRouting: ModelRoutingDisabled}, home); err != nil {
		t.Fatal(err)
	}
	afterDisabled, err := os.ReadFile(modelsPath)
	if err != nil || string(afterDisabled) != beforeDisabled {
		t.Fatalf("disabled routing changed models.json: %v\n%s", err, afterDisabled)
	}
}

func TestPiModelsFailureRollsBackSettingsAndModels(t *testing.T) {
	home := t.TempDir()
	settingsPath := piSettingsPath(home)
	modelsPath := piModelsPath(home)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		t.Fatal(err)
	}
	settingsBefore := []byte(`{"defaultProvider":"user","defaultModel":"model","defaultThinkingLevel":"high"}`)
	modelsBefore := []byte(`{"providers":[]}`)
	if err := os.WriteFile(settingsPath, settingsBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelsPath, modelsBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(Options{PiModelRouting: ModelRoutingEnabled}, home); err == nil {
		t.Fatal("invalid models.json succeeded")
	}
	settingsAfter, settingsErr := os.ReadFile(settingsPath)
	modelsAfter, modelsErr := os.ReadFile(modelsPath)
	if settingsErr != nil || string(settingsAfter) != string(settingsBefore) {
		t.Fatalf("settings rollback = %q, %v", settingsAfter, settingsErr)
	}
	if modelsErr != nil || string(modelsAfter) != string(modelsBefore) {
		t.Fatalf("models rollback = %q, %v", modelsAfter, modelsErr)
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
	settingsBefore, err := os.ReadFile(piSettingsPath(home))
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
	settingsAfter, err := os.ReadFile(piSettingsPath(home))
	if err != nil || string(settingsAfter) != string(settingsBefore) {
		t.Fatalf("settings rollback = %q, %v", settingsAfter, err)
	}
	foundRouting, foundSettings, foundModels := false, false, false
	for _, path := range piBackupPaths(home) {
		foundRouting = foundRouting || path == piModelRoutingPath(home)
		foundSettings = foundSettings || path == piSettingsPath(home)
		foundModels = foundModels || path == piModelsPath(home)
	}
	if !foundRouting || !foundSettings || !foundModels {
		t.Fatal("backup omitted Pi routing profile, settings, or models")
	}
}

func TestPiBackupRestoreRestoresModelRoutingProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	custom := DefaultPiRouting()
	efforts := DefaultPiRoutingEfforts()
	custom["operations"] = "anthropic/claude-haiku"
	efforts["operations"] = "high"
	if _, err := installPi(Options{PiModelRouting: ModelRoutingCustom, PiModelRoutingModels: custom, PiModelRoutingEfforts: efforts, PiOrchestratorModel: "anthropic/claude-sonnet", PiOrchestratorEffort: "medium"}, home); err != nil {
		t.Fatal(err)
	}
	backup, err := Backup(Options{Target: "pi", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piModelRoutingPath(home), []byte(`{"version":1,"roles":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piSettingsPath(home), []byte(`{"defaultModel":"changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piModelsPath(home), []byte(`{"providers":{"openai-codex":{"modelOverrides":{"gpt-6-sol":{"contextWindow":1}}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	assertPiRouting(t, home, custom, efforts)
	assertPiSettings(t, home, "anthropic/claude-sonnet", "medium")
	assertPiSolContextWindow(t, home, 1000000)
}

func TestPiCustomRoutingRequiresAllAndOnlyRolesAndValidEffort(t *testing.T) {
	custom := DefaultPiRouting()
	delete(custom, "operations")
	if _, err := resolvePiRouting(ModelRoutingCustom, custom, DefaultPiRoutingEfforts()); err == nil {
		t.Fatal("incomplete custom profile accepted")
	}
	custom = DefaultPiRouting()
	custom["unknown"] = "x/y"
	if _, err := resolvePiRouting(ModelRoutingCustom, custom, DefaultPiRoutingEfforts()); err == nil {
		t.Fatal("unknown custom role accepted")
	}
	efforts := DefaultPiRoutingEfforts()
	efforts["reviewer"] = "extreme"
	if _, err := resolvePiRouting(ModelRoutingCustom, DefaultPiRouting(), efforts); err == nil {
		t.Fatal("invalid custom effort accepted")
	}
	for _, badModel := range []string{"bad\fmodel/name", "bad\uFEFFmodel/name"} {
		custom = DefaultPiRouting()
		custom["reviewer"] = badModel
		if _, err := resolvePiRouting(ModelRoutingCustom, custom, DefaultPiRoutingEfforts()); err == nil {
			t.Fatalf("model containing JS-whitespace %q accepted", badModel)
		}
		if _, err := resolvePiOrchestratorRouting(ModelRoutingCustom, badModel, "low"); err == nil {
			t.Fatalf("orchestrator model containing JS-whitespace %q accepted", badModel)
		}
	}
	if _, err := resolvePiOrchestratorRouting(ModelRoutingCustom, "openai-codex/gpt-5.6-sol", "extreme"); err == nil {
		t.Fatal("invalid orchestrator effort accepted")
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
	assertPiRouting(t, home, map[string]string{}, nil)
}

func assertPiSettings(t *testing.T, home, wantModel, wantEffort string) {
	t.Helper()
	data, err := os.ReadFile(piSettingsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse settings: %v %s", err, data)
	}
	parts := strings.SplitN(wantModel, "/", 2)
	var provider, model, effort string
	_ = json.Unmarshal(got["defaultProvider"], &provider)
	_ = json.Unmarshal(got["defaultModel"], &model)
	_ = json.Unmarshal(got["defaultThinkingLevel"], &effort)
	if provider != parts[0] || model != parts[1] || effort != wantEffort {
		t.Fatalf("settings = %s, want %s %s", data, wantModel, wantEffort)
	}
}

func assertPiSolContextWindow(t *testing.T, home string, want int) {
	t.Helper()
	data, err := os.ReadFile(piModelsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var models struct {
		Providers map[string]struct {
			ModelOverrides map[string]struct {
				ContextWindow int `json:"contextWindow"`
			} `json:"modelOverrides"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &models); err != nil {
		t.Fatalf("parse models: %v %s", err, data)
	}
	for _, name := range []string{"gpt-6-sol", "gpt-6-luna"} {
		if got := models.Providers["openai-codex"].ModelOverrides[name].ContextWindow; got != want {
			t.Fatalf("%s context window = %d, want %d: %s", name, got, want, data)
		}
	}
}

func assertPiRouting(t *testing.T, home string, want map[string]string, wantEfforts map[string]string) {
	t.Helper()
	data, err := os.ReadFile(piModelRoutingPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version int                            `json:"version"`
		Roles   map[string]PiRoutingAssignment `json:"roles"`
	}
	if err := json.Unmarshal(data, &got); err != nil || got.Version != 1 {
		t.Fatalf("parse profile: %v %s", err, data)
	}
	if len(got.Roles) != len(want) {
		t.Fatalf("roles = %#v, want %#v", got.Roles, want)
	}
	for role, model := range want {
		if got.Roles[role].Model != model {
			t.Fatalf("%s model = %q, want %q", role, got.Roles[role].Model, model)
		}
		wantEffort := "low"
		if wantEfforts != nil {
			wantEffort = wantEfforts[role]
		}
		if got.Roles[role].Effort != wantEffort {
			t.Fatalf("%s effort = %q, want %q", role, got.Roles[role].Effort, wantEffort)
		}
	}
}
