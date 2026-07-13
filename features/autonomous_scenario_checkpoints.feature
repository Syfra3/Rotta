Feature: Autonomous Phase 3 scenario checkpoints
  Rotta users need validated TDD scenarios to checkpoint locally and progress automatically so the approved workflow retains clean, reviewable scenario boundaries without repeated human commit prompts.

  Background:
    Given the approved workspace contract identifies the eligible scenarios and their order
    And workspace files are the source of truth for the approved contract and workflow state

  @REQ-021 @SCN-025
  Scenario: Start autonomous Phase 3 in a new clean isolated feature worktree
    Given no autonomous Phase 3 worktree has been created for the approved contract
    When Rotta prepares the first approved scenario for implementation
    Then Rotta creates a new dedicated feature branch and an isolated worktree
    And the worktree contains the approved hard spec and Gherkin contract
    And the worktree has no non-ignored changes before the scenario begins
    And Rotta reports the selected branch and worktree

  @REQ-022 @SCN-026
  Scenario: Refuse autonomous execution without scoped human approval
    Given the next scenario is not explicitly approved for the current contract scope
    When Rotta is asked to start the autonomous scenario loop
    Then Rotta reports that explicit human Gherkin approval is required
    And Rotta does not launch a scenario agent
    And Rotta does not create a scenario commit

  @REQ-022 @REQ-023 @SCN-027
  Scenario: Checkpoint one approved scenario after strict TDD and objective validation pass
    Given an approved scenario has completed its Red, Green, and Refactor cycle
    And its required tests and active objective validation pass
    And all non-ignored changes are expected for that scenario
    When Rotta checkpoints the scenario
    Then Rotta creates exactly one local commit containing only that scenario's expected changes
    And the commit has no AI-generated, generated-by, or co-author attribution
    And Rotta records the scenario and local commit identifier in workflow state

  @REQ-024 @SCN-028
  Scenario: Halt for an unexpected tracked change before checkpointing
    Given an approved scenario has passed its required validation
    And the scenario agent changed an expected path
    And a non-ignored tracked path outside the scenario's expected change set is also changed
    When Rotta evaluates the scenario worktree for checkpointing
    Then Rotta identifies the unexpected path and halts
    And Rotta does not create a scenario commit
    And Rotta does not stash, discard, revert, delete, add, or commit the unexpected change
    And Rotta does not begin the next scenario

  @REQ-024 @SCN-029
  Scenario: Halt for an untracked non-ignored file before checkpointing
    Given an approved scenario has passed its required validation
    And an untracked non-ignored file exists in the scenario worktree
    When Rotta evaluates the scenario worktree for checkpointing
    Then Rotta identifies the untracked file and halts
    And Rotta does not create a scenario commit
    And Rotta does not begin the next scenario

  @REQ-023 @REQ-025 @SCN-030
  Scenario: Do not advance when validation or local commit creation fails
    Given an approved scenario has not passed a required test or active objective validation
    When Rotta evaluates the scenario for checkpointing
    Then Rotta does not create a local scenario commit
    And Rotta does not mark the scenario checkpointed
    And Rotta does not begin the next scenario
    And Rotta reports the failed validation or local commit failure when one occurs

  @REQ-025 @SCN-031
  Scenario: Continue automatically only from a clean successful checkpoint
    Given a local commit was successfully created for the current approved scenario
    And only explicitly ignored local runtime artifacts remain outside the commit
    And another approved scenario remains
    When Rotta completes the checkpoint boundary check
    Then the isolated worktree has no non-ignored changes
    And workflow state records the completed scenario, remaining scenarios, and next scenario
    And Rotta automatically begins the next approved scenario

  @REQ-025 @REQ-027 @SCN-032
  Scenario: Send the final checkpointed scenario to review without publishing
    Given the final approved scenario has a successful local checkpoint
    And no approved scenarios remain
    When Rotta completes its Phase 3 boundary check
    Then Rotta advances the workflow to the existing Phase 4 review gate
    And Rotta does not declare final human approval
    And Rotta does not push, create a remote branch, publish a tag, create a pull request, or merge

  @REQ-026 @SCN-033
  Scenario: Checkpoint an expected sensitive-scope scenario after ordinary validation passes
    Given an approved scenario's expected changes include a security, auth, payments, infra, migration, or secrets path
    And the scenario completes strict TDD, required tests, active objective validation, expected-diff validation, and clean-boundary checks
    When Rotta checkpoints the scenario
    Then Rotta does not halt solely because the expected path is sensitive in scope
    And Rotta creates the one local scenario commit
    And Rotta preserves the existing Phase 4 review gate

  @REQ-027 @SCN-034
  Scenario: Require a human to push once after review completes
    Given all approved scenarios have been checkpointed locally
    And the workflow and Phase 4 review are complete
    When Rotta reports the final workflow state
    Then Rotta states that a human may manually push the feature branch once
    And Rotta does not perform the push itself
