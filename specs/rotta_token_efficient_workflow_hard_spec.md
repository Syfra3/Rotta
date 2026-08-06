# Hard Spec: Token-Efficient Rotta Workflow

## Adversarial Pre-Mortem

- Failure mode 1: Output filtering hides a failed check or omits the full log, so the workflow appears cheaper by weakening evidence rather than eliminating work.
- Failure mode 2: A broad deterministic command becomes a second lifecycle authority, mutates source or Git state without the orchestrator, or bypasses an operational consent boundary.
- Failure mode 3: A two-cycle remediation limit stops a valid repair without preserving a precise resume boundary, or permits a third hidden repair because a review finding is relabelled.
- Failure mode 4: The native Question result is accepted after contract, snapshot, target, or policy drift, converting a clearer UI into weaker authority.
- Failure mode 5: Ancora or Vela are called routinely to collect context, increasing token use and latency while providing no decision-relevant evidence.

## Hidden Assumptions

- OpenCode exposes per-session input, output, reasoning, cache-read, cache-write, child-session, and message metrics when a benchmark is collected; some hosts may not expose all fields.
- A benchmark fixture can hold task intent, repository baseline, model, agent configuration, and verification expectations materially equivalent between compared runs.
- Existing workflow validation primitives can be exposed through the `rotta` CLI without granting a command lifecycle, publication, indexing, cleanup, credential, or source-edit authority it does not already have.
- RTK is optional infrastructure: its absence cannot change command semantics, pass/fail results, or durable evidence.
- The Question result can be recorded with the same contract, policy, snapshot, feature, and pending-action bindings as existing approval validation.
- A stopped task retains enough local state and evidence for a later explicit resume; it does not need conversational history as authority.

## Alternatives Considered

| Approach | Reason Rejected |
|---|---|
| Enable RTK and leave the workflow unchanged | It can shrink command output but cannot remove repeated discovery, review, remediation, or user-continuation turns. |
| Call Ancora and Vela for every role | More retrieval and graph output increases context cost and makes advisory services accidental dependencies. |
| Use shell scripts as a parallel workflow engine | Untested shell behavior would duplicate authority and drift from the Go workflow primitives. |
| Remove independent review or allow unlimited repair loops | The first weakens quality; the second recreates the measured high-cost failure mode. |
| Treat any native Question answer as approval | A transcript answer alone cannot survive contract, snapshot, target, or policy drift safely. |

## Summary

Rotta shall reduce avoidable workflow cost without reducing authority or evidence quality. It shall expose existing workflow checks as narrow, deterministic Go CLI commands; run those checks before an independent review; bound automatic remediation to two cycles; use fingerprint-bound native OpenCode Questions only for real decisions; pass compact validated context rather than transcripts; and collect comparable telemetry. On the approved benchmark, the workflow must reduce non-cache token usage by at least 35% and child-session count by at least 40% from the recorded quality-handoffs baseline while retaining passing evidence, independent review, and all existing operational consent gates.

## Benchmark Baseline And Definitions

The recorded reference run is OpenCode session `ses_02bf6cec7ffedRSE5XzIjY7G82` (`Spec for quality handoffs`):

| Measure | Baseline |
|---|---:|
| Input tokens | 2,206,600 |
| Output tokens | 216,129 |
| Reasoning tokens | 76,905 |
| Non-cache tokens | 2,499,634 |
| Cache-read tokens | 22,764,544 |
| Child sessions | 42 |
| Root-session human messages | 25 |
| Corrective/restart sessions | 9 |

For this specification:

- **Non-cache tokens** are input plus output plus reasoning tokens. Cache-read and cache-write tokens are reported separately and do not count toward the 35% reduction target.
- **Child session** is a session recursively parented by the root orchestrator session. The target is at most 25 child sessions for an equivalent benchmark.
- **Equivalent benchmark** uses the same feature request, repository baseline, model family, enabled integrations, operational permissions, and acceptance checks. It identifies any material deviation and is not comparable when a required input is unavailable.
- **Correction cycle** begins when a review or deterministic validation returns a material in-scope failure and ends with a fresh review of the resulting changed diff.
- **Not observable** is a reported metric state. It is not zero, passing, or evidence that a target was met.

## Requirements

### REQ-091: Collect Comparable Workflow Telemetry

**Description:** Rotta shall produce a bounded, machine-readable outcome record for each workflow run and a benchmark comparator that evaluates an equivalent run against the declared baseline.

**Acceptance Criteria:**

- The outcome record includes model identifier, root session ID, child-session count, role invocation counts, input/output/reasoning/cache token fields when exposed, human-decision count, continuation count, correction-cycle count, deterministic-command count, review result, and evidence references.
- Each unavailable host metric is recorded as `not_observable` with its source; it is never inferred from message count or represented as zero.
- The comparator rejects a comparison when feature request, repository baseline, model family, enabled integration set, verification checks, or required metric availability differs materially.
- For an accepted equivalent benchmark, the median non-cache token use across three recorded runs is at most 1,624,762 tokens, a reduction of at least 35% from 2,499,634.
- Every accepted equivalent benchmark run has at most 25 child sessions, a reduction of at least 40% from 42 child sessions.
- A benchmark result cannot pass on efficiency alone: it must also show the required deterministic validation, one independent final review, and passing durable evidence for the feature's applicable checks.
- The comparator reports cache tokens separately and does not claim a provider cost saving without provider pricing data.

**Edge Cases:**

- A provider reports input and output but not reasoning or cache tokens.
- One of the three runs fails, compacts, is cancelled, or has a different model identifier.
- An LLM retry occurs inside a session without creating a child session.
- A benchmark fixture changes source behavior or verification commands while preserving its title.

**Out of Scope:**

- Promising a provider-currency cost reduction.
- Comparing unrelated features or relying on historical transcript estimates.
- Making production task completion depend on meeting a benchmark target.

### REQ-092: Expose Narrow Deterministic Workflow Commands

**Description:** Rotta shall expose narrow built-in Go commands for preflight, handoff validation, scoped verification, and publication planning so agents do not recreate those checklists with repeated shell exploration.

**Acceptance Criteria:**

- The CLI exposes distinct non-authorizing commands for workflow preflight, handoff/recovery validation, scoped verification planning/execution, and publication planning. Each command has a versioned JSON result and a concise human result.
- A command accepts explicit repository/worktree, feature/submission, contract, baseline, scope, and evidence inputs as applicable; it rejects missing, ambiguous, absolute, traversal-containing, or out-of-worktree paths.
- Commands reuse canonical workflow validation logic. They do not duplicate lifecycle transition, approval, Git, or path-validation rules in shell code.
- Successful command output contains only result status, canonical inputs, evidence path/hash, bounded diagnostics, and remediation. Full command output remains durable local evidence.
- Preflight, handoff validation, and publication planning are source/Git read-only. Scoped verification may write only its durable local evidence; it cannot mutate source, approval, lifecycle, handoff, archive, Git, or host configuration state. A failed command exits nonzero and preserves those protected states unless its separately documented operation is explicitly authorized.
- Publication planning is read-only. Commit, push, pull-request creation, indexing, installation, cleanup, and any destructive action remain separate operations requiring their existing exact authority.
- Role instructions direct agents to invoke the deterministic command before manually reconstructing the same workflow check.

**Edge Cases:**

- A command is invoked from a subdirectory, a sibling worktree, a detached HEAD, or a changed worktree.
- Durable evidence persistence fails after the underlying command completes.
- A prior evidence hash exists but names another baseline, scope, or command version.
- A command receives a valid-looking handoff that is stale, malformed, or has a conflicting mirror sequence.

**Out of Scope:**

- A general-purpose script runner.
- Replacing implementation, product decisions, independent review, or operational executors with CLI commands.
- Adding an automatic commit, push, index, install, or cleanup action.

### REQ-093: Use Compact Evidence And RTK Without Losing Facts

**Description:** Rotta shall pass compact deterministic evidence to agents and offer RTK as an optional, explicitly selected TUI installer integration for command-output presentation.

**Acceptance Criteria:**

- A capsule includes a canonical command result, evidence path/hash, changed paths, scope, known risk, and remediation; it excludes raw logs, full prior transcripts, duplicated core policy, and unrelated history.
- The installer TUI presents `Install RTK (optional)` and `Skip RTK`, defaults to Skip, and explains that RTK compacts agent-facing command output without changing durable evidence or workflow correctness.
- RTK installation occurs only after the user explicitly selects it and confirms the displayed host-level installation action. The installer verifies the resolved executable with `rtk --version`, records its canonical path and version in host-local installer transaction evidence, and reports success, skip, or failure distinctly.
- When RTK is available and compatible with the command, role instructions use its compact Git, Go/test, diff, log, and error views for chat-facing output. RTK's absence falls back to the deterministic command's bounded summary without changing the command executed or its result.
- Generated role instructions use RTK only when the installer recorded a currently resolvable verified executable. They never infer availability from a prior conversation or invoke an arbitrary `rtk` path.
- RTK-filtered output never becomes the only passing evidence. The full command result remains durable and is referenced by path and hash.
- Prompt and capsule token sizes are recorded when the host exposes them. An unavailable measurement is `not_observable`.
- Existing OpenCode compaction and pruning remain enabled. A compacted or pruned tool output retains an evidence path/hash sufficient for later inspection.

**Edge Cases:**

- RTK suppresses a warning while the underlying command exits zero.
- A test failure produces more output than the chat-output limit.
- RTK selection is skipped, installation fails, `rtk --version` fails, the recorded executable later disappears, or RTK formats a non-Go project command unexpectedly.
- A capsule references evidence that was deleted, changed, unreadable, or outside the active worktree.

**Out of Scope:**

- Installing RTK without explicit TUI selection and confirmation.
- Treating filtered logs as proof of test success.
- Raising global chat-output limits to preserve raw logs.

### REQ-094: Bound Automatic Remediation And Preserve Review

**Description:** Rotta shall eliminate continuation prompts and allow at most two automatic correction cycles for a coherent approved slice while retaining one fresh independent review after every changed remediation diff.

**Acceptance Criteria:**

- Fast routing continues from validated implementation handoff through review and outcome without a `continue` question.
- A Strict task receives one complete contract decision before implementation. Approved implementation slices inside that unchanged contract do not require another approval.
- Before the first independent review and before each remediation re-review, Rotta runs applicable deterministic preflight and scoped verification commands.
- A material in-scope failure may create at most two correction cycles. Each cycle records the failing evidence, changed paths, deterministic results, and fresh independent review result.
- If the second fresh review still finds a material failure, Rotta stops without a third implementation delegation, reports the unresolved findings and evidence, and offers explicit resume, scope-change, or cancellation actions.
- A non-material observation, unavailable optional tool, stale advisory graph, or missing unrelated historical evidence does not consume a correction cycle.
- A contract, Gherkin, policy, baseline, target, or operational-authority change invalidates the prior decision and stops for a new bound decision; it cannot be disguised as a correction cycle.

**Edge Cases:**

- A remediation changes no files, changes files outside scope, or causes a deterministic check to fail before review.
- The process stops after a fix but before its review record is persisted.
- A reviewer reports a new material defect after a previous remediation, or repeats the same defect with no changed evidence.
- Two review reports arrive after the orchestrator has already stopped at the cycle limit.

**Out of Scope:**

- Removing independent review.
- Unlimited autonomous repair.
- Treating a stopped third failure as a pass or an implied human approval.

### REQ-095: Record Native OpenCode Decisions Safely

**Description:** Rotta shall use the native OpenCode Question interaction for real workflow decisions and persist a fingerprint-bound decision record before advancing workflow or operation state.

**Acceptance Criteria:**

- A Strict contract Question presents exactly `Approve exact contract`, `Request changes`, and `Cancel`, along with the contract path, feature ID, baseline, and contract fingerprint.
- A side-effecting operation Question names the exact operation, target, canonical path where applicable, and intended command/effect. Setup, indexing, publication, cleanup, and destructive actions remain separate Questions.
- A selected answer advances only when the stored question/prompt ID, session ID, feature ID, contract fingerprint, policy fingerprint, final snapshot where applicable, and exactly-one-pending-action binding still match current state.
- Contract approval covers required Gherkin examples when they are material. A material later change requires a new Question rather than a free-form acknowledgement.
- Fast continuation, ordinary handoff routing, valid degraded mirror recovery, review completion, and conditional deep-review selection never create a Question.
- The legacy free-form approval tokens cannot be used as a fallback for a native Question decision.
- The native Question adapter does not replace the separate installer TUI enforcement for host setup unless a later approved change explicitly integrates those UIs.

**Edge Cases:**

- A Question answer arrives after a session restart, a replaced prompt, contract/policy/snapshot drift, or a second pending action.
- A user selects `Request changes`, dismisses the Question, supplies custom text, or responds to an old Question.
- A prior Vela setup answer is presented as consent to index a different project.
- The host records the Question answer but durable decision persistence fails.

**Out of Scope:**

- Capturing a human identity or signature.
- Generic conversational approval parsing.
- Combining distinct operational authorizations into one broad consent.

### REQ-096: Restrict Ancora And Vela To Decision-Relevant Context

**Description:** When enabled, Ancora and Vela shall provide bounded advisory context only when their output answers a named workflow question that would otherwise require exploration.

**Acceptance Criteria:**

- The orchestrator performs at most one relevant Ancora recovery at task start or resume and writes compact decision/outcome summaries at material boundaries. It does not call Ancora once per role solely to reproduce context.
- Recovered context is distilled into a compact capsule containing only current task identity, approved contract references/fingerprints, baseline/snapshot, active slice, validated evidence references, remaining risk, and safe next action.
- Ancora failure reports an evidence gap and falls back to workspace/Git state. It cannot block Fast mode or authorize continuation, approval, completion, or operations.
- Vela is called only for a named dependency, impact, ownership, architectural-flow, or unfamiliar-module question. Its existing call budgets and source fallback remain enforced.
- A Vela response is distilled to the named question, relevant symbols/files, confidence, gaps, and safe action before entering a capsule. Raw graph output is not copied between roles.
- No install, setup, index, re-index, or retry occurs automatically because a role wants Vela context.

**Edge Cases:**

- Ancora returns a stale or missing pointer.
- Vela is unavailable, stale, ambiguous, returns a result outside the declared module, or conflicts with source evidence.
- A role requests Vela without a named structural question.
- A resume occurs after the local evidence path named by Ancora was removed.

**Out of Scope:**

- Using Ancora as lifecycle authority or a transcript store.
- Using Vela as a routine test, quality metric, or prerequisite for Fast work.
- Automatic graph indexing.

## Gherkin Acceptance Contract

```gherkin
@token_efficiency @REQ-091
Scenario: An equivalent benchmark meets the efficiency target without weakening evidence
  Given three equivalent benchmark runs expose the required OpenCode usage metrics
  And each run completes applicable deterministic checks and independent final review
  When Rotta compares their median non-cache tokens and child sessions to the declared baseline
  Then the median non-cache token use is at most 1624762
  And every run has at most 25 child sessions
  And cache tokens are reported separately from the target

@token_efficiency @REQ-092 @REQ-093
Scenario: Deterministic validation gives an agent compact durable evidence
  Given a valid feature worktree and scoped verification command
  When Rotta runs deterministic scoped verification
  Then it writes the full result as local evidence with a content hash
  And it returns a bounded summary with the result, path, hash, and remediation
  And RTK output, when available, cannot replace the durable evidence

@token_efficiency @REQ-093
Scenario: Explicit RTK installation remains optional and verified
  Given the installer presents RTK as an optional integration with Skip selected by default
  When the user selects and confirms RTK installation
  Then the installer records a resolved executable path and successful rtk --version result
  And generated roles use RTK only while that executable remains resolvable
  And when installation or verification fails, the workflow retains deterministic bounded summaries without RTK

@token_efficiency @REQ-094
Scenario: A third material review failure stops autonomous remediation
  Given an approved coherent slice has completed two correction cycles
  And each changed correction diff received fresh independent review
  When the second fresh review reports another material in-scope failure
  Then Rotta creates no third implementation delegation
  And it records unresolved findings and all evidence references
  And it offers explicit resume, scope-change, or cancellation actions

@token_efficiency @REQ-095
Scenario: A stale native Question answer cannot authorize a Strict contract
  Given a native Strict-contract Question is bound to one feature and contract fingerprint
  And the contract fingerprint changes before the answer arrives
  When the user selects Approve exact contract
  Then Rotta does not advance implementation state
  And it reports the stale binding and requires a new Question

@token_efficiency @REQ-096
Scenario: A valid degraded recovery does not ask for continuation
  Given Ancora is unavailable and the local workspace/Git evidence validates the current state
  When Rotta resumes the task
  Then it reports degraded recovery with the compact evidence reference
  And it does not ask a native Question solely to continue
  And it does not call Vela unless a named structural question exists
```

## Test Strategy

- Unit-test versioned JSON command schemas, canonical argument validation, read-only publication planning, error exits, durable evidence persistence, and evidence hash binding.
- Test deterministic commands against valid, dirty, detached, missing-baseline, stale-evidence, malformed-handoff, and cross-worktree inputs.
- Test TUI default-skip, explicit selection/confirmation, successful install/version verification, installation failure, vanished recorded executable, RTK-present, RTK-absent, RTK-failure, and oversized-output cases while asserting identical underlying command/result evidence.
- Test correction-cycle counting, a fresh review after each changed diff, stop-after-second-cycle behavior, interrupted persistence, and stale concurrent report rejection.
- Test native Question decision binding across changed prompt/session/feature/contract/policy/snapshot/target state and rejection of legacy free-form fallback.
- Test Ancora/Vela call budgets, failure fallback, capsule distillation, no per-role retrieval, no automatic Vela operation, and no Fast-mode block from missing advisory context.
- Run the equivalent benchmark three times and retain source inputs, run IDs, raw exposed telemetry, comparator result, evidence references, and all deviations from the declared baseline.

## Open Questions

- None. The user selected the benchmark targets, Go command implementation, two correction cycles, and the native Question decision adapter.

## Trade-offs

- A three-run benchmark is more expensive than a single measurement, but prevents a one-off stochastic result from defining success.
- Two correction cycles preserve autonomous recovery for ordinary defects but intentionally stop persistent failures rather than spending unbounded tokens.
- Built-in commands add maintained CLI surface, but keep workflow validation testable and avoid a second shell-based authority.
- RTK improves context efficiency only when present; durable evidence and pass/fail semantics remain independent of it.
- Restricting Ancora and Vela reduces repeated context, but may leave a visible advisory-evidence gap that source inspection must cover.

## Risk Level

high — Justification: This change touches workflow authority, review/remediation routing, host-native human interaction, evidence integrity, telemetry interpretation, and operational boundaries. A flawed implementation could hide failing evidence, accept a stale decision, or make quality dependent on an unavailable advisory integration.
