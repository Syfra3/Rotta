Feature: Context7 MCP integration in the TUI installer
  Rotta users receive Context7 as a checked-by-default optional MCP tool, can deselect it before installation, and receive verified host configuration without unwanted instruction changes when it remains selected.

  Background:
    Given the Rotta TUI installer is launched for a project
    And OpenCode and Claude Code are both target host configurations

  @SCN-101 @REQ-001 @REQ-005
  Scenario: Context7 is visible and selected by default
    When the installer shows the optional MCP tool choices
    Then Ancora, Vela, and Context7 are offered as main optional MCP tools
    And Context7 is selected by default
    And the Context7 option explains that it provides up-to-date library/API documentation through MCP

  @SCN-111 @REQ-001 @REQ-005
  Scenario: User can deselect the default-checked Context7 option before installation
    Given the installer shows Context7 selected by default
    When the user deselects Context7 before installation begins
    Then Context7 is not selected
    And installation can continue with Context7 skipped

  @SCN-102 @REQ-001 @REQ-006
  Scenario: Selecting Context7 does not change other optional MCP choices
    Given Ancora is not selected
    And Vela is selected
    When the user selects Context7
    Then Context7 is selected
    And Ancora remains not selected
    And Vela remains selected

  @SCN-103 @REQ-002 @REQ-003
  Scenario: Selected Context7 configures host MCP entries with their compatible command transports
    Given the user selected Context7
    When installation reaches host MCP configuration
    Then OpenCode receives one Rotta-managed MCP server named "context7"
    And Claude Code receives one Rotta-managed MCP server named "context7"
    And the OpenCode Context7 server uses local command "npx", "-y", and "@upstash/context7-mcp" in that order
    And the OpenCode Context7 server is enabled
    And the Claude Code Context7 server uses command "npx" with args "-y" and "@upstash/context7-mcp" in that order
    And the Claude Code Context7 server uses stdio command-based MCP transport
    And unrelated host MCP servers and user settings are preserved

  @SCN-104 @REQ-002 @REQ-006
  Scenario: Host configuration failures are reported per host instead of as full success
    Given the user selected Context7
    And OpenCode Context7 configuration succeeds
    And Claude Code Context7 configuration fails
    When installation reports Context7 results
    Then the result identifies OpenCode Context7 configuration as successful
    And the result identifies Claude Code Context7 configuration as failed
    And the result does not claim Context7 was fully configured for both hosts

  @SCN-105 @REQ-003 @REQ-004
  Scenario: Missing command availability fails Context7 without blaming host configuration
    Given the user selected Context7
    And the host configuration files are writable
    And "npx" is not available to the installer
    When the installer checks Context7
    Then Context7 is reported as failed because the command is unavailable
    And the failure is distinguished from an OpenCode host config write failure
    And the failure is distinguished from a Claude Code host config write failure

  @SCN-106 @REQ-004
  Scenario: Context7 health passes only after MCP initialization and tool discovery
    Given the user selected Context7
    And both host Context7 configurations were written
    When the installer runs the Context7 health check
    Then the check uses the same command, args, and transport written to host config
    And the check must initialize the MCP server successfully
    And the check must discover Context7 documentation tools including "resolve-library-id" and "query-docs"
    And the TUI reports Context7 health as passing only after those checks succeed

  @SCN-107 @REQ-004
  Scenario Outline: Context7 health rejects false positives
    Given the user selected Context7
    And both host Context7 configurations were written
    When the Context7 health check observes <condition>
    Then the TUI reports Context7 health as failed or retryable, not successful
    And the TUI identifies the failure category as <category>

    Examples:
      | condition                                      | category             |
      | configuration text exists but MCP init fails    | MCP initialization   |
      | the server process starts and exits immediately | server startup       |
      | the server initializes but exposes no tools      | tool discovery       |
      | the server exposes only one expected tool        | tool discovery       |
      | the health check times out                       | timeout              |

  @SCN-108 @REQ-005
  Scenario: Explicitly deselecting Context7 leaves host config and generated instructions unchanged for Context7
    Given the installer shows Context7 selected by default
    And the user deselects Context7 before installation begins
    When installation completes
    Then OpenCode receives no Rotta-managed Context7 MCP server entry
    And Claude Code receives no Rotta-managed Context7 MCP server entry
    And the installer does not run Context7 command availability checks
    And the installer does not run Context7 MCP health checks
    And the install summary shows Context7 as skipped or not selected
    And Rotta-generated workflow instructions do not mention using Context7 for library docs, API references, code examples, setup help, or similar prompts

  @SCN-109 @REQ-005 @REQ-006
  Scenario: Context7 skip does not affect selected Ancora and Vela installs
    Given the user selects Ancora
    And the user selects Vela
    And the user deselects Context7 before installation begins
    When installation runs optional MCP setup
    Then Ancora setup and checks run according to their own selected behavior
    And Vela setup and checks run according to their own selected behavior
    And Context7 setup and checks are skipped

  @SCN-110 @REQ-002 @REQ-003 @REQ-004
  Scenario: Re-running selected Context7 normalizes duplicate host entries before health reporting
    Given the user selected Context7
    And a host already has an existing Rotta-managed Context7 MCP entry
    When installation configures Context7 again
    Then the host has one Rotta-managed Context7 MCP entry named "context7"
    And that entry uses command "npx" with args "-y" and "@upstash/context7-mcp"
    And Context7 success is not reported until the health check also passes
