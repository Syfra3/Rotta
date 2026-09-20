package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakePiObservations(t *testing.T, graph bool) {
	t.Helper()
	oldLook, oldStat := piLookPath, piStat
	piLookPath = func(name string) (string, error) { return "/fake/" + name, nil }
	piStat = func(name string) (os.FileInfo, error) {
		if graph && strings.HasSuffix(name, filepath.Join(".vela", "graph.json")) {
			return os.Stat(t.TempDir())
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { piLookPath, piStat = oldLook, oldStat })
}

func TestPiInstallIsManagedIdempotentAndPreservesConflicts(t *testing.T) {
	home := t.TempDir()
	files, err := installPi(Options{}, home)
	if err != nil || len(files) != len(rottaAgents)+5 {
		t.Fatalf("first Pi install = %v, %v", files, err)
	}
	path := piExtensionPath(home)
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "rotta-next", "rotta-core", "SKILL.md")); err != nil {
		t.Fatalf("Pi core policy was not installed: %v", err)
	}
	if _, err := installPi(Options{}, home); err != nil {
		t.Fatalf("idempotent Pi install: %v", err)
	}
	originalConfig, err := os.ReadFile(piMCPConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(piMCPConfigPath(home), []byte("user config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(Options{}, home); err == nil {
		t.Fatal("modified managed Pi MCP config was overwritten")
	}
	if err := os.WriteFile(piMCPConfigPath(home), originalConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(Options{}, home); err == nil {
		t.Fatal("modified managed Pi extension was overwritten")
	}

	foreign := t.TempDir()
	foreignPath := piExtensionPath(foreign)
	if err := os.MkdirAll(filepath.Dir(foreignPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreignPath, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPi(Options{}, foreign); err == nil {
		t.Fatal("unmanaged Pi extension was overwritten")
	}
}

func TestInstallPiWritesSelectedMCPConfigAndTruthfulStatusesWithoutOtherHosts(t *testing.T) {
	fakePiObservations(t, true)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	project := filepath.Join(home, "project")

	result, err := Install(Options{Target: "pi", ProjectPath: project})
	if err != nil {
		t.Fatalf("install Pi with disabled services: %v", err)
	}
	assertPiMCPConfig(t, home, false, false, false)
	for _, service := range []string{"ancora", "vela", "context7"} {
		if got := result.MCPStatuses["pi"][service].Status; got != MCPStatusDisabled {
			t.Fatalf("disabled Pi %s status = %q, want disabled", service, got)
		}
	}
	for _, path := range []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex"), openCodeConfigDir(home)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Pi-only install wrote other host path %s: %v", path, err)
		}
	}

	result, err = Install(Options{Target: "pi", ProjectPath: project, SetupAncora: true, SetupVela: true, SetupContext7: true})
	if err != nil {
		t.Fatalf("install Pi with selected services: %v", err)
	}
	assertPiMCPConfig(t, home, true, true, true)
	for _, service := range []string{"ancora", "vela", "context7"} {
		status := result.MCPStatuses["pi"][service]
		if status.Status != MCPStatusConfiguredPendingHealth || !strings.Contains(status.Reason, "not observed") {
			t.Fatalf("Pi %s status = %#v, want configured-pending-health without observed health", service, status)
		}
		if capability := result.Hosts["pi"].Capabilities["mcp:"+service]; capability.Status != HostCapabilityStatusPending {
			t.Fatalf("Pi %s capability = %#v, want pending", service, capability)
		}
	}
	entry, err := os.ReadFile(piExtensionPath(home))
	if err != nil || !strings.Contains(string(entry), "rotta-mcp-bridge.ts") || !strings.Contains(string(entry), "registerMCPBridge") {
		t.Fatalf("installed parent does not explicitly load bridge: %v", err)
	}
	guard, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts"))
	if err != nil || !strings.Contains(string(guard), "rotta_(ancora|vela|context7)") {
		t.Fatalf("installed child guard lacks managed MCP routing: %v", err)
	}
}

func TestPiStatusReportsMissingBinaryAndGraphWithoutSpawning(t *testing.T) {
	oldLook, oldStat := piLookPath, piStat
	defer func() { piLookPath, piStat = oldLook, oldStat }()
	piLookPath = func(string) (string, error) { return "", os.ErrNotExist }
	piStat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if status := piMCPStatus(Options{SetupAncora: true}, "ancora"); status.Status != MCPStatusUnavailable || !strings.Contains(status.Reason, "binary") {
		t.Fatalf("missing binary = %#v", status)
	}
	piLookPath = func(string) (string, error) { return "/fake/vela", nil }
	if status := piMCPStatus(Options{SetupVela: true, ProjectPath: "/project"}, "vela"); status.Status != MCPStatusUnavailable || !strings.Contains(status.Reason, "graph") {
		t.Fatalf("missing graph = %#v", status)
	}
}

func TestPiInstallUsesEffectiveProjectPathAndReportsPreservedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldLook, oldStat := piLookPath, piStat
	defer func() { piLookPath, piStat = oldLook, oldStat }()
	piLookPath = func(string) (string, error) { return "/fake/vela", nil }
	seen := ""
	piStat = func(name string) (os.FileInfo, error) { seen = name; return nil, os.ErrNotExist }
	result, err := Install(Options{Target: "pi", SetupVela: true, ProjectPath: "~/project"})
	if err != nil {
		t.Fatalf("Pi install: %v", err)
	}
	if seen != filepath.Join(home, "project", ".vela", "graph.json") || result.MCPStatuses["pi"]["vela"].Status != MCPStatusUnavailable || result.Hosts["pi"].Capabilities["mcp:vela"].Status != HostCapabilityStatusFailed {
		t.Fatalf("effective graph/capability evidence = %q %#v %#v", seen, result.MCPStatuses["pi"]["vela"], result.Hosts["pi"].Capabilities["mcp:vela"])
	}
	configPath := piMCPConfigPath(home)
	original, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := os.WriteFile(configPath, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	failed, err := Install(Options{Target: "pi", SetupAncora: true, ProjectPath: ""})
	if err == nil || failed == nil || failed.MCPStatuses["pi"]["ancora"].Status != MCPStatusPreserved {
		t.Fatalf("ownership refusal did not report preserved-unvalidated: %#v %v", failed, err)
	}
	after, _ := os.ReadFile(configPath)
	if string(after) != "user-owned" {
		t.Fatalf("user-owned config overwritten: %q", after)
	}
	_ = original
}

func TestPiFreshPostWriteFailureIsNotPreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	piInstallFailureHook = func(stage string) error {
		if stage == "after-assets" {
			return errors.New("injected post-write failure")
		}
		return nil
	}
	t.Cleanup(func() { piInstallFailureHook = nil })
	result, err := Install(Options{Target: "pi", SetupAncora: true})
	if err == nil || result == nil {
		t.Fatalf("fresh injected failure succeeded: %#v", result)
	}
	if result.MCPStatuses["pi"]["ancora"].Status == MCPStatusPreserved {
		t.Fatalf("fresh failure fabricated preserved status: %#v", result.MCPStatuses["pi"])
	}
	if result.MCPStatuses["pi"]["ancora"].Status != MCPStatusUnavailable || result.Hosts["pi"].Capabilities["mcp:ancora"].Status != HostCapabilityStatusFailed {
		t.Fatalf("fresh failure status/capability = %#v %#v", result.MCPStatuses["pi"]["ancora"], result.Hosts["pi"].Capabilities["mcp:ancora"])
	}
	if _, statErr := os.Stat(piMCPConfigPath(home)); !os.IsNotExist(statErr) {
		t.Fatalf("fresh failed install left MCP config: %v", statErr)
	}
}

func assertPiMCPConfig(t *testing.T, home string, ancora, vela, context7 bool) {
	t.Helper()
	data, err := os.ReadFile(piMCPConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version  int                     `json:"version"`
		Services map[string]piMCPService `json:"services"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Services) != 3 || got.Services["ancora"].Enabled != ancora || got.Services["vela"].Enabled != vela || got.Services["context7"].Enabled != context7 {
		t.Fatalf("Pi MCP config = %s", data)
	}
}

func TestAllIncludesPiExactlyOnce(t *testing.T) {
	hosts := selectedHosts("all")
	count := 0
	for _, host := range hosts {
		if host == "pi" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("all Pi count = %d, want 1", count)
	}
}

func TestAllInstallsOnePiExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	result, err := Install(Options{Target: "all", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatalf("install all hosts: %v", err)
	}
	count := 0
	for _, file := range result.Files {
		if file == piExtensionPath(home) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("all installed Pi extension %d times, want 1", count)
	}
}

func TestPiInstallFailureRestoresAllBundleFiles(t *testing.T) {
	home := t.TempDir()
	if _, err := installPi(Options{}, home); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, path := range piBundlePaths(home) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = data
	}
	configPath := piMCPConfigPath(home)
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	before[configPath] = config
	piInstallFailureHook = func(stage string) error {
		if stage == "after-assets" {
			return errors.New("injected Pi failure")
		}
		return nil
	}
	t.Cleanup(func() { piInstallFailureHook = nil })
	if _, err := installPi(Options{}, home); err == nil {
		t.Fatal("injected Pi failure succeeded")
	}
	for path, want := range before {
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(want) {
			t.Fatalf("Pi rollback did not restore %s: %v", path, err)
		}
	}
}

func TestPiBackupScopeContainsEntireManagedBundle(t *testing.T) {
	home := t.TempDir()
	want := map[string]bool{}
	for _, path := range piBackupPaths(home) {
		want[path] = true
	}
	for _, path := range []string{piExtensionPath(home), filepath.Join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts"), filepath.Join(home, ".pi", "agent", "rotta-next", "rotta-mcp-bridge.ts"), piMCPConfigPath(home), filepath.Join(home, ".pi", "agent", "rotta-next")} {
		if !want[path] {
			t.Fatalf("Pi backup omitted %s", path)
		}
	}
}

func TestRestoreBackupRestoresEntirePiBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "project")
	if _, err := installPi(Options{}, home); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, path := range piBundlePaths(home) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(data)
	}
	configPath := piMCPConfigPath(home)
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	before[configPath] = string(config)
	backup, err := Backup(Options{Target: "pi", ProjectPath: project})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadBackupManifest(filepath.Join(backup, "manifest.json"))
	if err != nil || manifest.Status != "complete" {
		t.Fatalf("invalid Pi backup manifest: %v", err)
	}
	for path := range before {
		if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("restore did not recover %s: %v", path, err)
		}
	}
}
