Feature: Host-agnostic Rotta compatibility
  Rotta users need the same workflow, commands, generated guidance, MCPs, and lifecycle behavior across Claude Code, OpenCode, and Codex so they can choose their agentic coding host without losing Rotta guarantees.

  @SCN-201 @REQ-001 @REQ-002
  Scenario: Install Rotta into a single supported host
    Given the user selects "Codex" as the only Rotta host target
    When Rotta runs host installation
    Then Rotta installs only Codex-consumable integration artifacts
    And Rotta does not mutate Claude Code or OpenCode host configuration
    And the install summary reports Codex host installation as installed

  @SCN-202 @REQ-001 @REQ-002
  Scenario: Install Rotta into all supported hosts with independent results
    Given the user selects "Claude Code", "OpenCode", and "Codex" as Rotta host targets
    When Rotta runs host installation
    Then Rotta attempts installation for exactly those three hosts
    And the summary reports installed, skipped, failed, or partially installed status separately for each host
    And success for one host does not hide failure for another host

  @SCN-203 @REQ-001 @REQ-009
  Scenario: Reject an unsupported host before mutation
    Given the user requests Rotta installation for an unsupported host named "Cursor"
    When Rotta validates host targets
    Then Rotta rejects the host before writing host files
    And Rotta explains that supported hosts are exactly Claude Code, OpenCode, and Codex

  @SCN-204 @REQ-003 @REQ-008
  Scenario: Generate host-specific instructions from the canonical Rotta workflow
    Given the user selected Claude Code, OpenCode, and Codex
    When Rotta generates host guidance artifacts
    Then each selected host receives instructions in a format that host can consume
    And each generated instruction set preserves Rotta phase order, approval gates, TDD expectations, review expectations, memory policy, and no-AI-attribution rule
    And any host-specific adaptation is disclosed in the capability summary

  @SCN-205 @REQ-003 @REQ-008
  Scenario: Disclose when a host lacks an exact agent or skill primitive
    Given a selected host cannot represent OpenCode-style named sub-agents or skills exactly
    When Rotta generates artifacts for that host
    Then Rotta generates the closest supported instruction equivalent
    And Rotta marks the agent or skill capability as adapted or degraded for that host
    And the generated host instructions do not claim exact support for unsupported primitives

  @SCN-206 @REQ-004 @REQ-010
  Scenario: Configure selected MCP servers across selected hosts
    Given the user selects Ancora, Vela, and Context7 MCP support
    And the user selects Claude Code, OpenCode, and Codex as host targets
    When Rotta configures MCP servers
    Then Rotta creates or updates stable Ancora, Vela, and Context7 MCP entries for each host that supports them
    And Rotta preserves unrelated MCP servers and user settings
    And existing OpenCode and Claude Code Context7 configuration remains recognized and safely updated

  @SCN-207 @REQ-004 @REQ-008 @REQ-009
  Scenario: Report unsupported MCP capability without pretending parity
    Given the user selects a host that cannot support a selected MCP server through a known Rotta configuration shape
    When Rotta configures MCP servers
    Then Rotta marks that MCP capability as unsupported or degraded for that host
    And Rotta does not report full MCP parity for that host
    And Rotta continues configuring unrelated supported MCP capabilities where safe

  @SCN-208 @REQ-004 @REQ-009
  Scenario: MCP health check reports observable startup failure
    Given Rotta wrote a selected MCP server entry for a selected host
    And the MCP server command is unavailable or cannot initialize
    When Rotta runs MCP health checks
    Then Rotta reports the MCP health check as failed for that host and server
    And Rotta identifies whether the failure was command availability, startup, initialization, tool discovery, timeout, or unsupported capability
    And Rotta does not report the host installation as fully successful

  @SCN-209 @REQ-005 @REQ-006
  Scenario: Continue a Rotta workflow from a different supported host
    Given a Rotta workflow was started in OpenCode
    And workspace source-of-truth artifacts and `.rotta/` state exist
    When the user continues the workflow from Claude Code or Codex
    Then Rotta reads the shared workspace state and artifacts
    And Rotta preserves the same phase order, command semantics, and approval gates
    And Rotta does not require host-local config to become the workflow source of truth

  @SCN-210 @REQ-005 @REQ-008
  Scenario: Preserve command behavior when a host requires aliases or adapted command exposure
    Given a selected host cannot expose Rotta commands using the same slash-command mechanism as another host
    When Rotta generates command or instruction artifacts for that host
    Then Rotta provides a documented host-appropriate invocation path for the same canonical Rotta commands
    And the adapted invocation maps back to the same Rotta state transitions
    And the limitation is included in the host capability summary

  @SCN-211 @REQ-006
  Scenario: Preserve clean worktree expectations during host installation
    Given the workspace has no user-requested source changes
    When Rotta installs host compatibility artifacts
    Then Rotta distinguishes host configuration changes from Rotta lifecycle artifacts
    And Rotta does not require generated `.rotta/`, `features/`, `reports/`, or `specs/` lifecycle artifacts to be committed by default
    And the install summary lists changed files by host config, workspace host config, or lifecycle category

  @SCN-212 @REQ-006
  Scenario: Store memory state as compact pointers only
    Given Ancora memory is enabled for the Rotta workflow
    When Rotta records spec, feature, TDD, report, or workflow state
    Then Rotta stores workspace files as the source of truth
    And Rotta stores only compact pointers or status in memory
    And Rotta does not store full hard specs, feature files, TDD logs, or review reports in memory

  @SCN-213 @REQ-007 @REQ-010
  Scenario: Re-run installation without duplicating Rotta-managed artifacts
    Given Rotta-managed host artifacts already exist for Claude Code, OpenCode, and Codex
    When the user reruns installation with the same selected hosts and MCPs
    Then Rotta updates or confirms the existing Rotta-managed artifacts deterministically
    And Rotta does not create duplicate agents, skills, instructions, commands, or MCP entries
    And unrelated user configuration remains preserved

  @SCN-214 @REQ-007 @REQ-009
  Scenario: Recover safely from a partial multi-host install failure
    Given the user selected Claude Code, OpenCode, and Codex
    And OpenCode installation succeeds before Codex configuration fails
    When Rotta finishes the install attempt
    Then Rotta reports a partial failure identifying Codex and the failed artifact type
    And Rotta preserves valid completed host configuration
    And Rotta provides safe rerun or manual recovery guidance

  @SCN-215 @REQ-007 @REQ-009
  Scenario: Refuse to overwrite malformed host configuration silently
    Given a selected host has malformed existing configuration
    When Rotta prepares to mutate that host configuration
    Then Rotta reports the malformed file path and host name
    And Rotta does not claim successful installation for that host
    And Rotta does not overwrite the malformed configuration without backup or explicit recovery handling

  @SCN-216 @REQ-008
  Scenario: Present a per-host capability matrix
    Given the user selected one or more supported hosts
    When Rotta completes compatibility installation or generation
    Then Rotta presents a capability matrix for each selected host
    And the matrix covers installation, instructions or agents or skills, commands and workflow, MCP configuration, health checks, and lifecycle behavior
    And each capability is classified as exact, adapted, degraded, unsupported, skipped, failed, or not applicable

  @SCN-217 @REQ-010
  Scenario: Preserve existing OpenCode and Claude Code Context7 behavior when adding Codex
    Given the user already has Rotta-managed Context7 configuration for OpenCode and Claude Code
    When the user adds Codex compatibility
    Then Rotta does not remove, rename, duplicate, or silently degrade the existing OpenCode Context7 entry
    And Rotta does not remove, rename, duplicate, or silently degrade the existing Claude Code Context7 entry
    And Rotta reports the Codex Context7 result independently
