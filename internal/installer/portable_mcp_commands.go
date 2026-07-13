package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// serializeManagedMCPCommand replaces a command emitted by a Rotta-managed
// setup flow with the portable executable name Rotta owns for that server.
func serializeManagedMCPCommand(path, server, command string) error {
	return serializeRecoveredManagedMCPCommand(path, server, command)
}

// serializeRecoveredManagedMCPCommand serializes a proven managed MCP entry
// using its canonical portable executable command.
func serializeRecoveredManagedMCPCommand(path, server, command string) error {
	_, err := normalizeManagedMCPCommand(path, server, command)
	return err
}

// normalizeManagedMCPCommand replaces a stale command and reports whether its
// managed MCP command field changed.
func normalizeManagedMCPCommand(path, server, command string) (bool, error) {
	changed, _, err := normalizeProvenManagedMCPCommand(path, server, command)
	return changed, err
}

// normalizeProvenManagedMCPCommand changes an entry only when its expected
// Rotta-managed MCP shape proves ownership. It also reports an absolute or
// slash-containing command that was deliberately left untouched.
func normalizeProvenManagedMCPCommand(path, server, command string) (changed, ambiguous bool, err error) {
	data, err := readPrivateFile(path)
	if os.IsNotExist(err) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, false, fmt.Errorf("parse managed MCP config: %w", err)
	}
	entry := config
	if server != "" {
		mcp, _ := config["mcp"].(map[string]interface{})
		entry, _ = mcp[server].(map[string]interface{})
	}
	if entry == nil {
		return false, false, nil
	}
	if !isProvenManagedMCPEntry(entry) {
		return false, hasSlashMCPCommand(entry), nil
	}
	if !replaceManagedMCPCommand(entry, command) {
		return false, false, nil
	}

	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, false, fmt.Errorf("marshal managed MCP config: %w", err)
	}
	if err := writePrivateFile(path, data, 0o600); err != nil {
		return false, false, err
	}
	return true, false, nil
}

func isProvenManagedMCPEntry(entry map[string]interface{}) bool {
	// Rotta-managed entries start their MCP invocation with "mcp"; any remaining
	// arguments are valid managed-server options and are preserved.
	args, ok := entry["args"].([]interface{})
	return ok && len(args) > 0 && args[0] == "mcp"
}

func hasSlashMCPCommand(entry map[string]interface{}) bool {
	switch command := entry["command"].(type) {
	case string:
		return strings.Contains(command, "/")
	case []interface{}:
		return len(command) > 0 && command[0] != nil && strings.Contains(fmt.Sprint(command[0]), "/")
	default:
		return false
	}
}

func replaceManagedMCPCommand(entry map[string]interface{}, command string) bool {
	switch current := entry["command"].(type) {
	case string:
		if current == command {
			return false
		}
		entry["command"] = command
	case []interface{}:
		if len(current) == 0 || current[0] == command {
			return false
		}
		current[0] = command
	default:
		return false
	}
	return true
}
