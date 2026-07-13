// Package installer handles writing Rotta files to the target tool.
package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Options configures what and where to install.
type Options struct {
	Target          string // "claude-code" | "opencode" | "both"
	ProjectPath     string // project root; config files land here under .rotta/
	InstallSpec     bool
	InstallImpl     bool
	InstallReview   bool
	UseDefaultGates bool
	SetupAncora     bool // whether to install/configure Ancora memory
	SetupVela       bool // whether to install/configure Vela graph intelligence
	SetupContext7   bool // whether to configure Context7 documentation MCP
	CommandStdin    io.Reader
	CommandStdout   io.Writer
	CommandStderr   io.Writer
}

// Result describes what was installed.
type Result struct {
	Target                          string
	Files                           []string
	ChangedFiles                    map[FileChangeCategory][]string
	LifecycleArtifactsRequireCommit bool
	Hosts                           map[string]HostInstallResult
	BackupDir                       string
	Error                           string
	AncoraInstalled                 bool   // true if Ancora binary was installed during this run
	AncoraBin                       string // resolved path to the ancora binary
	VelaInstalled                   bool   // true if Vela binary was installed during this run
	VelaBin                         string // resolved path to the vela binary
	Context7                        Context7Result
	MCPStatuses                     map[string]map[string]MCPStatusResult
}

// MCPStatus reports a selected MCP's installation configuration or health state.
type MCPStatus string

const (
	MCPStatusConfigured MCPStatus = "configured"
	MCPStatusSkipped    MCPStatus = "skipped"
	MCPStatusDegraded   MCPStatus = "degraded"
	MCPStatusFailed     MCPStatus = "failed"
)

// MCPRuntimeFallbackState reports only runtime fallback observed during workflow use.
type MCPRuntimeFallbackState string

const (
	MCPRuntimeFallbackNotObserved MCPRuntimeFallbackState = "not observed during installation"
)

type MCPRuntimeFallback struct {
	State MCPRuntimeFallbackState
}

type MCPStatusResult struct {
	Status          MCPStatus
	Reason          string
	Remediation     string
	RuntimeFallback MCPRuntimeFallback
}

type FileChangeCategory string

const (
	FileChangeCategoryHostConfig          FileChangeCategory = "host_config"
	FileChangeCategoryWorkspaceHostConfig FileChangeCategory = "workspace_host_config"
	FileChangeCategoryLifecycle           FileChangeCategory = "lifecycle"
)

type HostInstallStatus string

const (
	HostInstallStatusInstalled HostInstallStatus = "installed"
	HostInstallStatusFailed    HostInstallStatus = "failed"
)

type HostCapabilityStatus string

const (
	HostCapabilityStatusExact         HostCapabilityStatus = "exact"
	HostCapabilityStatusAdapted       HostCapabilityStatus = "adapted"
	HostCapabilityStatusDegraded      HostCapabilityStatus = "degraded"
	HostCapabilityStatusUnsupported   HostCapabilityStatus = "unsupported"
	HostCapabilityStatusSkipped       HostCapabilityStatus = "skipped"
	HostCapabilityStatusFailed        HostCapabilityStatus = "failed"
	HostCapabilityStatusNotApplicable HostCapabilityStatus = "not applicable"
)

type HostCapability struct {
	Name        string
	Status      HostCapabilityStatus
	Reason      string
	Remediation string
}

type HostInstallResult struct {
	Host         string
	Status       HostInstallStatus
	Files        []string
	Capabilities map[string]HostCapability
}

func install(opts Options) (*Result, error) {
	result, home, projectPath, err := prepareInstall(opts)
	if err != nil {
		return result, err
	}

	if err := cleanPreviousInstallation(opts, home, projectPath); err != nil {
		return failedCleanInstall(result, opts, projectPath, err)
	}

	installResult, err := installSelectedHosts(opts, result, home, projectPath)
	if err != nil {
		recordChangedFiles(result, projectPath)
		return installResult, err
	}

	if err := setupAncora(opts, result, home); err != nil {
		return nil, err
	}
	if err := setupVela(opts, result, home, projectPath); err != nil {
		return nil, err
	}

	if keepResult, err := setupContext7(opts, result, home, projectPath); err != nil {
		if keepResult {
			return result, err
		}
		return nil, err
	}

	if err := setupCodexMCP(opts, result, home, projectPath); err != nil {
		return result, err
	}
	finalizeInstall(result, opts, projectPath)

	return result, nil
}

// Install runs the full installation and returns a summary.
func Install(opts Options) (*Result, error) {
	return install(opts)
}

func prepareInstall(opts Options) (*Result, string, string, error) {
	if !isSupportedInstallTarget(opts.Target) {
		return nil, "", "", fmt.Errorf("unsupported host target %q; supported hosts are exactly Claude Code, OpenCode, and Codex", opts.Target)
	}
	result := &Result{Target: opts.Target, Hosts: map[string]HostInstallResult{}}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	projectPath := resolveProjectPath(opts.ProjectPath, home)
	backupDir, err := createInstallBackup(opts, home, projectPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("backup failure prevented installation: %w", err)
	}
	result.BackupDir = backupDir
	return result, home, projectPath, nil
}

func failedCleanInstall(result *Result, opts Options, projectPath string, err error) (*Result, error) {
	installErr := fmt.Errorf("%s host configuration: %w", installTargetLabel(opts.Target), err)
	recordSelectedHostFailure(result, opts, installErr)
	recordChangedFiles(result, projectPath)
	return result, installErr
}

func setupAncora(opts Options, result *Result, home string) error {
	if !opts.SetupAncora {
		return nil
	}
	ar, err := SetupAncora(opts, home)
	if err != nil {
		return fmt.Errorf("ancora setup: %w", err)
	}
	result.Files = append(result.Files, ar.Files...)
	result.AncoraInstalled = ar.Installed
	result.AncoraBin = ar.BinPath
	return nil
}

func setupVela(opts Options, result *Result, home, projectPath string) error {
	if !opts.SetupVela {
		return nil
	}
	vr, err := SetupVela(opts, home, projectPath)
	if err != nil {
		return fmt.Errorf("vela setup: %w", err)
	}
	result.Files = append(result.Files, vr.Files...)
	result.VelaInstalled = vr.Installed
	result.VelaBin = vr.BinPath
	if len(vr.MCPAvailability) != 0 {
		markBackedUpVelaConfigurations(vr, result.BackupDir, home)
		if velaConfigurationNeedsRestore(vr) {
			if _, err := RestoreBackup(result.BackupDir); err != nil {
				return fmt.Errorf("restore previous Vela configuration: %w", err)
			}
		}
		recordVelaMCPAvailability(result, vr)
		return nil
	}
	files, err := installVelaFreshnessGuards(opts, home)
	if err != nil {
		return fmt.Errorf("vela freshness guard setup: %w", err)
	}
	result.Files = append(result.Files, files...)
	return nil
}

func markBackedUpVelaConfigurations(result *VelaResult, backupDir, home string) {
	// Host cleanup precedes Vela setup, so use the transaction backup to
	// distinguish a missing configuration from one that must be restored.
	manifest, err := loadBackupManifest(filepath.Join(backupDir, "manifest.json"))
	if err != nil {
		return
	}
	backedUp := map[string]bool{}
	for _, path := range manifest.BackedUpPaths {
		backedUp[path] = true
	}
	for host, statuses := range result.MCPAvailability {
		agent, configDir := velaHostConfig(host, home)
		if statuses["vela"].Status == MCPStatusSkipped && backedUp[velaMCPConfigPath(agent, configDir)] {
			statuses["vela"] = preservedVelaMCPStatus()
		}
	}
}

func velaConfigurationNeedsRestore(result *VelaResult) bool {
	for _, hostStatuses := range result.MCPAvailability {
		if hostStatuses["vela"].Status == MCPStatusDegraded {
			return true
		}
	}
	return false
}

func recordVelaMCPAvailability(result *Result, vela *VelaResult) {
	for host, statuses := range vela.MCPAvailability {
		hostResult := result.Hosts[host]
		if hostResult.Capabilities == nil {
			hostResult.Capabilities = map[string]HostCapability{}
		}
		status := statuses["vela"]
		capabilityStatus := HostCapabilityStatusSkipped
		if status.Status == MCPStatusDegraded {
			capabilityStatus = HostCapabilityStatusDegraded
		}
		hostResult.Capabilities["mcp:vela"] = HostCapability{
			Name:        "mcp:vela",
			Status:      capabilityStatus,
			Reason:      status.Reason,
			Remediation: status.Remediation,
		}
		result.Hosts[host] = hostResult
	}
}

func finalizeInstall(result *Result, opts Options, projectPath string) {
	recordCommandHostCapabilities(result, opts)
	recordMCPHostCapabilities(result, opts)
	recordHostCapabilityMatrix(result, opts)
	recordMCPStatuses(result, opts)
	recordChangedFiles(result, projectPath)
}

func recordSelectedHostFailure(result *Result, opts Options, err error) {
	for _, host := range selectedHosts(opts.Target) {
		result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusFailed}
	}
	result.Error = err.Error()
}

func installTargetLabel(target string) string {
	if target == "" {
		return "selected"
	}
	return target
}

func recordHostArtifactFailure(result *Result, host, artifactType string, opts Options) {
	hostResult, ok := result.Hosts[host]
	if !ok {
		hostResult = HostInstallResult{Host: host}
	}
	hostResult.Status = HostInstallStatusFailed
	if hostResult.Capabilities == nil {
		hostResult.Capabilities = map[string]HostCapability{}
	}
	for _, capabilityName := range selectedMCPCapabilities(opts) {
		hostResult.Capabilities[capabilityName] = HostCapability{
			Name:        capabilityName,
			Status:      HostCapabilityStatusFailed,
			Reason:      artifactType + " failed; completed host configuration remains valid and can be kept for retry.",
			Remediation: "It is safe to rerun Rotta after repairing the failed artifact path, or manually restore from the backup directory before retrying.",
		}
	}
	result.Hosts[host] = hostResult
}

func selectedMCPCapabilities(opts Options) []string {
	var capabilities []string
	if opts.SetupAncora {
		capabilities = append(capabilities, "mcp:ancora")
	}
	if opts.SetupVela {
		capabilities = append(capabilities, "mcp:vela")
	}
	if opts.SetupContext7 {
		capabilities = append(capabilities, "mcp:context7")
	}
	return capabilities
}

func recordChangedFiles(result *Result, projectPath string) {
	result.Files = deduplicateStrings(result.Files)
	changed := map[FileChangeCategory][]string{
		FileChangeCategoryHostConfig:          {},
		FileChangeCategoryWorkspaceHostConfig: {},
		FileChangeCategoryLifecycle:           {},
	}
	for _, file := range result.Files {
		category := classifyChangedFile(file, projectPath)
		changed[category] = append(changed[category], file)
	}
	result.ChangedFiles = changed
	result.LifecycleArtifactsRequireCommit = false
}

func deduplicateStrings(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func classifyChangedFile(path, projectPath string) FileChangeCategory {
	if isLifecycleArtifact(path, projectPath) {
		return FileChangeCategoryLifecycle
	}
	if isWithin(path, projectPath) {
		return FileChangeCategoryWorkspaceHostConfig
	}
	return FileChangeCategoryHostConfig
}

func isLifecycleArtifact(path, projectPath string) bool {
	for _, dir := range []string{".rotta", "features", "reports", "specs"} {
		if isWithin(path, filepath.Join(projectPath, dir)) {
			return true
		}
	}
	return false
}

func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != "" && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func recordCommandHostCapabilities(result *Result, opts Options) {
	for _, host := range selectedHosts(opts.Target) {
		hostResult, ok := result.Hosts[host]
		if !ok || hostResult.Status != HostInstallStatusInstalled {
			continue
		}
		if hostResult.Capabilities == nil {
			hostResult.Capabilities = map[string]HostCapability{}
		}
		hostResult.Capabilities["commands"] = commandCapability(host)
		result.Hosts[host] = hostResult
	}
}

func commandCapability(host string) HostCapability {
	if host == "opencode" {
		return HostCapability{Name: "commands", Status: HostCapabilityStatusExact}
	}
	return HostCapability{
		Name:        "commands",
		Status:      HostCapabilityStatusAdapted,
		Reason:      "Host exposes Rotta command behavior through documented natural-language invocation instead of OpenCode-style slash commands.",
		Remediation: "Invoke the same canonical Rotta commands by name, such as Rotta init, Rotta new, Rotta continue, Rotta status, Rotta skip, and Rotta back; Rotta maps them to the same canonical Rotta commands and state transitions.",
	}
}

func recordMCPHostCapabilities(result *Result, opts Options) {
	for _, host := range selectedHosts(opts.Target) {
		hostResult, ok := result.Hosts[host]
		if !ok || hostResult.Status != HostInstallStatusInstalled {
			continue
		}
		if hostResult.Capabilities == nil {
			hostResult.Capabilities = map[string]HostCapability{}
		}
		if opts.SetupAncora {
			hostResult.Capabilities["mcp:ancora"] = exactMCPCapability("mcp:ancora")
		}
		if opts.SetupVela {
			if _, reported := hostResult.Capabilities["mcp:vela"]; !reported {
				hostResult.Capabilities["mcp:vela"] = exactMCPCapability("mcp:vela")
			}
		}
		if opts.SetupContext7 {
			hostResult.Capabilities["mcp:context7"] = context7MCPCapability(host)
		}
		result.Hosts[host] = hostResult
	}
}

func recordHostCapabilityMatrix(result *Result, opts Options) {
	for _, host := range selectedHosts(opts.Target) {
		hostResult, ok := result.Hosts[host]
		if !ok {
			continue
		}
		if hostResult.Capabilities == nil {
			hostResult.Capabilities = map[string]HostCapability{}
		}
		hostResult.Capabilities["installation"] = installationCapability(hostResult.Status)
		hostResult.Capabilities["instructions"] = instructionsCapability(host)
		if _, ok := hostResult.Capabilities["commands"]; !ok {
			hostResult.Capabilities["commands"] = commandCapability(host)
		}
		hostResult.Capabilities["mcp"] = mcpCapability(opts, host)
		hostResult.Capabilities["health_checks"] = healthCheckCapability(opts, host)
		hostResult.Capabilities["lifecycle"] = exactCapability("lifecycle")
		result.Hosts[host] = hostResult
	}
}

func installationCapability(status HostInstallStatus) HostCapability {
	if status == HostInstallStatusFailed {
		return HostCapability{Name: "installation", Status: HostCapabilityStatusFailed}
	}
	return exactCapability("installation")
}

func instructionsCapability(host string) HostCapability {
	if host == "codex" {
		return HostCapability{
			Name:        "instructions",
			Status:      HostCapabilityStatusAdapted,
			Reason:      "Codex consumes Rotta workflow instructions through AGENTS.md rather than exact agent and skill artifacts.",
			Remediation: "Use the generated AGENTS.md instructions as the Codex entry point for the same canonical Rotta workflow.",
		}
	}
	return exactCapability("instructions")
}

func mcpCapability(opts Options, host string) HostCapability {
	if !opts.SetupAncora && !opts.SetupVela && !opts.SetupContext7 {
		return HostCapability{Name: "mcp", Status: HostCapabilityStatusSkipped, Reason: "No MCP integrations were selected for this installation."}
	}
	if host == "codex" && opts.SetupContext7 {
		return HostCapability{
			Name:        "mcp",
			Status:      HostCapabilityStatusDegraded,
			Reason:      "At least one selected Codex MCP integration lacks Codex-specific observable health validation.",
			Remediation: "Verify selected MCP servers from Codex after install.",
		}
	}
	return exactCapability("mcp")
}

func healthCheckCapability(opts Options, host string) HostCapability {
	if !opts.SetupContext7 {
		return HostCapability{Name: "health_checks", Status: HostCapabilityStatusSkipped, Reason: "No health-checked MCP integration was selected for this installation."}
	}
	if host == "codex" {
		return HostCapability{
			Name:        "health_checks",
			Status:      HostCapabilityStatusDegraded,
			Reason:      "Rotta does not have a Codex-specific observable MCP health check.",
			Remediation: "Verify MCP startup from Codex manually after install.",
		}
	}
	return exactCapability("health_checks")
}

func exactCapability(name string) HostCapability {
	return HostCapability{Name: name, Status: HostCapabilityStatusExact}
}
