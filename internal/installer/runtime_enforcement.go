package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RuntimeEnforcementCompatibility is retained as an explicit host gate. It is
// intentionally stricter than documentation: enforcement needs observed
// delegate/task call identity and session/call lineage, not merely hook types.
type RuntimeEnforcementCompatibility struct {
	OpenCodeVersion string
	Hook            string
	Supported       bool
	Reason          string
}

const runtimeEnforcementUnsupportedReason = "unsupported OpenCode runtime enforcement host: tool.execute.before hook types are documented, but this host has no retained probe proving the delegate/task tool ID and call/session lineage; enforcement remains disabled"

// ProbeOpenCodeRuntimeEnforcement is fail-closed. The installed CLI version is
// captured for audit, but a version string and static docs are not proof that a
// delegate call traverses the hook with the expected route/binding schema.
func ProbeOpenCodeRuntimeEnforcement(ctx context.Context, binary string) RuntimeEnforcementCompatibility {
	if binary == "" {
		path, err := exec.LookPath("opencode")
		if err != nil {
			return RuntimeEnforcementCompatibility{Hook: "tool.execute.before", Reason: "unsupported OpenCode runtime enforcement host: opencode CLI is unavailable; enforcement remains disabled"}
		}
		binary = path
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, binary, "--version").CombinedOutput()
	if err != nil {
		return RuntimeEnforcementCompatibility{Hook: "tool.execute.before", Reason: fmt.Sprintf("unsupported OpenCode runtime enforcement host: cannot inspect installed OpenCode version: %v; enforcement remains disabled", err)}
	}
	return RuntimeEnforcementCompatibility{OpenCodeVersion: strings.TrimSpace(string(out)), Hook: "tool.execute.before", Reason: runtimeEnforcementUnsupportedReason}
}

// InstallOpenCodeRuntimeEnforcement evaluates the compatibility gate during an
// OpenCode installation. An unsupported host is a successful, disabled result:
// it must not prevent the ordinary installation or cause a guessed runtime
// plugin to be added. This preserves every user plugin.
func InstallOpenCodeRuntimeEnforcement(ctx context.Context, opts Options, home string) (RuntimeEnforcementCompatibility, error) {
	if !targetsOpenCode(opts.Target) {
		return RuntimeEnforcementCompatibility{Hook: "tool.execute.before", Reason: "unsupported OpenCode runtime enforcement host: OpenCode was not selected; enforcement remains disabled"}, nil
	}
	if _, err := resolveOpenCodeConfig(opts, home); err != nil {
		return RuntimeEnforcementCompatibility{}, fmt.Errorf("runtime enforcement effective-config resolution: %w", err)
	}
	compatibility := ProbeOpenCodeRuntimeEnforcement(ctx, "")
	if !compatibility.Supported {
		return compatibility, nil
	}
	return compatibility, nil
}
