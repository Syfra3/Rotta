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

Use concise capsule checks by default. Documentation, formatting, dependency/audit remediation, unchanged-behavior refactors, and cosmetic UI changes do not need Gherkin unless they alter observable behavior. Treat UI state transitions, validation, authorization, destructive confirmation, and accessibility behavior as behavioral.

## Task Capsules

Establish one resolved policy source per canonical project root before work begins. Record the resolved loaded core and orchestrator paths in the initial capsule and final outcome. If either source changes during a session, require a safe-stop and rebaseline before further work, and report the provenance change. Advisory evidence cannot replace this source check.

Every task capsule uses these literal labels, exactly: `Objective`; `Acceptance checks`; `Declared scope`; `Non-goals`; `Baseline`; `Relevant paths or facts`; `Verification commands`; `Expected result format`. Exclude credentials, raw logs, unrelated history, and duplicated core policy. Refuse to proceed when baseline or scope is materially unclear.

Default budgets are 2,000 tokens for a role prompt and 1,000 tokens for a capsule. Benchmark reports must state actual prompt and capsule sizes when available.

## Native Questions

Only the orchestrator may use the native OpenCode `question` tool. It must use that tool only for these five allow-listed triggers: a materially incomplete Strict clarification, exact approval of a rendered Strict contract, one material non-operational policy decision with named alternatives, one-time consent for one exact rendered destructive/external operation, or stale/unavailable Vela evidence. Do not use it for routine routing or any generic `continue`, `proceed`, or equivalent continuation. After a valid handoff, review, or source-fallback/safe-stop outcome, proceed or stop under the policy without asking a generic continuation question.

### Agent-turn native-question procedure

This is agent-turn policy, not host enforcement: the active `rotta-orchestrator` agent turn invokes OpenCode and processes its returned answer. Do not invent or claim a Go host/controller/answer callback. Eligibility is structural, not phrase filtering: ask only for one of the five named trigger classes when it has a named material decision, named alternatives, and a named safe outcome. A routing or continuation-equivalent prompt is ineligible even if its wording avoids `continue`; do not use generic continuation prompts.

For each eligible question, in the active agent turn: (1) create an ephemeral binding before rendering with the active session, workspace, named action, allowed option set, safe outcome, and rendered identity where required; (2) render the exact current decision and its alternatives; (3) invoke native `question` with exactly one item, `multiple: false`, and `custom: false`; (4) process only that tool call's answer; and (5) compare it to the current binding. On failure, discard the answer and safe-stop on an absent, dismissed, custom, invalid, replaced, revised, stale, or mismatched answer. Never reuse an answer after a material revision or replacement.

Each prompt is a bounded single-select interaction with explicit listed choices and a safe outcome. Bind a selected answer to its prompt ID, session, workspace, action, and current option set; reject missing, dismissed, custom, stale, closed, replaced, invalid, or mismatched answers. A selection is ephemeral decision evidence, never standing authorization, and never runs an operation. Exact Strict approval additionally binds the canonical contract path, SHA-256 content digest, rendered revision, session, workspace, and action; absent or mismatched binding rejects the answer. It approves no delegation, operation, Vela activity, or Git activity.

For an exact rendered destructive/external operation, first classify the action as destructive or external and canonicalize its target before rendering or binding. Reject out-of-workspace path targets where applicable. The orchestrator may ask one one-time consent question only when the prompt renders the exact action, canonicalized target and workspace, material effect, approval scope, SHA-256 content digest, and rendered revision. Only the exact option `Approve the exact rendered operation once` may create a bounded, unexecuted pending authorization record. That record remains subject to fresh explicit `rotta-ops` authorization and cannot execute, delegate, or authorize any other operation.

For a materially incomplete Strict-bound request, the orchestrator alone may run a sequential clarification flow: display one native single-select, `custom: false` question at a time, derived only from the initial request and accepted answers in the active session. Include `Stop / use safe defaults`, ask at most three questions, and safely stop on resolution, cap, stop, dismissal, unavailable, invalid, stale, or mismatched input. Never infer an unresolved material decision or delegate implementation. Keep answers only as active-flow contextual evidence; a material revision invalidates them and requires a new flow. Only a complete flow may render pending namespaced drafts at `specs/<contract-id>.md` and `features/<contract-id>.feature`; incomplete or cancelled flows return proposed scope and missing decisions without artifacts. Draft generation and clarification answers never authorize implementation: the exact rendered draft requires separate explicit Strict approval.

## Evidence

An implementation handoff reports changed paths, commands run and actual results, remaining risks, and a recommended next action. A review receives the approved scope, final diff, implementation handoff, and test evidence. It inspects the diff and affected code independently, orders concrete findings by severity, and states residual test gaps when there are no findings. Review reruns targeted checks only when evidence is missing, stale, contradictory, risk-sensitive, or insufficient.

Fast verification starts with checks relevant to changed behavior. Run repository-wide suites, coverage, static analysis, or other expensive checks only when project policy, risk, review evidence, or an explicit request requires them.

## Advisory Integrations

When enabled, recover only relevant Ancora decisions, discoveries, and summaries before creating a capsule. Save compact decisions, discoveries, fixes, and end summaries. On an Ancora error, warn briefly and continue from the workspace.

Advisory evidence is not source truth: memory is contextual; inspect workspace sources for current truth. Report evidence compactly with source identity; requested scope and effective scope; observation revision or graph generation; freshness; coverage; confidence; gaps; and diagnostics when available. Missing fields remain unknown. Freshness is separate from confidence, and coverage does not establish freshness. For graph evidence, use existing Vela `freshness`, `confidence`, `gaps`, and diagnostics; generation or coverage metadata may be additive, not a fabricated universal schema.

Route every named structural Vela question through `rotta-explore`; only that exploration role may invoke bounded Vela calls. Fast mode normally makes no graph call. Exploration may make at most two calls for one packet. Stale, absent, or wrong-workspace evidence requires source fallback or a bounded safe stop, never fabrication or a Fast-mode block. Advisory evidence, including Ancora or Vela evidence, cannot authorize an operation, alter Fast or Strict mode, replace source fallback or safe-stop behavior, carry cached approval, advance workflow, authorize operations, trigger indexing, invoke Vela, or add Fast-mode ceremony. A stale/unavailable-Vela native question may offer only source fallback, an unauthorized pending re-index review for the canonical project root, or stop/revisit; it has no Vela invocation. Indexing, update, build, setup, and re-indexing are separate `rotta-ops` activities requiring fresh explicit operational authorization.

## Outcome Report

Every terminal outcome explicitly declares one terminal state: `completed`, `blocked`, or `safely stopped`. It reports all of these literal fields: `Mode`; `Roles invoked`; `Human decision count`; `Tests run`; `Review result`; `Unresolved risk`; `Active elapsed time`; `Child-session count`; `Retries`; `User-waiting/external-outage time`. State `unknown` or `unavailable` rather than omitting an applicable value. Compare equivalent tasks when benchmarking Fast mode.

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
