//go:build legacy_v2

package installer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	previous, wasSet := os.LookupEnv("XDG_CONFIG_HOME")
	previousState, stateWasSet := os.LookupEnv("XDG_STATE_HOME")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("XDG_STATE_HOME")
	exitCode := m.Run()
	if wasSet {
		_ = os.Setenv("XDG_CONFIG_HOME", previous)
	}
	if stateWasSet {
		_ = os.Setenv("XDG_STATE_HOME", previousState)
	}
	os.Exit(exitCode)
}

// REQ-089 → SCN-622 → TestSCN622_InstallerSelectsDocumentedEffectiveOpenCodeConfig
func TestSCN622_InstallerSelectsDocumentedEffectiveOpenCodeConfig(t *testing.T) {
	// Scenario: The installer selects the documented effective OpenCode JSON or JSONC config
	const (
		globalSource   = "XDG global configuration"
		overrideSource = "OPENCODE_CONFIG override"
		projectSource  = "documented project configuration"
	)
	precedence := []string{globalSource, overrideSource, projectSource}

	tests := []struct {
		name           string
		setup          func(t *testing.T, home, project string) string
		wantSource     string
		wantFormat     string
		wantTheme      string
		wantUserMarker string
	}{
		{
			name: "XDG global JSON",
			setup: func(t *testing.T, home, _ string) string {
				t.Setenv("OPENCODE_CONFIG", "")
				path := filepath.Join(home, ".config", "opencode", "opencode.json")
				writeTestFile(t, path, []byte(`{"$schema":"https://opencode.ai/config.json","theme":"global","agent":{"user-agent":{"description":"keep global"},"build":{"description":"keep built-in"}}}`))
				return path
			},
			wantSource: globalSource, wantFormat: "JSON", wantTheme: "global", wantUserMarker: "keep global",
		},
		{
			name: "OPENCODE_CONFIG override JSONC",
			setup: func(t *testing.T, home, _ string) string {
				global := filepath.Join(home, ".config", "opencode", "opencode.json")
				writeTestFile(t, global, []byte(`{"theme":"inactive global","agent":{"user-agent":{"description":"keep inactive"}}}`))
				path := filepath.Join(home, "custom", "opencode.jsonc")
				writeTestFile(t, path, []byte("// keep this user comment\n{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"theme\": \"override\",\n  \"agent\": {\"user-agent\": {\"description\": \"keep override\"}, \"build\": {\"description\": \"keep built-in\"}},\n}\n"))
				t.Setenv("OPENCODE_CONFIG", path)
				return path
			},
			wantSource: overrideSource, wantFormat: "JSONC", wantTheme: "override", wantUserMarker: "keep override",
		},
		{
			name: "documented project JSONC",
			setup: func(t *testing.T, home, project string) string {
				t.Setenv("OPENCODE_CONFIG", "")
				global := filepath.Join(home, ".config", "opencode", "opencode.json")
				writeTestFile(t, global, []byte(`{"theme":"inactive global","agent":{"user-agent":{"description":"keep inactive"}}}`))
				path := filepath.Join(project, "opencode.jsonc")
				writeTestFile(t, path, []byte("/* keep this project comment */\n{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"theme\": \"project\",\n  \"agent\": {\"user-agent\": {\"description\": \"keep project\"}, \"build\": {\"description\": \"keep built-in\"}},\n}\n"))
				return path
			},
			wantSource: projectSource, wantFormat: "JSONC", wantTheme: "project", wantUserMarker: "keep project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := filepath.Join(home, "project")
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			effectivePath := tt.setup(t, home, project)
			globalPath := filepath.Join(home, ".config", "opencode", "opencode.json")
			inactiveBefore, err := os.ReadFile(globalPath)
			if err != nil && effectivePath != globalPath {
				t.Fatalf("read inactive global config: %v", err)
			}

			resolution, err := resolveOpenCodeConfig(Options{ProjectPath: project}, home)
			if err != nil {
				t.Fatalf("resolveOpenCodeConfig() error = %v", err)
			}
			if resolution.Path != effectivePath || resolution.Source != tt.wantSource || resolution.Format != tt.wantFormat || !reflect.DeepEqual(resolution.Precedence, precedence) {
				t.Fatalf("resolution = %#v, want path=%q source=%q format=%q precedence=%#v", resolution, effectivePath, tt.wantSource, tt.wantFormat, precedence)
			}

			result, err := Install(Options{Target: "opencode", ProjectPath: project})
			if err != nil {
				t.Fatalf("Install() error = %v", err)
			}
			if got := result.Hosts["opencode"].OpenCodeConfig; !reflect.DeepEqual(got, resolution) {
				t.Fatalf("reported OpenCode config = %#v, want %#v", got, resolution)
			}

			data, err := os.ReadFile(effectivePath)
			if err != nil {
				t.Fatalf("read effective config: %v", err)
			}
			got := string(data)
			for _, want := range []string{"\"rotta-orchestrator\"", "\"theme\": \"" + tt.wantTheme + "\"", tt.wantUserMarker, "keep built-in"} {
				if !strings.Contains(got, want) {
					t.Errorf("effective config missing %q:\n%s", want, got)
				}
			}
			if tt.wantFormat == "JSONC" && !strings.Contains(got, "comment") {
				t.Errorf("JSONC user comment was not preserved:\n%s", got)
			}
			if effectivePath != globalPath {
				inactiveAfter, err := os.ReadFile(globalPath)
				if err != nil {
					t.Fatalf("read inactive global config after install: %v", err)
				}
				if !reflect.DeepEqual(inactiveAfter, inactiveBefore) {
					t.Errorf("inactive global config was modified:\n%s", inactiveAfter)
				}
			}
		})
	}
}

// REQ-089 → SCN-622 → TestSCN622_InstallerUsesNonDefaultXDGConfigHome
func TestSCN622_InstallerUsesNonDefaultXDGConfigHome(t *testing.T) {
	// Scenario: The installer selects the documented effective OpenCode JSON or JSONC config
	home := t.TempDir()
	project := filepath.Join(home, "project")
	xdgConfigHome := filepath.Join(home, "custom-xdg")
	effectivePath := filepath.Join(xdgConfigHome, "opencode", "opencode.json")
	defaultPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	t.Setenv("OPENCODE_CONFIG", "")
	writeTestFile(t, effectivePath, []byte(`{"$schema":"https://opencode.ai/config.json","theme":"xdg","agent":{"user-agent":{"description":"keep xdg"},"build":{"description":"keep built-in"}}}`))
	writeTestFile(t, defaultPath, []byte(`{"theme":"default must stay untouched","agent":{"user-agent":{"description":"keep default"}}}`))
	defaultBefore, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read default config before install: %v", err)
	}

	resolution, err := resolveOpenCodeConfig(Options{ProjectPath: project}, home)
	if err != nil {
		t.Fatalf("resolveOpenCodeConfig() error = %v", err)
	}
	if resolution.Path != effectivePath || resolution.Source != openCodeGlobalConfigSource || resolution.Format != "JSON" {
		t.Fatalf("resolution = %#v, want non-default XDG JSON configuration %q", resolution, effectivePath)
	}

	result, err := Install(Options{Target: "opencode", ProjectPath: project})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got := result.Hosts["opencode"].OpenCodeConfig; !reflect.DeepEqual(got, resolution) {
		t.Fatalf("reported OpenCode config = %#v, want %#v", got, resolution)
	}

	effective, err := os.ReadFile(effectivePath)
	if err != nil {
		t.Fatalf("read effective XDG config after install: %v", err)
	}
	for _, want := range []string{"\"rotta-orchestrator\"", "\"theme\": \"xdg\"", "keep xdg", "keep built-in"} {
		if !strings.Contains(string(effective), want) {
			t.Errorf("effective XDG config missing %q:\n%s", want, effective)
		}
	}
	defaultAfter, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read default config after install: %v", err)
	}
	if !reflect.DeepEqual(defaultAfter, defaultBefore) {
		t.Errorf("default home config was modified:\n%s", defaultAfter)
	}
}

// REQ-089 → SCN-623 → TestSCN623_AmbiguousOrSchemaInvalidConfigIsNotModified
func TestSCN623_AmbiguousOrSchemaInvalidConfigIsNotModified(t *testing.T) {
	// Scenario: Ambiguous or schema-invalid OpenCode configuration is not modified
	tests := []struct {
		name        string
		setup       func(t *testing.T, home, project string) []string
		wantBlocker string
	}{
		{
			name: "ambiguous documented project candidates",
			setup: func(t *testing.T, home, project string) []string {
				t.Setenv("OPENCODE_CONFIG", "")
				jsonPath := filepath.Join(project, "opencode.json")
				jsoncPath := filepath.Join(project, "opencode.jsonc")
				writeTestFile(t, jsonPath, []byte(`{"theme":"project JSON"}`))
				writeTestFile(t, jsoncPath, []byte("// competing documented candidate\n{\"theme\": \"project JSONC\"}\n"))
				return []string{jsonPath, jsoncPath}
			},
			wantBlocker: "effective-config resolution blocked",
		},
		{
			name: "schema-invalid documented configuration",
			setup: func(t *testing.T, home, _ string) []string {
				path := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
				writeTestFile(t, path, []byte("// agent must be an object in the documented OpenCode configuration\n{\n  \"agent\": \"not an object\",\n}\n"))
				t.Setenv("OPENCODE_CONFIG", path)
				return []string{path}
			},
			wantBlocker: "schema validation blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := filepath.Join(home, "project")
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			configPaths := tt.setup(t, home, project)
			writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["mcp__rotta__spec_mode"]},"user":"unchanged"}`))

			configsBefore := map[string][]byte{}
			for _, configPath := range configPaths {
				config, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read OpenCode config before install: %v", err)
				}
				configsBefore[configPath] = config
			}
			claudeSettingsPath := filepath.Join(home, ".claude", "settings.json")
			claudeBefore, err := os.ReadFile(claudeSettingsPath)
			if err != nil {
				t.Fatalf("read unrelated Claude settings before install: %v", err)
			}

			result, err := Install(Options{Target: "both", ProjectPath: project, InstallSpec: true})
			if err == nil {
				t.Fatal("Install() error = nil, want blocked OpenCode installation")
			}
			if !strings.Contains(err.Error(), tt.wantBlocker) || !strings.Contains(result.Error, tt.wantBlocker) {
				t.Fatalf("blocked installation report = error %q, result %q, want %q", err, result.Error, tt.wantBlocker)
			}

			for configPath, configBefore := range configsBefore {
				configAfter, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read OpenCode config after install: %v", err)
				}
				if !reflect.DeepEqual(configAfter, configBefore) {
					t.Errorf("OpenCode config %s was modified despite %s:\n%s", configPath, tt.wantBlocker, configAfter)
				}
			}
			claudeAfter, err := os.ReadFile(claudeSettingsPath)
			if err != nil {
				t.Fatalf("read unrelated Claude settings after install: %v", err)
			}
			if !reflect.DeepEqual(claudeAfter, claudeBefore) {
				t.Errorf("unrelated Claude installation was modified:\n%s", claudeAfter)
			}
		})
	}
}

// REQ-089 → SCN-623 → TestSCN623_AllHostInstallFailsClosedBeforeOpenCodeMutation
func TestSCN623_AllHostInstallFailsClosedBeforeOpenCodeMutation(t *testing.T) {
	// Scenario: Ambiguous or schema-invalid OpenCode configuration is not modified
	home := t.TempDir()
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG", "")

	openCodeConfigPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	openCodeBefore := []byte(`{"agent":"not an object"}`)
	writeTestFile(t, openCodeConfigPath, openCodeBefore)
	claudeSettingsPath := filepath.Join(home, ".claude", "settings.json")
	claudeBefore := []byte(`{"user":"keep Claude unchanged"}`)
	writeTestFile(t, claudeSettingsPath, claudeBefore)
	codexInstructionsPath := filepath.Join(home, ".codex", "AGENTS.md")
	codexBefore := []byte("keep Codex unchanged\n")
	writeTestFile(t, codexInstructionsPath, codexBefore)

	result, err := Install(Options{Target: "all", ProjectPath: project, InstallSpec: true})
	if err == nil {
		t.Fatal("Install() error = nil, want schema validation blocked")
	}
	if !strings.Contains(err.Error(), "schema validation blocked") || !strings.Contains(result.Error, "schema validation blocked") {
		t.Fatalf("blocked installation report = error %q, result %q, want schema validation blocked", err, result.Error)
	}
	for path, before := range map[string][]byte{
		openCodeConfigPath:    openCodeBefore,
		claudeSettingsPath:    claudeBefore,
		codexInstructionsPath: codexBefore,
	} {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s after blocked all-host install: %v", path, readErr)
		}
		if !reflect.DeepEqual(after, before) {
			t.Errorf("%s was modified despite schema validation block:\n%s", path, after)
		}
	}
}

// REQ-089 → SCN-623 → TestSCN623_AllHostInstallBlocksMalformedOpenCodeConfiguration
func TestSCN623_AllHostInstallBlocksMalformedOpenCodeConfiguration(t *testing.T) {
	// Scenario: Ambiguous or schema-invalid OpenCode configuration is not modified
	home := t.TempDir()
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG", "")

	openCodeConfigPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	openCodeBefore := []byte(`{"agent":`)
	writeTestFile(t, openCodeConfigPath, openCodeBefore)
	claudeSettingsPath := filepath.Join(home, ".claude", "settings.json")
	claudeBefore := []byte(`{"user":"keep Claude unchanged"}`)
	writeTestFile(t, claudeSettingsPath, claudeBefore)
	codexInstructionsPath := filepath.Join(home, ".codex", "AGENTS.md")
	codexBefore := []byte("keep Codex unchanged\n")
	writeTestFile(t, codexInstructionsPath, codexBefore)

	result, err := Install(Options{Target: "all", ProjectPath: project, InstallSpec: true})
	if err == nil {
		t.Fatal("Install() error = nil, want schema validation blocked")
	}
	if !strings.Contains(err.Error(), "schema validation blocked") || !strings.Contains(result.Error, "schema validation blocked") {
		t.Fatalf("blocked installation report = error %q, result %q, want schema validation blocked", err, result.Error)
	}
	for path, before := range map[string][]byte{
		openCodeConfigPath:    openCodeBefore,
		claudeSettingsPath:    claudeBefore,
		codexInstructionsPath: codexBefore,
	} {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s after blocked all-host install: %v", path, readErr)
		}
		if !reflect.DeepEqual(after, before) {
			t.Errorf("%s was modified despite schema validation block:\n%s", path, after)
		}
	}
}
