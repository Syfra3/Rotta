package installer

import "fmt"

func installSelectedHosts(opts Options, result *Result, home, projectPath string) (*Result, error) {
	if opts.Target == "all" {
		return installAllHosts(opts, result, home, projectPath)
	}
	if err := installNamedHosts(opts, result, home); err != nil {
		return nil, err
	}
	files, err := installConfig(projectPath)
	if err != nil {
		return nil, err
	}
	result.Files = append(result.Files, files...)
	return result, nil
}

func installNamedHosts(opts Options, result *Result, home string) error {
	for _, host := range selectedHosts(opts.Target) {
		files, err := installHost(opts, host, home)
		if err != nil {
			return err
		}
		result.Files = append(result.Files, files...)
		result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusInstalled, Files: files}
	}
	return nil
}

func installHost(opts Options, host, home string) ([]string, error) {
	switch host {
	case "claude-code":
		return installClaudeCode(opts, home)
	case "opencode":
		return installOpenCode(opts, home)
	case "codex":
		return installCodex(opts, home)
	default:
		return nil, fmt.Errorf("unsupported host target %q", host)
	}
}

func setupContext7(opts Options, result *Result, home, projectPath string) (bool, error) {
	if !opts.SetupContext7 {
		return false, nil
	}
	context7Result, err := ConfigureContext7(opts, home)
	if err != nil {
		return false, fmt.Errorf("context7 setup: %w", err)
	}
	result.Context7 = context7Result
	result.Files = append(result.Files, context7Result.Files...)
	if !context7ConfiguredForHealthCheck(context7Result) {
		return false, nil
	}
	return reportContext7Health(opts, result, projectPath)
}

func context7ConfiguredForHealthCheck(result Context7Result) bool {
	return result.OpenCode.OK || result.ClaudeCode.OK
}

func reportContext7Health(opts Options, result *Result, projectPath string) (bool, error) {
	health := CheckContext7Health(Context7ServerConfig())
	result.Context7.Health = health
	result.Context7.HealthRan = true
	if health.OK && result.Context7.FullyConfigured {
		result.Context7.Status = Context7StatusConfigured
		return false, nil
	}
	if health.OK {
		return false, nil
	}
	recordMCPHostCapabilities(result, opts)
	recordMCPHealthFailure(result, opts, "mcp:context7", health)
	recordMCPStatuses(result, opts)
	recordChangedFiles(result, projectPath)
	return true, fmt.Errorf("context7 health: %s", health.Category)
}

func setupCodexMCP(opts Options, result *Result, home, projectPath string) error {
	if !targetsCodex(opts.Target) || !hasSelectedMCP(opts) {
		return nil
	}
	files, err := configureCodexMCPServers(opts, home)
	if err != nil {
		return recordCodexMCPFailure(opts, result, projectPath, err)
	}
	result.Files = append(result.Files, files...)
	if opts.SetupContext7 {
		result.Context7.Codex = context7HostConfigResult{Host: "codex", OK: true}
	}
	return nil
}

func hasSelectedMCP(opts Options) bool {
	return opts.SetupAncora || opts.SetupVela || opts.SetupContext7
}

func recordCodexMCPFailure(opts Options, result *Result, projectPath string, err error) error {
	if opts.SetupContext7 {
		result.Context7.Codex = context7HostConfigResult{Host: "codex", OK: false, Err: err}
	}
	recordHostArtifactFailure(result, "codex", "Codex MCP config", opts)
	recordCommandHostCapabilities(result, opts)
	recordMCPHostCapabilities(result, opts)
	recordChangedFiles(result, projectPath)
	installErr := fmt.Errorf("codex MCP config: %w", err)
	result.Error = installErr.Error()
	return installErr
}
