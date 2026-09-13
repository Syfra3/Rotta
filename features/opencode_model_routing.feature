Feature: Installer-managed OpenCode model routing

  Background:
    Given the OpenCode global configuration target is resolved from XDG_CONFIG_HOME
    And the repository-root opencode.json is user-owned
    And OPENCODE_CONFIG, when set, names a user-owned configuration file

  @SCN-001
  Scenario: Omitted installer routing request defaults to enabled
    Given no OpenCode model-routing option was supplied
    When the installer is confirmed
    Then only the XDG-global OpenCode configuration may be mutated
    And the seven role model fields equal the immutable routing map
    And the repository-root opencode.json and OPENCODE_CONFIG file are byte-identical
    And field-level ownership is recorded for exactly those model fields

  @SCN-002
  Scenario: CLI explicitly disables routing
    Given all seven routing model fields match recorded Rotta field ownership
    When the user selects OpenCode model routing "disabled" through the CLI and confirms
    Then only those seven owned model fields are removed
    And each role's remaining agent fields, asset, permission, and object remain present
    And the corresponding model-field ownership records are removed
    And project and OPENCODE_CONFIG files are byte-identical

  @SCN-003
  Scenario: TUI selection is keyboard accessible and confirmation-gated
    Given the installer TUI is on the OpenCode model-routing selection
    When the user navigates by keyboard to "disabled"
    And confirms that selection by keyboard on the confirmation screen
    Then the request submitted to the adapter is explicitly disabled
    And no mutation occurs before confirmation
    But when the user cancels or declines confirmation
    Then no configuration, asset, backup, or ownership record is changed

  @SCN-004
  Scenario: User-owned configuration is preserved and conflicts refuse safely
    Given a mapped role has a non-owned model field or an ownership record that cannot prove a matching Rotta value
    When enable or disable would touch that role's model field
    Then the installer refuses before mutation with the target, role, and field identified
    And the XDG-global configuration and managed state are unchanged
    And the repository-root opencode.json and OPENCODE_CONFIG file are byte-identical

  @SCN-005
  Scenario: Disable honors field-level ownership only
    Given a mapped agent has a Rotta-owned matching model field and user-owned non-model fields
    And another mapped agent has a user-owned or diverged model field
    When a disabled routing request is confirmed
    Then the matching owned model field is removed without deleting its agent
    And the user-owned non-model fields are unchanged
    And the operation refuses rather than deleting or changing the user-owned or diverged model field

  @SCN-006
  Scenario: Custom-XDG transaction failure restores exact state
    Given XDG_CONFIG_HOME points to a custom directory containing the global OpenCode configuration
    And a backup of that exact target is created before the first routing mutation
    When an injected failure occurs after a routing write or managed-state update
    Then restoration uses the custom-XDG backup path
    And the global configuration and managed state equal their pre-request contents
    And no partial routing ownership or stale backup record remains
