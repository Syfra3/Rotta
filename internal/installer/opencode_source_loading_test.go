package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeCanonicalLoadingWithDuplicateSkills(t *testing.T) {
	for _, customXDG := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom-XDG"}[customXDG], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", "")
			if customXDG {
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config with spaces"))
			}
			// All these discovery candidates are outside the canonical bundle.
			duplicates := []string{
				filepath.Join(home, ".claude", "skills", "rotta-next", "rotta-core", "SKILL.md"),
				filepath.Join(home, ".claude", "skills", "rotta", "rotta-core", "SKILL.md"),
				filepath.Join(home, ".agents", "skills", "rotta-core", "SKILL.md"),
				filepath.Join(openCodeConfigDir(home), "skills", "rotta-core", "SKILL.md"),
				filepath.Join(home, "project", ".opencode", "skills", "rotta-core", "SKILL.md"),
			}
			for _, path := range duplicates {
				writeRottaNextTestFile(t, path, []byte("custom/stale skill: "+path))
			}
			if _, err := installOpenCode(Options{}, home); err != nil {
				t.Fatal(err)
			}
			config, err := readOpenCodeConfig(openCodeConfigPath(home))
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(openCodeConfigDir(home), "skills", "rotta-next")
			for _, agent := range rottaAgents {
				entry := config["agent"].(map[string]interface{})[agent.key].(map[string]interface{})
				prompt := entry["prompt"].(string)
				assertRottaNextContainsAll(t, prompt, []string{
					"read tool with absolute filePath", filepath.Join(root, "rotta-core", "SKILL.md"),
					filepath.Join(root, agent.skillName, "SKILL.md"), "Do not use the name-based skill tool",
					"missing, unreadable, denied", "stop; never fall back", "each child must read its own role",
				})
				if strings.Contains(prompt, "~/.config") || entry["mode"] != agent.mode || entry["model"] != immutableOpenCodeRouting[agent.key] {
					t.Fatalf("incorrect generated entry: %#v", entry)
				}
				installed := readRottaNextInstalledAsset(t, filepath.Join(root, agent.skillName, "SKILL.md"))
				original, err := readRenderedAsset(agent.assetPath, Options{})
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(string(original), "---\n") {
					end := strings.Index(string(original)[4:], "\n---\n") + 4 + len("\n---\n")
					if !strings.HasPrefix(installed, string(original[:end])) {
						t.Fatal("host binding broke skill frontmatter")
					}
				}
				assertRottaNextContainsAll(t, installed, []string{"## Resolved OpenCode policy bundle", filepath.Join(root, "rotta-core", "SKILL.md"), "every instruction to load rotta-core or this role means these exact files"})
			}
			for _, path := range duplicates {
				if got := readRottaNextInstalledAsset(t, path); got != "custom/stale skill: "+path {
					t.Fatalf("modified duplicate skill %s", path)
				}
			}
			capability := openCodeSourceLoadingCapability(home, config)
			if capability.Status != HostCapabilityStatusExact || !strings.Contains(capability.Reason, "have not been verified") {
				t.Fatalf("source loading evidence = %#v", capability)
			}
		})
	}
}

func TestOpenCodeCanonicalLoadingUpgradesLegacyAndThenNoOps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	root := filepath.Join(openCodeConfigDir(home), "skills", "rotta-next")
	legacyFiles := map[string][]byte{}
	agents := map[string]interface{}{}
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, Options{})
		if err != nil {
			t.Fatal(err)
		}
		legacyFiles[filepath.Join(root, agent.skillName, "SKILL.md")] = data
		entry := openCodeAgentEntry(agent)
		entry["description"] = "keep custom description"
		agents[agent.key] = entry
	}
	core, err := readRenderedAsset("core/rotta-core.md", Options{})
	if err != nil {
		t.Fatal(err)
	}
	legacyFiles[filepath.Join(root, "rotta-core", "SKILL.md")] = core
	if _, err := installManagedFiles(home, legacyFiles); err != nil {
		t.Fatal(err)
	}
	configPath := openCodeConfigPath(home)
	if err := writeOpenCodeConfig(configPath, map[string]interface{}{"agent": agents, "theme": "keep"}); err != nil {
		t.Fatal(err)
	}
	opts := Options{Target: "opencode", ModelRouting: ModelRoutingDisabled}
	noOp, err := preflightSelectedOpenCodeInstall(opts, home)
	if err != nil || noOp {
		t.Fatalf("legacy preflight = %v, %v", noOp, err)
	}
	// An upgrade is still one config/assets/ownership transaction.
	beforeConfig := readRottaNextInstalledAsset(t, configPath)
	beforeManifest := readRottaNextInstalledAsset(t, managedArtifactsManifestPath(home))
	routingFailureHook = func(stage string) error {
		if stage == "after-assets" {
			return errors.New("source-loading upgrade fault")
		}
		return nil
	}
	t.Cleanup(func() { routingFailureHook = nil })
	if _, err := installOpenCode(opts, home); err == nil || !strings.Contains(err.Error(), "source-loading upgrade fault") {
		t.Fatalf("expected upgrade fault, got %v", err)
	}
	routingFailureHook = nil
	if readRottaNextInstalledAsset(t, configPath) != beforeConfig || readRottaNextInstalledAsset(t, managedArtifactsManifestPath(home)) != beforeManifest {
		t.Fatal("upgrade failure did not restore exact config/ownership")
	}
	for path, data := range legacyFiles {
		if readRottaNextInstalledAsset(t, path) != string(data) {
			t.Fatalf("upgrade failure did not restore %s", path)
		}
	}
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	config, err := readOpenCodeConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range rottaAgents {
		entry := config["agent"].(map[string]interface{})[agent.key].(map[string]interface{})
		if entry["prompt"] != openCodeAgentPrompt(agent, root) || entry["description"] != "keep custom description" || entry["model"] != nil {
			t.Fatalf("incorrect upgrade: %#v", entry)
		}
	}
	paths := []string{configPath, managedArtifactsManifestPath(home)}
	for path := range legacyFiles {
		paths = append(paths, path)
	}
	before := map[string]os.FileInfo{}
	for _, path := range paths {
		before[path], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := Install(opts)
	if err != nil || result.BackupDir != "" || len(result.Files) != 0 {
		t.Fatalf("second install not a no-op: %#v, %v", result, err)
	}
	for _, path := range paths {
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(before[path], after) || !before[path].ModTime().Equal(after.ModTime()) || before[path].Size() != after.Size() {
			t.Fatalf("no-op rewrote %s: %v", path, err)
		}
	}
}

func TestOpenCodeCanonicalLoadingPreservesAndReportsUnknownPrompts(t *testing.T) {
	for _, prompt := range []string{"user modification", rottaAgents[0].prompt} {
		t.Run(prompt, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			entry := map[string]interface{}{"prompt": prompt, "tools": map[string]interface{}{"read": false}}
			if err := writeOpenCodeConfig(openCodeConfigPath(home), map[string]interface{}{"agent": map[string]interface{}{"rotta-orchestrator": entry}}); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				result, err := Install(Options{Target: "opencode"})
				if err != nil {
					t.Fatal(err)
				}
				capability := result.Hosts["opencode"].Capabilities["source_loading"]
				if capability.Status != HostCapabilityStatusDegraded || !containsAll(capability.Reason, openCodeConfigPath(home), "agent.rotta-orchestrator.prompt", "tools.read=false", "unknown") {
					t.Fatalf("missing precise unresolved report: %#v", capability)
				}
				config, err := readOpenCodeConfig(openCodeConfigPath(home))
				if err != nil {
					t.Fatal(err)
				}
				if config["agent"].(map[string]interface{})["rotta-orchestrator"].(map[string]interface{})["prompt"] != prompt {
					t.Fatal("overwrote unowned prompt")
				}
			}
		})
	}
}

func TestOpenCodeCanonicalLoadingRejectsRelativeXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative-config")
	_, err := openCodeManagedSkills(Options{}, t.TempDir())
	if err == nil || !containsAll(err.Error(), "source loading unresolved", "not absolute", "XDG_CONFIG_HOME") {
		t.Fatalf("relative source resolution error = %v", err)
	}
}
