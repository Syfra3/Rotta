@workflow-authority
Feature: Unified Rotta workflow authority and lifecycle
  To keep workflow progress reviewable and safe across hosts
  Rotta must use one committed feature-scoped authority and an orchestrator-owned lifecycle.

  @SCN-324 @REQ-001
  Scenario: A valid feature approval record authorizes its approved scenarios
    Given a feature-scoped approval record has a committed baseline containing the matching approved contract
    And the record identifies the feature, approved scenarios, contract fingerprints, baseline commit, and lifecycle status
    When the orchestrator prepares to enter or resume implementation or review for that feature
    Then it authorizes only the scenarios named in that record

  @SCN-325 @REQ-001
  Scenario Outline: An invalid approval record fails closed with its specific reason
    Given the feature approval record is <invalid_condition>
    When the orchestrator prepares to enter implementation, review, or completion
    Then it does not enter that activity
    And it reports <reported_condition>

    Examples:
      | invalid_condition | reported_condition |
      | missing | that the approval record is missing |
      | malformed | that the approval record is malformed |
      | uncommitted | that the baseline is not committed |
      | unreachable from the recorded worktree history | that the baseline is unreachable |
      | inconsistent with the recorded feature identity or scenario scope | the identity or scenario-scope mismatch |
      | inconsistent with the approved contract fingerprints | contract drift |

  @SCN-326 @REQ-001
  Scenario: Approval authority remains isolated between feature worktrees
    Given two feature worktrees have different valid approval records with overlapping scenario IDs
    When either worktree is prepared for workflow activity
    Then only its own feature-scoped record is considered
    And the other worktree's record does not authorize activity

  @SCN-327 @REQ-001
  Scenario: A partial human approval limits eligible scenarios
    Given a valid record approves only a subset of the feature contract's scenarios
    When the orchestrator selects work for the feature
    Then it selects only scenarios in the approved subset

  @SCN-328 @REQ-002
  Scenario: Legacy markers do not authorize a workflow
    Given no valid feature-scoped approval record exists
    And a legacy approval or workflow-state artifact is present
    When the workflow is resumed
    Then legacy artifacts have no effect on approval, phase selection, review, or completion
    And the workflow routes to the normal fresh Phase 1 and Phase 2 flow

  @SCN-329 @REQ-002
  Scenario: Installation does not recreate retired workflow authority
    Given a repository has no valid feature-scoped approval record
    When Rotta artifacts are installed or generated
    Then no retired approval marker or legacy workflow-state artifact is created

  @SCN-330 @REQ-003
  Scenario: Only the orchestrator persists lifecycle decisions
    Given a workflow needs an approval, phase transition, scenario acceptance, checkpoint, or lifecycle archive
    When that lifecycle decision is persisted
    Then it is created or changed by the orchestrator
    And phase-role output alone does not become lifecycle authority

  @SCN-331 @REQ-003
  Scenario: Spec work produces only its contract artifacts
    Given Spec Mode is delegated a specification task
    When it finishes its delegated work
    Then it produces only the hard spec and Gherkin contract artifacts assigned to it
    And it does not create an approval record, baseline, current state, lifecycle state, or commit

  @SCN-332 @REQ-003
  Scenario: Implementation work stops after its assigned scenario
    Given Implementation Mode is delegated one approved scenario
    When it returns its scenario result
    Then it reports its evidence and changed paths
    And it does not choose another scenario, transition lifecycle state, approve, commit, clean, or mark completion

  @SCN-333 @REQ-003
  Scenario: Review work returns evidence without advancing lifecycle state
    Given Review Mode is delegated review work
    When it finishes
    Then it returns pass, fail, or escalation evidence
    And it does not change approval, current-submission, lifecycle state, checkpoints, commits, or completion

  @SCN-334 @REQ-003
  Scenario: Late or direct phase-agent output cannot advance the workflow
    Given a phase agent is run directly, retries after timeout, or returns after the orchestrator has moved on
    When its output is received
    Then it does not independently advance lifecycle state
    And the orchestrator validates it against approved scope and required evidence before accepting any result

  @SCN-335 @REQ-004
  Scenario: Objective review success enters final human review rather than completion
    Given Phase 4 has passed on a committed implementation snapshot
    When the orchestrator records the review result durably
    Then it records that snapshot as reviewed_commit
    And it transitions the feature to final_human_review
    And it does not mark the feature complete

  @SCN-336 @REQ-004
  Scenario: Explicit human approval completes the reviewed snapshot
    Given a feature is in final_human_review
    And its current approved implementation snapshot matches reviewed_commit
    When explicit human approval is received for that snapshot
    Then the orchestrator transitions the feature to complete
    And no reviewer identity is recorded in the approval record

  @SCN-337 @REQ-004
  Scenario Outline: A changed or invalidated reviewed snapshot cannot complete
    Given a feature has final-human-review eligibility for a reviewed commit
    And <invalidating_event> occurs
    When final approval is evaluated or the feature is resumed
    Then the feature does not complete from the stale reviewed commit
    And it returns to review before completion can be possible

    Examples:
      | invalidating_event |
      | a later code change |
      | a manual commit, amendment, or rebase |
      | a dirty code change |
      | a subsequent review failure |

  @SCN-338 @REQ-004
  Scenario: Failure to persist review eligibility does not create final-review authority
    Given Phase 4 passes
    When recording reviewed_commit or the final_human_review transition fails
    Then the feature is not eligible for final approval
    And the persistence failure is reported

  @SCN-339 @REQ-005
  Scenario: User-invocable Claude phase requests route through the orchestrator
    Given a user invokes a Claude-facing request for specification, implementation, or review
    When the request is received
    Then the orchestrator evaluates workspace authority and legal phase order before phase work starts

  @SCN-340 @REQ-005
  Scenario Outline: A request for an illegal later phase cannot execute that phase directly
    Given a user requests a later phase while <precondition>
    When the request is evaluated
    Then the requested phase does not execute directly
    And the orchestrator stops or routes the request according to the required earlier phase

    Examples:
      | precondition |
      | approval is missing |
      | approval is invalid |
      | an earlier phase is required |

  @SCN-341 @REQ-005
  Scenario: Host capability differences do not permit direct phase execution
    Given a supported host lacks hidden subagents or slash commands
    When a user requests phase work
    Then the request still reaches the orchestrator decision point
    And no direct phase execution bypass occurs

  @SCN-342 @REQ-006
  Scenario: Review evaluates only the configured objective gates
    Given quality-gates configuration completely defines enabled objective gates and their evaluation data
    When review runs
    Then it evaluates configured gates in configured order
    And it uses only each gate's configured applicability, thresholds, commands, targets, parsing rules, severity, and remediation outcome

  @SCN-343 @REQ-006
  Scenario Outline: Invalid gate configuration stops review without defaults
    Given quality-gates configuration is <configuration_condition>
    When review is started
    Then review stops with a configuration error
    And no embedded default gate behavior is substituted

    Examples:
      | configuration_condition |
      | missing |
      | unreadable |
      | malformed |
      | incomplete for an enabled gate |
      | internally inconsistent |

  @SCN-344 @REQ-006
  Scenario: Configuration changes control subsequent review behavior
    Given quality-gates configuration changes a threshold, enabled status, severity, remediation outcome, command, or critical-function list
    When review next runs with that configuration
    Then the changed configuration takes effect without a review-code or instruction change

  @SCN-345 @REQ-006
  Scenario: An explicitly empty critical-function list is not applicable
    Given quality-gates configuration explicitly supplies an empty critical-function list
    When review evaluates critical-function coverage
    Then it records that sub-gate as not_applicable
    And it does not fail solely because no functions are named

  @SCN-346 @REQ-006
  Scenario: Review evidence identifies the configuration used and command outcomes
    Given review runs with valid quality-gates configuration
    When review evidence is produced
    Then it records the resolved configuration identity or fingerprint
    And it records configured command outcomes sufficient to audit the decision

  @SCN-347 @REQ-007
  Scenario: Phase 3 starts only from valid approved committed authority
    Given approved Gherkin scenarios exist
    When the orchestrator considers starting Phase 3
    Then it delegates implementation only if a valid matching feature record and committed baseline are present

  @SCN-348 @REQ-007
  Scenario: Each scenario delegation requires a clean recorded worktree
    Given the orchestrator is about to delegate an approved scenario
    When tracked or non-ignored worktree changes are present or recorded worktree identity does not match
    Then it stops non-destructively
    And it does not delegate that scenario

  @SCN-349 @REQ-007
  Scenario: Ignored local artifacts do not block a clean scenario boundary
    Given the recorded worktree has changes only in ignored local artifacts
    And tracked and non-ignored paths are clean
    When the orchestrator evaluates the next scenario boundary
    Then it may proceed with the approved scenario

  @SCN-350 @REQ-007
  Scenario: Implementation receives exactly one approved scenario and stops
    Given the orchestrator delegates an implementation task
    When the task is issued
    Then it contains one already-approved scenario
    And after reporting Red/Green/Refactor traceability and required evidence the implementation task stops

  @SCN-351 @REQ-007
  Scenario: The orchestrator validates a scenario result before continuing
    Given implementation reports a scenario result
    When the orchestrator evaluates that result
    Then it verifies required evidence, approved scope, and boundary cleanliness
    And only after successful validation does it accept, checkpoint, and continue to the next approved scenario

  @SCN-352 @REQ-007
  Scenario Outline: A scenario-loop anomaly halts without bypass
    Given Phase 3 is active
    And <anomaly> occurs
    When the orchestrator evaluates continuation
    Then it halts without bypassing approval, state validation, clean-boundary checks, or configured quality gates

    Examples:
      | anomaly |
      | checkpoint persistence and state persistence disagree |
      | another process changes the worktree during delegation |
      | contract drift is detected |
      | approval becomes invalid |
      | a required gate fails |

  @SCN-353 @REQ-008
  Scenario: Installing Claude artifacts does not require a local Claude executable
    Given the local environment has no Claude executable
    When the installer generates Claude integration artifacts
    Then artifact installation succeeds without claiming runtime compatibility verification

  @SCN-354 @REQ-008
  Scenario: CI records Claude version and compatibility verification
    Given CI runs Claude compatibility verification against generated Claude artifacts
    When the verification can run
    Then CI records the result and claude --version output in durable CI evidence

  @SCN-355 @REQ-008
  Scenario Outline: An unverifiable Claude compatibility claim fails in CI
    Given CI compatibility verification <failure_condition>
    When CI evaluates the compatibility claim
    Then the claim fails
    And CI does not report Claude support as verified

    Examples:
      | failure_condition |
      | cannot run |
      | cannot record the Claude version |
      | has a compatibility test failure |

  @SCN-356 @REQ-009
  Scenario: Supported hosts use the same workspace-controlled workflow authority
    Given a workflow is started or resumed from any supported host
    When phase work is requested
    Then the host directs the workflow to shared workspace authority and the orchestrator
    And it preserves the canonical approval gates, legal transitions, no-legacy rule, and final-human-review semantics

  @SCN-357 @REQ-009
  Scenario: Resume validates durable workspace state rather than host or memory state
    Given a workflow is resumed
    When the orchestrator evaluates its authority
    Then it validates the feature record, baseline, lifecycle state, recorded worktree, and relevant commit
    And it does not reconstruct approval or lifecycle authority from host-local state or memory pointers

  @SCN-358 @REQ-009
  Scenario: Stale host or memory artifacts cannot override canonical workspace state
    Given an old generated host asset or stale memory pointer references obsolete workflow state
    When the workflow is resumed
    Then the obsolete artifact does not authorize, transition, or recover workflow state
    And conflicts between concurrent host resumes fail closed rather than merging decisions
@SCN-359 @REQ-001
Scenario: A valid structured approved-scenario reference authorizes its exact scenario
  Given a feature-scoped approval record names a canonical repository-relative feature path, one scenario ID, and that scenario's exact requirement IDs
  And the named feature contract is covered by the recorded fingerprint
  And the record has a valid confirmed immutable baseline
  When the orchestrator validates the approved scenario scope
  Then that exact scenario is eligible for workflow execution

@SCN-360 @REQ-001
Scenario: A malformed structured approved-scenario reference blocks workflow progress
  Given a feature-scoped approval record contains an approved-scenario entry missing a required field or containing an additional authoritative field
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports the malformed approved-scenario reference

@SCN-361 @REQ-001
Scenario: A non-canonical scenario path cannot authorize a scenario
  Given a feature-scoped approval record contains an approved-scenario entry with an absolute, traversal-containing, or otherwise non-canonical feature path
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports the invalid feature path

@SCN-362 @REQ-001
Scenario: An unresolved or ambiguous scenario ID cannot authorize a scenario
  Given a feature-scoped approval record contains an approved-scenario entry whose scenario ID resolves to zero or multiple scenarios in its named feature file
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports that the scenario ID did not resolve exactly once

@SCN-363 @REQ-001
Scenario: A requirement-tag mismatch cannot authorize a scenario
  Given a feature-scoped approval record contains an approved-scenario entry whose requirement IDs differ from the resolved scenario's requirement tags
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports the requirement-ID mismatch

@SCN-364 @REQ-001
Scenario: Duplicate approved scenario identity blocks authorization
  Given an approved-scenario entry is duplicated in a feature-scoped record or its scenario ID is present in another active feature record
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports the duplicate scenario identity

@SCN-365 @REQ-001
Scenario: A display-oriented scenario reference cannot authorize a scenario
  Given a feature-scoped approval record uses a scenario title, source line, or opaque path-and-scenario string as authoritative scenario identity
  When the orchestrator validates the approved scenario scope
  Then it rejects the record before implementation, review, or completion
  And it reports that the reference is not structured authoritative identity

@SCN-366 @REQ-001 @REQ-007
Scenario: A pending baseline cannot start the TDD scenario loop
  Given a feature-scoped approval record has a pending baseline reference
  And no valid orchestrator-owned confirmation records the immutable baseline artifact commit
  When the orchestrator decides whether to begin Phase 3
  Then it stops before delegating any approved scenario
  And it reports that baseline confirmation is pending

@SCN-367 @REQ-001 @REQ-007
Scenario: A confirmed immutable baseline authorizes the approved scenario loop
  Given the immutable baseline artifact commit contains a pending approval record and the approved contract
  And an orchestrator-owned confirmation record identifies that baseline artifact commit
  And the identified commit is reachable in the recorded feature worktree history
  And the baseline contract matches the confirmation record's fingerprints and approved scenario scope
  When the orchestrator decides whether to begin Phase 3
  Then it may delegate only the confirmed approved scenarios

@SCN-368 @REQ-001
Scenario: An invalid baseline confirmation blocks workflow progress
  Given a feature-scoped approval record names a baseline that is missing, unreachable, self-referential, mutable, or different from the immutable baseline artifact commit
  When the orchestrator validates the approval authority
  Then it rejects the record before implementation, review, or completion
  And it reports the precise baseline-confirmation failure
