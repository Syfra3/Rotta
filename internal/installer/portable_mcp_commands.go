package installer

import (
	"encoding/json"
	"fmt"
	"os"
)

// serializeManagedMCPCommand replaces a command emitted by a Rotta-managed
// setup flow with the portable executable name Rotta owns for that server.
func serializeManagedMCPCommand(path, server, command string) error {
	_, err := normalizeManagedMCPCommand(path, server, command)
	return err
}

// normalizeManagedMCPCommand replaces a stale command and reports whether its
// managed MCP command field changed.
func normalizeManagedMCPCommand(path, server, command string) (bool, error) {
	data, err := readPrivateFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, fmt.Errorf("parse managed MCP config: %w", err)
	}
	entry := config
	if server != "" {
		mcp, _ := config["mcp"].(map[string]interface{})
		entry, _ = mcp[server].(map[string]interface{})
	}
	if entry == nil || !replaceManagedMCPCommand(entry, command) {
		return false, nil
	}

	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal managed MCP config: %w", err)
	}
	if err := writePrivateFile(path, data, 0o600); err != nil {
		return false, err
	}
	return true, nil
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
