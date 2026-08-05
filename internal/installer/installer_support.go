package installer

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Syfra3/Rotta/assets"
)

func recordMCPHealthFailure(result *Result, opts Options, capabilityName string, health Context7HealthResult) {
	for _, host := range selectedHosts(opts.Target) {
		hostResult, ok := result.Hosts[host]
		if !ok || hostResult.Status != HostInstallStatusInstalled {
			continue
		}
		if hostResult.Capabilities == nil {
			hostResult.Capabilities = map[string]HostCapability{}
		}
		hostResult.Status = HostInstallStatusFailed
		hostResult.Capabilities[capabilityName] = failedMCPCapability(capabilityName, health)
		result.Hosts[host] = hostResult
	}
}

func failedMCPCapability(name string, health Context7HealthResult) HostCapability {
	if health.Category == Context7FailureCommandUnavailable {
		return HostCapability{Name: name, Status: HostCapabilityStatusFailed, Reason: fmt.Sprintf("host command availability: %s", health.Message), Remediation: "Add the MCP command to the host process PATH, then restart the host and rerun Rotta."}
	}
	return HostCapability{Name: name, Status: HostCapabilityStatusFailed, Reason: fmt.Sprintf("MCP health check failed during %s: %s", health.Category, health.Message), Remediation: "Ensure the MCP command is available, starts successfully, initializes, and exposes expected tools before rerunning Rotta."}
}
func exactMCPCapability(name string) HostCapability {
	return HostCapability{Name: name, Status: HostCapabilityStatusExact}
}

func recordMCPStatuses(result *Result, opts Options) {
	result.MCPStatuses = map[string]map[string]MCPStatusResult{}
	for _, host := range selectedHosts(opts.Target) {
		hostStatuses := map[string]MCPStatusResult{}
		for _, capabilityName := range selectedMCPCapabilities(opts) {
			name := strings.TrimPrefix(capabilityName, "mcp:")
			status := mcpStatusResult(result.Hosts[host], capabilityName)
			if host == "opencode" {
				status = openCodeMCPStatus(result, name, status)
			}
			hostStatuses[name] = status
		}
		result.MCPStatuses[host] = hostStatuses
	}
}

func openCodeMCPStatus(result *Result, name string, status MCPStatusResult) MCPStatusResult {
	command := managedMCPCommand(name)
	if path, err := exec.LookPath(command); err == nil {
		status.CommandResolution = MCPObservation{Status: MCPObservationCompleted, Detail: "Installer preflight resolved the managed command."}
		status.ResolvedCommandPath = path
	} else {
		status.CommandResolution = MCPObservation{Status: MCPObservationNotObservable, Detail: "Managed command is unavailable to the installer."}
	}

	status.FileWrite, status.SchemaValidity = openCodeMCPConfigurationObservations(result.Hosts["opencode"].OpenCodeConfig, name)
	status.OpenCodeServerResolution = resolveOpenCodeMCPServer(name)
	status.ToolDiscovery = openCodeToolDiscovery(name, result.Context7)
	return status
}

func openCodeMCPConfigurationObservations(resolution OpenCodeConfigResolution, name string) (MCPObservation, MCPObservation) {
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return MCPObservation{Status: MCPObservationFailed, Detail: "Cannot read the selected OpenCode configuration after installation."}, MCPObservation{Status: MCPObservationNotObservable, Detail: "Selected OpenCode configuration could not be read for validation."}
	} else if err := validateOpenCodeConfigurationShape(document.config); err != nil {
		return MCPObservation{Status: MCPObservationFailed, Detail: "Selected OpenCode configuration is invalid after installation."}, MCPObservation{Status: MCPObservationFailed, Detail: err.Error()}
	}
	if openCodeMCPEntryExists(document.config, name) {
		return MCPObservation{Status: MCPObservationCompleted, Detail: "Selected managed MCP entry was written to the effective OpenCode configuration."}, MCPObservation{Status: MCPObservationCompleted, Detail: "Selected OpenCode configuration remains schema-valid."}
	}
	return MCPObservation{Status: MCPObservationFailed, Detail: "Selected managed MCP entry is absent from the effective OpenCode configuration."}, MCPObservation{Status: MCPObservationCompleted, Detail: "Selected OpenCode configuration remains schema-valid."}
}

func managedMCPCommand(name string) string {
	if name == context7ServerName {
		return Context7ServerConfig().Command
	}
	return name
}

func openCodeMCPEntryExists(config map[string]interface{}, name string) bool {
	mcp, _ := config["mcp"].(map[string]interface{})
	_, exists := mcp[name]
	return exists
}

func openCodeToolDiscovery(name string, context7 Context7Result) MCPObservation {
	if name != context7ServerName || !context7.HealthRan {
		return MCPObservation{Status: MCPObservationNotObservable, Detail: "No OpenCode tool enumeration was observed."}
	}
	if !context7.Health.ToolsDiscovered {
		return MCPObservation{Status: MCPObservationFailed, Detail: "Direct server tools/list did not discover the required tools.", Source: MCPDiscoverySourceDirectServer}
	}
	return MCPObservation{Status: MCPObservationCompleted, Detail: "Tools were discovered by direct server tools/list, not OpenCode.", Source: MCPDiscoverySourceDirectServer}
}

func mcpStatusResult(host HostInstallResult, capabilityName string) MCPStatusResult {
	status, reason, remediation := MCPStatusConfigured, "Selected MCP configuration completed for this host.", "Use the generated host rules to report and recover from any later runtime fallback."
	if host.Status == HostInstallStatusFailed {
		status, reason, remediation = MCPStatusFailed, "Host installation failed before this selected MCP could be confirmed.", "Repair the reported host configuration failure and safely rerun Rotta."
	}
	if capability, ok := host.Capabilities[capabilityName]; ok {
		status = statusForCapability(capability.Status)
		if capability.Reason != "" {
			reason = capability.Reason
		}
		if capability.Remediation != "" {
			remediation = capability.Remediation
		}
	}
	return MCPStatusResult{Status: status, Reason: reason, Remediation: remediation, RuntimeFallback: MCPRuntimeFallback{State: MCPRuntimeFallbackNotObserved}}
}

func statusForCapability(status HostCapabilityStatus) MCPStatus {
	switch status {
	case HostCapabilityStatusSkipped:
		return MCPStatusSkipped
	case HostCapabilityStatusDegraded, HostCapabilityStatusUnsupported:
		return MCPStatusDegraded
	case HostCapabilityStatusFailed:
		return MCPStatusFailed
	}
	return MCPStatusConfigured
}
func context7MCPCapability(host string) HostCapability {
	if host == "opencode" {
		return HostCapability{
			Name:        "mcp:context7",
			Status:      HostCapabilityStatusDegraded,
			Reason:      "portable-but-host-resolution-unverified",
			Remediation: "Launch OpenCode with npx available on PATH, then verify Context7 startup from OpenCode.",
		}
	}
	if host == "codex" {
		return HostCapability{Name: "mcp:context7", Status: HostCapabilityStatusDegraded, Reason: "Rotta can write Codex MCP TOML for Context7, but does not have a Codex-specific observable MCP health check.", Remediation: "Verify Context7 from Codex after install; rerun Rotta after Codex MCP health support is available."}
	}
	return exactMCPCapability("mcp:context7")
}
func selectedHosts(target string) []string {
	switch target {
	case "all":
		return []string{"claude-code", "opencode", "codex"}
	case "both":
		return []string{"claude-code", "opencode"}
	case "claude-code", "opencode", "codex":
		return []string{target}
	}
	return nil
}
func targetsCodex(target string) bool { return target == "codex" || target == "all" }
func isSupportedInstallTarget(target string) bool {
	switch target {
	case "", "claude-code", "opencode", "codex", "both", "all":
		return true
	}
	return false
}

func installAllHosts(opts Options, result *Result, home, projectPath string) (*Result, error) {
	var installErr error
	for _, host := range []string{"claude-code", "opencode", "codex"} {
		files, err := cleanAndInstallHost(opts, host, home)
		if err != nil {
			result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusFailed}
			installErr = fmt.Errorf("%s host installation: %w", host, err)
			continue
		}
		result.Files = append(result.Files, files...)
		hostResult, err := installedHostResult(opts, host, home, files)
		if err != nil {
			result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusFailed}
			installErr = fmt.Errorf("%s host configuration: %w", host, err)
			continue
		}
		result.Hosts[host] = hostResult
	}
	files, err := installConfig(projectPath)
	if err != nil {
		return result, err
	}
	result.Files = append(result.Files, files...)
	if installErr != nil {
		result.Error = installErr.Error()
		return result, installErr
	}
	return result, nil
}

func cleanAndInstallHost(opts Options, host, home string) ([]string, error) {
	hostOpts := opts
	hostOpts.Target = host
	switch host {
	case "claude-code":
		if err := cleanPreviousClaudeCodeInstallation(home); err != nil {
			return nil, err
		}
		return installClaudeCode(hostOpts, home)
	case "opencode":
		if err := cleanPreviousOpenCodeInstallation(hostOpts, home); err != nil {
			return nil, err
		}
		return installOpenCode(hostOpts, home)
	case "codex":
		if err := cleanPreviousCodexInstallation(home); err != nil {
			return nil, err
		}
		return installCodex(hostOpts, home)
	}
	return nil, fmt.Errorf("unsupported host target %q", host)
}
func resolveProjectPath(path, home string) string {
	if path == "" || path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func installConfig(projectPath string) ([]string, error) {
	return nil, nil
}

func cleanPreviousInstallation(opts Options, home, projectPath string) error {
	if opts.Target != "all" {
		if err := cleanSelectedHosts(opts, home); err != nil {
			return err
		}
	}
	return cleanSelectedIntegrationArtifacts(opts, home, projectPath)
}
func cleanSelectedIntegrationArtifacts(opts Options, home, projectPath string) error {
	return nil
}
func cleanSelectedHosts(opts Options, home string) error {
	for _, host := range selectedHosts(opts.Target) {
		if err := cleanHostInstallationWithOptions(opts, host, home); err != nil {
			return err
		}
	}
	return nil
}
func cleanHostInstallation(host, home string) error {
	return cleanHostInstallationWithOptions(Options{Target: host}, host, home)
}

func cleanHostInstallationWithOptions(opts Options, host, home string) error {
	switch host {
	case "opencode":
		return cleanPreviousOpenCodeInstallation(opts, home)
	case "claude-code":
		return cleanPreviousClaudeCodeInstallation(home)
	case "codex":
		return cleanPreviousCodexInstallation(home)
	}
	return nil
}
func cleanVelaArtifacts(target, home, projectPath string) error {
	paths := []string{filepath.Join(projectPath, ".vela", "graph.db")}
	if target == "claude-code" || target == "both" {
		if err := cleanClaudeCodeVelaFreshnessGuard(home); err != nil {
			return err
		}
		paths = append(paths, filepath.Join(home, ".claude", "vela-mcp.json"), filepath.Join(home, ".claude", "vela-instructions.md"))
	}
	if target == "opencode" || target == "both" {
		if err := cleanOpenCodeVelaFreshnessGuard(home); err != nil {
			return err
		}
		paths = append(paths, filepath.Join(home, ".config", "opencode", "instructions.md"))
	}
	return removeIntegrationArtifacts(paths...)
}
func removeIntegrationArtifacts(paths ...string) error {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove stale integration artifact %s: %w", path, err)
		}
	}
	return nil
}

func copySkillsToDir(opts Options, skillsDir string) ([]string, error) {
	managed := map[string][]byte{}
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		return nil, err
	}
	managed[filepath.Join(skillsDir, "rotta-next", "rotta-core", "SKILL.md")] = core
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, err
		}
		managed[filepath.Join(skillsDir, "rotta-next", agent.skillName, "SKILL.md")] = data
	}
	home := filepath.Clean(filepath.Join(skillsDir, "..", ".."))
	return installManagedFiles(home, managed)
}
func copySkillTree(opts Options, source, destination string) error {
	return fs.WalkDir(assets.FS, source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := readRenderedAsset(path, opts)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		output := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
			return err
		}
		return os.WriteFile(output, data, 0o600)
	})
}
