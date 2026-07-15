Feature: Reset and reinstall global OpenCode
  Users can reset global OpenCode state and reinstall it without changing
  project-local OpenCode configuration.

  @SCN-001 @REQ-001
  Scenario: The reset target is advertised and runnable
    Given the Rotta Makefile is available
    When the user requests Makefile help
    Then the help output lists reset-opencode and warns that it removes global OpenCode state before reinstalling OpenCode
    When the user runs make reset-opencode
    Then the global OpenCode reset-and-reinstall workflow starts

  @SCN-002 @REQ-002 @REQ-005
  Scenario: Reset default global OpenCode locations and reinstall without confirmation
    Given XDG_CONFIG_HOME, XDG_DATA_HOME, and XDG_CACHE_HOME are unset
    And global OpenCode configuration, data including credentials, and cache exist in their default locations
    When the user runs make reset-opencode non-interactively
    Then only the default global OpenCode configuration, data, and cache locations are removed
    And the official OpenCode Linux installer runs without asking for confirmation

  @SCN-003 @REQ-002 @REQ-005
  Scenario: Missing default OpenCode locations do not block reinstall
    Given XDG_CONFIG_HOME, XDG_DATA_HOME, and XDG_CACHE_HOME are unset
    And one or more default global OpenCode locations do not exist
    When the user runs make reset-opencode
    Then the absent locations are treated as already clean
    And the official OpenCode Linux installer runs

  @SCN-004 @REQ-003 @REQ-005
  Scenario: Reset custom XDG OpenCode locations without removing their roots
    Given XDG_CONFIG_HOME, XDG_DATA_HOME, and XDG_CACHE_HOME name custom absolute locations
    And each custom XDG location contains an OpenCode child and unrelated application content
    When the user runs make reset-opencode
    Then only the OpenCode child in each configured XDG location is removed
    And unrelated content in each configured XDG location remains
    And the official OpenCode Linux installer runs

  @SCN-005 @REQ-003 @REQ-005
  Scenario: Reset an additional configured OpenCode configuration directory
    Given OPENCODE_CONFIG_DIR names a distinct OpenCode-only global configuration directory
    And the XDG-derived OpenCode configuration location also exists
    When the user runs make reset-opencode
    Then the configured OpenCode configuration directory is removed
    And the XDG-derived OpenCode configuration location is removed
    And the official OpenCode Linux installer runs

  @SCN-006 @REQ-003 @REQ-005
  Scenario: Reject an unsafe custom path before deletion or installation
    Given an OpenCode custom-location environment variable is empty, relative, the filesystem root, the user's home directory, or otherwise not a distinct OpenCode-only target
    When the user runs make reset-opencode
    Then the command fails with an unsafe-path error
    And no unsafe path is removed
    And the official OpenCode Linux installer does not run

  @SCN-007 @REQ-003 @REQ-005
  Scenario: Duplicate or absent custom OpenCode paths are safe
    Given configured OpenCode paths overlap with each other or with a default OpenCode path
    And one or more configured OpenCode paths do not exist
    When the user runs make reset-opencode
    Then each existing OpenCode path is removed at most once
    And absent OpenCode paths do not cause failure
    And the official OpenCode Linux installer runs

  @SCN-008 @REQ-004 @REQ-005
  Scenario: Preserve project-local OpenCode configuration
    Given the current project contains opencode.json and a .opencode directory
    And global OpenCode configuration, data, or cache exists
    When the user runs make reset-opencode
    Then the project's opencode.json remains unchanged
    And the project's .opencode directory remains unchanged
    And the official OpenCode Linux installer runs after global cleanup

  @SCN-009 @REQ-005
  Scenario: Do not install after cleanup failure
    Given a required global OpenCode cleanup operation fails
    When the user runs make reset-opencode
    Then the command exits unsuccessfully
    And the official OpenCode Linux installer does not run

  @SCN-010 @REQ-005
  Scenario: Report official installer failure after successful cleanup
    Given all required global OpenCode cleanup operations complete successfully
    And the official OpenCode Linux installer cannot complete
    When the user runs make reset-opencode
    Then the command exits unsuccessfully
    And the installer failure is reported
