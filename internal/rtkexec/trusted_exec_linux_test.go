//go:build linux

package rtkexec

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustedRTKDescriptorSnapshotRejectsEscapesSymlinksAndMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Cellar", "rtk", "1.0.0", "bin", "rtk")
	mustExecutable(t, path, "#!/bin/sh\necho 'rtk 1.0.0'\n")

	for name, candidate := range map[string]string{
		"root escape":     filepath.Join(root+"-escape", "Cellar", "rtk", "1.0.0", "bin", "rtk"),
		"wrong formula":   filepath.Join(root, "Cellar", "other", "1.0.0", "bin", "rtk"),
		"old prefix form": filepath.Join(root, "Cellar", "rtk-1.0.0", "bin", "rtk"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := openRTKAtRoots(candidate, []string{root}); err == nil {
				t.Fatal("non-exact RTK path was accepted")
			}
		})
	}

	link := filepath.Join(root, "Cellar", "rtk", "1.0.0", "bin", "linked-rtk")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := openUnderRoot(root, []string{"Cellar", "rtk", "1.0.0", "bin", "linked-rtk"}); err == nil {
		t.Fatal("final symlink was accepted")
	}

	executable, err := openRTKAtRoots(path, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer executable.Close()
	first := "#!/bin/sh\necho 'rtk 1.0.0'\n"
	sum := sha256.Sum256([]byte(first))
	// Both a rename/symlink swap and an in-place rewrite affect only the
	// pathname after descriptor snapshotting, never the checked executable.
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	mustExecutable(t, path, "#!/bin/sh\necho 'rtk 2.0.0'\n")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'rtk 3.0.0'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	version, err := executable.Version()
	if err != nil || version != "rtk 1.0.0" {
		t.Fatalf("version after mutation = %q, %v", version, err)
	}
	hash, err := executable.Fingerprint()
	if err != nil || hash != hex.EncodeToString(sum[:]) {
		t.Fatalf("fingerprint after mutation = %q, %v", hash, err)
	}
}

func TestTrustedBrewPolicyIsExactAndPlatformGated(t *testing.T) {
	root := t.TempDir()
	brew := filepath.Join(root, "bin", "brew")
	mustExecutable(t, brew, "#!/bin/sh\necho 'Homebrew 1.0'\n")
	if executable, err := openBrewAtRoots([]string{root}); err != nil {
		t.Fatalf("exact brew was rejected: %v", err)
	} else {
		_ = executable.Close()
	}
	if _, err := openBrewAtRoots([]string{root + "-prefix"}); err == nil {
		t.Fatal("prefix-related non-root was accepted")
	}
	previous := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previous })
	if _, err := OpenBrew(); err == nil || !strings.Contains(err.Error(), "Linux descriptor guarantees") {
		t.Fatalf("unsupported platform did not fail closed: %v", err)
	}
	if _, err := OpenRTK(filepath.Join(root, "Cellar", "rtk", "1", "bin", "rtk")); err == nil || !strings.Contains(err.Error(), "Linux descriptor guarantees") {
		t.Fatalf("unsupported RTK platform did not fail closed: %v", err)
	}
}

func TestTrustedRTKExecutionClearsHostileEnvironmentForScripts(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "host-marker")
	maliciousBin := filepath.Join(root, "malicious-bin")
	if err := os.MkdirAll(maliciousBin, 0o700); err != nil {
		t.Fatal(err)
	}
	mustExecutable(t, filepath.Join(maliciousBin, "env"), "#!/bin/sh\ntouch '"+marker+"'\nprintf substituted\n")
	bashEnv := filepath.Join(root, "bash-env")
	if err := os.WriteFile(bashEnv, []byte("touch '"+marker+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Cellar", "rtk", "1.0.0", "bin", "rtk")
	// This is a fake RTK script. Its bash shebang and bare env invocation make
	// inherited BASH_ENV and PATH respectively observable without running RTK,
	// Brew, a package manager, or a network action.
	mustExecutable(t, path, "#!/usr/bin/env bash\nprintf safe-script\\n\nenv\n")

	for name, value := range map[string]string{
		"PATH":         maliciousBin,
		"BASH_ENV":     bashEnv,
		"ENV":          bashEnv,
		"HOME":         root,
		"PYTHONPATH":   root,
		"NODE_OPTIONS": "--require=" + bashEnv,
		"GEM_HOME":     root,
		"GOPATH":       root,
	} {
		t.Setenv(name, value)
	}

	executable, err := openRTKAtRoots(path, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer executable.Close()
	output, err := executable.Run(nil, "", 512)
	if err != nil {
		t.Fatalf("fake RTK script did not run under the sealed policy: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile environment caused a host marker: %v", err)
	}
	for _, forbidden := range []string{"substituted", "BASH_ENV=", "ENV=", "HOME=", "PYTHONPATH=", "NODE_OPTIONS=", "GEM_HOME=", "GOPATH="} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("hostile environment value reached fake RTK script: %q in %q", forbidden, output)
		}
	}
	if !strings.Contains(output, "safe-script") || !strings.Contains(output, "PATH=/usr/bin:/bin") {
		t.Fatalf("fake RTK script did not receive the minimal allowlisted environment: %q", output)
	}
}

func mustExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}
