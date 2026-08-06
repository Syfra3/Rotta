package installer

import (
	"fmt"
	"os/exec"
)

const (
	velaHomebrewTap = "Syfra3/tap"
	velaFormula     = "vela"
)

// VelaResult describes the bounded OpenCode-only host setup.
type VelaResult struct {
	BinPath                    string
	Installed                  bool
	Files                      []string
	NormalizedMCPEntries       []string
	SkippedAmbiguousMCPEntries []string
	MCPAvailability            map[string]map[string]MCPStatusResult
}

// SetupVela performs only the approved host bootstrap and writes the local
// OpenCode MCP entry. It deliberately has no project-path behaviour.
func SetupVela(opts Options, home, _ string) (*VelaResult, error) {
	if !opts.SetupVela {
		return &VelaResult{}, nil
	}
	if !targetsOpenCode(opts.Target) {
		return skippedVelaResult(opts), nil
	}
	if !opts.ConfirmVela {
		return skippedVelaResult(opts), nil
	}
	if err := bootstrapVela(opts); err != nil {
		return nil, err
	}
	binPath, err := exec.LookPath("vela")
	if err != nil {
		return nil, fmt.Errorf("vela version completed but command resolution failed: %w", err)
	}
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return nil, fmt.Errorf("resolve effective OpenCode config: %w", err)
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return nil, err
	}
	if err := validateOpenCodeConfigurationShape(document.config); err != nil {
		return nil, err
	}
	changed, err := ensureManagedOpenCodeVelaEntry(&document)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := writeResolvedOpenCodeConfig(document); err != nil {
			return nil, err
		}
	}
	result := &VelaResult{BinPath: binPath, Installed: true, Files: []string{resolution.Path}}
	if changed {
		result.NormalizedMCPEntries = []string{resolution.Path}
	}
	return result, nil
}

func skippedVelaResult(opts Options) *VelaResult {
	result := &VelaResult{MCPAvailability: map[string]map[string]MCPStatusResult{}}
	for _, host := range selectedHosts(opts.Target) {
		result.MCPAvailability[host] = map[string]MCPStatusResult{"vela": {
			Status: MCPStatusSkipped, Reason: "Vela setup requires the dedicated OpenCode confirmation.",
			Remediation: "Select Vela and confirm the OpenCode host action in the TUI.", RuntimeFallback: MCPRuntimeFallback{State: MCPRuntimeFallbackNotObserved},
		}}
	}
	return result
}

func bootstrapVela(opts Options) error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("resolve brew for Vela bootstrap: %w", err)
	}
	if err := runCommand(opts, brew, "tap", velaHomebrewTap); err != nil {
		return fmt.Errorf("brew tap %s: %w", velaHomebrewTap, err)
	}
	if err := runCommand(opts, brew, "install", velaFormula); err != nil {
		return fmt.Errorf("brew install %s: %w", velaFormula, err)
	}
	if err := runCommand(opts, "vela", "version"); err != nil {
		return fmt.Errorf("vela version: %w", err)
	}
	return nil
}

func ensureManagedOpenCodeVelaEntry(document *openCodeConfigDocument) (bool, error) {
	mcp, exists := document.config["mcp"]
	if !exists {
		mcp = map[string]interface{}{}
		document.config["mcp"] = mcp
	}
	entries, ok := mcp.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("OpenCode mcp configuration must be an object")
	}
	if current, exists := entries["vela"]; exists {
		entry, ok := current.(map[string]interface{})
		if !ok || !isProvenManagedMCPEntry(entry) {
			return false, fmt.Errorf("refusing to overwrite ambiguous or user-owned OpenCode mcp.vela entry")
		}
		return false, nil
	}
	entries["vela"] = map[string]interface{}{
		"type": "local", "command": []interface{}{"vela", "serve", "--mcp"}, "enabled": true,
	}
	return true, nil
}

func runCommand(opts Options, name string, args ...string) error {
	var command *exec.Cmd
	switch name {
	case "vela":
		command = exec.Command("vela")
	default:
		command = exec.Command("brew")
	}
	command.Args = append(command.Args, args...)
	configureCommandIO(command, opts)
	return command.Run()
}
