Feature: Native Claude Code Rotta orchestration
  Rotta users need Claude Code to provide the same guided, contract-driven workflow as OpenCode, while using Claude-native global agents and delegation.

  @SCN-301 @REQ-015
  Scenario: Install the global Claude orchestration surface and phase agents
    Given a user selects global Claude Code installation with every Rotta phase enabled
    When Rotta installs the Claude Code integration
    Then a normal feature request can discover the Rotta orchestration surface
    And global Claude Code agent definitions named "rotta-spec", "rotta-impl", and "rotta-review" are available for automatic delegation
    And the phase agents are not presented as the normal workflow entrypoints
    And each installed agent inherits its invoking model by default

  @SCN-302 @REQ-015
  Scenario: Handle a simple low-risk request directly
    Given the Rotta orchestration surface receives a simple, well-scoped, low-risk request
    When it assesses the workflow scope
    Then it guides the user through focused impact assessment and verification
    And it does not launch a Rotta phase agent
    And it does not require a spec, Gherkin approval, TDD phase, or review phase

  @SCN-303 @REQ-015
  Scenario: Guide a complex request through delegated specification and approval
    Given the Rotta orchestration surface receives a complex or high-risk request
    When it assesses the workflow scope
    Then it guides the user through the Draft phase
    And it delegates the specification and Gherkin work to "rotta-spec"
    And it presents the resulting contract for explicit user approval
    And it does not delegate implementation while questions remain open or approval is absent

  @SCN-304 @REQ-015
  Scenario: Delegate one approved scenario at a time
    Given the user has explicitly approved a Gherkin contract with multiple scenarios
    And the recorded implementation worktree is clean
    When the Rotta orchestration surface begins the TDD phase
    Then it delegates exactly one approved scenario to "rotta-impl"
    And it verifies the returned scenario result and worktree boundary before another implementation delegation
    And it does not delegate the next scenario until the required checkpoint or cleanup is complete

  @SCN-305 @REQ-015
  Scenario: Delegate evidence-based review after approved implementation
    Given every approved implementation scenario has completed with the required TDD evidence
    When the Rotta orchestration surface begins the Review phase
    Then it delegates the review to "rotta-review"
    And it presents the review result and any required remediation to the user
    And it does not treat an objective gate result as automatic human approval

  @SCN-306 @REQ-015
  Scenario: Preserve phase order when a user requests a phase agent directly
    Given the workspace workflow state requires specification before implementation
    When the user asks to run "rotta-impl" directly
    Then the Rotta orchestration surface explains the required earlier phase
    And it does not bypass the specification or approval gate

  @SCN-307 @REQ-015
  Scenario: Enforce role boundaries for delegated agents
    Given Rotta has installed the global Claude Code phase agents
    When each phase agent is invoked through the workflow
    Then "rotta-spec" cannot implement production or test code or recursively delegate work
    And "rotta-impl" can edit the project and run project test commands but cannot recursively delegate work
    And "rotta-review" produces review evidence without editing production code or recursively delegating work

  @SCN-308 @REQ-015
  Scenario: Preserve workflow gates when an MCP is degraded
    Given an orchestrator or delegated phase agent needs an installed Rotta MCP
    And that MCP is unavailable or degraded
    When the workflow reaches the affected step
    Then Rotta reports the affected MCP, active fallback, limitation, and safe recovery action
    And it continues from authoritative workspace artifacts when the defined fallback permits it
    And it does not bypass an approval gate, clean-worktree requirement, or quality gate

  @SCN-309 @REQ-015
  Scenario: Reinstall only Rotta-owned Claude artifacts
    Given a user has unrelated global Claude agents, skills, settings, hooks, and MCP entries
    And a prior Rotta installation has legacy phase skills or malformed Rotta-owned agent files
    When the user reruns global Claude Code installation
    Then Rotta updates or replaces only its owned orchestration and phase-agent artifacts
    And the unrelated Claude configuration remains available and unchanged
    And the installed Rotta orchestration surface and named phase agents are not duplicated

  @SCN-310 @REQ-015
  Scenario: Verify the current stable Claude Code release before claiming compatibility
    Given the Rotta compatibility pipeline is validating Claude Code integration
    When the latest stable Claude Code release is available
    Then the pipeline runs the Claude integration verification against that release
    And it records the tested "claude --version"
    And it reports Claude compatibility as verified only when that verification passes

  @SCN-311 @REQ-015
  Scenario: Refuse an unverified Claude compatibility claim
    Given the Rotta compatibility pipeline cannot run or identify the installed Claude Code release
    When it evaluates the Claude Code integration
    Then it reports Claude compatibility as unverified or failed
    And it does not claim that the current stable Claude Code release is supported
