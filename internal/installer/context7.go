package installer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const context7ServerName = "context7"

var context7HealthTimeout = 5 * time.Second

type Context7Status string

const (
	Context7StatusSkipped    Context7Status = "skipped"
	Context7StatusPending    Context7Status = "configured-pending-health"
	Context7StatusConfigured Context7Status = "configured"
	Context7StatusPartial    Context7Status = "partial"
)

type Context7FailureCategory string

const (
	Context7FailureNone               Context7FailureCategory = ""
	Context7FailureCommandUnavailable Context7FailureCategory = "command availability"
	Context7FailureStartup            Context7FailureCategory = "server startup"
	Context7FailureInitialization     Context7FailureCategory = "MCP initialization"
	Context7FailureToolDiscovery      Context7FailureCategory = "tool discovery"
	Context7FailureTimeout            Context7FailureCategory = "timeout"
)

type Context7MCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type Context7Result struct {
	Status          Context7Status
	OpenCode        context7HostConfigResult
	ClaudeCode      context7HostConfigResult
	Codex           context7HostConfigResult
	FullyConfigured bool
	CommandChecked  bool
	HealthRan       bool
	Health          Context7HealthResult
	Files           []string
}

type context7HostConfigResult struct {
	Host string
	OK   bool
	Err  error
}

type Context7HealthResult struct {
	OK              bool
	Category        Context7FailureCategory
	Message         string
	Command         string
	Args            []string
	Transport       string
	Initialized     bool
	ToolsDiscovered bool
	Tools           []string
}

func Context7ServerConfig() Context7MCPServer {
	return Context7MCPServer{Type: "stdio", Command: "npx", Args: []string{"-y", "@upstash/context7-mcp"}}
}

func ConfigureContext7(opts Options, home string) (Context7Result, error) {
	if !opts.SetupContext7 {
		return Context7Result{Status: Context7StatusSkipped}, nil
	}

	server := Context7ServerConfig()
	var files []string
	opencode := context7HostConfigResult{Host: "opencode"}
	claude := context7HostConfigResult{Host: "claude-code"}

	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	opencode.OK, opencode.Err = true, writeOpenCodeContext7MCP(path, server)
	if opencode.Err != nil {
		opencode.OK = false
	} else {
		files = append(files, path)
	}

	path = filepath.Join(home, ".claude", "mcp", "context7.json")
	claude.OK, claude.Err = true, writeClaudeContext7MCP(path, server)
	if claude.Err != nil {
		claude.OK = false
	} else {
		files = append(files, path)
	}

	result := summarizeContext7HostConfig(opencode, claude)
	result.Files = files
	return result, nil
}

func summarizeContext7HostConfig(results ...context7HostConfigResult) Context7Result {
	result := Context7Result{Status: Context7StatusPending, FullyConfigured: true}
	seen := false
	for _, host := range results {
		if host.Host == "" {
			continue
		}
		seen = true
		switch host.Host {
		case "opencode":
			result.OpenCode = host
		case "claude-code":
			result.ClaudeCode = host
		}
		if !host.OK {
			result.FullyConfigured = false
			result.Status = Context7StatusPartial
		}
	}
	if !seen {
		result.Status = Context7StatusSkipped
		result.FullyConfigured = false
	}
	return result
}

func writeOpenCodeContext7MCP(path string, server Context7MCPServer) error {
	config, err := readOpenCodeConfig(path)
	if err != nil {
		return err
	}
	mcp, _ := config["mcp"].(map[string]interface{})
	if mcp == nil {
		mcp = map[string]interface{}{}
	}
	delete(mcp, "rotta-context7")
	mcp[context7ServerName] = map[string]interface{}{
		"type":    "local",
		"command": append([]string{server.Command}, server.Args...),
		"enabled": true,
	}
	config["mcp"] = mcp
	return writeOpenCodeConfig(path, config)
}

func writeClaudeContext7MCP(path string, server Context7MCPServer) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create Claude MCP dir: %w", err)
	}
	data, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal Context7 Claude MCP: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func CheckContext7Health(server Context7MCPServer) Context7HealthResult {
	result := Context7HealthResult{Command: server.Command, Args: append([]string(nil), server.Args...), Transport: server.Type}
	if _, err := exec.LookPath(server.Command); err != nil {
		result.Category = Context7FailureCommandUnavailable
		result.Message = err.Error()
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), context7HealthTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.Category = Context7FailureStartup
		result.Message = err.Error()
		return result
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Category = Context7FailureStartup
		result.Message = err.Error()
		return result
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		result.Category = Context7FailureStartup
		result.Message = err.Error()
		return result
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	reader := bufio.NewReader(stdout)
	if err := writeJSONRPC(stdin, map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{"protocolVersion": "2024-11-05", "capabilities": map[string]interface{}{}, "clientInfo": map[string]interface{}{"name": "rotta", "version": "dev"}}}); err != nil {
		result.Category = Context7FailureStartup
		result.Message = err.Error()
		return result
	}
	initResp, err := readJSONRPC(ctx, reader)
	if err != nil {
		result.Category = categoryForReadError(ctx, Context7FailureStartup)
		result.Message = err.Error()
		return result
	}
	if _, failed := initResp["error"]; failed {
		result.Category = Context7FailureInitialization
		result.Message = fmt.Sprint(initResp["error"])
		return result
	}
	result.Initialized = true
	_ = writeJSONRPC(stdin, map[string]interface{}{"jsonrpc": "2.0", "method": "notifications/initialized"})
	if err := writeJSONRPC(stdin, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]interface{}{}}); err != nil {
		result.Category = Context7FailureToolDiscovery
		result.Message = err.Error()
		return result
	}
	toolsResp, err := readJSONRPC(ctx, reader)
	if err != nil {
		result.Category = categoryForReadError(ctx, Context7FailureToolDiscovery)
		result.Message = err.Error()
		return result
	}
	tools := extractToolNames(toolsResp)
	result.Tools = tools
	if !hasContext7Tools(tools) {
		result.Category = Context7FailureToolDiscovery
		result.Message = "expected resolve-library-id and query-docs tools"
		return result
	}
	result.ToolsDiscovered = true
	result.OK = true
	return result
}

func writeJSONRPC(stdin interface{ Write([]byte) (int, error) }, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = stdin.Write(data)
	return err
}

func readJSONRPC(ctx context.Context, reader *bufio.Reader) (map[string]interface{}, error) {
	type response struct {
		msg map[string]interface{}
		err error
	}
	ch := make(chan response, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			ch <- response{err: err}
			return
		}
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			ch <- response{err: err}
			return
		}
		ch <- response{msg: msg}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case got := <-ch:
		return got.msg, got.err
	}
}

func categoryForReadError(ctx context.Context, fallback Context7FailureCategory) Context7FailureCategory {
	if ctx.Err() != nil {
		return Context7FailureTimeout
	}
	return fallback
}

func extractToolNames(resp map[string]interface{}) []string {
	result, _ := resp["result"].(map[string]interface{})
	tools, _ := result["tools"].([]interface{})
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		tool, _ := item.(map[string]interface{})
		if name, ok := tool["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func hasContext7Tools(tools []string) bool {
	foundResolve := false
	foundQuery := false
	for _, tool := range tools {
		name := strings.ToLower(tool)
		if strings.Contains(name, "resolve-library-id") || strings.Contains(name, "resolve_library_id") {
			foundResolve = true
		}
		if strings.Contains(name, "query-docs") || strings.Contains(name, "query_docs") || strings.Contains(name, "get-library-docs") || strings.Contains(name, "get_library_docs") {
			foundQuery = true
		}
	}
	return foundResolve && foundQuery
}
