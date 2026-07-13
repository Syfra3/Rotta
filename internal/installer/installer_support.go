package installer

import (
	"fmt"
	"io/fs"
	"os"
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
			hostStatuses[strings.TrimPrefix(capabilityName, "mcp:")] = mcpStatusResult(result.Hosts[host], capabilityName)
		}
		result.MCPStatuses[host] = hostStatuses
	}
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
		result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusInstalled, Files: files}
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
		if err := cleanPreviousOpenCodeInstallation(home); err != nil {
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
	dir := filepath.Join(projectPath, ".rotta")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("cannot create .rotta dir: %w", err)
	}
	configs := map[string]string{"config/state-machine.yaml": filepath.Join(dir, "state-machine.yaml"), "config/quality-gates.yaml": filepath.Join(dir, "quality-gates.yaml")}
	var files []string
	for src, dst := range configs {
		data, err := assets.FS.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return nil, fmt.Errorf("cannot write %s: %w", dst, err)
		}
		files = append(files, dst)
	}
	return files, nil
}

func cleanPreviousInstallation(opts Options, home, projectPath string) error {
	if opts.Target != "all" {
		if err := cleanSelectedHosts(opts.Target, home); err != nil {
			return err
		}
	}
	return cleanSelectedIntegrationArtifacts(opts, home, projectPath)
}
func cleanSelectedIntegrationArtifacts(opts Options, home, projectPath string) error {
	if opts.SetupVela {
		if err := cleanVelaArtifacts(opts.Target, home, projectPath); err != nil {
			return err
		}
	}
	if opts.SetupAncora && (opts.Target == "claude-code" || opts.Target == "both") {
		return removeIntegrationArtifacts(filepath.Join(home, ".claude", "mcp", "ancora.json"))
	}
	return nil
}
func cleanSelectedHosts(target, home string) error {
	for _, host := range selectedHosts(target) {
		if err := cleanHostInstallation(host, home); err != nil {
			return err
		}
	}
	return nil
}
func cleanHostInstallation(host, home string) error {
	switch host {
	case "opencode":
		return cleanPreviousOpenCodeInstallation(home)
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
	modes := []struct {
		enabled   bool
		src, name string
	}{{opts.InstallSpec, "skills/spec-mode", "spec-mode"}, {opts.InstallImpl, "skills/implementation-mode", "implementation-mode"}, {opts.InstallReview, "skills/review-mode", "review-mode"}}
	var files []string
	for _, mode := range modes {
		if !mode.enabled {
			continue
		}
		dst := filepath.Join(skillsDir, "rotta", mode.name)
		if err := os.MkdirAll(dst, 0o750); err != nil {
			return nil, fmt.Errorf("cannot create dir %s: %w", dst, err)
		}
		if err := copySkillTree(opts, mode.src, dst); err != nil {
			return nil, fmt.Errorf("cannot copy %s: %w", mode.src, err)
		}
		files = append(files, filepath.Join(dst, "SKILL.md"))
	}
	return files, nil
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
