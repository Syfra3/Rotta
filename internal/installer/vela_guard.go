package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	openCodeVelaFreshnessPluginFile = "rotta-vela-freshness-guard.js"
	claudeVelaFreshnessHookFile     = "rotta-vela-freshness-guard.sh"
	claudeVelaPreToolUseMatcher     = "^(mcp__vela__.*|mcp__ancora__(ancora_)?vela_.*|vela_.*|ancora_vela_.*)$"
)

func installVelaFreshnessGuards(opts Options, home string) ([]string, error) {
	var files []string
	if opts.Target == "opencode" || opts.Target == "both" {
		installed, err := installOpenCodeVelaFreshnessGuard(home)
		if err != nil {
			return nil, err
		}
		files = append(files, installed...)
	}
	if opts.Target == "claude-code" || opts.Target == "both" {
		installed, err := installClaudeCodeVelaFreshnessGuard(home)
		if err != nil {
			return nil, err
		}
		files = append(files, installed...)
	}
	return files, nil
}

func openCodeVelaFreshnessPluginPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "plugin", openCodeVelaFreshnessPluginFile)
}

func claudeCodeVelaFreshnessHookPath(home string) string {
	return filepath.Join(home, ".claude", "hooks", claudeVelaFreshnessHookFile)
}

func installOpenCodeVelaFreshnessGuard(home string) ([]string, error) {
	pluginPath := openCodeVelaFreshnessPluginPath(home)
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create opencode plugin dir: %w", err)
	}
	if err := os.WriteFile(pluginPath, []byte(openCodeVelaFreshnessGuardPlugin()), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write opencode Vela freshness guard: %w", err)
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	config, err := readOpenCodeConfig(configPath)
	if err != nil {
		return nil, err
	}
	addOpenCodePluginEntry(config, openCodePluginFileURL(pluginPath))
	if err := writeOpenCodeConfig(configPath, config); err != nil {
		return nil, err
	}

	return []string{pluginPath, configPath}, nil
}

func cleanOpenCodeVelaFreshnessGuard(home string) error {
	pluginPath := openCodeVelaFreshnessPluginPath(home)
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	config, err := readOpenCodeConfig(configPath)
	if err != nil {
		return err
	}
	if removeOpenCodePluginEntry(config, openCodePluginFileURL(pluginPath), filepath.ToSlash(pluginPath)) {
		if err := writeOpenCodeConfig(configPath, config); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(pluginPath); err != nil {
		return fmt.Errorf("cannot remove stale opencode Vela freshness guard %s: %w", pluginPath, err)
	}
	return nil
}

func addOpenCodePluginEntry(config map[string]interface{}, pluginPath string) {
	config["plugin"] = appendUniquePluginPath(config["plugin"], pluginPath)
}

func openCodePluginFileURL(path string) string {
	return "file://" + filepath.ToSlash(path)
}

func removeOpenCodePluginEntry(config map[string]interface{}, pluginPaths ...string) bool {
	plugins, ok := config["plugin"].([]interface{})
	if !ok {
		return false
	}
	remove := map[string]bool{}
	for _, pluginPath := range pluginPaths {
		remove[pluginPath] = true
	}
	kept := make([]interface{}, 0, len(plugins))
	removed := false
	for _, plugin := range plugins {
		value, _ := plugin.(string)
		if remove[value] {
			removed = true
			continue
		}
		kept = append(kept, plugin)
	}
	config["plugin"] = kept
	return removed
}

func appendUniquePluginPath(raw interface{}, pluginPath string) []interface{} {
	var plugins []interface{}
	switch value := raw.(type) {
	case []interface{}:
		plugins = value
	case []string:
		for _, item := range value {
			plugins = append(plugins, item)
		}
	case string:
		plugins = append(plugins, value)
	}

	seen := map[string]bool{}
	deduped := make([]interface{}, 0, len(plugins)+1)
	for _, plugin := range plugins {
		value, ok := plugin.(string)
		if ok {
			if value == pluginPath {
				if seen[pluginPath] {
					continue
				}
				seen[pluginPath] = true
			}
			deduped = append(deduped, value)
			continue
		}
		deduped = append(deduped, plugin)
	}
	if !seen[pluginPath] {
		deduped = append(deduped, pluginPath)
	}
	return deduped
}

func installClaudeCodeVelaFreshnessGuard(home string) ([]string, error) {
	hookPath := claudeCodeVelaFreshnessHookPath(home)
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create Claude Code hook dir: %w", err)
	}
	if err := os.WriteFile(hookPath, []byte(claudeVelaFreshnessGuardScript()), 0o755); err != nil {
		return nil, fmt.Errorf("cannot write Claude Code Vela freshness hook: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := addClaudeCodeVelaFreshnessHooks(settingsPath, hookPath); err != nil {
		return nil, err
	}
	return []string{hookPath, settingsPath}, nil
}

func cleanClaudeCodeVelaFreshnessGuard(home string) error {
	hookPath := claudeCodeVelaFreshnessHookPath(home)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := removeClaudeCodeVelaFreshnessHooks(settingsPath, hookPath); err != nil {
		return err
	}
	if err := os.RemoveAll(hookPath); err != nil {
		return fmt.Errorf("cannot remove stale Claude Code Vela freshness hook %s: %w", hookPath, err)
	}
	return nil
}

func addClaudeCodeVelaFreshnessHooks(settingsPath, hookPath string) error {
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		settings = map[string]interface{}{}
	}
	hooks := claudeHooksMap(settings)
	command := shellCommandForPath(hookPath)
	appendClaudeCommandHook(hooks, "SessionStart", "", command)
	appendClaudeCommandHook(hooks, "PreToolUse", claudeVelaPreToolUseMatcher, command)
	settings["hooks"] = hooks
	return writeClaudeSettings(settingsPath, settings)
}

func removeClaudeCodeVelaFreshnessHooks(settingsPath, hookPath string) error {
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}
	changed := false
	for _, event := range []string{"SessionStart", "PreToolUse"} {
		if removeClaudeCommandHook(hooks, event, hookPath) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	settings["hooks"] = hooks
	return writeClaudeSettings(settingsPath, settings)
}

func readClaudeSettings(path string) (map[string]interface{}, error) {
	settings := map[string]interface{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("cannot parse settings.json: %w", err)
	}
	return settings, nil
}

func writeClaudeSettings(path string, settings map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create Claude Code settings dir: %w", err)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings.json: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}

func claudeHooksMap(settings map[string]interface{}) map[string]interface{} {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}
	return hooks
}

func appendClaudeCommandHook(hooks map[string]interface{}, event, matcher, command string) {
	entries, _ := hooks[event].([]interface{})
	if claudeHookCommandExists(entries, matcher, command) {
		return
	}
	entry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	}
	if matcher != "" {
		entry["matcher"] = matcher
	}
	hooks[event] = append(entries, entry)
}

func claudeHookCommandExists(entries []interface{}, matcher, command string) bool {
	for _, entry := range entries {
		entryMap, _ := entry.(map[string]interface{})
		if matcher != "" {
			existingMatcher, _ := entryMap["matcher"].(string)
			if existingMatcher != matcher {
				continue
			}
		}
		hookList, _ := entryMap["hooks"].([]interface{})
		for _, hook := range hookList {
			hookMap, _ := hook.(map[string]interface{})
			existingCommand, _ := hookMap["command"].(string)
			if existingCommand == command {
				return true
			}
		}
	}
	return false
}

func removeClaudeCommandHook(hooks map[string]interface{}, event, hookPath string) bool {
	entries, _ := hooks[event].([]interface{})
	if len(entries) == 0 {
		return false
	}
	changed := false
	keptEntries := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			keptEntries = append(keptEntries, entry)
			continue
		}
		hookList, _ := entryMap["hooks"].([]interface{})
		keptHooks := make([]interface{}, 0, len(hookList))
		for _, hook := range hookList {
			hookMap, _ := hook.(map[string]interface{})
			command, _ := hookMap["command"].(string)
			if isClaudeVelaFreshnessCommand(command, hookPath) {
				changed = true
				continue
			}
			keptHooks = append(keptHooks, hook)
		}
		if len(keptHooks) == 0 {
			changed = true
			continue
		}
		entryMap["hooks"] = keptHooks
		keptEntries = append(keptEntries, entryMap)
	}
	if len(keptEntries) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = keptEntries
	}
	return changed
}

func isClaudeVelaFreshnessCommand(command, hookPath string) bool {
	return command == hookPath || command == shellCommandForPath(hookPath) || strings.Contains(command, hookPath)
}

func shellCommandForPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func openCodeVelaFreshnessGuardPlugin() string {
	return `import { spawnSync } from "node:child_process";

const GRAPH_QUERY_TOOLS = new Set([
  "vela_explore",
  "vela_lookup",
  "vela_dependencies",
  "vela_reverse_dependencies",
  "vela_impact",
  "vela_path",
  "vela_explain",
  "vela_architecture",
  "vela_query_graph",
  "vela_federated_search",
  "vela_get_node",
  "vela_get_neighbors",
  "vela_graph_stats",
  "vela_shortest_path",
]);

function normalizeToolName(toolName) {
  if (!toolName) return "";
  if (toolName.startsWith("mcp__vela__")) {
    const suffix = toolName.slice("mcp__vela__".length);
    return suffix.startsWith("vela_") ? suffix : "vela_" + suffix;
  }
  if (toolName.startsWith("mcp__ancora__ancora_vela_")) return toolName.slice("mcp__ancora__".length);
  if (toolName.startsWith("mcp__ancora__vela_")) return toolName.slice("mcp__ancora__".length);
  return toolName;
}

function isVelaGraphQueryTool(toolName) {
  toolName = normalizeToolName(String(toolName || ""));
  if (!toolName) return false;
  if (toolName.includes("status")) return false; // Do not recursively guard vela_status.
  if (toolName === "vela_update" || toolName === "vela_build" || toolName === "vela_install" || toolName === "vela_extract") return false;
  return GRAPH_QUERY_TOOLS.has(toolName) || toolName.startsWith("ancora_vela_");
}

function resolveWorkspace(input, output, context) {
  return input?.cwd || input?.directory || input?.session?.cwd || input?.session?.directory ||
    output?.args?.cwd || output?.args?.directory || context.directory || context.worktree || process.cwd();
}

function runVela(command, workspace) {
  return spawnSync("vela", [command, workspace], {
    cwd: workspace,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

function refreshVelaGraph(workspace, reason) {
  const update = runVela("update", workspace); // vela update <workspace>
  if (update.status === 0) return;

  const build = runVela("build", workspace); // vela build <workspace>
  if (build.status === 0) return;

  throw new Error(
    "Rotta Vela freshness guard blocked " + reason + " for " + workspace + ". " +
      "Tried \"vela update " + workspace + "\" and \"vela build " + workspace + "\". " +
      "update: " + (update.stderr || update.stdout || update.error || "failed") + "; " +
      "build: " + (build.stderr || build.stdout || build.error || "failed"),
  );
}

export const RottaVelaFreshnessGuard = async ({ directory, worktree }) => {
  const context = { directory, worktree };
  let warmedSession = false;

  return {
    event: async ({ event }) => {
      if (event?.type !== "session.created" || warmedSession) return;
      warmedSession = true;
      refreshVelaGraph(resolveWorkspace({}, {}, context), "OpenCode session start");
    },
    "tool.execute.before": async (input, output) => {
      const toolName = input?.tool || input?.toolName || output?.tool || output?.name || "";
      if (!isVelaGraphQueryTool(toolName)) return;
      refreshVelaGraph(resolveWorkspace(input, output, context), "graph query " + toolName);
    },
  };
};
`
}

func claudeVelaFreshnessGuardScript() string {
	return `#!/usr/bin/env bash
# Rotta Vela freshness guard. Installed by Rotta; rerun rotta install to update it.
set -u

hook_input="$(cat || true)"
tool_name=""
cwd_from_input=""

if command -v python3 >/dev/null 2>&1; then
  mapfile -t parsed_hook_input < <(HOOK_INPUT_JSON="$hook_input" python3 - <<'PY'
import json
import os

raw = os.environ.get("HOOK_INPUT_JSON", "")
try:
    data = json.loads(raw) if raw else {}
except Exception:
    data = {}

def nested(obj, *keys):
    cur = obj
    for key in keys:
        if not isinstance(cur, dict):
            return ""
        cur = cur.get(key)
    return cur if isinstance(cur, str) else ""

tool = (
    nested(data, "tool_name")
    or nested(data, "tool")
    or nested(data, "tool", "name")
    or nested(data, "toolName")
)
cwd = (
    nested(data, "cwd")
    or nested(data, "project_dir")
    or nested(data, "workspace_dir")
    or nested(data, "session", "cwd")
    or nested(data, "session", "directory")
    or nested(data, "tool_input", "cwd")
)
print(tool)
print(cwd)
PY
)
  tool_name="${parsed_hook_input[0]:-}"
  cwd_from_input="${parsed_hook_input[1]:-}"
fi

normalize_tool_name() {
  local tool="$1"
  case "$tool" in
    mcp__vela__*)
      local suffix="${tool#mcp__vela__}"
      case "$suffix" in
        vela_*) printf '%s' "$suffix" ;;
        *) printf 'vela_%s' "$suffix" ;;
      esac
      ;;
    mcp__ancora__ancora_vela_*) printf '%s' "${tool#mcp__ancora__}" ;;
    mcp__ancora__vela_*) printf '%s' "${tool#mcp__ancora__}" ;;
    *) printf '%s' "$tool" ;;
  esac
}

isVelaGraphQueryTool() {
  local tool
  tool="$(normalize_tool_name "$1")"
  # Do not recursively guard vela_status; it is the freshness/readiness check itself.
  case "$tool" in
    *status*) return 1 ;;
    vela_update|vela_build|vela_install|vela_extract) return 1 ;;
    vela_explore|vela_lookup|vela_dependencies|vela_reverse_dependencies|vela_impact|vela_path|vela_explain|vela_architecture|vela_query_graph|vela_federated_search|vela_get_node|vela_get_neighbors|vela_graph_stats|vela_shortest_path) return 0 ;;
    ancora_vela_*) return 0 ;;
    *) return 1 ;;
  esac
}

if [ -n "$tool_name" ] && ! isVelaGraphQueryTool "$tool_name"; then
  exit 0
fi

project_dir="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$project_dir" ]; then
  project_dir="$cwd_from_input"
fi
if [ -z "$project_dir" ]; then
  project_dir="$(pwd)"
fi

if ! command -v vela >/dev/null 2>&1; then
  echo "Rotta Vela freshness guard blocked graph query: vela binary not found. Install Vela, then rerun rotta install." >&2
  exit 2
fi

if vela update "$project_dir"; then # vela update <workspace>
  exit 0
fi
update_status=$?

if vela build "$project_dir"; then # vela build <workspace>
  exit 0
fi
build_status=$?

echo "Rotta Vela freshness guard blocked graph query for $project_dir." >&2
echo "Tried: vela update $project_dir (exit $update_status), then vela build $project_dir (exit $build_status)." >&2
echo "Run 'vela build $project_dir' manually or fix Vela installation, then retry the graph query." >&2
exit 2
`
}
