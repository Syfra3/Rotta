# Hard Spec: GitHub Copilot CLI Compatibility

## Adversarial Pre-Mortem
- Failure mode 1: The installer assumes `~/.copilot` is always active, writes a historical/default location while Copilot uses a configured directory, and reports an integration that the running host cannot discover.
- Failure mode 2: Rotta treats partial MCP-schema evidence as a universal file contract, replacing an unknown user configuration or producing registrations that the verified CLI cannot load.
- Failure mode 3: Generated `.agent.md` prompts claim host-native delegation or healthy MCPs from artifact creation alone, letting a user believe an approval gate or runtime proof exists when it does not.

## Hidden Assumptions
- Copilot discovers global custom agents from the active user-global config root's `agents/` directory; current documentation identifies `.agent.md` Markdown files with YAML frontmatter.
- The active Copilot config root can differ from the historical/default `~/.copilot` through documented `--config-dir` and COPILOT_HOME-related behavior, and must be resolved before mutation.
- `mcpServers` wrapping is documented for temporary configuration, while support for MCP fields such as `args`, `type`, `env`, and `deferTools` is only partial evidence; the Rotta fixture therefore requires current-CLI validation.
- An interactive session supports `/mcp list` and `/mcp show <server-name>`; `copilot mcp list` is supported, but other noninteractive `copilot mcp` subcommands must not be assumed without version-specific `copilot mcp --help` proof.
- Custom-agent selection through `/agent` or `copilot --agent <name>` selects role guidance, not a guarantee of hidden subagent delegation, lifecycle ownership, or MCP health.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Treat `both` as the four-host aggregate | It silently changes the established CLI/API contract and can mutate a new host configuration without a caller selecting it. |
| Unconditionally write `~/.copilot/mcp-config.json` with a declared official schema | The path is historical/default evidence and the schema proof is incomplete; this can configure the wrong root or corrupt unknown user configuration. |
| Generate JSON agent definitions or prescribe a complete frontmatter schema | Current authoritative evidence requires `.agent.md` YAML-frontmatter artifacts and does not justify an invented complete schema. |
| Use `/mcp server detail` or unverified `copilot mcp add/show/detail` commands | The documented diagnostics are `/mcp list`, `/mcp show <server-name>`, and `copilot mcp list`; unsupported command claims are not acceptable. |
| Resolve the newest Copilot release during ordinary installation | Installation must remain offline-safe. Version/release evidence belongs to a separate compatibility-verification artifact. |

## Summary
Add GitHub Copilot CLI as a fourth supported Rotta host using only the active user-global Copilot configuration root. Rotta generates `.agent.md` role artifacts and global instructions, and safely merges a minimal, CLI-validated MCP interoperability fixture for Ancora, Vela, and Context7. The TUI exposes Copilot and an explicit all-supported-host target, while legacy `both` remains a two-host CLI/API alias. The installer reports the resolved configuration root, artifacts, capabilities, backups, and recovery outcomes; it distinguishes written configuration from verified runtime behavior. Compatibility is represented by a time-bound verification artifact with release provenance, installed-version capture, and observed diagnostics, never by an online installation dependency or an unlimited future-release promise.

## Requirements

### REQ-100: Add Copilot without breaking target selection contracts
**Description:** Rotta shall add the stable install target string `copilot-cli`. The aggregate target string shall be `all` and mean exactly Claude Code, OpenCode, Codex, and Copilot CLI. The TUI shall present the aggregate as `All supported hosts`, never `Both`, and shall include a separately selectable `Copilot CLI` entry. Existing programmatic/CLI target string `both` remains accepted and means exactly Claude Code plus OpenCode; it must not install, clean, back up, report, or configure Codex or Copilot. The confirmation view and result labels must enumerate the actual selected hosts and their resolved host-global files.

**Acceptance Criteria:**
- Selecting `copilot-cli` selects only Copilot CLI and dispatches only its host installation and selected integrations.
- Selecting `all` dispatches exactly four hosts—`claude-code`, `opencode`, `codex`, and `copilot-cli`—and reports an independent result for each.
- The TUI no longer renders `Both`; its aggregate label and description explicitly state all supported hosts and include Copilot CLI.
- `both` remains a supported legacy input whose selected-host set is exactly Claude Code and OpenCode. Its result and changed-file accounting contain no Copilot paths.
- Unsupported targets fail before backup, cleanup, or host mutation and list all four supported target values.

**Edge Cases:**
- A saved or scripted caller sends `both`, `all`, `copilot-cli`, an empty/default target, or an unknown target.
- A four-host aggregate install succeeds for one host and fails for another.
- The confirmation view is reached with optional MCP choices both enabled and disabled.

**Out of Scope:**
- Renaming or removing the legacy `both` input.
- Adding VS Code, JetBrains, GitHub Copilot Chat, or any repository-local Copilot target.

### REQ-101: Generate resolved-global Copilot Markdown role and instruction artifacts
**Description:** Before writing any Copilot artifact, Rotta shall safely resolve and report the active user-global Copilot configuration root, honoring documented explicit config-directory and COPILOT_HOME-related configuration behavior. `~/.copilot` is a historical/default candidate only; it may be used only when resolution proves it is active. Under the resolved root, Rotta shall generate `agents/rotta-orchestrator.agent.md`, and one `.agent.md` artifact for each selected phase role (`rotta-spec`, `rotta-impl`, `rotta-review`), plus `instructions/rotta.instructions.md`. Each agent file must be Markdown with YAML frontmatter and a role body. Rotta shall require only documented or current-CLI compatibility-tested frontmatter fields; it must not declare an invented complete frontmatter schema. The instruction content must be rendered from the same canonical lifecycle authority used for other hosts.

**Acceptance Criteria:**
- An installation with all phase modes creates the orchestrator, spec, implementation, and review `.agent.md` files and the global instruction artifact under the resolved root; a mode omitted by the user does not produce that phase-role file.
- A compatibility-verification run with the current CLI demonstrates discovery and role selection for each generated agent through `/agent` and `copilot --agent <name>`; malformed/unsupported frontmatter is a failed or degraded artifact, never silently accepted. Ordinary offline installation may write valid artifacts but must report unavailable CLI proof as degraded rather than verified.
- The generated Copilot instructions identify `/agent rotta-orchestrator` and `copilot --agent rotta-orchestrator` as role-selection entrypoints, and direct all phase-work requests through the orchestrator before phase execution.
- The generated capability summary classifies Copilot role agents and command invocation as **adapted**, explains that custom agents select role guidance, and does not claim host-native hidden subagent delegation, automatic delegation, or direct phase bypass.
- Copilot instructions preserve canonical workspace `specs/`, `features/`, and `.rotta/` authority, phase order, explicit approval, TDD/review boundaries, and no-AI-attribution rule.
- No generated Copilot artifact is written below the selected project, including `.github/agents`, `.github/instructions`, `.github/copilot-instructions.md`, `.mcp.json`, `.github/mcp.json`, `AGENTS.md`, or `CLAUDE.md`.

**Edge Cases:**
- The root is explicitly overridden, inherited through COPILOT_HOME-related behavior, unavailable, ambiguous, or not writable.
- A user selects only one phase mode, all phase modes, or no optional MCPs.
- A pre-existing Rotta-managed agent must be updated without multiplying role files; a user invokes a phase role directly.

**Out of Scope:**
- Installation of Copilot CLI, GitHub authentication, credential management, or a claim that Copilot will call one custom agent from another.
- Repository, organization, VS Code, or JetBrains instruction/agent files.

### REQ-102: Safely configure and verify a minimal Copilot MCP interoperability fixture
**Description:** For a resolved active Copilot config root, Rotta shall identify and report the active global MCP configuration path before mutation. A historical/default `<root>/mcp-config.json` candidate is not an unconditional universal path. Rotta's minimal interoperability fixture is a `mcpServers` object containing only selected Rotta-managed stdio registrations: `ancora` with command `ancora` and args `mcp`; `vela` with command `vela` and args `mcp`; and `context7` with command `npx` and args `-y @upstash/context7-mcp`. This is a Rotta fixture based on partial current documentation evidence, not a claim of a complete official universal MCP schema. Before treating it as configured, the implementation must validate the fixture against the current CLI in the resolved configuration context. It must preserve unknown user configuration and safely stop/degrade when the active path or compatible shape cannot be established.

**Acceptance Criteria:**
- The installer records the resolved active config root and MCP path, the resolution basis, and every Copilot path it may change; it makes no Copilot MCP write when that information is unavailable, ambiguous, or unsafe.
- Selecting Ancora, Vela, and Context7 attempts only the three named fixture registrations; unselected MCPs receive no new registration.
- Rotta preserves unknown top-level keys, unknown MCP entries, and user-owned settings without normalizing or deleting them. A same-named entry not proven Rotta-managed stops safely instead of being overwritten.
- Fixture validation uses the verified current CLI and records configuration outcome, command/version evidence, and observable result. A configuration may be reported configured only after the fixture is accepted; it is **exact** only after runtime proof.
- Runtime proof for every selected MCP records `copilot --version`, interactive `/mcp list`, and interactive `/mcp show <server-name>` evidence. `copilot mcp list` may be used where applicable. Any use of another noninteractive `copilot mcp` subcommand requires captured `copilot mcp --help` evidence for that exact CLI version before invocation; Rotta must not assume `add`, `show`, or `detail` commands exist.
- Missing CLI, unresolvable root/path, incompatible or unvalidated fixture, unavailable runtime observability, or unavailable server results in a visible `degraded` or `failed` status with reason and remediation; it is never presented as exact or healthy by inference.
- Ancora remains compact-pointer/status-only; Vela remains advisory structural graph intelligence subject to freshness/degradation rules; Context7 failure remains a visible documentation degradation that cannot invent API facts or bypass workflow gates.

**Edge Cases:**
- The active config root differs from `~/.copilot`; `mcp-config.json` is absent; the CLI changes the active root; or only a historical/default path is discoverable.
- The config has an incompatible shape, duplicate server names, comments/unsupported syntax, a non-object `mcpServers`, unknown fields such as `env` or `deferTools`, or invalid JSON.
- `npx`, `ancora`, or `vela` is unavailable; a server fails startup, initialization, tool discovery, or times out; `/mcp show` reports it unavailable.

**Out of Scope:**
- Repository-scoped MCP configurations, including `.mcp.json` and `.github/mcp.json` regardless of release-specific filename behavior.
- Claiming support for a complete MCP schema or unverified noninteractive MCP subcommands.

### REQ-103: Preserve Copilot user configuration through rerun, backup, and recovery
**Description:** Copilot configuration changes shall be merge-safe and transactional at the resolved managed-file level. Before the first mutation, the installer shall create a recoverable backup covering every resolved Copilot path eligible for change and record the resolved root/path in the transaction manifest and result. Managed `.agent.md` files, the managed instruction file, and a validated MCP configuration file must use same-directory temporary writes followed by atomic replacement (or equivalent all-or-nothing write). A rerun updates only proven Rotta-managed artifacts without duplicates and preserves unrelated current-user content. A malformed, unknown, or incompatible MCP configuration is a safe stop: report the resolved path and condition, leave it unchanged, and report no successful Copilot MCP configuration.

**Acceptance Criteria:**
- A second identical Copilot install produces one selected managed `.agent.md` artifact, one instruction file, and one named fixture registration per selected MCP while retaining unrelated agents, instructions, top-level config keys, and MCP entries.
- Backup manifests, backup previews, restore selection, and changed-file accounting include every resolved Copilot path eligible for mutation, with target `copilot-cli` or `all` represented correctly.
- Restoring a completed backup restores selected Copilot artifacts and the pre-existing MCP configuration byte-for-byte (or removes only a path recorded absent), retaining pre-restore safety-backup behavior and combined failure reporting.
- A failed managed-file write leaves prior valid content in place for that file; a multi-file failure identifies the Copilot artifact, completed work, and backup/recovery location without falsely reporting host success.
- Unknown user configuration, a same-named non-Rotta agent/MCP, or an unresolved active path is preserved and reported rather than replaced or guessed.

**Edge Cases:**
- The resolved root changes between backup and write, a process dies during replacement, backup is incomplete, or restore and pre-restore rollback both fail.
- Existing config contains unknown keys, unrelated registrations, or a schema the compatibility fixture cannot safely merge.
- A user has a same-named but non-Rotta MCP registration or `.agent.md` file.

**Out of Scope:**
- Automatic repair, reformatting, or migration of user-owned Copilot configuration.
- Removing unrelated Copilot configuration or using repository-local files as a fallback.

### REQ-104: Include Copilot in installer dispatch, result accounting, and canonical lifecycle authority
**Description:** The installer dispatch, all-host loop, cleanup, backup scope, capability matrix, MCP status matrix, confirmation view, and changed-file classification shall treat `copilot-cli` as an independent supported host. Copilot's generated instructions must be supplied by the same canonical lifecycle authority as Claude Code, OpenCode, and Codex, with only disclosed adaptation. The capability matrix must classify installation, instructions/agents, commands, MCP configuration, health checks, and lifecycle using exact/adapted/degraded/unsupported/skipped/failed/not-applicable, and must separately report resolved-config evidence from runtime verification evidence.

**Acceptance Criteria:**
- The installer dispatches Copilot for its single-host target and includes it exactly once in `all`; a Copilot error records a Copilot-specific result without hiding other aggregate-host outcomes.
- `Result.Files`, `Result.ChangedFiles`, backup metadata, confirmation output, and per-host result/capability/MCP maps include each changed resolved Copilot path exactly once and classify it as host configuration, not a workspace lifecycle artifact.
- Copilot reporting marks lifecycle authority exact only when generated instructions retain canonical workspace/orchestrator/approval rules; agent and command surfaces remain adapted; unresolved root, unvalidated fixture, or unavailable runtime proof is degraded or failed, not exact.
- Generated Copilot lifecycle guidance does not create or treat host-local configuration as approval, current state, baseline, checkpoint, review, or completion authority.
- The existing three hosts retain their supported selection, all-host behavior, capability semantics, and generated lifecycle authority while Copilot is added.

**Edge Cases:**
- Copilot succeeds while another aggregate host fails, or Copilot fails before another host is configured.
- A rerun changes only resolved Copilot files, only workspace `.rotta` files, or both; categories remain distinct.
- Optional integrations are skipped, partially configured, degraded at runtime, or unavailable during an aggregate install.

**Out of Scope:**
- Changing canonical phase order, approval records, baselines, TDD, review, or final-human-review semantics.
- Treating Copilot local state as distributed workflow authority.

### REQ-105: Document actual support, resolution behavior, verification boundary, and exclusions
**Description:** User-facing documentation shall describe all actual Rotta host integrations: Claude Code, OpenCode, Codex, and GitHub Copilot CLI. It shall identify Copilot's resolved user-global config root, `.agent.md` and instruction surfaces, the minimal validated MCP fixture, the `All supported hosts` aggregate, legacy `both` behavior, adapted orchestration/command limitations, global-only scope, and offline-safe versioned verification artifact. It shall explicitly exclude VS Code, JetBrains, and repository-local Copilot files.

**Acceptance Criteria:**
- README host tables and compatibility text no longer omit Codex and include Copilot CLI without claiming native hidden-subagent implementation.
- Documentation describes `~/.copilot` only as a historical/default example, explains that Rotta resolves and reports the active global root/path before mutation, and names generated `.agent.md` and instruction artifacts relative to that root.
- Documentation states that MCP entries are a minimal interoperability fixture validated against the current CLI, preserves unknown configuration, and uses `/mcp list` and `/mcp show <server-name>` for interactive diagnostics.
- Documentation states `all` selects four hosts and `both` remains the Claude Code/OpenCode compatibility input.
- Documentation describes verification as a time-bound artifact recording official release identity/source, timestamp, `copilot --version`, and observed diagnostics. Normal installation remains offline-safe and reports unavailable runtime proof as degraded rather than verified.
- Documentation explicitly says VS Code, JetBrains, and repository-local `.github`/`.mcp.json` integrations are excluded.

**Edge Cases:**
- A reader uses an existing `both` script, installs only Copilot, uses an overridden global config root, or has no local Copilot executable at artifact-install time.
- Documentation, TUI wording, and resolved-path result drift from actual selected-host or capability behavior.

**Out of Scope:**
- Documentation for unsupported Copilot products or repository-local integration setup.
- Presenting unavailable runtime proof as verified compatibility.

## Open Questions
- None. Active-root/path uncertainty, incomplete MCP-schema evidence, unavailable CLI/runtime diagnostics, and unavailable version provenance all have specified safe-stop or degraded outcomes. The verification artifact—not ordinary installation—records the time-bound release and runtime proof.

## Trade-offs
- Maintaining `both` while adding `all` retains a less descriptive alias but prevents legacy callers from receiving Copilot side effects.
- Resolving rather than assuming a global root avoids wrong-directory writes but adds a visible prerequisite for safe Copilot mutation.
- A small validated fixture gives interoperable behavior without pretending partial documentation is a complete schema, at the cost of degradation where the current CLI cannot validate it.
- `.agent.md` artifacts with minimal tested frontmatter avoid fabricated schema requirements but require compatibility evidence for the fields Rotta uses.
- Offline-safe installation separates artifact creation from versioned runtime verification, favoring truthful status over immediate universal compatibility claims.

## Risk Level
high — Justification: The feature changes a public target-selection contract, edits persistent user-global agent/MCP configuration whose active location and schema can vary, introduces multi-host recovery paths, and must preserve Rotta approval authority despite different Copilot host primitives.
