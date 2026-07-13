package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityInstallerArtifactsAreOwnerOnly(t *testing.T) {
	home := t.TempDir()
	opts := Options{InstallImpl: true}

	files, err := installCodex(opts, home)
	if err != nil {
		t.Fatalf("installCodex: %v", err)
	}
	if _, err := configureCodexMCPServers(Options{SetupAncora: true}, home); err != nil {
		t.Fatalf("configureCodexMCPServers: %v", err)
	}
	if _, err := installOpenCodeVelaFreshnessGuard(home); err != nil {
		t.Fatalf("installOpenCodeVelaFreshnessGuard: %v", err)
	}
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatalf("installClaudeCodeVelaFreshnessGuard: %v", err)
	}

	assertMode(t, files[0], 0o600)
	assertMode(t, filepath.Join(home, ".codex", "config.toml"), 0o600)
	assertMode(t, openCodeVelaFreshnessPluginPath(home), 0o600)
	assertMode(t, filepath.Join(home, ".config", "opencode", "opencode.json"), 0o600)
	assertMode(t, claudeCodeVelaFreshnessHookPath(home), 0o700)
	assertMode(t, filepath.Join(home, ".claude", "settings.json"), 0o600)
	assertMode(t, filepath.Join(home, ".codex"), 0o750)
	assertMode(t, filepath.Join(home, ".config", "opencode"), 0o750)
	assertMode(t, filepath.Join(home, ".claude"), 0o750)
}

func TestSecurityBackupsAreOwnerOnly(t *testing.T) {
	home := t.TempDir()
	backupDir, _, err := nextBackupDir(home)
	if err != nil {
		t.Fatalf("nextBackupDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "manifest.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	assertMode(t, filepath.Join(home, ".rotta", "backups"), 0o750)
	assertMode(t, backupDir, 0o750)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %o, want %o", path, got, want)
	}
}
