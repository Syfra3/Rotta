package installer

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHostSourceLoadingUsesActualInstalledBundles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home with spaces")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "external xdg"))
	claudeRoot := filepath.Join(home, ".claude", "skills", "rotta-next")
	codexPath := filepath.Join(home, ".codex", "AGENTS.md")
	duplicate := filepath.Join(home, ".agents", "skills", "rotta-core", "SKILL.md")
	writeRottaNextTestFile(t, duplicate, []byte("user-owned duplicate"))
	// Seed the raw, owned Claude files emitted by the previous installer to
	// exercise upgrades, including model/frontmatter and integration retention.
	opts := Options{SetupAncora: true}
	legacy := map[string][]byte{}
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		t.Fatal(err)
	}
	legacy[filepath.Join(claudeRoot, "rotta-core", "SKILL.md")] = core
	legacyCodex := "# Rotta Codex Instructions\n\nCodex adapts Rotta Next roles into this instruction file. Route work through the orchestrator and follow the shared policy below.\n\n" + string(core)
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			t.Fatal(err)
		}
		legacy[filepath.Join(claudeRoot, agent.skillName, "SKILL.md")] = data
		legacy[filepath.Join(home, ".claude", "agents", agent.key+".md")] = data
		legacyCodex += "\n\n" + string(data)
	}
	legacy[codexPath] = []byte(legacyCodex)
	if _, err := installManagedFiles(home, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := installOpenCode(opts, home); err != nil {
		t.Fatal(err)
	}
	openCodeFiles, err := openCodeManagedSkills(opts, home)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := installClaudeCode(opts, home); err != nil {
			t.Fatal(err)
		}
		if _, err := installCodex(opts, home); err != nil {
			t.Fatal(err)
		}
		for path, original := range legacy {
			if path == codexPath {
				continue
			}
			installed := readRottaNextInstalledAsset(t, path)
			assertRottaNextContainsAll(t, installed, []string{"## Resolved Claude Code policy bundle", claudeRoot,
				filepath.Join(claudeRoot, "rotta-core", "SKILL.md"), "Claude Code's Read tool", "source loading blocked/unknown", "another host's bundle"})
			if strings.Contains(installed, "## Resolved OpenCode policy bundle") {
				t.Fatalf("Claude source points at OpenCode: %s", path)
			}
			// Binding insertion must not modify the original frontmatter/model or body.
			if strings.HasPrefix(string(original), "---\n") {
				boundary := strings.Index(string(original)[4:], "\n---\n") + 4 + len("\n---\n")
				if !strings.HasPrefix(installed, string(original[:boundary])) || !strings.HasSuffix(installed, string(original[boundary:])) {
					t.Fatalf("binding modified canonical content in %s", path)
				}
			} else if !strings.HasSuffix(installed, string(original)) {
				t.Fatalf("binding modified canonical body in %s", path)
			}
		}
		for _, agent := range rottaAgents {
			skillPath := filepath.Join(claudeRoot, agent.skillName, "SKILL.md")
			agentPath := filepath.Join(home, ".claude", "agents", agent.key+".md")
			skill := readRottaNextInstalledAsset(t, skillPath)
			if !strings.Contains(skill, skillPath) || readRottaNextInstalledAsset(t, agentPath) != skill {
				t.Fatalf("Claude agent/role binding mismatch: %s", agent.key)
			}
		}
		codex := readRottaNextInstalledAsset(t, codexPath)
		assertRottaNextContainsAll(t, codex, []string{"## Resolved Codex inline policy bundle", codexPath,
			"Consuming these complete embedded sections satisfies all instructions", "no separate SKILL.md reads are required",
			"Do not search for SKILL.md files", "source loading blocked/unknown", "Pass that same file and section identity", "Ancora is enabled"})
		if strings.Contains(codex, claudeRoot) || strings.Contains(codex, openCodeConfigDir(home)) {
			t.Fatal("Codex inline bundle refers to another host's file paths")
		}
		for _, role := range append([]agentEntry{{skillName: "rotta-core"}}, rottaAgents...) {
			start := "## Embedded policy: " + role.skillName + "\n"
			end := "<!-- End embedded policy: " + role.skillName + " -->"
			if strings.Count(codex, start) != 1 || strings.Count(codex, end) != 1 || strings.Index(codex, start) >= strings.Index(codex, end) {
				t.Fatalf("Codex section boundaries missing/duplicated for %s", role.skillName)
			}
			section := codex[strings.Index(codex, start):strings.Index(codex, end)]
			if !strings.Contains(section, string(legacy[filepath.Join(claudeRoot, role.skillName, "SKILL.md")])) {
				t.Fatalf("Codex section lost canonical content for %s", role.skillName)
			}
		}
		for path, original := range openCodeFiles {
			if readRottaNextInstalledAsset(t, path) != string(original) {
				t.Fatalf("cross-host installation modified OpenCode source %s", path)
			}
		}
		if readRottaNextInstalledAsset(t, duplicate) != "user-owned duplicate" {
			t.Fatal("cross-host installation changed duplicate skills")
		}
	}
}

func TestClaudeAgentOnlyInstallerUsesClaudeBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	agentsDir := filepath.Join(home, ".claude", "agents")
	if _, err := installClaudeCodeAgents(Options{}, agentsDir); err != nil {
		t.Fatal(err)
	}
	for _, agent := range rottaAgents {
		text := readRottaNextInstalledAsset(t, filepath.Join(agentsDir, agent.key+".md"))
		assertRottaNextContainsAll(t, text, []string{
			filepath.Join(home, ".claude", "skills", "rotta-next", "rotta-core", "SKILL.md"),
			filepath.Join(home, ".claude", "skills", "rotta-next", agent.skillName, "SKILL.md"),
			"does not replace the core and role reads", "missing, unreadable, denied, or incomplete",
		})
	}
}

func TestHostSourceLoadingPreservesCustomTargets(t *testing.T) {
	for _, host := range []string{"claude-code", "codex"} {
		t.Run(host, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", "")
			path := filepath.Join(home, ".codex", "AGENTS.md")
			if host == "claude-code" {
				path = filepath.Join(home, ".claude", "agents", "rotta-impl.md")
			}
			writeRottaNextTestFile(t, path, []byte("custom policy"))
			if _, err := installHost(Options{}, host, home); err == nil || !strings.Contains(err.Error(), "unmanaged artifact") {
				t.Fatalf("expected custom target refusal, got %v", err)
			}
			if readRottaNextInstalledAsset(t, path) != "custom policy" {
				t.Fatal("custom policy overwritten")
			}
		})
	}
}
