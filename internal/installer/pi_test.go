package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPiInstallIsManagedIdempotentAndPreservesConflicts(t *testing.T) {
	home := t.TempDir()
	files, err := installPi(Options{}, home)
	if err != nil || len(files) != len(rottaAgents)+3 {
		t.Fatalf("first Pi install = %v, %v", files, err)
	}
	path := piExtensionPath(home)
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "rotta-next", "rotta-core", "SKILL.md")); err != nil {
		t.Fatalf("Pi core policy was not installed: %v", err)
	}
	if _, err := installPi(Options{}, home); err != nil {
		t.Fatalf("idempotent Pi install: %v", err)
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
	for _, path := range []string{piExtensionPath(home), filepath.Join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts"), filepath.Join(home, ".pi", "agent", "rotta-next")} {
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
