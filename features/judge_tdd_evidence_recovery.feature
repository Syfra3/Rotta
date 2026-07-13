Feature: Judge TDD evidence recovery
  Rotta maintainers need fresh, observable recovery contracts for portable MCP command behavior and a truthful lifecycle submission scope, so Judge review can rely on recorded evidence rather than reconstructed history.

  @REQ-028 @SCN-231
  Scenario: Serialize a recovered managed MCP executable as a canonical bare command
    Given a supported host is configured with a Rotta-managed MCP server
    And the managed executable resolves to a versioned or absolute executable location
    When Rotta serializes the managed MCP server configuration
    Then the serialized MCP executable command is its canonical bare command
    And the serialized executable command does not contain the resolved absolute or versioned location

  @REQ-029 @SCN-232
  Scenario: Normalize a recovered stale managed executable during reinstall
    Given a supported host has a proven Rotta-managed MCP entry with a stale versioned executable command
    When the user reinstalls Rotta
    Then Rotta changes the managed MCP executable command to its canonical bare command
    And Rotta reports that the managed entry was normalized
    When the user reinstalls Rotta again
    Then Rotta makes no further command-field change to that entry

  @REQ-030 @SCN-233
  Scenario: Preserve a non-command absolute reference while recovering executable normalization
    Given a supported host configuration contains a Rotta-generated hook script referenced by an absolute location
    And it contains a proven Rotta-managed MCP command with an absolute executable location
    When the user reinstalls Rotta
    Then Rotta normalizes only the managed MCP executable command to its canonical bare name
    And Rotta preserves the absolute hook-script reference

  @REQ-031 @SCN-234
  Scenario: Submit only a truthfully reconciled recovery scope for review
    Given an authorized lifecycle decision defines the recovery scenarios submitted for review
    And every scenario in that submitted approval scope has recorded required TDD evidence
    And every submitted completed scenario has recorded implementation completion evidence
    When the recovery submission is prepared for Judge review
    Then the approval marker identifies exactly the submitted scenario scope
    And the implementation-complete marker identifies only completed scenarios in that scope
    And no marker represents new recovery evidence as historical evidence for a legacy scenario
