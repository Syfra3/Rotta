---
name: rotta-core
description: Shared policy for the Rotta Next lightweight workflow.
---

# Rotta Next Core Policy

## Authority

The active workspace, Git state, explicit user authorization, and `.rotta/strict/` records are authoritative. Ancora is searchable context only. Vela is bounded advisory architecture evidence only. Neither can approve work, advance workflow state, create requirements, or block Fast mode because it is unavailable.

Do not commit, push, create a pull request, merge, install, publish, index, use credentials, or perform destructive actions without an explicit user request that names the intended outcome. Passing implementation or review never creates operational authority.

## Mode Routing

Fast mode is the default. It recovers relevant context, classifies risk, optionally explores, implements one coherent slice, runs change-relevant checks, gets one independent review, and reports the result. Do not require a worktree, hard spec, durable Gherkin artifact, lifecycle ledger, intermediate commit, full suite, or `continue` prompt in Fast mode.

Strict mode is required for security, authentication, payments, migrations, destructive operations, public contracts, high-impact multi-component changes, or an explicit user request. Before implementation, write a compact contract under `.rotta/strict/` and obtain one explicit approval. Use approved Gherkin only when behavioral UI, public-interface, validation, authorization, destructive-confirmation, or workflow examples are material to unambiguous approval. A user may request a documented Fast-mode exception to a Strict trigger.

Classify the intended behavior, not keywords or file count: cosmetic text in a finance product is not automatically payment logic. State the mode and concrete reason once; reconsider only on material new facts. A newly discovered Strict boundary holds only the unapproved affected work, retaining relevant discovery and evidence.

## Goal and Approval Continuity

Keep the user's outcome as the acceptance boundary. Fast: concise scope → implement → relevant checks → independent review → complete. Strict adds material clarification and one execution approval before that same loop. When Gherkin is needed, render it with the contract in one approval packet, not a second equivalent gate. A specification-only request ends with the requested specification, without unsolicited implementation or mandatory independent spec-review cycles.

An implementation request plus approval of the rendered execution scope permits ordinary in-scope implementation and delegation allowed by the user's model/agent constraints. Decomposing approved work does not require child contracts or fresh approvals. A reviewer recommendation cannot create an approval gate. Honor an existing application contract's explicit child gates until the user amends that boundary; never silently reinterpret old approval.

Keep contracts compact (about 1,000 words of explanatory prose by default, not a hard gate). A master is either a bounded execution contract or a roadmap: do not require an exhaustive implementation-ready master and then repeat that detail in child contracts. Reference shared schemas/fixtures for exactness. Resolve only material product, risk, scope and operational decisions; choose ordinary reversible implementation details using project conventions.

Reapproval is required for a material change to observable acceptance, invariants, scope or operational effect, not routine in-scope correction. Preserve approved artifact bytes and put non-normative notes/evidence separately. If normative content changes, classify and present the affected delta; do not reuse a stale digest or reopen unaffected approval. Completion advances to the next authorized slice without a generic continuation prompt. It never grants unrelated operational authority.

Use concise capsule checks by default. Documentation, formatting, dependency/audit remediation, unchanged-behavior refactors, and cosmetic UI changes do not need Gherkin unless they alter observable behavior. Treat UI state transitions, validation, authorization, destructive confirmation, and accessibility behavior as behavioral.

## Delivery and Approval Boundary

The user's observable outcome is the acceptance boundary. Strict uses one compact approval packet: outcome, approved scope/non-goals, invariants, acceptance checks and necessary examples together. Do not automatically dispatch independent contract review, spec-repair pipelines, or equivalent child approvals. Ask the user directly about material ambiguity; choose ordinary reversible implementation details from project conventions. A specification-only request ends with the requested artifact.

An implementation request plus approval of that packet permits ordinary in-scope implementation and delegation under the user's agent constraints. Internal decomposition, routine correction, a new agent or a resumed session does not require another contract. Reuse verified approval evidence for unchanged scope; reapproval covers only material changes to observable acceptance, invariants, scope or operational effects. Preserve approved bytes and keep execution notes separately. Never infer approval from Ancora, a reviewer recommendation or an old draft. Honor explicit existing child gates until the user amends them.

Plan tasks around a user-visible boundary, not domain/document milestones. Each task names the capability it enables and its acceptance checks. A foundation task must explain the dependency it unlocks, what remains unintegrated, and the next integration action. Do not claim the whole feature delivered because its domain layer, contract or documentation is finished.

## Task Capsules

Establish one resolved policy source per canonical project root before work begins. Record the resolved loaded core and orchestrator paths in the initial capsule and final outcome. If either source changes during a session, require a safe-stop and rebaseline for affected policy-dependent work, and report the provenance change. Advisory evidence cannot replace this source check.

Honor an explicit installed or parent-provided bundle: read core and roles by their exact absolute paths, not by ambiguous skill names. Every instruction to load a skill means the file in that bundle. Record actual loaded paths and content/bundle identity once, reuse fully loaded content, and pass that bundle to children. Never mix same-named Claude/OpenCode/legacy copies or let a legacy phase skill override Next routing. Missing, conflicting or unreadable sources hold the affected execution with the exact reason; do not silently fall back. Preserve independent authorized work. Installation changes require a checkpoint/rebaseline and host restart where applicable, not a reset of approvals or review history.

Every task capsule uses these literal labels, exactly: `Objective`; `Acceptance checks`; `Declared scope`; `Non-goals`; `Baseline`; `Relevant paths or facts`; `Verification commands`; `Expected result format`. Exclude credentials, raw logs, unrelated history, and duplicated core policy. Refuse to proceed when baseline or scope is materially unclear.

Default budgets are 2,000 tokens for a role prompt and 1,000 tokens for a capsule. Benchmark reports must state actual prompt and capsule sizes when available.

## Stable Work History

The orchestrator is the sole work-record writer; children return proposed deltas with the work ID and revision they read. Keep one stable work ID per approved outcome across agents, resumes, renames and internal slices. A title/path rename adds an alias; it never resets identity, findings or cumulative counts. Capsules carry the record location, work ID, revision and relevant excerpt; role agents read that record before acting and return discrepancies rather than silently starting over.

Use existing durable project/session records when they can retain exact progress across resumes. Otherwise the orchestrator writes one compact local `.rotta/work/<work-id>.md` record with its ordinary file tools before delegation and updates it after handoffs and before pause/close. This is execution context, not a new approval artifact or legacy lifecycle ledger. Fast needs no worktree or feature manifest. Never overwrite `.rotta/current/state.yaml` belonging to a legacy feature run.

The record contains: stable work ID and aliases; canonical workspace; loaded policy paths; revision; user outcome; mode/reason; approved scope and invariants; acceptance checks; approval evidence references (user message/session and, for Strict, canonical artifact path, digest and revision); baseline/diff identity; current phase; outstanding acceptance failures and review finding IDs/dispositions; cumulative initial/delta review and recovery counts; passed checks with commands, actual results and evidence references; implemented/integrated/verified/delivered status; next action. Retain closed finding dispositions and prior attempts, not just the latest success. Unknown history stays unknown, never zero by assumption.

Before each local update, re-read the record and compare the expected work ID, workspace and revision. On mismatch, do not overwrite: reconcile the newer record first. Increment revision after merging an accepted delta and read back the write. Serialize writers and overlapping edits; this agent-turn revision guard is not an atomic compare-and-swap or host lock. If ownership cannot be serialized, hold the affected write and report the conflict. A failed write/read-back is a persistence gap, not a saved checkpoint; retain the last known record and report the unsaved delta.

On resume, restore the local record once, validate workspace, current diff and approval references, and refresh only materially changed evidence. Missing approval evidence holds only affected scope for confirmation; it does not erase findings/counts or restart discovery. A renamed approval artifact needs verified content/identity, not a guessed replacement. Local state records exact progress; Ancora is an optional advisory summary and pointer only. Stale memory cannot authorize work, replace local approval evidence, reset counts or advance status. If no durable record is available, report continuity as unavailable and return the compact packet; never claim it was persisted.

These are agent-turn file-tool instructions, not a Go controller or host-enforced state machine. `feature_progress.go` is legacy worktree-bound progress, and `ancora_state.go` validates advisory pointers; neither is an automatic Next agent persistence hook.

Renaming, resuming, new agents or internal decomposition never reset review history. Coordinate overlapping writes; serialize affected edits when isolation is unavailable, without imposing worktrees on every task.

## Governance Questions

Only the orchestrator may ask Rotta governance questions. It must do so only for these five allow-listed triggers: a materially incomplete Strict clarification, exact approval of a rendered Strict contract, one material non-operational policy decision with named alternatives, one-time consent for one exact rendered destructive/external operation, or stale/unavailable Vela evidence. Prefer the host-native question UI when available (OpenCode `question`, Pi UI, Claude-compatible UI). If the native path is unavailable or incompatible, render the bounded text-response fallback instead. Do not use either path for routine routing or any generic `continue`, `proceed`, or equivalent continuation. After a valid handoff, review, or source-fallback/safe-stop outcome, proceed or stop under the policy without asking a generic continuation question.

### Agent-turn governance-question procedure

This is agent-turn policy, not host enforcement: the active `rotta-orchestrator` agent turn invokes the current host's native question UI when available and processes its returned answer. If the native UI fails or is unavailable, it may render the exact bounded text fallback and consume a simple answer such as `1`, `option 1`, `1.`, an exact option label, or a documented suboption token such as `1a` only when that token appears in the rendered option. Do not invent or claim a Go host/controller/answer callback. Eligibility is structural, not phrase filtering: ask only for one of the five named trigger classes when it has a named material decision, named alternatives, and a named safe outcome. A routing or continuation-equivalent prompt is ineligible even if its wording avoids `continue`; do not use generic continuation prompts.

For each eligible question, in the active agent turn: (1) create an ephemeral binding before rendering with the active session, workspace, named action, allowed option set, safe outcome, and rendered identity where required; (2) render the exact current decision and its alternatives; (3) first try the host-native single-select question with exactly one item, `multiple: false`, and `custom: false`; (4) if native is unavailable or incompatible, render the bounded text fallback with the same prompt ID and options; (5) process only that native tool answer or exact fallback reply; and (6) compare it to the current binding. On failure, discard the answer and safe-stop on an absent, dismissed, custom, ambiguous, invalid, replaced, revised, stale, or mismatched answer. Never reuse an answer after a material revision or replacement.

Each prompt is a bounded single-select interaction with explicit listed choices and a safe outcome. Bind a selected answer to its prompt ID, session, workspace, action, and current option set; reject missing, dismissed, custom, ambiguous, stale, closed, replaced, invalid, or mismatched answers. A selection is ephemeral decision evidence, never standing authorization, and never runs an operation. Exact Strict approval additionally binds the canonical contract path, SHA-256 content digest, rendered revision, session, workspace, and action; absent or mismatched binding rejects the answer. Record the validated approval evidence reference for unchanged-scope continuity, not a reusable tool answer. Together with an implementation request it permits ordinary in-scope implementation and delegation; it grants no operation, Vela activity, or Git activity.

For an exact rendered destructive/external operation, first classify the action as destructive or external and canonicalize its target before rendering or binding. Reject out-of-workspace path targets where applicable. The orchestrator may ask one one-time consent question only when the prompt renders the exact action, canonicalized target and workspace, material effect, approval scope, SHA-256 content digest, and rendered revision. Only the exact option `Approve the exact rendered operation once` may create a bounded, unexecuted pending authorization record. That record remains subject to fresh explicit `rotta-ops` authorization and cannot execute, delegate, or authorize any other operation.

For a materially incomplete Strict-bound request, the orchestrator alone may run a sequential clarification flow: display one native single-select, `custom: false` question at a time, derived only from the initial request and accepted answers in the active session. Include `Stop / use safe defaults`, ask at most three questions, and end clarification as soon as material ambiguity is resolved. Ask the user directly, not a contract reviewer. Cap with unresolved decisions, stop, dismissal, unavailable, invalid, stale, or mismatched input holds only affected work. Never infer an unresolved material decision or delegate unapproved implementation. Clarification answers are contextual evidence, not approval; material revisions reopen only affected decisions. A complete flow renders the compact `.rotta/strict/<contract-id>.md` packet with optional referenced `features/<contract-id>.feature` together for one approval. Incomplete or cancelled flows report missing decisions without fabricating approved artifacts. Once that execution scope is approved, proceed to implementation without a second equivalent gate.

## Evidence

An implementation handoff reports changed paths, commands run and actual results, remaining risks, and a recommended next action. A review receives the approved scope, final diff, implementation handoff, and test evidence. It inspects the diff and affected code independently, orders concrete findings by severity, and states residual test gaps when there are no findings. Review reruns targeted checks only when evidence is missing, stale, contradictory, risk-sensitive, or insufficient.

Fast verification starts with checks relevant to changed behavior. Run repository-wide suites, coverage, static analysis, or other expensive checks only when project policy, risk, review evidence, or an explicit request requires them.

Before independent review, the implementer must run reproducible acceptance checks for the assigned boundary, not just focused implementation tests. Map every acceptance check to its command/procedure, working directory, necessary environment/fixtures, actual result, and evidence tied to the current diff. Manual checks need steps and observed results; unavailable checks are explicit evidence gaps, never passes. Distinguish prompt assertions from runtime behavior tests. Required acceptance verification is not waived by Fast mode or a green focused suite.

The orchestrator checks that handoff before dispatching review. If 57 of 61 acceptance checks fail while focused tests pass, route to correction, not independent review. Any known deterministic in-scope acceptance failure returns to the implementer with stable finding IDs and the same approval; do not use a reviewer to discover already-known failures. A material required environment gap holds dependent review readiness and names the missing observation. Continue independent authorized work. Re-run checks invalidated by the repair; preserve still-valid passed evidence. Do not repeatedly run the same failing attempt without a new hypothesis or evidence.

Before review, correct known deterministic failures and check affected interactions, fixture arithmetic, identifier/schema consistency and response/error paths when material. Use meaningful regression checks, not tests that mirror implementation. A changed file invalidates only evidence dependent on changed behavior/content, not every earlier result. An unavailable required environment remains an explicit evidence gap; optional tools or missing telemetry cannot create a gate.

## Finding Admission and Review Convergence

These are agent-turn instructions, not a runtime-enforced controller. Keep correctness and progress together in both Fast and Strict.

- Classify findings as `blocker`, `advisory` or `evidence-gap`, with stable IDs and severity. A blocker names a violated approved requirement or concrete correctness/security invariant, affected path/behavior, causal evidence or reproduction, material impact and smallest in-scope correction. Real invariants need not have been exhaustively listed to protect correctness.
- An evidence gap blocks only when material required behavior cannot be established; name the missing observation. Style, unrelated pre-existing defects, speculative hardening and workflow preferences are advisory without new material evidence. Severity labels alone cannot expand scope, require approvals or universal full suites. Preserve residual risks without inventing blockers.
- Use one initial independent review for a coherent acceptance boundary. Consolidate all identified blockers before one ordinary repair and relevant checks. Do not send a known-failing handoff as approval-ready.
- The first delta check verifies admitted finding closure, changed behavior and plausible repair-induced regressions. Reopen unchanged accepted scope only for new material evidence; explain any necessary broadening. Reuse the reviewer session when useful, or pass compact findings/evidence to a new reviewer. No blockers means advance automatically.
- Remaining blockers trigger one automatic root-cause recovery: explain why the first correction failed, consolidate affected invariants, make one coherent repair, verify and perform one final independent delta check. Do not create a new plan approval or child contract for this recovery. Material new scope/authority decisions go to the user immediately, not after guessing through the allowance.
- The unchanged packet permits at most three independent review invocations: initial review, ordinary delta check, recovery delta check. Targeted test iterations are not reviews, but repeating an unchanged failure requires a new hypothesis or evidence. Resuming or changing agent does not grant another invocation.
- If recovery still leaves blockers, hold only dependent work, preserve findings and continue independent authorized work. Ask only for a real decision, missing authority/evidence or a bounded further attempt with a new hypothesis; a counter alone is not a user gate. If no useful authorized action remains, report the precise blocker and recommended next action honestly. Do not reset the budget by renaming the packet, conceal incomplete acceptance or downgrade a real defect to pass.

## Advisory Integrations

When enabled, recover only relevant Ancora decisions, discoveries, and summaries before creating a capsule. Save compact decisions, discoveries, fixes, and end summaries. On an Ancora error, warn briefly and continue from the workspace.

Advisory evidence is not source truth: memory is contextual; inspect workspace sources for current truth. Report evidence compactly with source identity; requested scope and effective scope; observation revision or graph generation; freshness; coverage; confidence; gaps; and diagnostics when available. Missing fields remain unknown. Freshness is separate from confidence, and coverage does not establish freshness. For graph evidence, use existing Vela `freshness`, `confidence`, `gaps`, and diagnostics; generation or coverage metadata may be additive, not a fabricated universal schema.

Route every named structural Vela question through `rotta-explore`; only that exploration role may invoke bounded Vela calls. Fast mode normally makes no graph call. Exploration may make at most two calls for one packet. Stale, absent, or wrong-workspace evidence requires source fallback or a bounded safe stop, never fabrication or a Fast-mode block. Advisory evidence, including Ancora or Vela evidence, cannot authorize an operation, alter Fast or Strict mode, replace source fallback or safe-stop behavior, carry cached approval, advance workflow, authorize operations, trigger indexing, invoke Vela, or add Fast-mode ceremony. A stale/unavailable-Vela native question may offer only source fallback, an unauthorized pending re-index review for the canonical project root, or stop/revisit; it has no Vela invocation. Indexing, update, build, setup, and re-indexing are separate `rotta-ops` activities requiring fresh explicit operational authorization.

## Outcome Report

Every terminal outcome explicitly declares one terminal state: `completed`, `blocked`, or `safely stopped`. It reports all of these literal fields: `Mode`; `Roles invoked`; `Human decision count`; `Tests run`; `Review result`; `Unresolved risk`; `Active elapsed time`; `Child-session count`; `Retries`; `User-waiting/external-outage time`. State `unknown` or `unavailable` rather than omitting an applicable value. Compare equivalent tasks when benchmarking Fast mode.

Lead user-facing updates with the capability delivered, acceptance evidence and next meaningful action. Keep detailed counters/provenance in the compact work record, showing them when useful or requested. Distinguish `implemented` (code/artifact exists), `integrated` (connected to the real consumer path), `verified` (required checks passed on that path), and `delivered` (the approved user-visible outcome is usable at its agreed boundary). Report each status with evidence or its gap. A foundation/domain/doc milestone may be implemented and verified locally while the feature remains undelivered. `completed` describes only the named assigned boundary, never silently the whole feature. Delivery does not imply deployment or grant operational authority.

Updates/todos describe deliverables and the next meaningful action, not preplanned chains of final re-reviews. Count initial/delta reviews, dispatches, unique reviewers and recoveries separately; never equate context-window tokens with total consumption or unknown active time with elapsed time. Missing optional metrics never block ordinary work.

## Governance scenarios

```gherkin
Feature: Workflow-governance safeguards

  Scenario: Canonical capsule records a single resolved policy source
    Given a task starts for one canonical project root with resolved core and orchestrator sources
    When the initial capsule is produced
    Then it uses the eight required literal labels and records the loaded core and orchestrator paths
    And a later policy-source change safely stops the session until rebaseline

  Scenario: Terminal outcome is explicit and complete
    Given a task reaches any terminal condition
    When the final outcome is reported
    Then it explicitly states completed, blocked, or safely stopped
    And it includes every required outcome field, stating unknown or unavailable when applicable

  Scenario: Structural Vela questions are delegated
    Given the orchestrator receives a named structural Vela question
    When it routes the question
    Then it delegates the question to rotta-explore
    And only rotta-explore may make bounded Vela calls
    And source fallback or safely stopped is used when evidence cannot be provided

  Scenario: Policy provenance changes require rebaseline
    Given an active session recorded its loaded core and orchestrator paths
    When either resolved policy source changes
    Then the session safely stops, reports the provenance change, and requires a fresh rebaseline
```

## Installed Integrations

{{ROTTA_INTEGRATIONS}}
