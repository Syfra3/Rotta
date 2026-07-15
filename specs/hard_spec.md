# Hard Spec: Host-Agnostic Rotta Compatibility for Claude Code, OpenCode, and Codex

## Adversarial Pre-Mortem
- Failure mode 1: Rotta claims host parity but writes OpenCode-shaped files or MCP settings into Claude Code or Codex locations, leaving users with generated artifacts that are syntactically valid files but ignored by the target host.
- Failure mode 2: Compatibility work fragments Rotta's workflow into host-specific behavior, causing commands, lifecycle artifacts, MCP availability, or phase gates to diverge silently between Claude Code, OpenCode, and Codex.
- Failure mode 3: Installers mutate user-level host configuration without backups, ownership markers, or idempotency, producing duplicate MCP entries, broken personal settings, dirty project worktrees, or unrecoverable partial installs.
- Failure mode 4: An optional Ancora outage is treated as a workflow outage, so an agent cannot recover state, save a pointer, or continue a human-approved phase even though the reviewed workspace artifacts remain available.
- Failure mode 5: Vela or Context7 failure is hidden, causing unbounded source exploration, invented library/API details, or a false claim that documentation or graph evidence informed a decision.
- Failure mode 6: Phase 2 writes a contract in the initiating checkout and Phase 3 later creates a different worktree from an unrelated HEAD, so the approved baseline, approval evidence, and implementation history diverge.
- Failure mode 7: a shared ignored approval marker is stale, missing, or names scenarios from another feature, allowing unapproved work to run or blocking an approved submission.
- Failure mode 8: host orchestration treats a successful single scenario as an invitation to ask for a commit or continuation, or archives only local state and leaves the terminal feature lifecycle ambiguous.

## Hidden Assumptions
- Claude Code, OpenCode, and Codex each provide a supported way to consume generated instructions and at least some combination of agent, skill, command, or MCP configuration files; where a host lacks an exact primitive, Rotta can generate the closest supported equivalent and disclose the limitation.
- Rotta's canonical behavior is host-independent: phase order, command names, approval gates, lifecycle artifact semantics, memory policy, MCP semantics, and review/TDD expectations are defined once and adapted to host surfaces.
- Workspace files remain the source of truth for hard specs, features, reports, and lifecycle artifacts; Ancora or any other memory MCP stores compact pointers/status only.
- Generated Rotta lifecycle artifacts such as `.rotta/`, `features/`, `reports/`, and `specs/` are not committed by default unless the user explicitly chooses to do so.
- Existing OpenCode and Claude Code support from recent Context7 installer work is expected behavior and must not regress while adding Codex and formal host abstraction.
- The installed OpenSpec workflow artifacts can preserve Rotta's phase, approval, gate, and source-of-truth rules when an optional MCP is unavailable; no MCP owns authoritative contract content.
- A detected runtime MCP failure can be surfaced by generated host rules and workflow status even when a prior installer run reported a healthy configuration.
- A durable approval record can be a tracked, feature-scoped artifact committed with its exact hard-spec and Gherkin baseline; it is distinct from ignored `.rotta/current/` execution state and from the legacy shared `specs/.approved` marker.
- The recorded feature worktree, branch, contract paths, approval-record identity, and contract fingerprints are sufficient to prove that autonomous Phase 3 is executing the approved baseline rather than a later edit.
- A human remains responsible for publication confirmation and explicitly requested cleanup; review completion alone must not remove the worktree or branch needed for inspection and manual PR handoff.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Keep OpenCode as the only first-class host and document manual setup for Claude Code and Codex | Violates the requirement that Rotta be agnostic to the agentic coding agent and work across all three supported hosts. |
| Implement three independent workflows, one per host | Maximizes short-term host fit but creates divergent commands, MCP behavior, approvals, and lifecycle semantics that users cannot rely on. |
| Generate only generic markdown instructions and require users to configure agents/MCPs manually | Avoids host mutation risk but fails the explicit scope covering installation, generated host files, MCP configuration, and command/workflow preservation. |
| Normalize all hosts to OpenCode's file layout | Simple internally, but unsafe because Claude Code and Codex may ignore or misinterpret OpenCode-specific locations and schema. |
| Add Claude Code and Codex but postpone host-specific limitations | Hides real parity gaps and prevents users from knowing which Rotta capabilities are exact, adapted, degraded, or unsupported per host. |
| Stop the workflow whenever any optional MCP is unavailable | Makes an advisory or pointer-only integration a single point of failure and contradicts the workspace-source-of-truth model. |
| Replace a failed MCP with another remote MCP | Introduces a second availability and trust dependency instead of using the durable workflow artifacts and bounded local exploration already available. |
| Continue silently after an MCP failure | Hides reduced evidence or persistence capability and prevents users from verifying assumptions, remediation, and the active fallback state. |
| Continue using ignored shared `specs/.approved` as the approval authority | It cannot provide feature-scoped, committed, tamper-evident approval evidence and can cross-contaminate concurrent workflows. |
| Create the contract checkpoint only when Phase 3 starts | The contract can be changed or lost between approval and implementation, and a later worktree can be created from the wrong baseline. |
| Stop after each scenario to ask whether to commit or continue | Reintroduces a mechanical gate after explicit contract approval and successful required validation, while leaving scenario boundaries less reliable. |
| Delete the worktree at review completion | Removes the user's manual inspection, push, and PR handoff context before publication or deliberate abandonment. |

## Summary
Add a host-agnostic compatibility layer so Rotta installs into exactly Claude Code, OpenCode, and Codex while preserving the same Rotta workflow, commands, MCPs, generated instructions, approval gates, lifecycle artifacts, and user-facing behavior as much as each host permits. Rotta must generate host-appropriate agent, skill, instruction, command, and MCP configuration artifacts from one canonical Rotta contract, report exact/adapted/unsupported capabilities per host, remain idempotent and recovery-safe, preserve clean worktree expectations for generated lifecycle artifacts, and fail clearly without silently degrading workflow guarantees. Optional MCP runtime failures must be visible and non-blocking: Ancora falls back to OpenSpec workflow artifacts as durable state, Vela falls back to bounded simple source exploration, and Context7 continues without documentation lookup while clearly identifying assumptions and verification needs.

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

### REQ-011: Continue in an OpenSpec Artifact Fallback When Ancora Fails
**Description:** For Claude Code, OpenCode, and Codex, Rotta must treat Ancora as optional pointer/state assistance and continue in an explicitly reported fallback state when Ancora is missing, unavailable, times out, is denied permission, cannot recover state, cannot save state, or otherwise cannot be used.
**Acceptance Criteria:**
- Generated host rules classify each listed condition as an Ancora degradation rather than a workflow failure.
- On an Ancora degradation, Rotta reads and updates the available workspace and installed-system OpenSpec workflow artifacts as the durable source of truth and state, including applicable `specs/`, `features/`, `.rotta/`, reports, approval markers, and workflow configuration.
- Rotta does not reconstruct authoritative contract content from Ancora, overwrite reviewed workspace artifacts from memory, fabricate recovered state, or require a successful Ancora call before continuing.
- The fallback preserves the canonical phase order, explicit human approval gate, TDD preconditions, quality gates, source-of-truth precedence, and no-attribution rule.
- Rotta records and surfaces that the active workflow is in Ancora fallback, identifies the failure category, and provides a safe retry or recovery action without blocking unrelated workflow progress.
- Restoring Ancora availability permits future pointer/state operations but does not replace the workspace/OpenSpec artifacts as the source of truth.
**Edge Cases:**
- Ancora fails before any state recovery, after workspace state has been read, or while saving a phase transition.
- A timeout, permission denial, and missing tool occur in different sessions or on different supported hosts.
- Ancora returns stale pointers while workspace artifacts are available.
- A workflow begins on one host in fallback and continues from another supported host.
**Out of Scope:**
- Repairing Ancora infrastructure, credentials, permissions, or server-side data.
- Storing full hard-spec, feature, TDD-log, or review-report content in an alternate memory service.

### REQ-012: Use Bounded Source Exploration When Vela Fails
**Description:** For Claude Code, OpenCode, and Codex, a Vela failure must not block Rotta or invoke a replacement graph MCP; Rotta must continue with bounded, focused source/code exploration and disclose the resulting evidence limits.
**Acceptance Criteria:**
- Vela unavailability, timeout, permission failure, stale/unusable graph, missing graph tools, or graph query failure enters a visible Vela-degraded state.
- Rotta uses no replacement graph MCP and performs at most five focused source/code exploration actions for the affected structural question before reporting the available evidence and remaining gap.
- The fallback explores concrete files, symbols, callers, or configuration relevant to the question rather than expanding into broad or unbounded repository searching.
- Rotta labels the conclusion as source-derived, reports that Vela graph proof was unavailable, and does not allow the missing graph evidence to alter phase order, approval gates, or quality-gate requirements.
**Edge Cases:**
- Vela fails after returning partial or stale graph data.
- The source exploration budget is exhausted without resolving an architectural question.
- A user requests graph proof while Vela remains unavailable.
**Out of Scope:**
- Rebuilding, repairing, or substituting the Vela graph service automatically.
- Claiming dependency, impact, path, ownership, or ranking proof that the bounded source exploration cannot establish.

### REQ-013: Continue Safely Without Context7 Documentation
**Description:** For Claude Code, OpenCode, and Codex, Context7 failure is non-blocking: Rotta must continue without a documentation lookup, must not invent library/API details, and must surface assumptions and verification work needed to proceed safely.
**Acceptance Criteria:**
- Missing/unavailable Context7 tools, timeout, permission failure, command/initialization failure, or documentation-query failure enters a visible Context7-degraded state.
- Rotta continues the applicable workflow without substituting undocumented library/API claims for the failed lookup.
- Any library-specific behavior that cannot be verified from available project evidence is labeled as an assumption or verification need in the response or workflow status.
- Rotta uses only available project evidence and user-provided information for the affected decision; it asks for or defers verification when the unavailable documentation is required to make a safe claim.
- Context7 degradation does not bypass or weaken phase order, approval, TDD, review, quality-gate, or source-of-truth requirements.
**Edge Cases:**
- Context7 fails after a library ID resolves but before documentation is returned.
- The project contains a pinned dependency version but no local usage evidence.
- The requested library behavior is security-sensitive or required to satisfy an approved scenario.
**Out of Scope:**
- Guessing library/API syntax, version behavior, configuration, or migration details.
- Installing or replacing Context7 during a workflow session.

### REQ-014: Expose MCP Degradation and Fallback State in Generated Rules and Installer Status
**Description:** Rotta must make optional MCP degradation observable at both installation and workflow use, so users of Claude Code, OpenCode, and Codex can distinguish configured capability from the active runtime fallback and understand its effect.
**Acceptance Criteria:**
- Generated host rules for all three supported hosts state the fallback behavior, durable source of truth, and reporting obligation for Ancora, Vela, and Context7 degradation.
- Generated workflow status or response output identifies the affected MCP, failure category, active fallback mode, evidence or persistence limitation, and safe next action.
- Installer/TUI status summarizes each selected MCP as configured, skipped, degraded, or failed with a concise reason and remediation; when installation detects a failure, it must not present the MCP as fully healthy.
- Installer/TUI status and generated rules distinguish configuration/health status from a later runtime degradation and do not claim that a successful installation prevents fallback use.
- Status reporting is per selected host where host-specific configuration or health differs, while fallback semantics remain the same across Claude Code, OpenCode, and Codex.
**Edge Cases:**
- One host has a healthy MCP configuration while another host is degraded.
- An MCP was intentionally skipped and later is unavailable at runtime.
- More than one optional MCP is degraded during the same workflow step.
**Out of Scope:**
- Continuous background telemetry or a guarantee that a closed installer UI can observe future runtime failures.
- Concealing a degradation merely because another host or MCP remains healthy.

### REQ-015: Provide Native Claude Code Orchestration and Delegated Phase Agents
**Description:** Extend the global Claude Code integration so normal user requests enter a Rotta orchestration surface that selects the proportional workflow, guides the user through its phases, and delegates phase work to native hidden Claude Code agents named `rotta-spec`, `rotta-impl`, and `rotta-review`. This is an extension to the approved host-compatibility contract; it must preserve all existing approved host behavior and must not silently replace it with a manual, user-driven phase-skill workflow.
**Acceptance Criteria:**
- Global installation creates a Claude Code-consumable Rotta orchestration entrypoint under `~/.claude/` that is discoverable for normal user feature requests and contains the canonical proportional workflow-selection policy.
- The entrypoint performs the same observable routing as the OpenCode orchestrator: it uses the direct, focused-verification path only for simple, well-scoped, low-risk work; otherwise it guides the user through Draft, Spec + Gherkin, explicit approval, one-scenario-at-a-time TDD, and Review.
- Global installation creates Claude Code-consumable, automatically delegated phase-agent definitions named exactly `rotta-spec`, `rotta-impl`, and `rotta-review` under the host's supported global agent location.
- `rotta-spec`, `rotta-impl`, and `rotta-review` are not the normal user-facing workflow entrypoints; the orchestrator selects and delegates to them at the canonical phase boundaries.
- The orchestrator does not write production or test code itself. It delegates specification/Gherkin work only to `rotta-spec`, exactly one approved scenario implementation task at a time to `rotta-impl`, and evidence-based review only to `rotta-review`.
- The orchestrator preserves the explicit human approval gate after Gherkin, refuses to advance with unresolved open questions, verifies the clean-worktree precondition before every implementation delegation, and performs the same post-scenario boundary/cleanup behavior required by the canonical workflow.
- Phase-agent tool access is least-privilege and behaviorally equivalent to OpenCode: `rotta-spec` cannot run arbitrary shell commands or implement production/test code; `rotta-impl` can edit and run project test commands; `rotta-review` reviews evidence without editing production code; none of the three phase agents can recursively delegate.
- The orchestrator and every delegated agent that needs them receive the installed Ancora, Vela, and Context7 MCP surfaces and follow the same generated fallback/degradation instructions. A missing or degraded MCP never bypasses an approval or quality gate.
- Claude agents inherit the invoking model unless the user later configures an explicit model policy.
- Installation, rerun, update, cleanup, and failure recovery preserve unrelated `~/.claude/` settings, skills, agents, hooks, and MCP entries; they remove or update only Rotta-owned artifacts.
- Automated integration tests cover the generated global entrypoint, the three named agent definitions, delegation/tool boundaries, role instructions, MCP visibility/configuration, idempotent reinstall, and safe cleanup.
- Compatibility verification runs against the latest stable Claude Code release available when the Rotta test/release pipeline executes, records the tested `claude --version`, and fails rather than claiming verified Claude compatibility when that verification cannot run. Earlier Claude Code releases are best-effort only.
**Edge Cases:**
- A simple request is handled directly and never launches a phase agent.
- A complex or high-risk request triggers the orchestrated workflow and waits for user clarification or approval where required.
- A user explicitly asks for a phase agent, but the workspace state requires an earlier phase; the orchestrator preserves phase order rather than bypassing it.
- A selected phase agent, MCP, or required tool is unavailable; the orchestrator reports the specific degradation and safe recovery action without fabricating task completion.
- A previous Rotta installation contains only legacy phase skills or malformed Rotta-owned agent files.
- A reinstall is performed while unrelated user-defined Claude agents, skills, permissions, hooks, or MCPs coexist.
- The installed Claude Code executable is absent, unsupported, or cannot report its version during verification.
**Out of Scope:**
- Project-local Claude installation or per-repository agent configuration.
- Guaranteed support for every historical or future Claude Code release.
- Changing the OpenCode agent contract or requiring OpenCode to use Claude-specific artifact shapes.
- Selecting specialised models per phase agent in this feature.

### REQ-045: Create and Bind One Isolated Feature Worktree Before Specification
**Description:** For every full Rotta workflow, the orchestrator must establish one clean, recorded `feature/<slug>` worktree before any Phase 2 contract, approval evidence, execution state, report, production code, test, or checkpoint is written. The initiating checkout is read-only for that submission.
**Acceptance Criteria:**
- Before launching Spec/Gherkin work, Rotta verifies that the initiating checkout is a Git worktree with no non-ignored changes and creates or selects exactly one exclusively owned isolated `feature/<validated-slug>` worktree from the recorded base branch.
- Rotta records and reports the absolute worktree path, feature branch, base branch, submission identity, and contract destinations before Phase 2 writes.
- Phase 2, Phase 3, and Phase 4 use that recorded worktree; they must not create a second post-approval worktree or fall back to the initiating checkout.
- All submission-scoped artifacts are written only in the recorded worktree.
- An initiating-checkout cleanliness failure, worktree creation failure, path/branch collision, detached/incorrect identity, or post-delegation identity failure stops the workflow before the unsafe write or next delegation and reports a non-destructive recovery action.
**Edge Cases:**
- The initiating checkout becomes dirty, changes branch, or is removed after its first validation.
- A requested feature branch or sibling path already exists, is occupied by another worktree, or is a file or symlink.
- A supported host resumes a workflow started by another host and is launched from a different current directory.
**Out of Scope:**
- Stashing, committing, resetting, deleting, moving, or otherwise repairing user changes in the initiating checkout.
- Reusing an existing feature worktree whose exclusive ownership cannot be proven.

### REQ-046: Persist an Approved Contract Baseline and Feature-Scoped Approval Evidence
**Description:** Explicit Gherkin approval must automatically create one local contract-baseline checkpoint in the recorded feature worktree. The checkpoint contains the approved hard spec, feature contract, and a tracked durable approval record scoped to that feature and baseline; ignored shared `specs/.approved` is not approval authority.
**Acceptance Criteria:**
- On explicit human approval, Rotta creates a feature-scoped durable approval record under `specs/approvals/<feature-slug>.yaml` containing the approved scenario IDs, approval timestamp, submission/feature identity, and fingerprints of the approved hard-spec and feature files.
- Rotta creates one local baseline commit on the recorded feature branch containing only the approved hard spec, feature contract, and feature-scoped approval record, then records its commit identifier in current workflow state.
- Phase 3 treats only that feature-scoped record and its matching committed baseline as approval authority; it neither reads nor writes `specs/.approved` to decide eligibility.
- If approval is missing, ambiguous, incomplete for the next scenario, cannot be durably checkpointed, or no longer matches the contract fingerprints, Rotta stops before implementation and reports the recovery action.
- After approval, any contract modification invalidates implementation eligibility until a new explicit approval produces a new matching baseline checkpoint; Rotta does not amend, overwrite, or silently reapprove the earlier record.
**Edge Cases:**
- A human approves only a subset of scenarios.
- The baseline commit succeeds but writing current local state or Ancora pointer state fails.
- The contract is edited, renamed, deleted, or replaced after approval, including by another host or agent.
**Out of Scope:**
- Backfilling or trusting legacy shared approval markers.
- Remote publication of the contract-baseline checkpoint.

### REQ-047: Execute the Approved Phase 3 Scenario Loop Autonomously at Clean Boundaries
**Description:** After a valid baseline checkpoint, Rotta must run Phase 3 in the same recorded worktree without asking whether to continue or commit: exactly one approved scenario is delegated, validated through strict TDD and required gates, automatically checkpointed, and followed by a clean boundary before the next approved scenario.
**Acceptance Criteria:**
- Rotta launches only the next approved scenario in the recorded order and provides the implementation role with the recorded worktree and approved contract scope.
- For each scenario, Rotta requires Red/Green/Refactor evidence, traceable tests, required tests, and active objective gates before checkpointing.
- On success, Rotta creates exactly one scenario-scoped local checkpoint commit, records its commit and evidence in current state, verifies the recorded worktree/branch identity and absence of non-ignored changes, and automatically starts the next approved scenario when one remains.
- When the final approved scenario is checkpointed and the clean boundary passes, Rotta enters Phase 4 review; it does not request a conversational continuation decision or treat completion as human acceptance.
- Rotta never pushes, opens a pull request, merges, rebases, resets, tags, or directly modifies the initiating/base checkout during this loop.
**Edge Cases:**
- A scenario has no production change but has an approved test, configuration, documentation, or orchestration-asset change.
- A required test/gate fails, a checkpoint commit fails, or state persistence fails at a scenario boundary.
- An ignored local runtime artifact changes while all non-ignored paths are clean.
**Out of Scope:**
- Parallel implementation of two scenarios in one worktree.
- Automatic retry that changes the approved scenario scope or bypasses failed evidence.

### REQ-048: Fail Closed Without Discarding Unexpected Changes
**Description:** The autonomous loop must halt rather than checkpoint or advance whenever it cannot prove the approved baseline, recorded worktree identity, expected scenario diff, required evidence, or clean boundary.
**Acceptance Criteria:**
- Rotta stops before checkpointing or launching another scenario on a missing approval, contract drift, identity failure, failed required test/gate, unexpected tracked change, untracked non-ignored file, or dirty post-checkpoint boundary.
- The stop report names the failed category, affected path/scenario when available, recorded worktree, and a safe recovery action.
- On every stop, Rotta preserves user and agent changes: it does not discard, stash, revert, reset, delete, automatically add, or commit ambiguous/unexpected changes.
- A stopped workflow retains its recorded worktree and branch for inspection; it does not redirect execution to the initiating checkout or choose an alternative worktree.
**Edge Cases:**
- Another process modifies the worktree during a delegated scenario.
- A Git hook, lock, permission failure, or host working-directory bug interferes with a checkpoint.
- Contract drift is detected after the baseline checkpoint but before or after a scenario delegation.
**Out of Scope:**
- Automatic conflict resolution, Git repair, or classification of ambiguous changes as expected scenario work.

### REQ-049: Archive Terminal Active State While Retaining the Reviewable Feature Worktree
**Description:** After Phase 4 reaches a terminal review result, Rotta must archive `.rotta/current` execution state without removing the recorded feature branch or worktree, so the user can inspect, push, and create a PR manually. Explicit cleanup after confirmed publication or abandonment removes the worktree only when safe.
**Acceptance Criteria:**
- A completed, review-failed terminal, abandoned, or cancelled workflow archives its active `.rotta/current` state and removes it from active workflow scope while preserving committed contracts and their durable approval record.
- Review completion preserves the recorded feature branch and worktree and presents the existing manual inspection/publication handoff rather than deleting either.
- An explicit cleanup request is accepted only for a workflow recorded as published by the user or explicitly abandoned/cancelled; it removes the recorded feature worktree and its local branch/worktree registration only after validating identity and reporting what will be removed.
- Cleanup never removes the initiating checkout, a different worktree, active current state, uncommitted changes, or durable contract artifacts.
- If archiving or cleanup cannot safely complete, Rotta reports the failure and leaves the existing artifacts intact.
**Edge Cases:**
- A user requests cleanup immediately after review but before publication/abandonment.
- The worktree has unexpected non-ignored changes, was manually removed, or its branch is checked out elsewhere.
- Archive destination collision or archive write failure occurs.
**Out of Scope:**
- Automatic push, PR creation, merge, remote-branch deletion, or cleanup merely because review passed.

### REQ-050: Apply the Lifecycle Consistently Through Generated Host Orchestration
**Description:** Generated orchestration assets for OpenCode, Claude Code, and Codex must enforce the same worktree-before-Spec, durable-approval-baseline, autonomous-scenario-loop, safety-stop, archive, and manual-cleanup lifecycle. Claude native delegation must align with this lifecycle without becoming a host-specific exception.
**Acceptance Criteria:**
- Every supported host's generated orchestration instructions direct a full workflow to prepare and record the isolated feature worktree before Spec/Gherkin writes.
- Every supported host uses the feature-scoped durable approval baseline rather than `specs/.approved`, and preserves contract-drift and missing-approval stops.
- OpenCode and Claude native orchestration delegate the same one-scenario autonomous loop and checkpoint boundaries; Codex's adapted invocation preserves the same gates even where it lacks named subagents.
- Generated phase-agent instructions and state-machine transitions do not ask for a commit/continue decision after a successful approved scenario.
- Host installation and generated-artifact tests prove the lifecycle policy is present for all supported hosts and that Claude orchestration does not create a second worktree after approval.
**Edge Cases:**
- A workflow is resumed on a different supported host.
- A host cannot delegate or cannot invoke a Git operation directly.
- An older generated host asset still references the legacy marker or post-approval worktree creation.
**Out of Scope:**
- Making host-specific user interfaces, credentials, or native delegation primitives identical.

### REQ-051: Preserve Feature-Scoped Lifecycle State as the Only Active Authority
**Description:** The current submission manifest/state, committed feature-scoped approval record, and approved contract baseline together define a submission's active scope. Legacy global markers are migration-era artifacts only and cannot authorize, block, or expand a workflow.
**Acceptance Criteria:**
- Current state records the feature branch/worktree, baseline commit, approval-record path and fingerprint, approved scenarios, completed checkpoints, remaining scenario, phase, and terminal/archive status.
- Resume and review verify the recorded contract and approval baseline before acting and use only the recorded feature scope to select implementation or review scenarios.
- `specs/.approved` is neither created nor consulted as approval authority for new workflows; if encountered, it is reported as legacy/non-authoritative without altering the current feature scope.
- Ancora remains pointer-only and cannot substitute for a missing durable approval record, contract baseline, or workflow state.
**Edge Cases:**
- Multiple feature worktrees coexist with different approved scenario subsets.
- A stale Ancora pointer references a removed worktree or superseded baseline.
- A legacy marker disagrees with the feature-scoped approval record.
**Out of Scope:**
- Automatic migration of legacy approval-marker content into a new approved record.

## Open Questions
- None.

## Trade-offs
- A canonical Rotta contract plus host adapters reduces behavioral drift but requires careful capability mapping and explicit limitation reporting for hosts that lack OpenCode-equivalent primitives.
- Idempotent safe writes, backups, and health checks increase installer complexity and runtime, but prevent false success and protect user-level AI host configuration.
- Preserving exact workflow semantics across hosts may require adapted command surfaces or composed instruction files where a host does not support named agents, skills, or slash commands.
- Keeping lifecycle artifacts out of commits by default protects clean worktree expectations, but teams that want committable specs/features will need an explicit opt-in path.
- Continuing without optional MCPs preserves progress and durable contracts, but users receive less memory, graph, or documentation evidence and must act on disclosed verification needs.
- A fixed source-exploration budget prevents an unavailable Vela graph from causing unbounded investigation, but may require an explicit follow-up when evidence is incomplete.
- Native Claude orchestration provides actual delegation parity but adds host-specific entrypoint and agent-discovery behavior that must be tested against the current stable Claude Code release rather than inferred from OpenCode behavior.
- A committed baseline and explicit approval-record schema add Git history and artifact-management work, but make approval scope auditable and remove shared ignored-marker ambiguity.
- Autonomous scenario progression reduces conversational control between slices, but retains fail-closed validation and non-destructive stops at every boundary.
- Retaining terminal worktrees consumes local disk and branch namespace until explicit cleanup, but preserves inspection and manual publication authority.

## Risk Level
critical — Justification: This feature governs every full workflow's Git authority boundary, approval authority, automatic commits, destructive-cleanup prohibition, and cross-host orchestration. A defect can contaminate an initiating checkout, execute an unapproved or changed contract, commit user changes, strand a reviewable branch, or make hosts follow incompatible safety semantics; the extension also relies on evolving Claude Code delegation behavior.
