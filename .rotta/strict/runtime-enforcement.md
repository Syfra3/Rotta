# Runtime enforcement contract

**Baseline:** `feature/workflow-runtime-enforcement@9a6ff507a243b1cad8bcfd54078e089a6da232d1`  
**Approval:** Approve runtime enforcement contract  
**Scope:** OpenCode only. Implementation may change the OpenCode runtime plugin/integration and its workflow/installer coverage; this approval does not authorize other hosts.

## Contract

1. Before enablement, run and retain a compatibility fixture/probe that verifies the installed OpenCode version's `tool.execute.before` registration, invocation timing, binding, denial/result semantics, and error behavior. Enable enforcement only on a passing probe; unsupported, missing, ambiguous, or failed probes are **fail-closed** (no dispatch and a clear unsupported-host reason).
2. The hook enforces the resolved effective configuration path (not a guessed/default path), preserves all user plugins, and runs before tool dispatch. It must deny before dispatch for: exhausted/over-budget reserve, invalid or stale binding, illegal route, and a second remediation attempt.
3. Maintain a separate crash-safe ledger at `.rotta/current/runtime-enforcement.json` (not this contract). Its durable, idempotent transitions are `refresh`, `handoff`, `reserve`, `commit`, `cancel`, and `remediation`; each call may be replayed safely after a crash. Dispatch is permitted only after a successful pre-dispatch `reserve`; success commits, non-dispatch/failure cancels, and remediation is limited to one legal transition before handoff/terminal denial.
4. Preserve existing workflow ownership across refresh/handoff, reject stale or cross-workflow bindings, and make recovery reconstruct a consistent reservation/terminal state without double charging, duplicate dispatch, or a second remediation.

## Acceptance checks

- A passing compatibility fixture/probe is required before OpenCode enforcement can be enabled; every unsupported-host path fails closed before dispatch.
- The verified `tool.execute.before` hook denies each prohibited condition before dispatch and reports the applicable reason.
- Effective config resolution retains user plugins while adding/enabling only the required OpenCode integration.
- Ledger transitions are durable and idempotent, including repeated reserve/commit/cancel/refresh/handoff/remediation calls and interrupted recovery.
- Reserve/commit/cancel ordering, route/binding validation, budget accounting, handoff, and the one-remediation limit are covered by focused workflow/installer tests.

## Required verification (implementation phase)

1. Focused workflow and installer tests.
2. OpenCode plugin compatibility fixture/probe.
3. `go vet ./...` (or the repository's equivalent vet command).
4. Diff check confirming only approved OpenCode runtime-enforcement paths changed and user plugins remain preserved.

## Exclusions

This approval authorizes only the approved OpenCode runtime plugin/integration, the separate runtime ledger at `.rotta/current/runtime-enforcement.json`, directly necessary workflow, installer, and effective-config-resolution code, and focused tests, fixtures, and compatibility probe work. No Vela action. No non-OpenCode host support. No unrelated host or configuration operation. No commit, push, rebase, merge, or PR creation. No work outside runtime enforcement. The ledger is an implementation-phase artifact and must remain separate from this file.
