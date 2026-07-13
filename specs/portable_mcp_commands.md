# Hard Spec: Portable Serialized MCP Executable Commands

## Adversarial Pre-Mortem
- Failure mode 1: an installer resolves an executable to a Homebrew Cellar version path and writes that path into an MCP configuration; the next Homebrew upgrade removes it and the host cannot start the server.
- Failure mode 2: broad path rewriting mistakes a generated hook-script reference or a user-owned MCP entry for an executable command, corrupting a valid host integration during reinstall.
- Failure mode 3: Rotta validates its own inherited `PATH`, reports success, but OpenCode is launched from a GUI, service, or shell with a different `PATH`; its MCP server then fails without actionable diagnosis.

## Hidden Assumptions
- Rotta-managed MCP server entries can be identified by their known Rotta server identity and expected managed configuration shape; entries whose ownership cannot be established must not be rewritten.
- `ancora`, `vela`, and `npx` are intended to be invoked by name through the host process environment, rather than through a pinned executable location.
- A host can resolve a bare executable only if its own launch environment exposes that executable on `PATH`; the installer cannot prove this solely from its own process.
- An absolute path can remain valid when it is a non-executable resource reference, including a Rotta-generated hook script, and is not subject to executable-command normalization.
- Existing installers and delegated setup tools may have emitted stale absolute command paths, so migration must inspect the serialized configuration rather than rely on the current installation method.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Serialize the result of `exec.LookPath` | It can be a versioned Homebrew Cellar path or another host-specific absolute location that becomes stale after upgrades. |
| Write a platform-specific absolute fallback alongside the bare command | It preserves the same staleness problem and creates different serialized contracts by platform. |
| Rewrite every absolute string in managed host configuration | It can corrupt generated hook paths and unrelated user settings that are not executable MCP commands. |
| Skip migration of existing entries | Existing installations retain upgrade-sensitive configuration and violate the portability requirement until manually repaired. |

## Summary
Rotta must serialize bare executable names, never absolute executable paths, in every Rotta-managed MCP server command field across supported host configuration formats. Installation and reinstallation must recognize and normalize stale Rotta-managed MCP command entries for Ancora, Vela, Context7, and future managed servers, while preserving absolute paths that are not MCP executable commands, such as generated hook scripts. Rotta may use its current environment to validate that a command is presently available, but validation results must not be serialized. Because a host such as OpenCode can run with a different `PATH`, the installer must distinguish installer-side availability from host-side resolution, avoid non-portable path fallback, and report explicit degraded or failed states with remediation.

## Requirements

### REQ-015: Serialize Canonical Bare MCP Executable Names
**Description:** Every Rotta-managed serialized MCP server executable command must be the canonical bare executable name (for example `ancora`, `vela`, or `npx`), not an absolute path, Homebrew Cellar path, symlink target, or installation-specific location.
**Acceptance Criteria:**
- Rotta writes `ancora`, `vela`, and `npx` as bare command values in all Rotta-managed MCP configuration formats it owns.
- The requirement applies to direct Rotta writers and to configuration produced or subsequently retained by delegated Rotta setup flows.
- Rotta never serializes the result of `exec.LookPath`, a Homebrew prefix, Cellar path, manual-install path, or another absolute executable path into a Rotta-managed MCP command field.
- `exec.LookPath` or equivalent current-environment discovery may be used only for validation or immediate process invocation.
- Future Rotta-managed MCP servers use the same canonical-bare-command policy.
**Edge Cases:**
- `exec.LookPath` resolves through a Homebrew symlink whose target is inside a versioned Cellar directory.
- A manually installed command resolves from `~/.local/bin` or another user-specific directory.
- The command is a relative path containing a slash rather than a bare executable name.
**Out of Scope:**
- Making third-party, user-owned MCP server entries portable.
- Rewriting command arguments unless they are themselves a Rotta-defined executable-command field.

### REQ-016: Normalize Only Proven Rotta-Managed Stale MCP Commands
**Description:** During install and reinstall, Rotta must inspect supported host configurations and replace a stale absolute or slash-containing executable command only when the MCP entry is proven Rotta-managed; it must preserve non-command absolute references and entries of unknown ownership.
**Acceptance Criteria:**
- Reinstall normalizes recognized Rotta-managed Ancora, Vela, Context7, legacy Rotta Context7, and future explicitly registered managed MCP entries to their canonical bare command names.
- Recognition uses server identity plus the expected Rotta-managed configuration shape or explicit ownership metadata; an ambiguous entry is preserved and reported rather than rewritten.
- Normalization occurs before install success is reported and is idempotent: a subsequent reinstall produces no further command-field change.
- Rotta preserves generated hook-script paths and other absolute values that are not the executable command field of a proven managed MCP entry.
- Rotta preserves unrelated user MCP servers and unrelated user configuration.
**Edge Cases:**
- A legacy managed entry contains `/opt/homebrew/Cellar/<tool>/<version>/bin/<tool>`.
- A known server name has been manually repurposed with an incompatible command or arguments.
- One configuration includes both a legacy managed Context7 entry and the current managed entry.
- A JSON, JSONC, or TOML host configuration is malformed before normalization.
**Out of Scope:**
- Repairing arbitrary malformed third-party host configuration.
- Inferring ownership from an absolute path alone.

### REQ-017: Validate Availability Without Persisting Validation Locations
**Description:** Rotta must validate each selected managed executable in the installer process environment without allowing the discovered location to influence serialized MCP configuration.
**Acceptance Criteria:**
- Rotta reports command availability separately from configuration normalization and serialization.
- When the installer can resolve the command, it may run the command for installation or health validation using the current environment, while the serialized MCP command remains bare.
- When the installer cannot resolve a required selected command, Rotta does not substitute an absolute fallback path into configuration.
- An unavailable command produces an explicit per-host/per-server degraded or failed result that identifies command availability and supplies installation or `PATH` remediation.
- Existing stale managed entries are still normalized to their canonical bare command during reinstall even when the installer cannot currently resolve that command, and the result is reported as degraded or failed rather than healthy.
**Edge Cases:**
- A command becomes unavailable between preflight validation and health checking.
- `brew`, `curl`, or a manual installer is available but the installed command is not visible to the current process afterward.
- Only one of several selected MCP commands is unavailable.
**Out of Scope:**
- Persisting a discovered executable directory into a host configuration environment variable.
- Guaranteeing a command installation method succeeds on every operating system.

### REQ-018: Handle Host PATH Discrepancies Explicitly
**Description:** Rotta must not treat successful installer-process lookup as proof that a host process, including OpenCode, resolves the same bare command. It must provide deterministic status and remediation when host-side resolution cannot be verified or fails.
**Acceptance Criteria:**
- Installer output distinguishes `available to Rotta` from `verified by host` where host-side verification is supported.
- A host-side MCP startup or health failure caused by command lookup is reported as host command availability, not as a reason to serialize an absolute command path.
- When host-side verification is unavailable, Rotta reports the command as portable-but-host-resolution-unverified and directs the user to launch the host with a `PATH` containing the bare command.
- For OpenCode and other hosts with a different process environment, Rotta does not inject a Homebrew, Cellar, or other machine-specific path as an automatic fallback.
- A failed or unverified host resolution must not be presented as a healthy MCP configuration.
**Edge Cases:**
- OpenCode is launched from a desktop environment whose `PATH` omits the user's shell initialization.
- Rotta is run from an interactive shell with a richer `PATH` than the host process.
- A host is restarted after a package-manager upgrade or after the user repairs its `PATH`.
**Out of Scope:**
- Modifying a user's shell profile, desktop-session environment, service manager environment, or host launch mechanism automatically.
- Guaranteeing host-specific health verification where the host exposes no observable startup result.

### REQ-019: Preserve Portable Command Policy Through Upgrades and Reporting
**Description:** Rotta must expose migration and runtime outcomes clearly enough that an upgrade does not silently leave a non-portable managed command or falsely claim health.
**Acceptance Criteria:**
- Installation results identify each normalized managed MCP entry and any entry skipped because ownership is ambiguous.
- The capability/status summary identifies configuration normalization, installer command availability, and host resolution/health as separate observable states.
- A detected command availability or host-resolution problem includes a safe remediation and does not block unrelated selected MCP configuration where the host configuration remains valid.
- Reinstall after a Homebrew upgrade, curl/manual reinstall, or OS change retains canonical bare commands and does not reintroduce versioned or absolute executable locations.
**Edge Cases:**
- Multiple hosts disagree about the availability of the same command.
- A user upgrades a package after Rotta install but before the next host launch.
- A normalization succeeds while a later health check fails for an unrelated startup reason.
**Out of Scope:**
- Automatic package upgrades or automatic repair of a host process environment.

## Open Questions
- None. The contract intentionally treats host-side command resolution as unverified unless Rotta has observable host startup or health evidence; it must not infer that result from the installer process `PATH`.

## Trade-offs
- Bare commands eliminate upgrade-stale serialized paths, but users must ensure the host process inherits a suitable `PATH`.
- Conservative ownership recognition may leave ambiguous legacy entries untouched, but prevents Rotta from rewriting user-owned configuration.
- Separating normalization from availability can produce a valid portable configuration that is currently degraded; this is more accurate than retaining a temporarily working absolute path.

## Risk Level
high — Justification: this migration changes user-level MCP configuration across hosts and installation methods. Incorrect ownership detection can damage configuration, while false health claims leave users with host-startup failures that may only appear after upgrades or when a GUI host has a different environment.
