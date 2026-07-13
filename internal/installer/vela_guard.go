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
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o750); err != nil {
		return nil, fmt.Errorf("cannot create opencode plugin dir: %w", err)
	}
	if err := writePrivateFile(pluginPath, []byte(openCodeVelaFreshnessGuardPlugin()), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o750); err != nil {
		return nil, fmt.Errorf("cannot create Claude Code hook dir: %w", err)
	}
	if err := writePrivateFile(hookPath, []byte(claudeVelaFreshnessGuardScript()), 0o700); err != nil {
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
	data, err := readPrivateFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create Claude Code settings dir: %w", err)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings.json: %w", err)
	}
	return writePrivateFile(path, out, 0o600)
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

func openCodeVelaFreshnessGuardPlugin() string { return openCodeVelaFreshnessGuardPluginSource }

func claudeVelaFreshnessGuardScript() string { return claudeVelaFreshnessGuardScriptSource }
