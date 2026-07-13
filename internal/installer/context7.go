package installer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const context7ServerName = "context7"

var context7CommandArgs = []string{"-y", "@upstash/context7-mcp"}

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
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create Claude MCP dir: %w", err)
	}
	data, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal Context7 Claude MCP: %w", err)
	}
	return writePrivateFile(path, data, 0o600)
}

func CheckContext7Health(server Context7MCPServer) Context7HealthResult {
	result := Context7HealthResult{Command: server.Command, Args: append([]string(nil), server.Args...), Transport: server.Type}
	if err := validateContext7HealthCommand(server); err != nil {
		setContext7HealthFailure(&result, Context7FailureCommandUnavailable, err)
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), context7HealthTimeout)
	defer cancel()
	process, err := startContext7HealthProcess(ctx, server)
	if err != nil {
		setContext7HealthFailure(&result, Context7FailureStartup, err)
		return result
	}
	defer process.close()
	if category, err := initializeContext7(ctx, process); err != nil {
		setContext7HealthFailure(&result, category, err)
		return result
	}
	result.Initialized = true
	tools, err := discoverContext7Tools(ctx, process)
	if err != nil {
		setContext7HealthFailure(&result, categoryForReadError(ctx, Context7FailureToolDiscovery), err)
		return result
	}
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

func validateContext7HealthCommand(server Context7MCPServer) error {
	if server.Command != "npx" || !sameArguments(server.Args, context7CommandArgs) {
		return fmt.Errorf("Context7 health checks require the managed npx @upstash/context7-mcp command")
	}
	_, err := exec.LookPath(server.Command)
	return err
}

type context7HealthProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
}

func startContext7HealthProcess(ctx context.Context, server Context7MCPServer) (context7HealthProcess, error) {
	cmd := exec.CommandContext(ctx, "npx")
	cmd.Args = append(cmd.Args, server.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return context7HealthProcess{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return context7HealthProcess{}, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return context7HealthProcess{}, err
	}
	return context7HealthProcess{cmd: cmd, stdin: stdin, reader: bufio.NewReader(stdout)}, nil
}

func (process context7HealthProcess) close() {
	_ = process.cmd.Process.Kill()
	_ = process.cmd.Wait()
}

func initializeContext7(ctx context.Context, process context7HealthProcess) (Context7FailureCategory, error) {
	if err := writeJSONRPC(process.stdin, context7InitializeRequest()); err != nil {
		return Context7FailureStartup, err
	}
	initResp, err := readJSONRPC(ctx, process.reader)
	if err != nil {
		return categoryForReadError(ctx, Context7FailureStartup), err
	}
	if _, failed := initResp["error"]; failed {
		return Context7FailureInitialization, fmt.Errorf("%v", initResp["error"])
	}
	_ = writeJSONRPC(process.stdin, map[string]interface{}{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return Context7FailureNone, nil
}

func context7InitializeRequest() map[string]interface{} {
	return map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{"protocolVersion": "2024-11-05", "capabilities": map[string]interface{}{}, "clientInfo": map[string]interface{}{"name": "rotta", "version": "dev"}}}
}

func discoverContext7Tools(ctx context.Context, process context7HealthProcess) ([]string, error) {
	if err := writeJSONRPC(process.stdin, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]interface{}{}}); err != nil {
		return nil, err
	}
	toolsResp, err := readJSONRPC(ctx, process.reader)
	if err != nil {
		return nil, err
	}
	return extractToolNames(toolsResp), nil
}

func setContext7HealthFailure(result *Context7HealthResult, category Context7FailureCategory, err error) {
	result.Category = category
	result.Message = err.Error()
}

func sameArguments(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
