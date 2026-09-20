package installer

import (
	"fmt"
	"strings"
)

func installSelectedHosts(opts Options, result *Result, home, projectPath string) (*Result, error) {
	if opts.Target == "all" {
		return installAllHosts(opts, result, home, projectPath)
	}
	if err := installNamedHosts(opts, result, home); err != nil {
		if opts.Target == "pi" {
			result.Hosts["pi"] = HostInstallResult{Host: "pi", Status: HostInstallStatusFailed, Capabilities: map[string]HostCapability{}}
			recordMCPStatuses(result, opts)
			for name, enabled := range map[string]bool{"ancora": opts.SetupAncora, "vela": opts.SetupVela, "context7": opts.SetupContext7} {
				if !enabled {
					continue
				}
				result.MCPStatuses["pi"][name] = MCPStatusResult{Status: MCPStatusUnavailable, Reason: "Pi installation failed before managed MCP configuration was available: " + err.Error(), Remediation: "Repair the Pi installation failure and rerun Rotta.", RuntimeFallback: MCPRuntimeFallback{State: MCPRuntimeFallbackNotObserved}}
				result.Hosts["pi"].Capabilities["mcp:"+name] = HostCapability{Name: "mcp:" + name, Status: HostCapabilityStatusFailed, Reason: err.Error(), Remediation: "Repair the Pi installation failure and rerun Rotta."}
			}
		}
		if piConfigOwnershipRefusal(err, home) {
			// Ownership refusal is deliberately not a successful installation.
			// Still expose the preserved bytes as unvalidated evidence so callers
			// do not mistake the selected service for pending/healthy.
			for name, enabled := range map[string]bool{"ancora": opts.SetupAncora, "vela": opts.SetupVela, "context7": opts.SetupContext7} {
				if enabled {
					result.MCPStatuses["pi"][name] = MCPStatusResult{Status: MCPStatusPreserved, Reason: "Managed Pi configuration was preserved unvalidated after ownership refusal: " + err.Error(), Remediation: "Reconcile the user-owned Pi file before rerunning Rotta.", RuntimeFallback: MCPRuntimeFallback{State: MCPRuntimeFallbackNotObserved}}
					result.Hosts["pi"].Capabilities["mcp:"+name] = HostCapability{Name: "mcp:" + name, Status: HostCapabilityStatusFailed, Reason: result.MCPStatuses["pi"][name].Reason, Remediation: result.MCPStatuses["pi"][name].Remediation}
				}
			}
		}
		return result, err
	}
	files, err := installConfig(projectPath)
	if err != nil {
		return nil, err
	}
	result.Files = append(result.Files, files...)
	return result, nil
}

// Ownership is the sole error class that preserves an existing user file.
// Asset, snapshot, write, injected-failure and rollback errors must remain
// ordinary failed installation evidence rather than a fabricated preservation.
func piConfigOwnershipRefusal(err error, home string) bool {
	path := piMCPConfigPath(home)
	return err != nil && strings.Contains(err.Error(), path) &&
		strings.Contains(err.Error(), "managed") &&
		(strings.Contains(err.Error(), "modified") || strings.Contains(err.Error(), "unmanaged"))
}

func installNamedHosts(opts Options, result *Result, home string) error {
	for _, host := range selectedHosts(opts.Target) {
		files, err := installHost(opts, host, home)
		if err != nil {
			return err
		}
		result.Files = append(result.Files, files...)
		hostResult, err := installedHostResult(opts, host, home, files)
		if err != nil {
			return err
		}
		result.Hosts[host] = hostResult
	}
	return nil
}

func installedHostResult(opts Options, host, home string, files []string) (HostInstallResult, error) {
	result := HostInstallResult{Host: host, Status: HostInstallStatusInstalled, Files: files}
	if host != "opencode" {
		return result, nil
	}
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return HostInstallResult{}, err
	}
	result.OpenCodeConfig = resolution
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return HostInstallResult{}, err
	}
	result.Capabilities = map[string]HostCapability{"source_loading": openCodeSourceLoadingCapability(home, document.config)}
	return result, nil
}

func installHost(opts Options, host, home string) ([]string, error) {
	switch host {
	case "claude-code":
		return installClaudeCode(opts, home)
	case "opencode":
		return installOpenCode(opts, home)
	case "codex":
		return installCodex(opts, home)
	case "pi":
		return installPi(opts, home)
	default:
		return nil, fmt.Errorf("unsupported host target %q", host)
	}
}

func setupContext7(opts Options, result *Result, home, projectPath string) (bool, error) {
	if !opts.SetupContext7 || opts.Target == "pi" {
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
