@language_agnostic_quality_gates
Feature: Language-agnostic PR-readiness quality gates
  To provide reliable Phase 4 decisions across project ecosystems
  As a Rotta user
  I need generic gates to be discovered, evaluated, persisted, and handed off without guessed commands or hidden exceptions

  @REQ-073 @SCN-501
  Scenario: A newly generated quality-gates configuration defines only required generic categories
    Given a project receives a newly generated Rotta quality-gates configuration
    When the configuration is validated
    Then it uses the current rotta.quality-gates format
    And it contains exactly build, tests, changed-file scope, static analysis, dependency checks, and security checks
    And it contains no coverage, mutation, complexity, named-function, or language-profile gate

  @REQ-073 @SCN-502
  Scenario: An unsupported configuration is rejected with remediation
    Given the active quality-gates configuration uses an unsupported format
    When Phase 4 review is requested
    Then the review state is blocked
    And the result explicitly says that the format is unsupported and is not automatically migrated
    And the result identifies the configuration remediation path
    And no command from the unsupported configuration is executed

  @REQ-074 @SCN-503
  Scenario Outline: Declared project conventions resolve a reproducible generic gate plan
    Given the recorded review snapshot declares an unambiguous supported convention for <gate category>
    When Rotta resolves the Phase 4 review plan twice without changing metadata, configuration, baseline, or snapshot
    Then both plans contain the same resolved command for <gate category>
    And each plan records the metadata source and discovery rule for <gate category>
    And each plan is persisted with its plan and configuration fingerprints

    Examples:
      | gate category        |
      | build                |
      | tests                |
      | static analysis      |
      | dependency checks    |
      | security checks      |

  @REQ-074 @SCN-504
  Scenario: Changed-file scope is measured from trusted snapshots
    Given a current submission has a recorded baseline and committed review snapshot
    And an untrusted caller supplies a different changed-file list
    When Rotta resolves changed-file scope
    Then the result compares the recorded baseline to the recorded review snapshot
    And it persists those comparison inputs and the measured scope
    And it does not use the caller-supplied list or an arbitrary working-tree diff

  @REQ-074 @SCN-505
  Scenario: Missing required command discovery blocks review instead of guessing
    Given no supported project convention or metadata resolves the security-check command
    When Phase 4 review is requested
    Then the security-check gate is blocked with remediation to declare or configure a supported convention
    And the overall review state is blocked
    And Rotta does not invent, substitute, or silently pass a security command

  @REQ-074 @SCN-506
  Scenario: Ambiguous command candidates block review
    Given equally preferred supported conventions resolve conflicting commands for static analysis
    When Phase 4 review is requested
    Then the static-analysis gate is blocked with ambiguity remediation
    And neither conflicting command is selected or run
    And the overall review state is blocked

  @REQ-075 @REQ-077 @SCN-507
  Scenario: Successful generic review writes evidence and enters final human review
    Given a current submission has valid current TDD evidence and a committed snapshot
    And all six required generic gates resolve and pass against that snapshot
    When Phase 4 evaluation completes and persists its state
    Then .rotta/current/review-evidence.yaml records the baseline SHA, snapshot SHA, configuration fingerprint, plan fingerprint, ordered gate outcomes, commands, outputs, and measurements
    And the overall PR readiness state is ready
    And the snapshot is recorded as reviewed_commit
    And the submission enters final_human_review rather than complete

  @REQ-075 @SCN-508
  Scenario: Root and archived TDD logs cannot satisfy current review evidence
    Given the active current submission requires scenario evidence in .rotta/current/tdd-log.md
    And only .rotta/tdd-log.md or an archived TDD log contains the required scenario ID
    When Phase 4 evaluation validates TDD evidence
    Then the review reports missing current-submission evidence
    And it does not treat the root or archived log as active evidence
    And it does not report ready

  @REQ-075 @REQ-076 @SCN-509
  Scenario: A failed required gate produces not-ready evidence
    Given all required generic gate commands resolve for the recorded snapshot
    And the build gate exits unsuccessfully
    When Phase 4 evaluation completes
    Then review evidence records the build gate as failed with its exit result and remediation
    And the overall PR readiness state is not_ready
    And the submission does not enter final_human_review

  @REQ-076 @SCN-510
  Scenario: A valid waiver remains visible and produces ready with waivers
    Given a persisted review records a failed dependency-check gate for snapshot SHA "abc123" and configuration fingerprint "cfg-1"
    And a durable waiver names that gate, has a non-empty reason, scope, timestamp, snapshot SHA "abc123", and configuration fingerprint "cfg-1"
    When Rotta derives PR readiness for snapshot SHA "abc123" and configuration fingerprint "cfg-1"
    Then the dependency-check gate status is waived rather than passed
    And the evidence retains the underlying failed outcome and the waiver record separately
    And the overall PR readiness state is ready_with_waivers
    And no reviewer identity is recorded

  @REQ-076 @SCN-511
  Scenario Outline: An invalid waiver cannot authorize readiness
    Given a review has a non-passed tests gate for the recorded snapshot and configuration
    And its waiver is <invalid condition>
    When Rotta derives PR readiness
    Then the waiver is rejected with remediation
    And the tests gate is not passed
    And the overall PR readiness state is blocked

    Examples:
      | invalid condition                                      |
      | expired                                                |
      | bound to a different snapshot SHA                      |
      | bound to a different configuration fingerprint         |
      | missing a non-empty reason                             |
      | naming a gate that is not in the persisted review plan |

  @REQ-077 @SCN-512
  Scenario: A changed reviewed snapshot invalidates final-review eligibility
    Given a submission is in final_human_review for reviewed_commit "abc123"
    And its persisted evidence is ready for that same snapshot and configuration
    When the approved implementation snapshot changes to "def456"
    Then final-review eligibility for "abc123" is invalidated
    And explicit human approval cannot complete the submission
    And a new Phase 4 review is required for "def456"

  @REQ-078 @SCN-513
  Scenario: PR handoff is refused when caller input contradicts persisted review evidence
    Given persisted review evidence for the current submission is blocked
    And a caller supplies a ready status, reviewed commit, branch, and reviewed paths
    When manual GitHub PR handoff is requested
    Then no push or pull-request command is generated
    And the output reports the persisted blocked metrics and remediation
    And the caller-supplied readiness assertions do not change the decision

  @REQ-078 @SCN-514
  Scenario: Eligible PR handoff is derived from matching ready evidence
    Given persisted review evidence is ready_with_waivers for the current reviewed_commit and configuration fingerprint
    And the current approved snapshot still matches reviewed_commit
    And exactly one GitHub-capable push remote is resolvable from the recorded worktree
    When manual GitHub PR handoff is requested
    Then the output identifies ready_with_waivers and the waived gate IDs and reasons
    And it prints the recorded absolute worktree path and git status inspection command
    And it derives manual push and pull-request commands from trusted recorded state and repository inspection
    And it does not execute publication commands

  @REQ-079 @SCN-515
  Scenario: The TUI selects threshold defaults without selecting language behavior
    Given a user opens the installer quality-gates screen
    When the user selects a generic threshold-default option
    Then the confirmation identifies .rotta/quality-gates.yaml as the generated configuration path
    And the generated configuration reflects only the selected threshold policy values
    And the screen describes generic command detection and blocked remediation
    And the screen displays no language, profile, coverage, mutation, complexity, or named-function choice

  @REQ-079 @SCN-517
  Scenario: The TUI presents generic detection and blocked metrics without executing review
    Given the selected project has detected build and tests conventions but no security-check convention
    When the installer quality-gates screen presents its detection preview
    Then it lists the resolved build and tests commands with their metadata sources
    And it lists security checks as blocked with remediation and includes it in blocked metrics
    And it identifies .rotta/quality-gates.yaml as the generated configuration path
    And it does not run Phase 4 commands or create review evidence

  @REQ-080 @SCN-516
  Scenario: Generated review guidance preserves the executable generic-gate contract
    Given Rotta installs or updates its supported host artifacts
    When the generated review guidance is inspected
    Then it directs Phase 4 to use generic-gate discovery and persisted current review evidence
    And it states that unresolved commands block review rather than pass
    And it preserves waiver, final-human-review, and evidence-derived PR-handoff semantics
    And it does not reintroduce obsolete configuration formats, root TDD-log authority, Go-specific commands, or language-specific quality gates
