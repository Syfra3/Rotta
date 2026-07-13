Feature: Portable Rotta-managed MCP executable commands
  Rotta users need managed MCP configurations that survive package upgrades and work across supported installation methods without embedding machine-specific executable paths.

  @SCN-223 @REQ-015
  Scenario Outline: Serialize a managed MCP executable as a bare command
    Given Rotta configures the <server> MCP server for a supported host
    And Rotta resolves <resolved location> in its current environment
    When Rotta serializes the managed MCP server configuration
    Then the MCP command is <bare command>
    And the serialized MCP command does not contain an absolute or versioned executable path

    Examples:
      | server   | resolved location                                               | bare command |
      | Ancora   | /opt/homebrew/Cellar/ancora/1.2.3/bin/ancora                  | ancora       |
      | Vela     | /home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela         | vela         |
      | Context7 | /home/user/.local/bin/npx                                      | npx          |

  @SCN-224 @REQ-016
  Scenario: Normalize a stale managed Homebrew MCP executable during reinstall
    Given a supported host has a proven Rotta-managed Vela MCP entry with a versioned Cellar executable path
    When the user reinstalls Rotta
    Then Rotta changes that managed MCP command to "vela"
    And Rotta reports that the managed entry was normalized
    And a subsequent reinstall makes no further command-field change

  @SCN-225 @REQ-016
  Scenario: Preserve non-executable absolute references during MCP normalization
    Given a supported host configuration contains a Rotta-generated hook script referenced by an absolute path
    And it contains a proven Rotta-managed MCP command with an absolute executable path
    When the user reinstalls Rotta
    Then Rotta normalizes only the MCP executable command to its bare name
    And Rotta preserves the absolute generated hook-script reference

  @SCN-226 @REQ-016 @REQ-019
  Scenario: Preserve an ambiguous MCP entry rather than rewriting user configuration
    Given a supported host has an MCP entry whose ownership cannot be proven
    And the entry contains an absolute command path
    When the user reinstalls Rotta
    Then Rotta leaves that entry unchanged
    And Rotta reports that normalization was skipped because ownership is ambiguous

  @SCN-227 @REQ-017
  Scenario: Report an unavailable installer command without serializing a fallback path
    Given a selected Rotta-managed MCP command is unavailable in Rotta's current environment
    And an existing proven Rotta-managed entry uses a stale absolute executable path
    When the user reinstalls Rotta
    Then Rotta normalizes the managed command to its canonical bare name
    And Rotta does not serialize an absolute fallback executable path
    And Rotta reports the server as degraded or failed for command availability with remediation

  @SCN-228 @REQ-018
  Scenario: Distinguish OpenCode PATH uncertainty from installer command availability
    Given Rotta can resolve "npx" in its current environment
    And OpenCode host-side command resolution cannot be verified
    When Rotta configures the Context7 MCP server for OpenCode
    Then Rotta serializes the command as "npx"
    And Rotta reports that the configuration is portable but OpenCode command resolution is unverified
    And Rotta directs the user to launch OpenCode with "npx" available on PATH
    And Rotta does not serialize a Homebrew or other absolute fallback path

  @SCN-229 @REQ-018 @REQ-019
  Scenario: Report a host-side command lookup failure without masking it
    Given a Rotta-managed MCP command is available to Rotta
    And a supported host fails to start that MCP because the command is absent from the host process PATH
    When Rotta receives observable host health or startup failure evidence
    Then Rotta reports host command availability as the failure category
    And Rotta does not report the MCP as healthy
    And Rotta provides PATH remediation without changing the serialized command to an absolute path

  @SCN-230 @REQ-019
  Scenario: Retain portable managed commands after an executable upgrade
    Given Rotta previously configured a supported host with canonical bare MCP commands
    And the user upgrades a managed executable through Homebrew, curl/manual reinstall, or an OS-specific package mechanism
    When the user reinstalls Rotta
    Then every proven Rotta-managed MCP executable command remains a canonical bare name
    And Rotta does not introduce a versioned Cellar path or other absolute binary location
