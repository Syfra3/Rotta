Feature: Full-workflow feature worktree lifecycle
  Rotta users need every full workflow to create its isolated feature submission before specification, preserve an approved committed contract baseline, and run validated scenarios automatically without risking their initiating checkout or publication authority.

  @REQ-045 @SCN-312
  Scenario: Prepare the isolated feature worktree before specification writes
    Given a full Rotta workflow starts from a clean initiating checkout
    And the selected base branch and valid feature slug resolve successfully
    When Rotta begins the specification phase
    Then Rotta records and reports one isolated "feature/<slug>" worktree and its base branch
    And the hard spec and Gherkin contract are written only in that recorded worktree
    And the initiating checkout remains unmodified by the submission

  @REQ-045 @REQ-048 @SCN-313
  Scenario Outline: Stop before specification when isolation is unsafe
    Given a full Rotta workflow has the unsafe condition "<condition>"
    When Rotta prepares the feature worktree
    Then Rotta reports the failed validation and a non-destructive recovery action
    And Rotta does not write a submission artifact in the initiating checkout
    And Rotta does not start specification or implementation

    Examples:
      | condition |
      | the initiating checkout has a non-ignored change |
      | the feature branch or worktree path cannot be created exclusively |
      | the recorded worktree is detached or is not on its feature branch |

  @REQ-046 @REQ-051 @SCN-314
  Scenario: Checkpoint an explicitly approved feature contract
    Given the recorded feature worktree contains a hard spec and Gherkin contract awaiting approval
    When a human explicitly approves its listed scenarios
    Then Rotta creates a feature-scoped durable approval record with the scenario scope and contract fingerprints
    And Rotta creates one local baseline checkpoint containing the approved contract and approval record
    And current workflow state records the baseline checkpoint and approval-record identity
    And Rotta does not use or modify "specs/.approved" as approval authority

  @REQ-046 @REQ-048 @SCN-315
  Scenario Outline: Refuse implementation without a matching approved baseline
    Given the recorded feature workflow has "<approval-state>"
    When Rotta attempts to begin Phase 3
    Then Rotta reports that implementation is blocked and states the recovery action
    And Rotta does not delegate an implementation scenario or create a scenario checkpoint

    Examples:
      | approval-state |
      | no explicit feature-scoped approval record |
      | an approval record that excludes the next scenario |
      | a contract changed after its approved baseline checkpoint |
      | an approval baseline that cannot be committed |

  @REQ-047 @SCN-316
  Scenario: Run exactly one approved scenario through its required evidence and gate boundary
    Given a matching approved baseline identifies the next scenario in the recorded feature worktree
    When Rotta begins autonomous Phase 3
    Then Rotta delegates only that next approved scenario
    And it requires the scenario's Red, Green, Refactor, traceable-test, required-test, and active-gate evidence
    And it verifies the recorded feature-worktree identity before checkpointing

  @REQ-047 @SCN-317
  Scenario: Automatically checkpoint and advance from a clean successful scenario boundary
    Given the current approved scenario has passed its required TDD evidence and gates
    And its non-ignored changes are expected for that scenario
    When Rotta completes the scenario boundary
    Then Rotta creates exactly one local scenario checkpoint on the recorded feature branch
    And current state records its evidence, checkpoint, remaining scenarios, and next scenario
    And Rotta verifies that the non-ignored recorded worktree is clean
    And Rotta automatically begins the next approved scenario without asking whether to continue or commit

  @REQ-047 @SCN-318
  Scenario: Send the final clean checkpoint to review without publication
    Given the final approved scenario has a successful checkpoint and clean recorded worktree boundary
    When Rotta completes Phase 3
    Then Rotta enters Phase 4 review
    And it does not ask for a continuation decision
    And it does not push, create a pull request, merge, rebase, reset, or tag

  @REQ-048 @SCN-319
  Scenario Outline: Halt autonomously without discarding evidence or user changes
    Given autonomous Phase 3 encounters "<failure>" in the recorded worktree
    When Rotta evaluates the scenario boundary
    Then Rotta reports the failure, affected context, and safe recovery action
    And Rotta does not checkpoint the scenario or start the next scenario
    And Rotta does not stash, discard, revert, reset, delete, automatically add, or commit ambiguous changes

    Examples:
      | failure |
      | a required test or objective gate failure |
      | an unexpected tracked or untracked non-ignored change |
      | contract fingerprint drift after approval |
      | feature-worktree or branch identity failure |

  @REQ-049 @SCN-320
  Scenario: Archive terminal state while retaining the reviewable feature worktree
    Given Phase 4 has reached a terminal result for the recorded feature workflow
    When Rotta completes terminal lifecycle handling
    Then Rotta archives the active ".rotta/current" execution state
    And it retains the recorded feature worktree and branch for manual inspection, push, or pull-request handoff
    And it retains the committed contract and feature-scoped approval record

  @REQ-049 @SCN-321
  Scenario Outline: Remove a feature worktree only through eligible explicit cleanup
    Given a terminal feature workflow is "<terminal-status>"
    When a human explicitly requests cleanup
    Then Rotta removes only the recorded feature worktree after safe identity and cleanliness validation
    And it does not remove the initiating checkout or durable contract artifacts

    Examples:
      | terminal-status |
      | published by the user |
      | explicitly abandoned |
      | explicitly cancelled |

  @REQ-049 @SCN-322
  Scenario: Refuse premature or unsafe feature-worktree cleanup
    Given the recorded feature workflow has passed review but is not published, abandoned, or cancelled
    When cleanup is requested
    Then Rotta preserves the feature worktree and branch
    And it reports that publication confirmation or explicit abandonment is required
    And it does not remove a worktree with unexpected non-ignored changes

  @REQ-050 @REQ-051 @SCN-323
  Scenario: Generate the same lifecycle authority rules for every supported host
    Given Rotta generates orchestration assets for OpenCode, Claude Code, and Codex
    When a full workflow is started or resumed on any supported host
    Then the host applies the recorded pre-spec feature-worktree, approved-baseline, autonomous-checkpoint, safety-stop, archive, and cleanup lifecycle
    And it treats the feature-scoped approval record as authoritative instead of "specs/.approved"
    And Claude native orchestration does not create a second worktree after approval
