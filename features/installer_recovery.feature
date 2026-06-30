Feature: Safe installer backup, clean install, and full restore
  Users need every install to protect their current AI-agent configuration before rotta removes previous settings and installs a fresh copy.

  Background:
    Given a user has a rotta project path
    And the backup root is "~/.rotta/backups"

  @REQ-001 @REQ-002 @SCN-001
  Scenario: Install creates a timestamped backup before any mutation
    Given the user has existing opencode, Claude Code, and project rotta configuration
    When the user starts a normal rotta install
    Then a timestamped backup is created under "~/.rotta/backups"
    And the backup manifest records the project path, target, selected modes, optional integrations, backed-up paths, and missing paths
    And no install or cleanup mutation occurs before the backup succeeds

  @REQ-004 @REQ-010 @SCN-002
  Scenario: Successful install cleans previous rotta settings before fresh install
    Given a valid backup has been created for the install
    And a previous rotta installation exists for the selected target
    When the install continues
    Then previous rotta-owned skills, agent entries, permissions, generated config, and selected integration settings are removed or normalized
    And unrelated user settings are preserved
    And the fresh rotta installation is written according to the selected target and modes

  @REQ-003 @SCN-003
  Scenario: Backup failure aborts install completely
    Given the user starts a normal rotta install
    When creating the backup fails
    Then the install fails before cleanup begins
    And no fresh rotta files or settings are written
    And any partial backup is removed or marked unusable
    And the user sees that backup failure prevented installation

  @REQ-006 @SCN-004
  Scenario: TUI lists available backups from recovery
    Given multiple valid backups exist
    When the user opens recovery in the TUI
    Then the TUI lists the available backups
    And each listed backup is distinguishable by timestamp and project metadata

  @REQ-006 @REQ-009 @SCN-005
  Scenario: TUI previews backup contents and metadata
    Given the user is viewing the backup list in recovery
    When the user selects a backup to preview
    Then the TUI shows the backup timestamp, project path, target, selected modes, optional integrations, backed-up paths, and missing paths
    And the preview is derived from the backup manifest
    And the preview states that restore is full-backup restore only

  @REQ-006 @REQ-007 @SCN-006
  Scenario: TUI requires confirmation before full restore
    Given the user is previewing a valid backup
    When the user chooses restore
    Then the TUI asks for explicit confirmation
    And restore does not begin until the user confirms

  @REQ-007 @SCN-007
  Scenario: Restore applies the full backup and removes paths that were absent
    Given the user confirmed restore of a valid backup
    When the restore runs
    Then the current in-scope configuration is protected in a pre-restore safety backup
    And every backed-up file and directory is restored to its original destination
    And in-scope destinations recorded as missing in the selected backup are absent after restore
    And restore reports success only after all destination changes complete

  @REQ-008 @SCN-008
  Scenario: Failed restore rolls back to pre-restore state
    Given the user confirmed restore of a valid backup
    And the pre-restore safety backup was created successfully
    When restore fails after changing a destination path
    Then the system attempts to restore the pre-restore safety backup automatically
    And restore is reported as failed
    And the user is told whether rollback to the pre-restore state succeeded

  @REQ-008 @SCN-009
  Scenario: Restore failure with rollback failure provides manual recovery locations
    Given the user confirmed restore of a valid backup
    And the pre-restore safety backup was created successfully
    When restore fails and rollback also fails
    Then the user sees the selected backup location
    And the user sees the pre-restore safety backup location
    And restore is not reported as successful

  @REQ-005 @REQ-010 @SCN-010
  Scenario: CLI install path cannot skip backup during normal usage
    Given the user starts a non-interactive rotta install command
    When the command performs a normal install
    Then it creates a backup before cleanup or installation
    And there is no normal install option that skips backup
    And existing version command behavior remains available

  @REQ-009 @SCN-011
  Scenario: Generated acceptance artifacts and user-facing text avoid external-reference wording
    Given a behavioral precedent was supplied only as private context
    When spec, feature, CLI, TUI, docs, comments, summaries, or commit text are generated for installer recovery
    Then they use neutral installer recovery wording
    And they do not mention or identify the external behavioral precedent
