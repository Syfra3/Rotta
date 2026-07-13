Feature: Scoped Rotta workflow lifecycle
  Rotta contributors need each active submission to have isolated, resumable workflow state so that unrelated contracts and historical execution evidence cannot alter implementation or review decisions.

  @SCN-234 @REQ-032
  Scenario: Create an isolated active submission
    Given a contributor begins a new approved Rotta submission in a Git worktree
    When Rotta initializes its execution lifecycle
    Then Rotta creates a current submission manifest that identifies the submission, contract files, scenario IDs, worktree, and lifecycle status
    And Rotta creates current state that identifies the phase, completed work, remaining work, last action, and safe resume point
    And Rotta does not use a legacy global approval marker or TDD log to determine the submission scope

  @SCN-235 @REQ-032 @REQ-034
  Scenario: Reject malformed or missing active submission state
    Given the active submission manifest is missing, malformed, or references a missing feature file
    When Rotta attempts to continue or review the submission
    Then Rotta reports that the current submission state cannot be safely used
    And Rotta does not infer scenario scope from unrelated global or archived workflow artifacts

  @SCN-236 @REQ-033 @REQ-036
  Scenario: Resume an interrupted submission from local state when memory is unavailable
    Given an in-progress submission has current local state, TDD evidence, and contract file references
    And Ancora is unavailable or has a stale pointer
    When a contributor resumes the submission in its worktree
    Then Rotta identifies completed, remaining, and blocked work from the current local state
    And Rotta continues without reconstructing contract content from Ancora
    And Rotta reports or repairs a stale Ancora pointer when memory is available again

  @SCN-237 @REQ-034
  Scenario: Review only scenarios declared by the current submission
    Given the current submission manifest declares a bounded set of approved scenarios
    And legacy global markers or archived submissions name additional scenarios
    When Rotta runs its Judge review
    Then the Judge evaluates evidence only for the scenarios declared by the current submission manifest
    And missing evidence for unrelated historical scenarios does not fail the review
    And legacy artifacts may be reported as warnings but do not expand review scope

  @SCN-238 @REQ-035
  Scenario: Archive a completed submission without removing durable contracts
    Given a submission has reached a terminal completed, abandoned, or cancelled state
    And its feature changes have been safely committed
    When Rotta completes lifecycle cleanup
    Then Rotta moves the current execution state to a local archive for that submission
    And Rotta removes the archived submission from active review scope
    And Rotta retains the hard spec and Gherkin feature files as repository contracts

  @SCN-239 @REQ-035
  Scenario: Retain and manually clean archived execution state
    Given a terminal submission has local archived execution state
    When the archive is less than 30 days old
    Then Rotta retains the archive without treating it as active submission scope
    When a contributor explicitly requests archive cleanup
    Then Rotta removes the requested archive without removing durable contract files

  @SCN-240 @REQ-036
  Scenario: Save only a compact lifecycle pointer to Ancora
    Given a current submission has local contract and execution artifacts
    When Rotta records lifecycle state in Ancora
    Then the memory record identifies the submission, phase, status, scenario progress, last action, local state pointer, and evidence references
    And the memory record does not become the sole source of a hard spec, feature contract, TDD log, or Judge report
