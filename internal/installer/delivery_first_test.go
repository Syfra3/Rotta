package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/assets"
)

// DF-001: an upgrade refreshes owned policy assets but must preserve the old
// generated agent, warn about its disabled tools, and keep warning on no-op runs.
func TestDeliveryFirstUpgradeWarnsAndPreservesExistingConfig(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "untouched prior generated agent"
		if custom {
			name = "customized prior agent"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			xdg := filepath.Join(home, "custom-xdg")
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", xdg)
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			t.Setenv("OPENCODE_CONFIG", "")
			opts := Options{Target: "opencode", ProjectPath: filepath.Join(home, "project"), ModelRouting: ModelRoutingDisabled}
			configPath := filepath.Join(xdg, "opencode", "opencode.json")
			agents := map[string]interface{}{}
			for _, agent := range rottaAgents {
				agents[agent.key] = openCodeAgentEntry(agent)
			}
			// Literal prior generated values: do not derive these from new defaults.
			owner := agents["rotta-orchestrator"].(map[string]interface{})
			owner["tools"] = map[string]bool{"bash": false, "delegate": true, "delegation_list": true, "delegation_read": true, "edit": false, "read": true, "write": false}
			owner["prompt"] = "You are Rotta-Orchestrator. Load rotta-core and rotta-orchestrator from ~/.config/opencode/skills/rotta-next/ before acting. Do not implement code or execute ordinary operations."
			if custom {
				owner["prompt"] = "Keep my custom policy loader"
				owner["tools"].(map[string]bool)["write"] = true
			}
			agents["user-agent"] = map[string]interface{}{"prompt": "preserve my assistant", "tools": map[string]bool{"bash": false}}
			original, err := json.MarshalIndent(map[string]interface{}{"agent": agents, "default_agent": "user-agent", "theme": "user-theme"}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			writeRottaNextTestFile(t, configPath, original)
			managed, err := openCodeManagedSkills(opts, home)
			if err != nil {
				t.Fatal(err)
			}
			corePath := filepath.Join(xdg, "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md")
			managed[corePath] = []byte("previous owned core policy\n")
			if _, err := installManagedFiles(home, managed); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				result, err := Install(opts)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Warnings) != 1 {
					t.Fatalf("attempt %d: expected reconciliation warning, got %#v", attempt, result.Warnings)
				}
				warning := result.Warnings[0]
				assertRottaNextContainsAll(t, warning, []string{configPath, "agent.rotta-orchestrator.tools.edit=false", "Refreshed policies alone do not enable local work-record persistence", "restart OpenCode", "bash does not need enabling"})
				if strings.Contains(warning, "agent.rotta-orchestrator.tools.write=false") == custom {
					t.Fatalf("warning misidentifies disabled tools: %s", warning)
				}
				got, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, original) {
					t.Fatalf("attempt %d changed existing config bytes", attempt)
				}
				assertRottaNextFileContains(t, corePath, "## Stable Work History")
				if attempt == 1 && (result.BackupDir != "" || len(result.Files) != 0) {
					t.Fatalf("warning turned no-op into a mutation: %#v", result)
				}
			}
		})
	}
}

func TestDeliveryFirstFreshInstallHasNoReconciliationWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("OPENCODE_CONFIG", "")
	result, err := Install(Options{Target: "opencode", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("fresh writable default produced warning: %#v", result.Warnings)
	}
	assertDeliveryFirstOpenCodeTools(t, home)
}

// These scenarios assert shipped prompt relationships, not runtime agent decisions.
// The installation test below separately exercises the real consumer/tool wiring.
func TestDeliveryFirstPolicyScenarios(t *testing.T) {
	scenarios := []struct {
		name   string
		asset  string
		want   []string
		absent []string
	}{
		{
			name:  "approved scope proceeds without a second contract pipeline",
			asset: "core/rotta-core.md",
			want: []string{
				"one compact approval packet: outcome, approved scope/non-goals, invariants, acceptance checks and necessary examples together",
				"Do not automatically dispatch independent contract review, spec-repair pipelines, or equivalent child approvals",
				"Ask the user directly about material ambiguity",
				"reapproval covers only material changes",
				"Together with an implementation request it permits ordinary in-scope implementation and delegation",
			},
			absent: []string{"It approves no delegation", "safely stop on resolution", "requires a new flow", "exact rendered draft requires separate explicit Strict approval"},
		},
		{
			name:  "orchestrator corrects failed acceptance before dispatch",
			asset: "agents/rotta-orchestrator.md",
			want: []string{
				"Before independent review, require the implementer's reproducible acceptance results",
				"If 57 of 61 acceptance checks fail while focused tests pass, route to correction, not independent review",
				"Return deterministic in-scope failures to the implementer under the same approval",
				"Hold dependent review readiness for material required evidence gaps",
			},
			absent: []string{"Stop for a materially incomplete capsule, scope contradiction, unapproved expansion, failed required verification"},
		},
		{
			name:  "implementer supplies reproducible results rather than a green focused suite",
			asset: "agents/rotta-impl.md",
			want: []string{
				"Before independent review, run reproducible acceptance checks for every check in the assigned boundary",
				"working directory, environment/fixtures, actual results and evidence tied to the current diff",
				"A skipped/unavailable required check is an evidence gap, not a pass",
				"If 57 of 61 acceptance checks fail while focused tests pass, route to correction, not independent review",
				"Label prompt assertions separately from runtime behavior tests",
			},
		},
		{
			name:  "unready review handoff preserves failures and invocation count",
			asset: "agents/rotta-review.md",
			want: []string{
				"return the unready handoff with its existing failures",
				"An accidentally dispatched review still counts as an invocation",
				"reproducible acceptance checks, commands/procedures, working directory, environment/fixtures, actual results",
				"Workflow preferences cannot create contract-review pipelines, equivalent child approvals or extra scope",
			},
		},
		{
			name:  "resume rename and stale memory preserve one history",
			asset: "core/rotta-core.md",
			want: []string{
				"one stable work ID per approved outcome across agents, resumes, renames and internal slices",
				"approval evidence references (user message/session",
				"outstanding acceptance failures and review finding IDs/dispositions",
				"cumulative initial/delta review and recovery counts",
				"passed checks with commands, actual results and evidence references",
				"Unknown history stays unknown, never zero by assumption",
				"Missing approval evidence holds only affected scope for confirmation",
				"Stale memory cannot authorize work, replace local approval evidence, reset counts or advance status",
			},
		},
		{
			name:  "conflicting and failed writes cannot claim a saved checkpoint",
			asset: "core/rotta-core.md",
			want: []string{
				"The orchestrator is the sole work-record writer",
				"re-read the record and compare the expected work ID, workspace and revision",
				"On mismatch, do not overwrite",
				"Increment revision after merging an accepted delta and read back the write",
				"not an atomic compare-and-swap or host lock",
				"A failed write/read-back is a persistence gap, not a saved checkpoint",
				"Fast needs no worktree or feature manifest",
			},
		},
		{
			name:  "foundation is not whole-feature delivery",
			asset: "core/rotta-core.md",
			want: []string{
				"Each task names the capability it enables and its acceptance checks",
				"A foundation task must explain the dependency it unlocks",
				"`implemented` (code/artifact exists)",
				"`integrated` (connected to the real consumer path)",
				"`verified` (required checks passed on that path)",
				"`delivered` (the approved user-visible outcome is usable at its agreed boundary)",
				"while the feature remains undelivered",
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			data, err := assets.FS.ReadFile(scenario.asset)
			if err != nil {
				t.Fatal(err)
			}
			assertRottaNextContainsAll(t, string(data), scenario.want)
			assertRottaNextLacksAll(t, string(data), scenario.absent)
		})
	}
}

// Real installer integration: all four policies reach each host consumer, and
// OpenCode can actually read/write the record and run implementation checks.
// No host process or model is launched; this does not test agent compliance.
func TestDeliveryFirstInstalledConsumersAndTools(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	for _, host := range []string{"opencode", "claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			home := t.TempDir()
			var err error
			switch host {
			case "opencode":
				_, err = installOpenCode(Options{}, home)
			case "claude":
				_, err = installClaudeCode(Options{}, home)
			case "codex":
				_, err = installCodex(Options{}, home)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range []string{"rotta-core", "rotta-orchestrator", "rotta-impl", "rotta-review"} {
				assetPath := "agents/" + role + ".md"
				if role == "rotta-core" {
					assetPath = "core/rotta-core.md"
				}
				data, err := assets.FS.ReadFile(assetPath)
				if err != nil {
					t.Fatal(err)
				}
				// The integration placeholder is rendered per install choice.
				policy := strings.Split(string(data), "## Installed Integrations")[0]
				var paths []string
				switch host {
				case "opencode":
					paths = []string{filepath.Join(home, ".config", "opencode", "skills", "rotta-next", role, "SKILL.md")}
				case "claude":
					paths = []string{filepath.Join(home, ".claude", "skills", "rotta-next", role, "SKILL.md")}
					if role != "rotta-core" {
						paths = append(paths, filepath.Join(home, ".claude", "agents", role+".md"))
					}
				case "codex":
					paths = []string{filepath.Join(home, ".codex", "AGENTS.md")}
				}
				for _, path := range paths {
					// Host bindings are inserted between frontmatter and body.
					// Require both original sections without rejecting that binding.
					for _, section := range strings.SplitN(policy, "\n\n", 2) {
						assertRottaNextFileContains(t, path, section)
					}
				}
			}
			if host == "opencode" {
				assertDeliveryFirstOpenCodeTools(t, home)
			}
		})
	}
}

func assertDeliveryFirstOpenCodeTools(t *testing.T, home string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Agents map[string]struct {
			Prompt string          `json:"prompt"`
			Tools  map[string]bool `json:"tools"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"rotta-orchestrator", "rotta-impl", "rotta-review"} {
		agent, ok := config.Agents[role]
		if !ok {
			t.Fatalf("missing consumer %s", role)
		}
		root := filepath.Join(home, ".config", "opencode", "skills", "rotta-next")
		assertRottaNextContainsAll(t, agent.Prompt, []string{
			"use the read tool with absolute filePath",
			filepath.Join(root, "rotta-core", "SKILL.md"),
			filepath.Join(root, role, "SKILL.md"),
		})
		assertRottaNextLacksAll(t, agent.Prompt, []string{"Load rotta-core and " + role, "~/.config/opencode/skills/rotta-next/"})
		if !agent.Tools["read"] {
			t.Fatalf("%s cannot read shared work record", role)
		}
	}
	owner := config.Agents["rotta-orchestrator"]
	if !owner.Tools["edit"] || !owner.Tools["write"] || owner.Tools["bash"] {
		t.Fatalf("record owner must have file tools without ordinary shell operations: %#v", owner.Tools)
	}
	assertRottaNextContainsAll(t, owner.Prompt, []string{"Use file edits only for workflow records and approval packets", "Do not implement code"})
	if !config.Agents["rotta-impl"].Tools["bash"] {
		t.Fatal("implementer cannot run acceptance commands")
	}
	if config.Agents["rotta-review"].Tools["write"] || config.Agents["rotta-review"].Tools["edit"] {
		t.Fatal("reviewer gained shared-record write tools")
	}
}
