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

Every subagent call must include: objective; acceptance checks; declared scope; non-goals; baseline; relevant paths or facts; verification commands; and expected result format. Exclude credentials, raw logs, unrelated history, and duplicated core policy. Refuse to proceed when baseline or scope is materially unclear.

Default budgets are 2,000 tokens for a role prompt and 1,000 tokens for a capsule. Benchmark reports must state actual prompt and capsule sizes when available.

## Evidence

An implementation handoff reports changed paths, commands run and actual results, remaining risks, and a recommended next action. A review receives the approved scope, final diff, implementation handoff, and test evidence. It inspects the diff and affected code independently, orders concrete findings by severity, and states residual test gaps when there are no findings. Review reruns targeted checks only when evidence is missing, stale, contradictory, risk-sensitive, or insufficient.

Fast verification starts with checks relevant to changed behavior. Run repository-wide suites, coverage, static analysis, or other expensive checks only when project policy, risk, review evidence, or an explicit request requires them.

## Advisory Integrations

When enabled, recover only relevant Ancora decisions, discoveries, and summaries before creating a capsule. Save compact decisions, discoveries, fixes, and end summaries. On an Ancora error, warn briefly and continue from the workspace.

Use Vela only for a named structural question about dependencies, impact, ownership, architectural flow, or an unfamiliar module. Fast mode normally makes no graph call. Exploration may make at most two calls for one packet; review may make one targeted call at an architectural boundary. A packet contains relevant symbols or files, risks, confidence, gaps, and a safe next action. Missing or stale graph data requires source fallback, never fabrication or a Fast-mode block. Indexing or re-indexing is an explicit `rotta-ops` action requiring user consent.

## Outcome Report

Every completed task reports: mode, roles invoked, human decision count, tests run, review result, unresolved risk, active elapsed time, child-session count, retries, and any known user-waiting or external-outage time. Compare equivalent tasks when benchmarking Fast mode.

## Installed Integrations

{{ROTTA_INTEGRATIONS}}
