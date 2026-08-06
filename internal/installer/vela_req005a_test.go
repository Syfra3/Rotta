package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ005A_SetupVelaBootstrapsExactCommandsAndWritesOnlyManagedOpenCodeEntry(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	log := filepath.Join(home, "commands.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeREQ005AExecutable(t, filepath.Join(bin, "brew"), "#!/bin/sh\nprintf 'brew %s %s\\n' \"$1\" \"$2\" >> \"$LOG\"\n")
	writeREQ005AExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nprintf 'vela %s\\n' \"$1\" >> \"$LOG\"\n")
	t.Setenv("LOG", log)
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeREQ005AFile(t, configPath, []byte(`{"theme":"dark","mcp":{"other":{"type":"local","command":["other"],"enabled":true}}}`))
	project := filepath.Join(home, "project")

	first, err := SetupVela(Options{Target: "opencode", ProjectPath: project, SetupVela: true, ConfirmVela: true}, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if first.BinPath != filepath.Join(bin, "vela") || !first.Installed {
		t.Fatalf("setup result = %#v", first)
	}
	assertREQ005AVelaEntry(t, configPath)
	if _, err := os.Stat(filepath.Join(project, ".vela")); !os.IsNotExist(err) {
		t.Fatalf("setup created project graph state: %v", err)
	}

	if _, err := SetupVela(Options{Target: "opencode", ProjectPath: project, SetupVela: true, ConfirmVela: true}, home, project); err != nil {
		t.Fatal(err)
	}
	commands, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(commands), "brew tap Syfra3/tap\nbrew install vela\nvela version\nbrew tap Syfra3/tap\nbrew install vela\nvela version\n"; got != want {
		t.Fatalf("bootstrap commands = %q, want %q", got, want)
	}
}

func TestREQ005A_SetupVelaRefusesUserOwnedEntryWithoutSideEffects(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", "")
	writeREQ005AExecutable(t, filepath.Join(bin, "brew"), "#!/bin/sh\nexit 0\n")
	writeREQ005AExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nexit 0\n")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	original := []byte(`{"mcp":{"vela":{"type":"local","command":["mine"],"enabled":true},"keep":{"command":["keep"]}}}`)
	writeREQ005AFile(t, path, original)

	_, err := SetupVela(Options{Target: "opencode", SetupVela: true, ConfirmVela: true}, home, filepath.Join(home, "project"))
	if err == nil || !strings.Contains(err.Error(), "ambiguous or user-owned") {
		t.Fatalf("error = %v, want ownership refusal", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatalf("user configuration changed: %s", got)
	}
}

func TestREQ005A_UnobservableOpenCodeStatusRollsBackManagedEntry(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", "")
	writeREQ005AExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nexit 0\n")
	writeREQ005AExecutable(t, filepath.Join(bin, "opencode"), "#!/bin/sh\nprintf 'vela disconnected\\n'\n")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	baseline := []byte(`{"theme":"dark","mcp":{"keep":{"command":["keep"]}}}`)
	writeREQ005AFile(t, path, baseline)
	transaction, err := beginOpenCodeVelaTransaction(filepath.Join(home, "state"), path)
	if err != nil {
		t.Fatal(err)
	}
	resolution := newOpenCodeConfigResolution(path, openCodeGlobalConfigSource, nil)
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := ensureManagedOpenCodeVelaEntry(&document); err != nil || !changed {
		t.Fatalf("managed entry changed=%v err=%v", changed, err)
	}
	if err := writeResolvedOpenCodeConfig(document); err != nil {
		t.Fatal(err)
	}
	result := &Result{Hosts: map[string]HostInstallResult{"opencode": {Host: "opencode", OpenCodeConfig: resolution}}}
	statusErr := observeOpenCodeVelaStatus(result, home, Options{Target: "opencode"})
	if statusErr == nil || !strings.Contains(statusErr.Error(), path) {
		t.Fatalf("status error = %v, want visible effective config path", statusErr)
	}
	rollback, err := transaction.recover(statusErr)
	if err != nil || !rollback.Restored || rollback.ConcurrentModification {
		t.Fatalf("rollback=%#v err=%v", rollback, err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(baseline) {
		t.Fatalf("rollback config = %s, want %s", got, baseline)
	}
	if _, err := os.Stat(filepath.Join(home, "state", "vela-rollback.json")); err != nil {
		t.Fatalf("rollback evidence: %v", err)
	}
}

func TestREQ005A_VelaPendingWithAnotherServerConnectedRollsBack(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeREQ005AExecutable(t, filepath.Join(bin, "brew"), "#!/bin/sh\nexit 0\n")
	writeREQ005AExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nexit 0\n")
	writeREQ005AExecutable(t, filepath.Join(bin, "opencode"), "#!/bin/sh\nprintf 'vela pending\\nother connected\\n'\n")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	baseline := []byte(`{"theme":"dark","mcp":{"keep":{"command":["keep"]}}}`)
	writeREQ005AFile(t, path, baseline)

	result, err := Install(Options{Target: "opencode", ProjectPath: project, SetupVela: true, ConfirmVela: true})
	if err == nil || !strings.Contains(err.Error(), "without a connected status") {
		t.Fatalf("Install() error = %v, want Vela-specific pending status failure", err)
	}
	if result == nil || !strings.Contains(result.Error, "without a connected status") {
		t.Fatalf("Install result = %#v, want visible Vela status gap", result)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(got, &config); err != nil {
		t.Fatal(err)
	}
	mcp := config["mcp"].(map[string]interface{})
	if _, exists := mcp["vela"]; exists {
		t.Fatalf("Vela entry remained after rollback: %s", got)
	}
	if _, exists := mcp["keep"]; !exists {
		t.Fatalf("pre-existing MCP entry was not preserved: %s", got)
	}
	evidence, readErr := os.ReadFile(filepath.Join(result.BackupDir, "vela-rollback.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(evidence), `"restored": true`) {
		t.Fatalf("rollback evidence = %s", evidence)
	}
}

func assertREQ005AVelaEntry(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	entry := config["mcp"].(map[string]interface{})["vela"].(map[string]interface{})
	if entry["type"] != "local" || entry["enabled"] != true {
		t.Fatalf("entry = %#v", entry)
	}
	command := entry["command"].([]interface{})
	if strings.Join([]string{command[0].(string), command[1].(string), command[2].(string)}, " ") != "vela serve --mcp" {
		t.Fatalf("command = %#v", command)
	}
}

func writeREQ005AExecutable(t *testing.T, path, body string) {
	t.Helper()
	writeREQ005AFile(t, path, []byte(body))
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeREQ005AFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
