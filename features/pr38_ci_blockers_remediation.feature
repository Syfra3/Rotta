@pr38-ci-remediation
Feature: PR #38 CI blocker remediation
  To make recovery failures actionable and workflow tests portable
  Rotta must preserve both backup locations on combined failure and use only local test-repository Git identity.

  @SCN-397 @REQ-070
  Scenario: A selected restore and its rollback both fail
    Given a selected backup is being restored
    And a pre-restore safety backup has been created
    And applying the selected backup fails
    And restoring the pre-restore safety backup also fails
    When the restore operation reports its error
    Then the error identifies the selected backup location
    And the error identifies the pre-restore safety backup location
    And the error reports that rollback failed
    And the error does not claim that rollback succeeded

  @SCN-398 @REQ-070
  Scenario: The stranded rollback behavior is restored without its temporary source file
    Given the rollback-failure behavior is restored to the tracked installer source
    When the remediation is inspected
    Then no file named "internal/installer/backup.go.tmp" remains
    And a restore failure with a successful rollback remains distinguishable from a rollback failure

  @SCN-399 @REQ-071
  Scenario: A workflow test commit succeeds without host Git identity
    Given a workflow test creates a temporary Git repository that needs a commit
    And host, global, and system Git identity are unavailable
    When the test prepares its temporary repository through its test helper
    Then the repository has a local test-only Git identity
    And the test can create its required commit
    And no global Git identity is written or required

  @SCN-400 @REQ-071
  Scenario: An intentional local identity removal remains observable
    Given a workflow test deliberately removes its temporary repository's local Git identity
    When it performs the operation that requires Git identity
    Then it observes the expected missing-identity behavior
    And no helper silently restores local identity for that deliberate case

  @SCN-401 @REQ-072
  Scenario: The remediation passes existing verification and awaits human review
    Given the remediation is applied without changing CI, Make targets, quality-gate policy, or active unified-workflow-authority artifacts
    When the repository runs "go test ./..." and "make verify"
    Then both commands pass
    And the implementation changes remain uncommitted for human review
