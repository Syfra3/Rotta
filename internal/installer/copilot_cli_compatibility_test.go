package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-100 → SCN-402 → TestSCN402_InstallsOnlyCopilotCLI
func TestSCN402_InstallsOnlyCopilotCLI(t *testing.T) {
	// Scenario: Select GitHub Copilot CLI as the only installation target
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	unchanged := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):             "claude settings",
		filepath.Join(home, ".config", "opencode", "opencode.json"): "opencode settings",
		filepath.Join(home, ".codex", "config.toml"):                "codex settings",
	}
	for path, content := range unchanged {
		writeTestFile(t, path, []byte(content))
	}

	result, err := Install(Options{Target: "copilot-cli", ProjectPath: projectPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 1 || result.Hosts["copilot-cli"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected only Copilot CLI to be installed and reported, got %#v", result.Hosts)
	}
	for path, want := range unchanged {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read unchanged host configuration %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("expected %s to remain unchanged, got %q", path, got)
		}
	}
}

// REQ-100 → REQ-104 → SCN-403 → TestSCN403_InstallsAndReportsEverySupportedHost
func TestSCN403_InstallsAndReportsEverySupportedHost(t *testing.T) {
	// Scenario: Select every supported host through an explicit aggregate label
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := Install(Options{Target: "all", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatal(err)
	}

	wantHosts := []string{"claude-code", "opencode", "codex", "copilot-cli"}
	if len(result.Hosts) != len(wantHosts) {
		t.Fatalf("expected one result for each supported host, got %#v", result.Hosts)
	}
	for _, host := range wantHosts {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("expected %s to be attempted and installed once, got %#v", host, result.Hosts[host])
		}
	}
}

// REQ-100 → REQ-105 → SCN-404 → TestSCN404_PreservesLegacyBothWithoutCopilotSideEffects
func TestSCN404_PreservesLegacyBothWithoutCopilotSideEffects(t *testing.T) {
	// Scenario: Preserve the legacy two-host target string
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	copilotRoot := filepath.Join(home, "copilot-global")
	copilotArtifact := filepath.Join(copilotRoot, "existing.md")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", copilotRoot)
	writeTestFile(t, copilotArtifact, []byte("existing Copilot configuration"))

	result, err := Install(Options{Target: "both", ProjectPath: projectPath})
	if err != nil {
		t.Fatal(err)
	}

	wantHosts := []string{"claude-code", "opencode"}
	if len(result.Hosts) != len(wantHosts) {
		t.Fatalf("expected legacy both target to report only Claude Code and OpenCode, got %#v", result.Hosts)
	}
	for _, host := range wantHosts {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("expected legacy both target to install %s, got %#v", host, result.Hosts[host])
		}
	}
	if _, reported := result.Hosts["copilot-cli"]; reported {
		t.Fatalf("legacy both target must not report Copilot CLI, got %#v", result.Hosts)
	}

	data, err := os.ReadFile(copilotArtifact)
	if err != nil {
		t.Fatalf("read existing Copilot artifact: %v", err)
	}
	if string(data) != "existing Copilot configuration" {
		t.Fatalf("legacy both target must not clean Copilot artifacts, got %q", data)
	}
	assertPathMissing(t, filepath.Join(result.BackupDir, "files", "home", "copilot-global"))
	for _, file := range result.Files {
		if file == copilotRoot || strings.HasPrefix(file, copilotRoot+string(os.PathSeparator)) {
			t.Fatalf("legacy both target must not report a Copilot artifact, got %#v", result.Files)
		}
	}

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read user-facing documentation: %v", err)
	}
	if !strings.Contains(string(readme), "`both` (legacy compatibility input)") || !strings.Contains(string(readme), "Claude Code and OpenCode") {
		t.Fatalf("expected user-facing documentation to identify both as the legacy Claude Code and OpenCode compatibility input")
	}
}

// REQ-100 → SCN-405 → TestSCN405_RejectsUnsupportedTargetsBeforeConfigurationChanges
func TestSCN405_RejectsUnsupportedTargetsBeforeConfigurationChanges(t *testing.T) {
	// Scenario: Reject an unknown target before changing user configuration
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	unchanged := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):               "claude settings",
		filepath.Join(home, ".config", "opencode", "instructions.md"): "opencode instructions",
		filepath.Join(home, ".codex", "AGENTS.md"):                    "codex guidance",
	}
	for path, content := range unchanged {
		writeTestFile(t, path, []byte(content))
	}

	for _, target := range []string{"copilot-vscode", "unsupported-target"} {
		t.Run(target, func(t *testing.T) {
			result, err := Install(Options{Target: target, ProjectPath: projectPath})
			if err == nil {
				t.Fatal("expected unsupported target to be rejected")
			}
			if result != nil {
				t.Fatalf("expected no installation result for unsupported target, got %#v", result)
			}
			for path, want := range unchanged {
				got, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatalf("read unchanged host configuration %s: %v", path, readErr)
				}
				if string(got) != want {
					t.Fatalf("expected %s to remain unchanged, got %q", path, got)
				}
			}
			assertPathMissing(t, filepath.Join(home, ".rotta", "backups"))
			assertPathMissing(t, filepath.Join(projectPath, ".rotta"))
			for _, host := range []string{"Claude Code", "OpenCode", "Codex", "Copilot CLI"} {
				if !strings.Contains(err.Error(), host) {
					t.Fatalf("expected unsupported-target error to list %q, got %q", host, err)
				}
			}
		})
	}
}

// REQ-101 → SCN-406 → TestSCN406_GeneratesGlobalCopilotRoleDefinitionsForSelectedModes
func TestSCN406_GeneratesGlobalCopilotRoleDefinitionsForSelectedModes(t *testing.T) {
	// Scenario: Generate global Copilot Markdown role definitions for selected Rotta modes
	for _, test := range []struct {
		name          string
		installSpec   bool
		installImpl   bool
		installReview bool
		roles         []string
	}{
		{
			name:          "all phases",
			installSpec:   true,
			installImpl:   true,
			installReview: true,
			roles:         []string{"rotta-orchestrator", "rotta-spec", "rotta-impl", "rotta-review"},
		},
		{
			name:        "review omitted",
			installSpec: true,
			installImpl: true,
			roles:       []string{"rotta-orchestrator", "rotta-spec", "rotta-impl"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "active-copilot-root")
			t.Setenv("HOME", home)
			t.Setenv("COPILOT_HOME", root)

			result, err := Install(Options{
				Target:        "copilot-cli",
				ProjectPath:   filepath.Join(home, "project"),
				InstallSpec:   test.installSpec,
				InstallImpl:   test.installImpl,
				InstallReview: test.installReview,
			})
			if err != nil {
				t.Fatal(err)
			}

			for _, role := range test.roles {
				assertCopilotAgentFixture(t, filepath.Join(root, "agents", role+".agent.md"), role)
			}
			if !test.installReview {
				assertPathMissing(t, filepath.Join(root, "agents", "rotta-review.agent.md"))
			}
			assertFileContains(t, filepath.Join(root, "instructions", "rotta.instructions.md"), "Rotta")

			capability := result.Hosts["copilot-cli"].Capabilities["agents"]
			if capability.Status != HostCapabilityStatusDegraded || capability.Reason == "" || capability.Remediation == "" {
				t.Fatalf("expected unavailable Copilot CLI role acceptance proof to be degraded with remediation, got %#v", capability)
			}
		})
	}
}

func assertCopilotAgentFixture(t *testing.T, path, role string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Copilot agent %s: %v", role, err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") || !strings.Contains(content, "\n---\n") {
		t.Fatalf("expected Copilot agent %s to have YAML frontmatter, got %q", role, content)
	}
	assertContainsAll(t, content, []string{"name: " + role, "Rotta"})
}
