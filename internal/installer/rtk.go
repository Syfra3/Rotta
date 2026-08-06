package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syfra3/Rotta/internal/rtkexec"
	"github.com/Syfra3/Rotta/internal/workflow"
)

const rtkFormula = "rtk"

type RTKInstallResult = workflow.RTKExecutableRecord

const (
	RTKStatusSuccess = workflow.RTKStatusSuccess
	RTKStatusSkipped = workflow.RTKStatusSkipped
	RTKStatusFailure = workflow.RTKStatusFailure
)

// RTKInstaller is the only host-action seam for RTK. Tests use fakes; the
// production implementation is reached only after both TUI confirmations.
type RTKInstaller interface {
	Install(Options) error
	Resolve() (workflow.RTKExecutable, error)
}

type homebrewRTKInstaller struct{}

func (installer homebrewRTKInstaller) Install(opts Options) error {
	brew, err := installer.trustedBrew()
	if err != nil {
		return fmt.Errorf("resolve brew for RTK installation: %w", err)
	}
	if _, err := brew.Version(); err != nil {
		_ = brew.Close()
		return fmt.Errorf("verify trusted brew version: %w", err)
	}
	if _, err := brew.Fingerprint(); err != nil {
		_ = brew.Close()
		return fmt.Errorf("fingerprint trusted brew: %w", err)
	}
	_, err = brew.Run([]string{"install", rtkFormula}, "", 8*1024)
	_ = brew.Close()
	if err != nil {
		return fmt.Errorf("brew install %s: %w", rtkFormula, err)
	}
	return nil
}

func (installer homebrewRTKInstaller) Resolve() (workflow.RTKExecutable, error) {
	brew, err := installer.trustedBrew()
	if err != nil {
		return nil, fmt.Errorf("resolve trusted brew for RTK: %w", err)
	}
	if _, err := brew.Version(); err != nil {
		_ = brew.Close()
		return nil, fmt.Errorf("verify trusted brew version: %w", err)
	}
	if _, err := brew.Fingerprint(); err != nil {
		_ = brew.Close()
		return nil, fmt.Errorf("fingerprint trusted brew: %w", err)
	}
	output, err := brew.Run([]string{"list", "--versions", rtkFormula}, "", 512)
	_ = brew.Close()
	if err != nil {
		return nil, fmt.Errorf("resolve RTK package prefix: %w", err)
	}
	parts := strings.Fields(output)
	if len(parts) != 2 || parts[0] != rtkFormula || strings.ContainsAny(output, "\r\n") {
		return nil, fmt.Errorf("resolve RTK package version: invalid output")
	}
	executable, err := rtkexec.OpenRTKVersion(parts[1])
	if err != nil {
		return nil, fmt.Errorf("open resolved RTK executable: %w", err)
	}
	return executable, nil
}

func (installer homebrewRTKInstaller) trustedBrew() (*rtkexec.Executable, error) {
	return rtkexec.OpenBrew()
}

func setupRTK(opts Options, installer RTKInstaller) (RTKInstallResult, error) {
	if !opts.SetupRTK {
		return RTKInstallResult{Status: workflow.RTKStatusSkipped, FailureReason: "RTK was not selected"}, nil
	}
	if !opts.ConfirmRTK {
		return RTKInstallResult{Status: workflow.RTKStatusSkipped, FailureReason: "RTK host-level installation action was not confirmed"}, nil
	}
	if installer == nil {
		return RTKInstallResult{}, fmt.Errorf("RTK installation requires an approved installer")
	}
	if err := installer.Install(opts); err != nil {
		return RTKInstallResult{Status: workflow.RTKStatusFailure, FailureReason: err.Error()}, nil
	}
	executable, err := installer.Resolve()
	if err != nil {
		return RTKInstallResult{Status: workflow.RTKStatusFailure, FailureReason: err.Error()}, nil
	}
	defer executable.Close()
	version, err := executable.Version()
	if err != nil {
		return RTKInstallResult{Status: workflow.RTKStatusFailure, FailureReason: err.Error()}, nil
	}
	hash, err := executable.Fingerprint()
	if err != nil {
		return RTKInstallResult{Status: workflow.RTKStatusFailure, FailureReason: err.Error()}, nil
	}
	return RTKInstallResult{Status: workflow.RTKStatusSuccess, ExecutablePath: executable.Path(), Version: version, ExecutableHash: hash}, nil
}

func writeRTKTransactionEvidence(transactionDir string, result RTKInstallResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize RTK transaction evidence: %w", err)
	}
	if err := writePrivateFile(filepath.Join(transactionDir, "rtk.json"), data, 0o600); err != nil {
		return fmt.Errorf("write RTK transaction evidence: %w", err)
	}
	return nil
}

// writeRTKRuntimeState stores the latest selected state in the host state
// directory for optional runtime presentation. Transaction evidence remains
// durable history; this small state file intentionally records success, skip,
// and failure distinctly so no prior conversation is treated as availability.
func writeRTKRuntimeState(home string, result RTKInstallResult) error {
	path := runtimeRTKStatePath(home)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize RTK runtime state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create RTK runtime state directory: %w", err)
	}
	if err := writePrivateFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write RTK runtime state: %w", err)
	}
	return nil
}

func runtimeRTKStatePath(home string) string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "rotta", "rtk.json")
}
