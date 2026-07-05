# Hard Spec: Context7 MCP Integration in TUI Installation Flow

## Adversarial Pre-Mortem
- Failure mode 1: Context7 is checked by default but the installer does not make deselection clear or ignores explicit deselection, causing unwanted host configuration mutations.
- Failure mode 2: The installer writes Context7 MCP configuration for one host but reports success for both OpenCode and Claude Code, leaving a silent partial installation.
- Failure mode 3: The health check only verifies that configuration text exists or that `npx` can start, producing false positives when the MCP server cannot initialize or expose Context7 tools.

## Hidden Assumptions
- OpenCode and Claude Code both support a stdio MCP server entry with command, args, and optional environment values.
- The most compatible cross-host Context7 MCP setup is the npm MCP server package launched through `npx -y @upstash/context7-mcp` over stdio, rather than a host-specific setup command or generated skill/rule.
- Node.js and `npx` availability can be checked before or during the Context7 health check and reported distinctly from host configuration failures.
- Context7 can operate without Rotta generating workflow instructions that tell agents when to use it.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Install Context7 as mandatory with no deselection path | Violates the optional opt-in requirement and risks unexpected host config changes. |
| Use `npx ctx7 setup` | It is an interactive/agent-specific setup flow and may install skills or rules outside Rotta's controlled host config contract. |
| Configure only the remote Context7 HTTP endpoint | Host support for remote MCP transports is less uniformly compatible than command/args stdio entries across OpenCode and Claude Code. |
| Configure Context7 only for whichever host is currently selected first | The requirement explicitly targets both OpenCode and Claude Code host configs. |
| Add generated instructions requiring Context7 use for library docs | Explicitly out of scope; host MCP availability is enough. |

## Summary
Add Context7 as a main optional MCP tool in Rotta's TUI installation flow, presented alongside Ancora and Vela with a clear description and checked by default. Context7 remains optional: the user can deselect it before installation or configuration begins. When Context7 remains selected, Rotta must configure Context7 for both OpenCode and Claude Code using the most compatible stdio MCP package command and must run a real MCP health check that proves the configured server initializes and exposes Context7 tools. Skipping Context7 by deselecting it must leave host configs and generated workflow instructions free of Context7 changes.

## Requirements

### REQ-001: Present Context7 as an Opt-In Main Optional MCP Tool
**Description:** The TUI installer must present Context7 alongside Ancora and Vela as a main optional MCP tool, with Context7 selected by default and described in user-facing language while preserving the user's ability to deselect it before installation or configuration.
**Acceptance Criteria:**
- Context7 appears in the same optional MCP tool selection area or decision level as Ancora and Vela.
- Context7 is preselected when the installer starts a new installation or returns to the MCP tool selection step before the user changes it.
- The user can deselect Context7 before installation or configuration begins.
- The Context7 option includes a concise description stating that it provides up-to-date library/API documentation through MCP.
- Selecting or deselecting Context7 must not change the selected state of Ancora, Vela, or any other main optional MCP tool.
- The final install summary distinguishes Context7 selected, Context7 skipped, and Context7 failed states.
**Edge Cases:**
- User deselects all optional MCP tools, including default-checked Context7.
- User selects Context7 but skips Ancora and Vela.
- User navigates backward after selecting Context7 and then deselects it before confirmation.
**Out of Scope:**
- Automatically recommending Context7 based on project dependencies.

### REQ-002: Configure Context7 for Both Target Hosts When Selected
**Description:** When Context7 is selected, Rotta must configure a Context7 MCP server entry for both OpenCode and Claude Code host configurations without claiming success for a host that was not updated.
**Acceptance Criteria:**
- OpenCode host configuration receives a Context7 MCP server entry only when Context7 is selected.
- Claude Code host configuration receives a Context7 MCP server entry only when Context7 is selected.
- The configured server name is stable and unambiguous, using `context7` unless an existing host convention requires a deterministic equivalent.
- Existing unrelated host MCP servers and user settings are preserved.
- If OpenCode configuration succeeds and Claude Code configuration fails, the install result reports partial Context7 failure and identifies the failed host.
- If Claude Code configuration succeeds and OpenCode configuration fails, the install result reports partial Context7 failure and identifies the failed host.
- Re-running installation with Context7 selected updates or normalizes the existing Rotta-managed Context7 entry instead of creating duplicates.
**Edge Cases:**
- One host configuration file is missing and must be created while the other already exists.
- One host configuration file exists but contains unrelated user MCP entries.
- One host configuration file is malformed or not writable.
**Out of Scope:**
- Configuring Context7 for hosts other than OpenCode and Claude Code.

### REQ-003: Use the Compatible Context7 Stdio MCP Command
**Description:** Rotta must configure Context7 through the cross-host stdio MCP command that is most compatible with OpenCode and Claude Code command/args host config.
**Acceptance Criteria:**
- The configured command is `npx`.
- The configured args are exactly `-y` and `@upstash/context7-mcp` in that order, unless a future Context7 upstream compatibility change is explicitly reflected in this hard spec.
- The configured transport is stdio/command-based MCP, not a generated skill, prompt rule, or host-specific interactive setup.
- Rotta does not require an API key to configure Context7, but may preserve or pass a user-provided `CONTEXT7_API_KEY` if such input already exists in the install flow.
- The installer reports a command availability problem separately from a host config write problem when `npx` or a compatible Node.js runtime is unavailable.
**Edge Cases:**
- `npx` is missing from PATH.
- Node.js exists but is too old for Context7's current MCP package requirements.
- The package download or startup fails because the network or npm registry is unavailable.
**Out of Scope:**
- Installing Node.js, npm, or a global Context7 package.

### REQ-004: Health Check Must Prove Context7 MCP Is Working
**Description:** After configuring Context7, the TUI must check that Context7 works with behavior comparable to Ancora and Vela MCP checks, using observable MCP server capability rather than config-file presence alone.
**Acceptance Criteria:**
- The health check runs only when Context7 is selected.
- The health check starts or connects to the configured Context7 MCP server using the same command/args/transport written to host config.
- A passing health check requires successful MCP initialization and discovery of Context7 documentation tools, including `resolve-library-id` and `query-docs` or their documented upstream equivalents.
- A config file write without successful MCP initialization is reported as failed health, not success.
- A process that starts and exits before tool discovery is reported as failed health, not success.
- Health check output shown in the TUI identifies whether failure came from command availability, server startup, MCP initialization, tool discovery, or timeout.
- Health check timeout must produce a failure or retryable warning state; it must not be reported as success.
**Edge Cases:**
- The package starts but exposes no tools.
- The package exposes only one expected tool.
- The MCP server prints warnings to stderr but initializes and exposes expected tools.
- Network is unavailable during startup or tool discovery.
**Out of Scope:**
- Verifying the accuracy or completeness of third-party library documentation returned by Context7.

### REQ-005: Skipping Context7 Must Leave No Context7 Configuration or Instruction Changes
**Description:** When the user deselects or otherwise leaves Context7 unchecked before installation/configuration, Rotta must skip Context7 setup and health checks without adding Context7 host entries or generated workflow-instruction text.
**Acceptance Criteria:**
- An install where the user explicitly deselects Context7 does not add a Context7 MCP server entry to OpenCode config.
- An install where the user explicitly deselects Context7 does not add a Context7 MCP server entry to Claude Code config.
- The installer does not run Context7 command availability checks or MCP health checks when Context7 is skipped.
- The install summary shows Context7 as skipped or not selected, not failed.
- Rotta-generated workflow instructions do not mention using Context7 for library docs, API references, code examples, setup help, or similar prompts.
- Skipping Context7 does not remove unrelated user-managed Context7 configuration unless that cleanup is separately covered by an approved backup/cleanup spec.
**Edge Cases:**
- User previously installed Context7 manually outside Rotta.
- User selects Context7, then deselects it before installation.
- Ancora and Vela are selected while Context7 is skipped.
**Out of Scope:**
- Removing existing manual Context7 installations.

### REQ-006: Preserve Optional Tool Independence and Recovery-Safe Reporting
**Description:** Context7 integration must not alter the optionality, installation result, or health-check semantics of Ancora and Vela.
**Acceptance Criteria:**
- Ancora, Vela, and Context7 can each be selected independently.
- Failure to configure or check Context7 does not mask Ancora or Vela results.
- Failure to configure or check Ancora or Vela does not mask Context7 results.
- The final result includes enough per-tool and per-host detail for the user to know what was configured, skipped, or failed.
- If the installer has backup/restore safety behavior, Context7 host-config paths are included in the same mutation-safety model as other MCP host config changes.
**Edge Cases:**
- All three optional MCP tools are selected and one fails health check.
- Context7 is the only selected optional MCP tool and its health check fails.
- Host config succeeds but the user cancels before health checks complete.
**Out of Scope:**
- Changing Ancora or Vela command semantics except where necessary to present them as peer optional tools.

## Open Questions
- None.

## Trade-offs
- Using `npx -y @upstash/context7-mcp` over stdio prioritizes broad host compatibility and controlled config generation, but depends on Node.js, `npx`, npm registry access, and package startup at health-check time.
- Requiring MCP tool discovery reduces false positives, but can make installation slower and sensitive to transient network/package failures.
- Not generating workflow instructions avoids unsolicited agent behavior changes, but users may need to invoke Context7 according to their host's normal MCP usage patterns.

## Risk Level
medium — Justification: The feature mutates user-level AI host configuration for two hosts and executes an external MCP package during health checks, but it remains optional with explicit deselection before configuration and does not change production project code.
