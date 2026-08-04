@workflow_ergonomics
Feature: Ergonomic, isolated Rotta workflow
  To keep feature delivery safe without repetitive workflow overhead
  As a Rotta user
  I need isolated authority, bounded context, compact execution, and honest host installation status.

  @REQ-081 @SCN-601
  Scenario: A new feature is bootstrapped only in its explicit-base worktree
    Given the initiating checkout is clean and the selected base resolves to an explicit commit SHA
    When a full workflow starts for a valid unused feature ID
    Then Rotta creates the feature branch and isolated worktree from that exact SHA
    And the initiating checkout has no submission artifact writes
    And the feature worktree contains one manifest, the feature-local policy, and the pending contract artifacts

  @REQ-081 @SCN-602
  Scenario: Parallel feature worktrees keep mutable workflow state isolated
    Given two valid feature IDs have distinct isolated worktrees
    And each worktree has its own current manifest and state
    When one feature records progress or evidence
    Then the other feature's current state and evidence are unchanged
    And neither feature can accept the other's manifest, worktree, or checkpoint state

  @REQ-081 @SCN-603
  Scenario Outline: Unsafe worktree preparation preserves the initiating checkout
    Given a full workflow has <unsafe condition>
    When Rotta prepares its isolated feature worktree
    Then it creates no submission artifact in the initiating checkout
    And it does not begin specification or implementation
    And it reports a non-destructive recovery action

    Examples:
      | unsafe condition |
      | a non-ignored initiating-checkout change |
      | an existing feature branch or worktree path collision |
      | a detached initiating HEAD or unresolved base SHA |

  @REQ-081 @SCN-604
  Scenario: Resume and archive retain a verified feature recovery boundary
    Given a feature worktree has matching manifest, state, baseline, and fingerprints
    And its latest execution stopped after a checkpoint boundary
    When Rotta resumes the feature
    Then it selects only the recorded feature and recorded scenario or slice
    And when that feature reaches a verified terminal state it archives only its current runtime directory
    And the feature contract, approval, branch, and worktree remain available for handoff or inspection

  @REQ-082 @SCN-605
  Scenario: Review sees baseline-to-HEAD changes after checkpoint commits
    Given an approved feature has a confirmed baseline commit
    And one or more scenario checkpoints have left its worktree clean
    When Rotta resolves changed files for review
    Then it records the baseline SHA and HEAD SHA
    And it reports changed, renamed, and deleted paths from the baseline-to-HEAD comparison
    And it does not treat an empty working-tree diff as an empty feature diff

  @REQ-082 @SCN-606
  Scenario Outline: Only feature-local policy and evidence paths are active
    Given a feature worktree has valid manifest-bound current policy and evidence
    And <legacy artifact> is also present
    When Rotta prepares continuation or review
    Then it uses only the feature-local policy and current evidence paths
    And the legacy artifact is reported as retired but cannot authorize, satisfy evidence, or expand scope

    Examples:
      | legacy artifact |
      | root .rotta/tdd-log.md |
      | root .rotta/state.yaml |
      | specs/.approved |
      | root review evidence or a legacy state-machine reference |

  @REQ-082 @SCN-607
  Scenario: Missing or drifted feature-local policy blocks rather than falls back
    Given a feature manifest records a policy fingerprint
    And the feature-local policy is missing, unreadable, or has a different fingerprint
    When Rotta prepares continuation or review
    Then it blocks the operation with policy remediation
    And it does not read policy from the initiating checkout or a sibling worktree

  @REQ-083 @SCN-608
  Scenario Outline: A sole displayed acknowledgement authorizes its bound action
    Given exactly one current approval action is displayed with its feature ID and unchanged contract fingerprint
    When the human replies exactly <acknowledgement>
    Then Rotta consumes that displayed action once
    And it advances only the lifecycle action named by that display

    Examples:
      | acknowledgement |
      | x |
      | yes |
      | agree |
      | approved |
      | approve |

  @REQ-083 @SCN-609
  Scenario Outline: A stale or ambiguous acknowledgement cannot advance the workflow
    Given an acknowledgement is <condition>
    When Rotta evaluates the acknowledgement
    Then it does not change approval, checkpoint, review, archive, or completion state
    And it reports why the acknowledgement was rejected

    Examples:
      | condition |
      | for a replaced prompt or a restarted session |
      | received after feature, contract, policy, or final-snapshot drift |
      | a multiple-intent message |
      | received while more than one approval action is pending |
      | an unsupported acknowledgement token |

  @REQ-083 @SCN-610
  Scenario: Final acknowledgement completes only the displayed reviewed snapshot
    Given a feature is in final_human_review
    And its current committed snapshot, evidence, and policy fingerprints match reviewed_commit
    And the sole displayed final approval action names that feature and reviewed commit
    When the human replies "approve"
    Then Rotta marks that feature complete
    And it records no human actor identity

  @REQ-084 @SCN-611
  Scenario: A valid scoped override is consumed once and remains auditable
    Given a displayed override action names one feature, rule, operation, baseline, contract fingerprint, reason, and expiry
    And its target is a persisted matching non-passing gate outcome or one eligible process rule
    When the human authorizes the displayed override
    Then Rotta records an actor-less feature-local override with one remaining use
    And it applies it to only the named operation
    And after use the override remains evidence but cannot authorize another operation

  @REQ-084 @SCN-612
  Scenario Outline: An invalid override is rejected on drift or expiry
    Given a feature-local override is <invalid condition>
    When Rotta evaluates the named operation
    Then it does not apply the override
    And it reports remediation without changing the underlying gate or process outcome to pass

    Examples:
      | invalid condition |
      | expired or already consumed |
      | missing its required reason or expiry |
      | bound to a different baseline, contract, policy, evidence, or scope |
      | competing with another override for the same operation |

  @REQ-084 @SCN-613
  Scenario Outline: Non-waivable integrity failure provides safe recovery
    Given a requested override would bypass <integrity safeguard>
    When Rotta evaluates that request
    Then it refuses the override without destructive cleanup
    And it identifies preserved paths and a safe repair, handoff, or verified-archive alternative

    Examples:
      | integrity safeguard |
      | malformed or inconsistent manifest or approval authority |
      | an unknown or destructive cleanup target |
      | an incorrect or missing recorded worktree identity |

  @REQ-085 @SCN-614
  Scenario: OpenCode context limits retain complete local failure evidence
    Given an OpenCode target accepts automatic compaction/pruning, a 10,000-token reserve, and 120-line/12,288-byte tool-output limits
    And a lifecycle-relevant command produces output beyond the chat summary limit
    When Rotta captures and reports the command result
    Then the full command output and result metadata are retained under current feature evidence
    And chat reports an at-most-80-line/8-KiB summary with truncation metadata and the evidence path
    And the lower host or summary limit wins when both limits apply
    And the truncated chat output alone cannot be accepted as passing evidence

  @REQ-085 @SCN-615
  Scenario Outline: A recorded role budget stop does not hide unfinished work
    Given <role> reaches its recorded default budget of <steps> steps before completing its assigned scenario or slice
    When Rotta records the stop
    Then it reports the current manifest, slice, and evidence references
    And it does not declare success, skip validation, or supply the full prior transcript on resume

    Examples:
      | role | steps |
      | exploration | 8 |
      | spec | 12 |
      | orchestration | 16 |
      | review | 16 |
      | implementation | 24 |

  @REQ-086 @SCN-616
  Scenario: Local scope below the uncertainty threshold does not create an exploration capsule
    Given focused local inspection resolves owners and invariants within eight actions
    And the likely change affects no more than two top-level components and five direct dependents
    When Rotta delegates the approved work
    Then it delegates only the current scenario or slice without an exploration capsule
    And it records that no capsule was required

  @REQ-086 @SCN-617
  Scenario: Cross-component uncertainty produces a bounded traceable capsule
    Given local inspection leaves a required owner unresolved or exceeds the component or dependency threshold
    When Rotta creates an exploration capsule
    Then the capsule records its objective, scope, bounded files and symbols, invariants, test commands, risks, and blockers within its configured limits
    And implementation receives the capsule path plus only its current scenario or slice
    And a stale or bound-exhausted capsule stops with a blocker instead of expanding the transcript

  @REQ-087 @SCN-618
  Scenario: A compact execution slice preserves per-scenario traceability with one full checkpoint
    Given three related approved scenarios fit one component scope and the expected-path limit
    And the manifest explicitly selects compact_slice mode
    When Rotta executes the slice
    Then each scenario has its own Red, Green, Refactor, and focused-test evidence
    And Rotta runs full validation and creates one checkpoint for the completed slice
    And state links every scenario to the slice, capsule decision, evidence, and checkpoint

  @REQ-087 @SCN-619
  Scenario: Strict mode keeps one scenario per full checkpoint
    Given approved scenarios are eligible for compact grouping
    And the manifest explicitly selects strict_per_scenario mode
    When Rotta executes the work
    Then each slice contains one scenario
    And each scenario receives its own full validation and checkpoint boundary
    And Rotta does not silently coalesce scenarios

  @REQ-087 @SCN-620
  Scenario: Slice failure preserves work without a partial checkpoint
    Given a compact slice has completed focused evidence for an earlier scenario
    And a later scenario or the slice validation fails
    When Rotta stops the slice
    Then it creates no slice checkpoint
    And it preserves current changes and evidence with the affected scenario identified
    And it reports resume, handoff, or recovery guidance

  @REQ-088 @SCN-621
  Scenario: Generated roles share one lifecycle authority without contradictory ownership
    Given Rotta generates instructions for supported hosts and role modes
    When the generated instructions are verified
    Then every role references the source/runtime rotta.lifecycle/v1 model and canonical path policy
    And only the orchestrator is authorized to approve, checkpoint, archive, recover, or complete a feature
    And no generated role activates the retired state-machine asset, reintroduces retired paths, or embeds generic quality-gate policy

  @REQ-089 @SCN-622
  Scenario Outline: The installer selects the documented effective OpenCode JSON or JSONC config
    Given OpenCode resolution identifies <effective source> as the effective configuration in <format>
    And OpenCode schema validation accepts the selected managed configuration
    When the installer configures only the selected OpenCode host
    Then it writes only Rotta-managed selected entries to that effective configuration
    And it preserves the effective JSON or JSONC format and unrelated user entries
    And it reports the resolved source, format, and precedence

    Examples:
      | effective source | format |
      | XDG global configuration | JSON |
      | OPENCODE_CONFIG override | JSONC |
      | documented project configuration | JSONC |

  @REQ-089 @SCN-623
  Scenario: Ambiguous or schema-invalid OpenCode configuration is not modified
    Given JSON and JSONC candidates cannot be deterministically resolved by OpenCode's documented mechanism or the candidate fails schema validation
    When OpenCode MCP installation is requested
    Then the installer writes no OpenCode configuration
    And it reports whether effective-config resolution or schema validation blocked the request
    And it leaves unrelated host installations unchanged

  @REQ-089 @SCN-624
  Scenario: MCP status distinguishes direct discovery from OpenCode resolution
    Given selected managed MCP servers have completed their available preflight and configuration steps
    When the installer reports OpenCode MCP status
    Then it separately reports command resolution, file write, schema validity, OpenCode server resolution, and tool discovery
    And it records those statuses and resolved command paths in the host-local installer transaction evidence
    And a direct server tools/list result is labeled direct discovery unless OpenCode enumerated those tools itself
    And an unavailable or unsupported observation is reported as not_observable rather than successful host discovery

  @REQ-089 @SCN-625
  Scenario: A Vela failure rolls back only its own selected managed changes
    Given Ancora or Context7 configuration has succeeded for a selected OpenCode host
    And Vela configuration for that host fails after its scoped transaction begins
    When the installer performs recovery
    Then it restores only Vela's known managed changes when that restore is safe
    And it retains the successful Ancora or Context7 configuration and status
    And it retains the scoped rollback result only in the host-local installer transaction evidence
    And if concurrent modification prevents scoped restore it reports that condition without broad rollback

  @REQ-090 @SCN-626
  Scenario: Missing quality-gates interface blocks only dependent workflow behavior
    Given the language-agnostic quality-gates v2 policy or current review-evidence interface is unavailable
    When a gate-targeted override or dependent review continuation is requested
    Then Rotta blocks that request with interface remediation
    And lifecycle-only work remains independently traceable
    And Rotta does not invent generic gate categories, commands, thresholds, or readiness rules

  @REQ-090 @SCN-627
  Scenario: Rollback preserves contracts and never revives legacy authority
    Given a workflow or selected-host configuration needs rollback or recovery
    When Rotta performs the available safe rollback action
    Then it preserves feature contracts, approval records, current evidence, and archives
    And it restores only the affected selected-host transaction when applicable
    And it offers handoff, archive, repair, or restart instead of reactivating legacy workflow artifacts or copying installer evidence into an initiating checkout

  @REQ-085 @REQ-089 @SCN-628
  Scenario: Installer transaction evidence is host-local, bounded, and non-authoritative
    Given installation runs for a selected host before any feature worktree exists
    When the installer starts its transaction
    Then it writes redacted command/configuration status and scoped backup references only below its host-local transaction ID
    And it retains that transaction evidence for 30 days and permits explicit cleanup only for that transaction ID
    And it does not copy the evidence into an initiating checkout, feature worktree, archive, approval, or lifecycle state
    And the transaction evidence cannot authorize, resume, or complete a feature
