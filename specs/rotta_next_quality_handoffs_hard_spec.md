# Hard Spec: Rotta Next Quality Roles And Recoverable Handoffs

## Objectives

- Preserve Rotta Next Fast mode as the default: one coherent implementation slice, relevant verification, and one independent review without a mandatory specification, Gherkin feature, worktree, intermediate commit, or multi-role quality pipeline.
- Install `rotta-cleaner` and `rotta-architect` as specialized, conditional roles that add Swarm Forge-style quality depth only when risk or evidence justifies their cost.
- Make an Ancora handoff index recoverable across sessions while validating every continuation against workspace and Git state.
- Provide a compact `.rotta/handoffs/` recovery mirror for Ancora failure without turning either memory or the mirror into behavioral or operational authority.
- Use coverage, CRAP/complexity, duplication, mutation, and architecture evidence proportionally and only from declared, available project tooling.

## Non-Goals

- Require Gherkin, a hard spec, acceptance-test generation, or human approval for ordinary Fast-mode work.
- Copy Swarm Forge's tmux runtime, daemon, inbox/outbox queue, dedicated worktrees, automatic agent wake-up, or commit-per-handoff model.
- Add `rotta-hardener` in this change. It is deferred until cleaner and architect evidence demonstrates a distinct, recurring responsibility that cannot remain bounded in those roles.
- Introduce universal CRAP, coverage, mutation, duplication, or architecture thresholds; install missing tools; or guess project commands.
- Permit Ancora, a handoff record, a review result, or an automated metric to authorize commits, publication, indexing, cleanup, credentials, or destructive actions.

## Adversarial Pre-Mortem

- Failure mode 1: every implementation invokes cleaner and architect, turning Fast mode into a fixed serial pipeline with worse latency than the workflow it replaced.
- Failure mode 2: an Ancora handoff points to a stale or rebased snapshot, and a resumed role acts on a different diff than the previous role reviewed.
- Failure mode 3: a missing CRAP or mutation tool is treated as a pass, or the system installs/runs an expensive tool automatically, producing false confidence or unexpected cost.
- Failure mode 4: cleaner changes observable behavior while performing cleanup, but its changes receive no fresh independent review.
- Failure mode 5: a fallback handoff mirror conflicts with Ancora and one is chosen by timestamp or convenience, reviving ambiguous state.

## Explicit Decisions

- Gherkin remains optional in Fast mode. It is required only when a Strict behavioral contract needs observable examples to make approval unambiguous.
- `rotta-cleaner` and `rotta-architect` are installed roles, but neither is invoked by default for a Fast-mode slice.
- `rotta-hardener` is deferred. Cleaner owns targeted robustness/mutation evidence until a future approved spec proves a separate role is needed.
- Ancora is the primary handoff index. A compact project-local recovery mirror lives under `.rotta/handoffs/` so a later Ancora outage does not erase an already recorded handoff.
- The workspace, Git baseline/snapshot, approved Strict contract, and applicable Gherkin remain authoritative. Handoff records are routing and recovery metadata only.
- A recovered handoff is valid only when its baseline SHA, snapshot SHA, scope, role transition, and referenced artifacts agree with the current workspace. Any mismatch is `blocked`, never merged or inferred.

## Workflow

### Fast Default

```text
orchestrator -> optional explore -> impl -> review -> outcome report
```

The orchestrator may choose the deep path before implementation when the task is Strict, high-impact, or has known structural risk. The reviewer may request one deep escalation when its evidence is insufficient or reveals a bounded structural concern.

### Deep Review Escalation

```text
orchestrator -> impl -> cleaner -> architect -> review -> outcome report
```

This path is allowed only once per coherent slice. A cleaner or architect finding that requires behavior change returns to `rotta-impl`; the changed diff invalidates prior review evidence and receives one fresh review. No role may create an unbounded cleaner/architect loop.

### Strict Behavioral Work

```text
orchestrator -> approved Strict contract -> optional approved Gherkin -> impl ->
conditional cleaner/architect -> review -> outcome report
```

Strict mode does not automatically require cleaner or architect. The orchestrator selects them from the approved risk classification, repository policy, or review evidence.

## Ancora And Vela Placement

### Current Rotta Next

Ancora is optional compact continuity. When enabled, the orchestrator recovers relevant decisions, discoveries, and summaries before it creates a capsule; roles save compact decisions, discoveries, fixes, and end summaries. Workspace and Git remain authoritative.

Vela is optional, bounded structural evidence. Fast mode normally makes no graph call. `rotta-explore` may use at most two calls for one named structural question, and `rotta-review` may use one targeted call at an architectural boundary. Implementation does not call Vela by default. `rotta-ops` is the only role that may index or re-index Vela, after explicit user consent.

```text
Ancora context -> orchestrator -> optional explore [Vela: up to 2 calls]
                                      -> impl -> review [Vela: up to 1 call]
                                      -> Ancora outcome summary
```

### Proposed Quality And Handoff Flow

Ancora records the compact handoff index after each role transition and is consulted before resuming a task. It never substitutes for the current Git baseline, snapshot, contract, or test evidence.

Vela remains a structural tool, not a routine quality metric. Cleaner uses local changed-code evidence and does not call Vela by default. Architect may use at most two targeted calls for one named dependency, impact, ownership, or boundary question. Final review may use one more call only to verify a material architectural finding or contradictory graph evidence.

```text
Ancora recovery
  -> orchestrator
  -> optional explore [Vela: up to 2 calls]
  -> impl
  -> Ancora handoff index
  -> optional cleaner [local quality evidence; no Vela by default]
  -> optional architect [Vela: up to 2 calls]
  -> review [Vela: up to 1 validation call]
  -> Ancora handoff index and outcome summary
```

Vela call budgets are per coherent slice. A missing, stale, ambiguous, or unavailable graph requires source fallback and a reported evidence gap; it never blocks Fast work. The only exception is an explicitly requested `rotta-ops` indexing action, which remains operational work and requires named user authorization.

## Role Ownership

| Role | Invoked when | Owns | Does not own |
|---|---|---|---|
| `rotta-review` | Every completed implementation slice | Independent diff/evidence/behavior review, severity-ordered findings, targeted verification decision, deep-review escalation recommendation | Editing implementation, approving operations, treating metrics as a substitute for review |
| `rotta-cleaner` | Deep review selected or requested by review | Behavior-preserving cleanup, changed-code coverage gaps, meaningful duplication, complexity/CRAP evidence, and targeted mutation/robustness evidence | New product behavior, unconditional expensive tools, publication, or final acceptance |
| `rotta-architect` | Deep review selected or requested by cleaner/review | Dependency direction, boundary leakage, cohesion, encapsulation, adapter separation, and architecture risk | Broad redesign, unrelated cleanup, automatic operations, or final acceptance |
| `rotta-impl` | Approved coherent slice or isolated remediation | Product behavior and isolated remediation, focused tests, implementation evidence | Approving scope expansion or self-review |
| `rotta-orchestrator` | Every task | Risk routing, capsule creation, handoff validation, role selection, final outcome report | Production edits, ordinary operations, or inventing approval |

Cleaner may edit only behavior-preserving, approved cleanup in its capsule. It must report changed paths and verification. Architect is read-only by default; it returns findings or an isolated remediation capsule for `rotta-impl`. This keeps architectural changes explicit and independently reviewed.

## Handoff Index And Recovery

### Record Schema

Each handoff uses `rotta.handoff/v1` and contains only compact routing/evidence metadata:

```yaml
format: rotta.handoff/v1
handoff_id: checkout-validation/003
sequence: 3
from: rotta-impl
to: rotta-review
status: ready # ready | accepted | blocked | completed | superseded
priority: normal # low | normal | high
baseline_sha: 42ab19c
snapshot_sha: 91ee2a0
scope:
  - internal/checkout/
strict_contract_ref: .rotta/strict/checkout-validation.md
gherkin_ref: features/checkout-validation.feature
evidence:
  commands:
    - go test ./internal/checkout
  result: passed
  recorded_at: 2026-08-05T00:00:00Z
disposition: ready_for_review
```

The record must not contain credentials, full specifications, complete Gherkin content, raw command logs, user-private data, or copied core policy.

### Persistence And Validation

1. The orchestrator creates or updates the primary Ancora index using a stable `handoff/<task-id>` topic and a monotonically increasing sequence.
2. It writes the same compact record to `.rotta/handoffs/<task-id>-<sequence>.yaml` as a recovery mirror. The mirror is a fallback index, not an independent lifecycle authority.
3. Before a receiving role begins, the orchestrator verifies the recorded baseline SHA is an ancestor or approved baseline for the current snapshot, the snapshot/diff still matches, the declared scope is applicable, and the role transition is legal.
4. A receiver records `accepted`, `blocked`, or `completed` through the orchestrator. Roles do not claim or overwrite a handoff directly.
5. If Ancora is unavailable, the orchestrator selects the newest valid matching `.rotta/handoffs/` record, reports degraded recovery, and continues only after the same Git/workspace validation.
6. If Ancora and the mirror disagree, if either record is malformed, or if more than one valid record claims the same next role and sequence, stop with a recovery action. Never pick by timestamp alone.
7. The outcome report includes handoff ID, recovery source, validation result, active elapsed time, child-session count, and retry count.

## Verification Escalation

### Default

Fast mode runs change-relevant checks first. `rotta-review` receives their command/result evidence and reruns targeted checks only for missing, stale, contradictory, risk-sensitive, or insufficient evidence.

### Cleaner Evidence

Cleaner may request changed-code-only coverage, complexity/CRAP, duplication, or mutation evidence only when all of the following are true:

- The task is Strict, deep review was selected, or review identifies a concrete changed-code risk.
- The command/tool is declared by project metadata, repository configuration, or explicit user instruction.
- The tool is already available and can run within the capsule's stated verification budget.

CRAP is advisory by default. Cleaner reports newly introduced or worsened complexity/coverage risk relative to the slice; it does not impose a universal threshold such as `CRAP <= 6`. A repository may later define an explicit policy threshold in a separate approved configuration feature.

Mutation work is targeted to changed, weakly covered, risk-sensitive behavior. It is not a full-repository default, does not install tooling, and cannot block solely because an unconfigured tool is absent.

### Architect Evidence

Architect checks only the affected modules and interfaces for dependency direction, framework/IO leakage into core rules, unnecessary coupling, weak cohesion, representation leakage, and untestable adapter boundaries. It must distinguish a concrete defect from a stylistic preference and state the likely behavior/maintenance consequence.

## Behavioral Traceability

- A Strict contract with Gherkin records each material accepted example and its implementing test or an explicit test gap in the implementation handoff.
- Fast work without Gherkin records acceptance checks in the capsule and maps each changed behavior to focused test evidence where a test is appropriate.
- Documentation-only, formatting-only, dependency remediation, behavior-preserving refactors, and cosmetic UI work do not gain Gherkin merely to satisfy a handoff field.
- No traceability record authorizes work outside the approved scope or turns an unverified assertion into a passing result.

## Performance Invariants

- Fast mode invokes no cleaner or architect unless explicit risk/evidence selects deep review.
- A standard Fast slice has at most one implementation role and one independent review role; optional exploration remains bounded by the existing core call budget.
- Deep review adds at most one cleaner and one architect invocation before the final review; retries require a concrete changed diff or missing-evidence reason.
- No worktree, tmux session, daemon, queue poller, automatic commit, full test suite, coverage suite, CRAP run, mutation run, or architecture pass is required by default.
- Benchmark reports compare equivalent Fast and deep slices using active elapsed time, role count, child-session count, retries, verification commands, and outcome quality.

## Requirements

### REQ-001: Install conditional cleaner and architect roles

**Description:** Rotta shall install `rotta-cleaner` and `rotta-architect` alongside the existing Next roles, each loading `rotta-core` and declaring narrow authority, inputs, outputs, and stop conditions.

**Acceptance Criteria:**

- OpenCode, Claude Code, and Codex receive the two roles through their existing host-specific integration path.
- The core policy and orchestrator state that these roles are conditional and never required for a standard Fast slice.
- Cleaner is limited to approved behavior-preserving cleanup and targeted evidence; architect is read-only by default.
- Installer ownership digests protect the two new role files and definitions under the existing managed-artifact rules.

### REQ-002: Persist and recover compact handoff indexes

**Description:** Rotta shall persist one compact, versioned handoff index in Ancora and a project-local `.rotta/handoffs/` recovery mirror, validating both against Git before continuation.

**Acceptance Criteria:**

- Every delegated implementation, cleaner, architect, and review transition has a `rotta.handoff/v1` record with ID, sequence, roles, status, priority, baseline SHA, snapshot SHA, scope, evidence summary, and disposition.
- Ancora uses one stable task-scoped topic; the project mirror is written atomically and excludes sensitive/large payloads.
- Recovery from Ancora and from the mirror rejects a changed/missing baseline, changed snapshot, illegal role transition, malformed record, conflicting sequence, or incompatible scope.
- An Ancora failure reports degraded recovery and can use only a valid project mirror; it never fabricates a handoff from conversational memory.
- A conflicting Ancora/mirror record blocks with a concrete recovery action.

### REQ-003: Apply evidence-driven cleaner checks

**Description:** Cleaner shall use complexity/CRAP, coverage, duplication, and mutation evidence only when selected by risk/evidence and supported by the project.

**Acceptance Criteria:**

- Fast mode does not run these checks by default.
- Cleaner does not install, guess, substitute, or silently skip project tools.
- CRAP output is reported as changed-code evidence and delta; no global threshold is enforced without later explicit repository policy.
- Missing declared tooling is a visible evidence gap, not a passing quality result or an automatic block for Fast work.
- Any cleaner edit invalidates prior review evidence and triggers relevant verification plus one fresh independent review.

### REQ-004: Enforce bounded deep-review routing

**Description:** The orchestrator shall select deep review only from Strict classification, explicit user request, repository policy, or concrete evidence from review.

**Acceptance Criteria:**

- Standard Fast routing remains `orchestrator -> impl -> review`.
- Deep routing is recorded in the capsule and handoff index with the trigger and expected evidence.
- Cleaner and architect cannot recursively schedule themselves or each other.
- A deep-review request requiring product behavior changes returns an isolated remediation capsule to implementation and does not self-approve the result.
- Outcome reports distinguish Fast from deep review and report the added roles, elapsed time, retries, and evidence gained.

## Test Strategy

- Installer tests verify all hosts install cleaner and architect and preserve managed-file/agent ownership protections.
- Policy tests verify Fast assets do not make cleaner, architect, CRAP, mutation, coverage, or full suites mandatory.
- Handoff tests cover Ancora success, Ancora write failure, Ancora read failure with valid mirror, baseline/snapshot drift, malformed records, sequence conflicts, and Ancora/mirror disagreement.
- Role tests verify cleaner cannot add behavior or self-approve and architect cannot edit or schedule deep review recursively.
- Routing tests cover Fast default, Strict-selected deep review, review-triggered single escalation, and fresh review after cleaner edits.
- Performance tests compare equivalent Fast and deep slices and assert the Fast path does not spawn cleaner/architect roles or their verification tools.

## Open Questions

- None. The approved defaults are optional Fast Gherkin, installed conditional cleaner/architect roles, deferred hardener, Ancora-primary handoff indexes, and `.rotta/handoffs/` recovery mirrors.
