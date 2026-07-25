Feature: GitHub Copilot CLI compatibility
  Rotta users need a global GitHub Copilot CLI integration that preserves the contract-driven workflow without assuming a fixed configuration directory, an unproven MCP schema, or unsupported host-native delegation and diagnostics.

  @SCN-402 @REQ-100
  Scenario: Select GitHub Copilot CLI as the only installation target
    Given the user selects "Copilot CLI" in the Rotta installer
    When the user confirms installation
    Then Rotta installs and reports only the Copilot CLI host integration
    And Rotta does not mutate Claude Code, OpenCode, or Codex host configuration
    And the confirmation identifies the resolved Copilot global files that may change

  @SCN-403 @REQ-100 @REQ-104
  Scenario: Select every supported host through an explicit aggregate label
    Given the user selects "All supported hosts" in the Rotta installer
    When the user confirms installation
    Then Rotta attempts Claude Code, OpenCode, Codex, and Copilot CLI exactly once each
    And the result reports a separate outcome for each of the four hosts
    And the aggregate label does not say "Both"

  @SCN-404 @REQ-100 @REQ-105
  Scenario: Preserve the legacy two-host target string
    Given an existing caller requests the target string "both"
    When Rotta validates and runs the installation
    Then Rotta selects exactly Claude Code and OpenCode
    And Rotta does not create, clean, back up, or report a Copilot CLI artifact
    And documentation identifies "both" as the legacy Claude Code and OpenCode compatibility input

  @SCN-405 @REQ-100
  Scenario: Reject an unknown target before changing user configuration
    Given a caller requests the target string "copilot-vscode"
    When Rotta validates the target
    Then Rotta rejects the request before backup, cleanup, or host-file mutation
    And Rotta lists Claude Code, OpenCode, Codex, and Copilot CLI as the supported hosts

  @SCN-406 @REQ-101
  Scenario: Generate global Copilot Markdown role definitions for selected Rotta modes
    Given the user selects Copilot CLI with Spec, Implementation, and Review modes
    And Rotta resolves an active user-global Copilot configuration root
    When Rotta installs Copilot role guidance
    Then the resolved root contains rotta-orchestrator, rotta-spec, rotta-impl, and rotta-review ".agent.md" files under its "agents" directory
    And the resolved root contains "instructions/rotta.instructions.md"
    And every generated agent is Markdown with YAML frontmatter and a Rotta role body accepted by the verified current CLI

  @SCN-407 @REQ-101 @REQ-104
  Scenario: Route Copilot phase requests through adapted orchestration
    Given Copilot CLI has Rotta's generated global role and instruction artifacts
    When a user selects "rotta-orchestrator" through "/agent" or "copilot --agent rotta-orchestrator" and requests phase work
    Then the generated guidance routes the request through the Rotta-Orchestrator decision point before phase execution
    And it preserves workspace authority, phase ordering, explicit human approval, TDD, review, and final-human-review rules
    And it describes role-agent and command support as adapted rather than guaranteed host-native subagent delegation

  @SCN-408 @REQ-101 @REQ-105
  Scenario: Keep Copilot integration global and out of repositories
    Given the user selects Copilot CLI for a project containing existing repository files
    When Rotta completes Copilot artifact generation
    Then Rotta creates no ".github" Copilot integration file in the project
    And Rotta creates no ".mcp.json", "AGENTS.md", or "CLAUDE.md" in the project
    And documentation identifies the Copilot integration as global-only

  @SCN-409 @REQ-102
  Scenario: Register selected MCPs through a validated active-root fixture
    Given the user selects Copilot CLI with Ancora, Vela, and Context7
    And Rotta resolves and reports the active global Copilot configuration root and MCP path
    When Rotta validates and configures the Copilot MCP interoperability fixture with the current CLI
    Then the validated fixture registers only the selected servers named "ancora", "vela", and "context7"
    And those registrations respectively use "ancora mcp", "vela mcp", and "npx -y @upstash/context7-mcp"
    And Rotta does not claim that the fixture is a complete universal Copilot MCP schema

  @SCN-410 @REQ-102
  Scenario: Report exact Copilot MCP health only from documented host evidence
    Given Copilot CLI accepted the selected MCP interoperability fixture in its resolved configuration context
    And the verification artifact captures successful "copilot --version" output
    And "copilot mcp list" lists the selected registered servers
    And the current Copilot session shows each selected server through "/mcp list" and "/mcp show <server-name>"
    When Rotta records Copilot compatibility results
    Then the Copilot MCP capability status is exact for each server proved healthy
    And the result retains the resolved root/path, captured version, and per-server diagnostic evidence
    And the result distinguishes installation configuration from later runtime fallback state

  @SCN-411 @REQ-102 @REQ-104
  Scenario Outline: Degrade rather than infer Copilot MCP configuration or health
    Given Rotta is preparing or has written selected Copilot MCP artifacts
    And Copilot <proof condition>
    When Rotta records the Copilot host result
    Then the affected MCP capability is reported as degraded or failed, not exact
    And the result identifies the missing or failed proof and a safe remediation
    And Rotta preserves the canonical workflow gates

    Examples:
      | proof condition |
      | cannot resolve an active global configuration root or MCP path safely |
      | cannot validate the interoperability fixture with the current CLI |
      | cannot run copilot --version |
      | cannot obtain /mcp list or /mcp show diagnostics |
      | reports the server unavailable |
      | cannot start the configured MCP command |
      | times out during MCP initialization or tool discovery |

  @SCN-412 @REQ-102 @REQ-103
  Scenario: Rerun Copilot installation without changing unrelated configuration
    Given the resolved active Copilot MCP configuration contains unrelated settings and an unrelated MCP server
    And the resolved global Copilot directories contain unrelated user agents and instruction files
    When the user reruns the same Copilot installation with Ancora, Vela, and Context7 selected
    Then Rotta keeps one validated managed registration for each selected Rotta MCP
    And Rotta keeps one instance of each selected Rotta ".agent.md" artifact
    And Rotta preserves the unrelated settings, MCP server, agents, and instruction files

  @SCN-413 @REQ-102 @REQ-103
  Scenario Outline: Refuse unsafe active-root MCP configuration mutation
    Given the resolved active Copilot MCP configuration is <invalid configuration>
    When Rotta prepares to configure Copilot MCP integrations
    Then Rotta reports the resolved global configuration path and the blocking condition
    And Rotta leaves the existing configuration unchanged
    And Rotta does not report successful Copilot MCP configuration

    Examples:
      | invalid configuration |
      | malformed JSON |
      | a non-object mcpServers value |
      | an unknown incompatible configuration shape |
      | a same-named entry not proven Rotta-managed |

  @SCN-414 @REQ-103
  Scenario: Recover safely after a partial Copilot global write
    Given Rotta has backed up the resolved Copilot files eligible for mutation
    And a Copilot managed-file write fails before installation completes
    When Rotta returns the installation result
    Then the failing file retains its prior valid content
    And the result identifies the resolved Copilot artifact, completed work, and backup or recovery location
    And the result does not report the Copilot host as successfully installed

  @SCN-415 @REQ-103
  Scenario: Restore Copilot configuration from a completed backup
    Given a completed Rotta backup contains prior resolved Copilot global artifacts
    And the current Copilot managed artifacts differ from that backup
    When the user restores that backup
    Then Rotta restores every backed-up Copilot artifact and removes only a path recorded absent in the backup
    And Rotta creates a pre-restore safety backup
    And a restore failure reports both the selected backup and the pre-restore recovery outcome

  @SCN-416 @REQ-104
  Scenario: Account for Copilot changes separately from workspace lifecycle artifacts
    Given the user installs Copilot CLI with one or more selected MCPs
    When Rotta produces the installation result
    Then each changed resolved Copilot global path appears once in the Copilot host result and changed-file accounting
    And each Copilot global path is classified as host configuration
    And no Copilot global path is classified as a workspace lifecycle artifact

  @SCN-417 @REQ-104
  Scenario: Preserve canonical lifecycle authority in Copilot guidance
    Given Rotta generates Copilot global guidance
    When a workflow is started or resumed from Copilot CLI
    Then the guidance treats workspace specs, features, and .rotta artifacts as the durable source of truth
    And it does not treat Copilot configuration, MCP state, or Ancora memory as approval or lifecycle authority
    And it does not permit a direct phase role to advance approval, baseline, checkpoint, review, or completion state

  @SCN-418 @REQ-105
  Scenario: Describe all actual supported hosts and corrected Copilot boundaries
    Given Rotta documentation is generated for this release
    When a user reads the supported-host and installation sections
    Then the documentation lists Claude Code, OpenCode, Codex, and GitHub Copilot CLI
    And it identifies resolved global configuration behavior, ".agent.md" artifacts, adapted orchestration, global-only MCP scope, and offline-safe verification
    And it explicitly excludes VS Code, JetBrains, and repository-local Copilot integration files

  @SCN-419 @REQ-105
  Scenario: Record time-bound verification without making installation online-dependent
    Given a Copilot compatibility verification has completed
    When Rotta presents its compatibility status
    Then it records the official release identity/source, timestamp, exact "copilot --version" output, and observed MCP diagnostics
    And ordinary installation does not require online latest-release resolution
    And unavailable version or runtime proof is presented as unverified or degraded rather than verified
