# Hard Spec: Host-Agnostic Rotta Compatibility for Claude Code, OpenCode, and Codex

## Adversarial Pre-Mortem
- Failure mode 1: Rotta claims host parity but writes OpenCode-shaped files or MCP settings into Claude Code or Codex locations, leaving users with generated artifacts that are syntactically valid files but ignored by the target host.
- Failure mode 2: Compatibility work fragments Rotta's workflow into host-specific behavior, causing commands, lifecycle artifacts, MCP availability, or phase gates to diverge silently between Claude Code, OpenCode, and Codex.
- Failure mode 3: Installers mutate user-level host configuration without backups, ownership markers, or idempotency, producing duplicate MCP entries, broken personal settings, dirty project worktrees, or unrecoverable partial installs.

## Hidden Assumptions
- Claude Code, OpenCode, and Codex each provide a supported way to consume generated instructions and at least some combination of agent, skill, command, or MCP configuration files; where a host lacks an exact primitive, Rotta can generate the closest supported equivalent and disclose the limitation.
- Rotta's canonical behavior is host-independent: phase order, command names, approval gates, lifecycle artifact semantics, memory policy, MCP semantics, and review/TDD expectations are defined once and adapted to host surfaces.
- Workspace files remain the source of truth for hard specs, features, reports, and lifecycle artifacts; Ancora or any other memory MCP stores compact pointers/status only.
- Generated Rotta lifecycle artifacts such as `.rotta/`, `features/`, `reports/`, and `specs/` are not committed by default unless the user explicitly chooses to do so.
- Existing OpenCode and Claude Code support from recent Context7 installer work is expected behavior and must not regress while adding Codex and formal host abstraction.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Keep OpenCode as the only first-class host and document manual setup for Claude Code and Codex | Violates the requirement that Rotta be agnostic to the agentic coding agent and work across all three supported hosts. |
| Implement three independent workflows, one per host | Maximizes short-term host fit but creates divergent commands, MCP behavior, approvals, and lifecycle semantics that users cannot rely on. |
| Generate only generic markdown instructions and require users to configure agents/MCPs manually | Avoids host mutation risk but fails the explicit scope covering installation, generated host files, MCP configuration, and command/workflow preservation. |
| Normalize all hosts to OpenCode's file layout | Simple internally, but unsafe because Claude Code and Codex may ignore or misinterpret OpenCode-specific locations and schema. |
| Add Claude Code and Codex but postpone host-specific limitations | Hides real parity gaps and prevents users from knowing which Rotta capabilities are exact, adapted, degraded, or unsupported per host. |

## Summary
Add a host-agnostic compatibility layer so Rotta installs into exactly Claude Code, OpenCode, and Codex while preserving the same Rotta workflow, commands, MCPs, generated instructions, approval gates, lifecycle artifacts, and user-facing behavior as much as each host permits. Rotta must generate host-appropriate agent, skill, instruction, command, and MCP configuration artifacts from one canonical Rotta contract, report exact/adapted/unsupported capabilities per host, remain idempotent and recovery-safe, preserve clean worktree expectations for generated lifecycle artifacts, and fail clearly without silently degrading workflow guarantees.

## Requirements

### REQ-001: Support Exactly Three Compatibility Hosts
**Description:** Rotta must treat Claude Code, OpenCode, and Codex as the complete supported compatibility target set for this feature, with a canonical host abstraction that avoids hard-coding OpenCode behavior as the implicit default for all hosts.
**Acceptance Criteria:**
- The installer exposes Claude Code, OpenCode, and Codex as selectable Rotta host targets.
- The supported host set for this feature is exactly Claude Code, OpenCode, and Codex.
- Selecting any supported host routes installation, generated files, MCP configuration, and command surfaces through that host's adapter or equivalent compatibility contract.
- Unsupported hosts are not presented as supported and are rejected with a clear message if requested by config or CLI input.
- Existing OpenCode behavior remains supported after adding Claude Code and Codex.
- Existing Claude Code behavior from recent MCP installer work, including Context7 support, remains supported after adding Codex and host abstraction.
**Edge Cases:**
- User selects multiple supported hosts in one run.
- User requests an unsupported host by typo, stale config, or manual invocation.
- User reruns installation after the default host detection changes.
**Out of Scope:**
- Supporting Cursor, Windsurf, Zed, VS Code extensions, Gemini CLI, custom in-house agents, or other hosts.

### REQ-002: Install Rotta into Each Selected Host Using Host-Appropriate Locations
**Description:** Rotta must install its host integration artifacts into the correct user-level or workspace-level locations for each selected host without writing files where that host cannot consume them.
**Acceptance Criteria:**
- For each selected host, Rotta writes only to locations documented or configured for that host unless the user explicitly overrides the target path.
- Installation creates missing Rotta-managed host directories when safe and reports permission/path failures before claiming success.
- Installation preserves unrelated user files and settings in host configuration directories.
- Installation records enough per-host result detail for the user to distinguish installed, skipped, failed, partially installed, and unsupported capabilities.
- Installation can target one host without mutating the other supported hosts.
- Installing to multiple hosts in one run reports each host independently; success for one host must not hide failure for another.
**Edge Cases:**
- A host is installed but not currently available on PATH.
- A host config directory exists but is not writable.
- A host has no existing config and requires first-time Rotta-managed setup.
- User supplies a custom host config path.
**Out of Scope:**
- Installing the host applications themselves.

### REQ-003: Generate Canonical Rotta Instructions as Host-Specific Agent, Skill, and Instruction Artifacts
**Description:** Rotta must generate equivalent workflow instructions for Claude Code, OpenCode, and Codex from one canonical Rotta instruction contract, adapting output shape to each host's supported agent, skill, command, rule, or instruction mechanism.
**Acceptance Criteria:**
- Generated host artifacts preserve Rotta's phase model, delegation expectations, strict TDD/review expectations, no-AI-attribution rule, memory policy, Vela advisory policy, lifecycle artifact policy, and command semantics.
- OpenCode receives artifacts in OpenCode-consumable forms, including agents/skills/instructions where supported by Rotta's current OpenCode integration.
- Claude Code receives artifacts in Claude Code-consumable forms, using the closest supported equivalent when Claude Code does not share OpenCode's exact agent/skill model.
- Codex receives artifacts in Codex-consumable forms, using the closest supported equivalent when Codex does not share OpenCode's exact agent/skill model.
- Generated files include host metadata or deterministic Rotta ownership markers sufficient for safe updates without duplicating stale versions.
- If a host cannot represent a Rotta concept exactly, the generated artifact must state the limitation and the installer must include it in the capability summary.
**Edge Cases:**
- Host supports global instructions but not named sub-agents.
- Host supports MCP but not custom slash commands.
- Host supports one instruction file and requires all Rotta roles to be composed into that file.
- A previous Rotta-generated artifact exists from an older template version.
**Out of Scope:**
- Changing Rotta's canonical workflow to match one host's limitations.

### REQ-004: Configure Ancora, Vela, Context7, and Future Rotta MCP Servers Per Host
**Description:** Rotta must configure MCP servers such as Ancora, Vela, and Context7 for each selected host using that host's supported MCP configuration shape, while preserving current OpenCode and Claude Code expectations and adding Codex where supported.
**Acceptance Criteria:**
- Ancora MCP configuration is generated or updated for each selected host when Ancora is selected.
- Vela MCP configuration is generated or updated for each selected host when Vela is selected.
- Context7 MCP configuration is generated or updated for each selected host when Context7 is selected.
- OpenCode and Claude Code Context7 behavior remains compatible with the recent installer contract and continues to configure Context7 for both hosts when selected.
- MCP entries use stable server names, deterministic command/args/env fields, and host-correct transport/config schema.
- Existing unrelated MCP servers and user settings are preserved.
- If a host lacks supported MCP configuration for a selected MCP server, Rotta reports the capability as unsupported or degraded for that host instead of pretending parity.
- MCP health checks, when available, verify observable MCP initialization/tool discovery rather than config-file presence alone.
**Edge Cases:**
- One host supports stdio MCP while another requires a different config shape.
- One selected MCP succeeds on OpenCode and fails on Codex.
- Existing manual MCP entries conflict with Rotta-managed server names.
- Required command/runtime for an MCP server is unavailable.
**Out of Scope:**
- Implementing new MCP server functionality inside Ancora, Vela, or Context7.

### REQ-005: Preserve Rotta Commands and Workflow Parity Across Hosts
**Description:** Users must be able to run the same Rotta workflow and command set across Claude Code, OpenCode, and Codex, with host-specific command exposure adapted only where the host lacks an exact command primitive.
**Acceptance Criteria:**
- The supported Rotta command set remains consistent across hosts, including init/new/continue/status/skip/back and the full spec → Gherkin → TDD → review lifecycle where currently supported by Rotta.
- Command names, phase order, approval gates, and required human approval points are preserved across hosts unless a host limitation is explicitly disclosed.
- Host-specific wrappers or aliases map back to the same canonical Rotta behavior and state transitions.
- A workflow started in one supported host can be continued in another supported host through the shared workspace state and source-of-truth artifacts.
- Host adapters must not bypass spec, Gherkin, TDD, review, quality gate, memory pointer, or clean-worktree rules.
**Edge Cases:**
- User starts a workflow in OpenCode and continues in Claude Code.
- User invokes a command alias that exists in one host but not another.
- Host session lacks a previously generated command surface but workspace state exists.
**Out of Scope:**
- Guaranteeing identical keyboard shortcuts, UI rendering, or autocomplete behavior across hosts.

### REQ-006: Preserve Workspace Source-of-Truth and Clean Worktree Expectations
**Description:** Host compatibility must preserve Rotta's lifecycle artifact model: workspace files are source of truth, memory stores compact pointers/status only, and generated lifecycle artifacts are not committed by default.
**Acceptance Criteria:**
- Specs, Gherkin features, TDD logs, reports, and `.rotta/` lifecycle state remain workspace artifacts and are not replaced by host-local config as the source of truth.
- Ancora or other memory-backed integrations store compact pointers/status, not full hard specs, feature files, TDD logs, or review reports.
- Installation does not require committing generated lifecycle artifacts such as `.rotta/`, `features/`, `reports/`, or `specs/` by default.
- Rotta preserves clean worktree expectations by distinguishing user-requested source changes from generated lifecycle/config artifacts.
- Host installation reports which files it changed and whether those files are user-level host config, workspace host config, or Rotta lifecycle artifacts.
**Edge Cases:**
- User runs install from a dirty worktree.
- User asks to make generated specs/features committable for team sharing.
- Host config lives inside the workspace instead of the user's home directory.
**Out of Scope:**
- Forcing a universal `.gitignore` policy across projects without user approval.

### REQ-007: Provide Idempotent, Versioned, and Recoverable Host Configuration Updates
**Description:** Re-running Rotta installation or generation must update Rotta-managed host artifacts deterministically without duplicating entries, corrupting user config, or losing the ability to recover from partial failures.
**Acceptance Criteria:**
- Rotta-managed generated files and config blocks include deterministic ownership markers or metadata.
- Re-running install with the same selections produces no duplicate agents, skills, instructions, commands, or MCP entries.
- Re-running install after template changes updates Rotta-managed content to the current template version while preserving unrelated user content.
- Before mutating existing host config files, Rotta creates backups or uses an equivalent safe write strategy consistent with existing installer recovery behavior.
- Partial failures report which host, artifact type, and MCP/server failed and leave enough state for retry or manual recovery.
- A failed update must not leave a host config syntactically invalid if Rotta can detect the write or parse failure.
**Edge Cases:**
- Existing Rotta-managed artifacts were manually edited.
- Existing host config is malformed before Rotta starts.
- Install is interrupted after one host succeeds and before another host starts.
- Filesystem write succeeds but validation fails afterward.
**Out of Scope:**
- Merging arbitrary user edits inside Rotta-owned generated blocks beyond preserving or backing up the original file.

### REQ-008: Surface Host-Specific Limitations Explicitly
**Description:** Rotta must treat host gaps as first-class compatibility data, not hidden behavior, so users know whether a capability is exact, adapted, degraded, unsupported, or failed for each selected host.
**Acceptance Criteria:**
- The installer or generation summary includes a capability matrix for selected hosts covering installation, instructions/agents/skills, commands/workflow, MCP configuration, health checks, and lifecycle behavior.
- Each capability is classified as exact, adapted, degraded, unsupported, skipped, failed, or not applicable.
- Adapted/degraded/unsupported capabilities include a concise reason and user-facing remediation where available.
- Unsupported host capabilities do not block unrelated supported capabilities unless they are required for a selected workflow guarantee.
- The generated instructions for a host include only claims that are true for that host.
**Edge Cases:**
- Codex supports instructions but not an MCP server shape required by one selected MCP.
- Claude Code supports MCP and instructions but not OpenCode-style sub-agent files.
- Host documentation changes after Rotta templates were written.
**Out of Scope:**
- Promising perfect feature parity where the host lacks a corresponding primitive.

### REQ-009: Fail Fast and Clearly on Unsafe or Invalid Host Operations
**Description:** Rotta must detect unsupported hosts, invalid config, permission issues, schema mismatches, runtime/MCP failures, and unsafe writes early enough to avoid false success and guide recovery.
**Acceptance Criteria:**
- Unsupported host selection fails before file mutation.
- Invalid or malformed existing host config is reported with the host name and file path before Rotta overwrites it.
- Permission failures identify the host, artifact type, path, and operation attempted.
- MCP health-check failures identify whether the failure came from command availability, startup, initialization, tool discovery, timeout, or unsupported host capability.
- Installer summaries never report full success when any selected host or required selected capability failed.
- Retry guidance distinguishes safe rerun, manual config repair, missing dependency installation, and unsupported host capability.
**Edge Cases:**
- Multiple hosts fail for different reasons in one run.
- The host config file changes concurrently during install.
- Health checks are unavailable in a non-interactive or sandboxed environment.
**Out of Scope:**
- Automatically repairing arbitrary corrupted third-party host configuration files.

### REQ-010: Maintain Backward Compatibility for Existing Rotta Installations
**Description:** Adding host-agnostic compatibility must not break existing OpenCode users, existing Claude Code MCP setup, existing Context7 behavior, or existing Rotta workflow state.
**Acceptance Criteria:**
- Existing OpenCode Rotta installations continue to load generated instructions, commands, agents/skills, MCP servers, and workflow state after upgrade.
- Existing Claude Code MCP entries produced by recent Rotta installer work continue to be recognized and updated safely.
- Existing Context7 configuration for OpenCode and Claude Code is not removed, renamed, duplicated, or silently degraded by adding Codex support.
- Existing `.rotta/` workflow state and workspace source-of-truth artifacts remain readable by all supported hosts after upgrade.
- Migration or regeneration steps are explicit, idempotent, and reversible through backups where host config is mutated.
**Edge Cases:**
- User installed Rotta before host metadata/version markers existed.
- User has only OpenCode configured and later adds Codex.
- User has manually edited generated OpenCode instructions.
**Out of Scope:**
- Supporting pre-Rotta or manually invented configuration formats that Rotta never generated and cannot detect safely.

## Open Questions
- None.

## Trade-offs
- A canonical Rotta contract plus host adapters reduces behavioral drift but requires careful capability mapping and explicit limitation reporting for hosts that lack OpenCode-equivalent primitives.
- Idempotent safe writes, backups, and health checks increase installer complexity and runtime, but prevent false success and protect user-level AI host configuration.
- Preserving exact workflow semantics across hosts may require adapted command surfaces or composed instruction files where a host does not support named agents, skills, or slash commands.
- Keeping lifecycle artifacts out of commits by default protects clean worktree expectations, but teams that want committable specs/features will need an explicit opt-in path.

## Risk Level
high — Justification: This feature mutates user-level configuration for three AI coding hosts, generates behavior-shaping agent/instruction artifacts, configures multiple MCP servers, and must preserve workflow parity across hosts with different capabilities while maintaining idempotency, recoverability, and clean worktree expectations.
