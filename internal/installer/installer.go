// Package installer handles writing Rotta files to the target tool.
package installer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syfra3/Rotta/assets"
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

// Install runs the full installation and returns a summary.
func Install(opts Options) (*Result, error) {
	if !isSupportedInstallTarget(opts.Target) {
		return nil, fmt.Errorf("unsupported host target %q; supported hosts are exactly Claude Code, OpenCode, and Codex", opts.Target)
	}

	result := &Result{Target: opts.Target, Hosts: map[string]HostInstallResult{}}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}

	projectPath := resolveProjectPath(opts.ProjectPath, home)

	backupDir, err := createInstallBackup(opts, home, projectPath)
	if err != nil {
		return nil, fmt.Errorf("backup failure prevented installation: %w", err)
	}
	result.BackupDir = backupDir

	if err := cleanPreviousInstallation(opts, home, projectPath); err != nil {
		installErr := fmt.Errorf("%s host configuration: %w", installTargetLabel(opts.Target), err)
		recordSelectedHostFailure(result, opts, installErr)
		recordChangedFiles(result, projectPath)
		return result, installErr
	}

	if opts.Target == "all" {
		if _, err := installAllHosts(opts, result, home, projectPath); err != nil {
			recordChangedFiles(result, projectPath)
			return result, err
		}
	} else {
		if opts.Target == "claude-code" || opts.Target == "both" {
			files, err := installClaudeCode(opts, home)
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
			result.Hosts["claude-code"] = HostInstallResult{Host: "claude-code", Status: HostInstallStatusInstalled, Files: files}
		}

		if opts.Target == "opencode" || opts.Target == "both" {
			files, err := installOpenCode(opts, home)
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
			result.Hosts["opencode"] = HostInstallResult{Host: "opencode", Status: HostInstallStatusInstalled, Files: files}
		}

		if opts.Target == "codex" {
			files, err := installCodex(opts, home)
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
			result.Hosts["codex"] = HostInstallResult{Host: "codex", Status: HostInstallStatusInstalled, Files: files}
		}

		files, err := installConfig(projectPath)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
	}

	if opts.SetupAncora {
		ar, err := SetupAncora(opts, home)
		if err != nil {
			return nil, fmt.Errorf("ancora setup: %w", err)
		}
		result.Files = append(result.Files, ar.Files...)
		result.AncoraInstalled = ar.Installed
		result.AncoraBin = ar.BinPath
	}

	if opts.SetupVela {
		vr, err := SetupVela(opts, home, projectPath)
		if err != nil {
			return nil, fmt.Errorf("vela setup: %w", err)
		}
		result.Files = append(result.Files, vr.Files...)
		result.VelaInstalled = vr.Installed
		result.VelaBin = vr.BinPath

		files, err := installVelaFreshnessGuards(opts, home)
		if err != nil {
			return nil, fmt.Errorf("vela freshness guard setup: %w", err)
		}
		result.Files = append(result.Files, files...)
	}

	if opts.SetupContext7 {
		context7Result, err := ConfigureContext7(opts, home)
		if err != nil {
			return nil, fmt.Errorf("context7 setup: %w", err)
		}
		result.Context7 = context7Result
		result.Files = append(result.Files, context7Result.Files...)
		if context7Result.OpenCode.OK || context7Result.ClaudeCode.OK {
			health := CheckContext7Health(Context7ServerConfig())
			result.Context7.Health = health
			result.Context7.HealthRan = true
			if health.OK && context7Result.FullyConfigured {
				result.Context7.Status = Context7StatusConfigured
			} else if !health.OK {
				recordMCPHostCapabilities(result, opts)
				recordMCPHealthFailure(result, opts, "mcp:context7", health)
				recordMCPStatuses(result, opts)
				recordChangedFiles(result, projectPath)
				return result, fmt.Errorf("context7 health: %s", health.Category)
			}
		}
	}

	if targetsCodex(opts.Target) && (opts.SetupAncora || opts.SetupVela || opts.SetupContext7) {
		files, err := configureCodexMCPServers(opts, home)
		if err != nil {
			if opts.SetupContext7 {
				result.Context7.Codex = context7HostConfigResult{Host: "codex", OK: false, Err: err}
			}
			recordHostArtifactFailure(result, "codex", "Codex MCP config", opts)
			recordCommandHostCapabilities(result, opts)
			recordMCPHostCapabilities(result, opts)
			recordChangedFiles(result, projectPath)
			installErr := fmt.Errorf("codex MCP config: %w", err)
			result.Error = installErr.Error()
			return result, installErr
		}
		result.Files = append(result.Files, files...)
		if opts.SetupContext7 {
			result.Context7.Codex = context7HostConfigResult{Host: "codex", OK: true}
		}
	}
	recordCommandHostCapabilities(result, opts)
	recordMCPHostCapabilities(result, opts)
	recordHostCapabilityMatrix(result, opts)
	recordMCPStatuses(result, opts)
	recordChangedFiles(result, projectPath)

	return result, nil
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
			hostResult.Capabilities["mcp:vela"] = exactMCPCapability("mcp:vela")
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
	return HostCapability{
		Name:        name,
		Status:      HostCapabilityStatusFailed,
		Reason:      fmt.Sprintf("MCP health check failed during %s: %s", health.Category, health.Message),
		Remediation: "Ensure the MCP command is available, starts successfully, initializes, and exposes expected tools before rerunning Rotta.",
	}
}

func exactMCPCapability(name string) HostCapability {
	return HostCapability{Name: name, Status: HostCapabilityStatusExact}
}

func recordMCPStatuses(result *Result, opts Options) {
	result.MCPStatuses = map[string]map[string]MCPStatusResult{}
	for _, host := range selectedHosts(opts.Target) {
		hostStatuses := map[string]MCPStatusResult{}
		for _, capabilityName := range selectedMCPCapabilities(opts) {
			mcp := strings.TrimPrefix(capabilityName, "mcp:")
			hostStatuses[mcp] = mcpStatusResult(result.Hosts[host], capabilityName)
		}
		result.MCPStatuses[host] = hostStatuses
	}
}

func mcpStatusResult(host HostInstallResult, capabilityName string) MCPStatusResult {
	status := MCPStatusConfigured
	reason := "Selected MCP configuration completed for this host."
	remediation := "Use the generated host rules to report and recover from any later runtime fallback."
	if host.Status == HostInstallStatusFailed {
		status = MCPStatusFailed
		reason = "Host installation failed before this selected MCP could be confirmed."
		remediation = "Repair the reported host configuration failure and safely rerun Rotta."
	}
	if capability, ok := host.Capabilities[capabilityName]; ok {
		switch capability.Status {
		case HostCapabilityStatusExact:
			status = MCPStatusConfigured
			reason = "Selected MCP configuration completed for this host."
			remediation = "Use the generated host rules to report and recover from any later runtime fallback."
		case HostCapabilityStatusSkipped:
			status = MCPStatusSkipped
		case HostCapabilityStatusDegraded, HostCapabilityStatusUnsupported:
			status = MCPStatusDegraded
		case HostCapabilityStatusFailed:
			status = MCPStatusFailed
		}
		if capability.Reason != "" {
			reason = capability.Reason
		}
		if capability.Remediation != "" {
			remediation = capability.Remediation
		}
	}
	return MCPStatusResult{
		Status:      status,
		Reason:      reason,
		Remediation: remediation,
		RuntimeFallback: MCPRuntimeFallback{
			State: MCPRuntimeFallbackNotObserved,
		},
	}
}

func context7MCPCapability(host string) HostCapability {
	if host == "codex" {
		return HostCapability{
			Name:        "mcp:context7",
			Status:      HostCapabilityStatusDegraded,
			Reason:      "Rotta can write Codex MCP TOML for Context7, but does not have a Codex-specific observable MCP health check.",
			Remediation: "Verify Context7 from Codex after install; rerun Rotta after Codex MCP health support is available.",
		}
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
	default:
		return nil
	}
}

func targetsCodex(target string) bool {
	return target == "codex" || target == "all"
}

func isSupportedInstallTarget(target string) bool {
	switch target {
	case "", "claude-code", "opencode", "codex", "both", "all":
		return true
	default:
		return false
	}
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
	default:
		return nil, fmt.Errorf("unsupported host target %q", host)
	}
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

// installConfig writes state-machine.yaml and quality-gates.yaml to <project>/.rotta/
func installConfig(projectPath string) ([]string, error) {
	dir := filepath.Join(projectPath, ".rotta")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("cannot create .rotta dir: %w", err)
	}

	configs := map[string]string{
		"config/state-machine.yaml": filepath.Join(dir, "state-machine.yaml"),
		"config/quality-gates.yaml": filepath.Join(dir, "quality-gates.yaml"),
	}

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
	if opts.Target == "all" {
		return cleanSelectedIntegrationArtifacts(opts, home, projectPath)
	}
	if opts.Target == "opencode" || opts.Target == "both" {
		if err := cleanPreviousOpenCodeInstallation(home); err != nil {
			return err
		}
	}
	if opts.Target == "claude-code" || opts.Target == "both" {
		if err := cleanPreviousClaudeCodeInstallation(home); err != nil {
			return err
		}
	}
	if opts.Target == "codex" {
		if err := cleanPreviousCodexInstallation(home); err != nil {
			return err
		}
	}
	if err := cleanSelectedIntegrationArtifacts(opts, home, projectPath); err != nil {
		return err
	}
	return nil
}

func cleanSelectedIntegrationArtifacts(opts Options, home, projectPath string) error {
	if opts.SetupVela {
		paths := []string{filepath.Join(projectPath, ".vela", "graph.db")}
		if opts.Target == "claude-code" || opts.Target == "both" {
			if err := cleanClaudeCodeVelaFreshnessGuard(home); err != nil {
				return err
			}
			paths = append(paths,
				filepath.Join(home, ".claude", "vela-mcp.json"),
				filepath.Join(home, ".claude", "vela-instructions.md"),
			)
		}
		if opts.Target == "opencode" || opts.Target == "both" {
			if err := cleanOpenCodeVelaFreshnessGuard(home); err != nil {
				return err
			}
			paths = append(paths, filepath.Join(home, ".config", "opencode", "instructions.md"))
		}
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("cannot remove stale integration artifact %s: %w", path, err)
			}
		}
	}

	if opts.SetupAncora && (opts.Target == "claude-code" || opts.Target == "both") {
		path := filepath.Join(home, ".claude", "mcp", "ancora.json")
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove stale integration artifact %s: %w", path, err)
		}
	}

	return nil
}

// copySkillsToDir copies selected SKILL.md files into skillsDir/rotta/<mode>/
func copySkillsToDir(opts Options, skillsDir string) ([]string, error) {
	type modeEntry struct {
		enabled bool
		src     string // path inside assets.FS
		name    string // subdirectory name
	}
	modes := []modeEntry{
		{opts.InstallSpec, "skills/spec-mode", "spec-mode"},
		{opts.InstallImpl, "skills/implementation-mode", "implementation-mode"},
		{opts.InstallReview, "skills/review-mode", "review-mode"},
	}

	var files []string
	for _, m := range modes {
		if !m.enabled {
			continue
		}
		dst := filepath.Join(skillsDir, "rotta", m.name)
		if err := os.MkdirAll(dst, 0o750); err != nil {
			return nil, fmt.Errorf("cannot create dir %s: %w", dst, err)
		}
		err := fs.WalkDir(assets.FS, m.src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			data, err := readRenderedAsset(path, opts)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(m.src, path)
			out := filepath.Join(dst, rel)
			if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
				return err
			}
			return os.WriteFile(out, data, 0o600)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot copy %s: %w", m.src, err)
		}
		files = append(files, filepath.Join(dst, "SKILL.md"))
	}
	return files, nil
}
