# TDD Implementation Plan: Workflow Artifact Lifecycle

Status: planning only. Do not implement production code or tests from this artifact until the blockers below are resolved.

Source contracts:
- Spec: `specs/workflow_artifact_lifecycle.md`
- Feature: `features/workflow_artifact_lifecycle.feature`
- Design: `design/workflow_artifact_lifecycle.md`

Protected existing artifacts:
- `specs/hard_spec.md`
- `features/installer_recovery.feature`
- `specs/.approved`

## Implementation Gate Blockers

Resolve these before `bob-impl` starts coding:

- Dirty working tree: the current branch has intentional untracked artifacts. At planning time these include `.vela/`, `design/`, `features/workflow_artifact_lifecycle.feature`, and `specs/workflow_artifact_lifecycle.md`. After this plan is written, `tasks/workflow_artifact_lifecycle.md` is also expected to be untracked until the orchestrator decides how to include it.
- Untracked contract artifacts: `specs/workflow_artifact_lifecycle.md`, `features/workflow_artifact_lifecycle.feature`, and `design/workflow_artifact_lifecycle.md` are contract/design artifacts, not disposable cleanup debris. They must be tracked for review, or explicitly kept pending by a reviewed workflow decision, before implementation relies on them.
- `.vela/` cache hygiene: `.vela/` is local generated graph/cache state unless deliberately promoted. Decide whether to ignore it, leave it local outside the review set, or make a reviewed project-artifact decision. Do not let `.vela/` become part of the contract artifact commit by accident.
- Approval marker scoping: the legacy `specs/.approved` marker is not safe as a global approval gate. Before implementing scenarios from this contract, add scoped approval records or an equivalent unambiguous gate that names the approved spec path, feature path, and scenario references. Treat this contract as pending until that scoped gate approves `features/workflow_artifact_lifecycle.feature#SCN-012` through `#SCN-024`.
- Existing installer recovery artifacts must remain untouched. Do not rewrite, move, delete, or reinterpret `specs/hard_spec.md`, `features/installer_recovery.feature`, or the existing `specs/.approved` as part of this planning step or as a cleanup shortcut.

## First Safe Slice

First recommended implementation scenario: `SCN-018`.

Reason: implementation must fail closed before any other scenario can safely consume the new contract files. `SCN-018` proves that pending generated contracts do not pass the implementation gate and that the legacy `specs/.approved` marker cannot approve unrelated pending scenarios.

Prerequisite refactor, only if needed:
- Introduce or isolate a small approval-gate seam that can answer whether a given contract scope is approved.
- Fence the legacy `specs/.approved` behavior to its known existing scope instead of treating it as global.
- Keep this refactor behavior-preserving except for explicit fail-closed ambiguity handling.
- Pin current behavior with tests before changing it if approval handling is already embedded in CLI, TUI, installer instructions, or workflow text generation.
- If there is no approval-gate code yet, skip a standalone refactor and introduce the approval resolver as the production code driven by the `SCN-018` test.

## Scenario-Sized TDD Backlog

Each scenario is one Bob implementation slice. Start with the failing test for that scenario, implement only enough production behavior to pass it, then run the verification commands listed below.

| Order | Scenario | Behavior to Drive | Test Proof to Write Later | Notes |
|-------|----------|-------------------|---------------------------|-------|
| 1 | `SCN-018` | Pending generated contracts fail the implementation gate unless a scoped approval record names them. | Unit test for approval resolver with `specs/.approved` present but no scoped record; integration or CLI test for implementation request reporting human approval required. | First slice. Fail closed on ambiguity. |
| 2 | `SCN-012` | Active hard spec and feature files are repository-file source of truth and must be tracked or explicitly approved before implementation. | Unit test for artifact source classification; git-backed integration test using a temp repo to distinguish tracked contract files from untracked files; CLI test only if a command reports readiness. | Builds on scoped approval. |
| 3 | `SCN-013` | Namespaced workflow-policy artifacts are generated without overwriting existing active installer recovery contracts. | Unit test for path selection; integration test with existing `specs/hard_spec.md` and `features/installer_recovery.feature` asserting unchanged content and new namespaced paths. | Protect legacy artifacts. |
| 4 | `SCN-019` | Untracked active contracts are surfaced for tracking or explicit approval, never deleted to make the tree clean. | Git-backed integration test with untracked approved spec/feature files; cleanup/readiness plan asserts `track` or `approve explicitly`, not `delete`. | Do not implement deletion logic as the default path. |
| 5 | `SCN-016` | Ancora state contains pointers, phase/status, risk, requirements, scenario IDs, and optional hashes, not authoritative full contract text. | Unit test for state serialization; assert paths and IDs are present and full Markdown/Gherkin bodies are absent. | Ancora remains pointer/state/index only. |
| 6 | `SCN-017` | Repository content wins when an Ancora pointer is stale. | Unit test for pointer validation; integration-style test with renamed or changed repo file and stale pointer; assert repair/report behavior and no overwrite from memory text. | Prefer reporting over destructive repair if uncertain. |
| 7 | `SCN-015` | Tests and QA traces reference stable scenario IDs plus feature identity. | Unit test for scenario tag parser and trace validator; example test metadata/subtest naming convention includes `features/workflow_artifact_lifecycle.feature#SCN-015`. | Do not renumber scenarios when ordering changes. |
| 8 | `SCN-023` | QA and strict TDD planning enumerate approved scenarios from repository feature files, excluding pending scenarios. | Unit test for Gherkin scanner; integration test with one approved scoped record and one pending feature; QA planning output references feature path plus SCN ID. | No browser QA unless behavior becomes browser-observable. |
| 9 | `SCN-014` | Implemented feature files remain active regression contracts after verification. | Unit test for lifecycle classifier; integration test for completion/archive preparation asserting active feature remains under `features/`. | Implementation complete is not retirement. |
| 10 | `SCN-020` | Retired, superseded, or process-only artifacts may archive only with explicit retirement reason while active contracts stay discoverable. | Unit test for archive eligibility; integration test that archive plan moves only retired/process-only artifacts and writes or reports a retirement reason. | Keep active regression contracts out of archive moves. |
| 11 | `SCN-021` | Local graph/cache artifacts such as `.vela/` are excluded unless intentionally promoted. | Unit test for artifact classifier; integration test for review-set preparation excluding `.vela/` while keeping `specs/` and `features/`. | Update ignore policy only if the project chooses that path. |
| 12 | `SCN-022` | Backup outputs and sensitive config captures are rejected as workflow artifacts. | Unit test with representative backup, restore, user config, token-like, and private machine-state paths; integration or CLI/TUI warning test if cleanup guidance is user-facing. | Fail closed for uncertain sensitive files. |
| 13 | `SCN-024` | Cleanup guidance labels artifacts as track, keep pending, archive, ignore, or delete, and never labels active contracts for deletion only to satisfy clean-tree rules. | Unit test for classification report; CLI or TUI output test if cleanup guidance is exposed to users; QA planning check that labels are actionable and traceable. | Final policy aggregation slice. |

## Per-Slice TDD Rules

- The failing test must name the scenario ID and feature identity, for example `SCN018` plus `features/workflow_artifact_lifecycle.feature` in the test name, subtest name, or metadata.
- Each slice should change only the minimum production surface needed for that scenario.
- Do not combine scenarios unless the second scenario is a direct assertion inside the same behavior and does not expand the diff materially.
- If a slice discovers missing production seams, add the smallest seam needed for the test and keep behavior changes scoped to that scenario.
- Keep `.vela/`, backups, generated caches, and local config captures out of review unless an explicit project-artifact decision says otherwise.
- Treat active contract deletion, global approval fallback, and Ancora full-text recovery as fail-closed paths.

## Expected Test Types By Concern

- Approval gate: unit tests for scope resolution; CLI/integration tests if implementation readiness is exposed through commands.
- Git tracking and dirty-tree readiness: temp git repository integration tests, with no reliance on the developer's real working tree.
- Namespaced artifact generation: unit tests for path decisions and integration tests that assert legacy files are unchanged.
- Pointer-only Ancora state: unit tests around serializers/resolvers using fake memory state, not live Ancora.
- Gherkin and scenario traceability: parser/validator unit tests and naming-convention tests that reference feature path plus SCN ID.
- Archive lifecycle: unit tests for classification plus integration tests for archive-plan output; avoid destructive filesystem moves until behavior is proven in temp directories.
- Cache and sensitive-output handling: unit tests with representative paths and content markers; CLI/TUI tests only if classification is displayed to users.
- QA planning: repository feature scanner tests that enumerate approved scenarios and exclude pending contracts.

## Recommended Work-Unit Strategy

Do not commit from this planning step. When implementation starts, use one work unit per scenario where practical:

- Work unit 0: planning and contract hygiene only, including tracking or explicitly pending the spec, feature, design, and this task file; no production behavior.
- Work unit 1: `SCN-018` scoped approval gate and fail-closed pending-contract behavior.
- Work unit 2: `SCN-012` repository source-of-truth and tracked-contract readiness.
- Work unit 3: `SCN-013` namespaced artifact generation protection.
- Work unit 4: `SCN-019` untracked active contract cleanup guidance.
- Work unit 5: `SCN-016` pointer-only Ancora state.
- Work unit 6: `SCN-017` stale pointer validation and repair/report behavior.
- Work unit 7: `SCN-015` scenario trace validation.
- Work unit 8: `SCN-023` QA/TDD scenario enumeration.
- Work unit 9: `SCN-014` active regression contract retention.
- Work unit 10: `SCN-020` archive eligibility and retirement reasons.
- Work unit 11: `SCN-021` local graph/cache exclusion.
- Work unit 12: `SCN-022` backup and sensitive config rejection.
- Work unit 13: `SCN-024` cleanup guidance aggregation.

If a future diff grows beyond a focused reviewable size, split by lifecycle concern rather than by package. Keep tests and production changes for each scenario in the same work unit.

## Verification Commands After Each Slice

Run these after every implementation slice:

```sh
go test ./...
make fmt-check
make lint
```

The project `Makefile` currently defines both `fmt-check` and `lint`, so Bob should treat all three commands as expected verification unless a future environment blocker is explicitly documented.

Optional review checks before asking for review:

```sh
git status --short
git diff --stat
```

Do not use a clean-tree check as a reason to delete active or pending contract artifacts.

## Handoff To bob-impl

Before launching `bob-impl`, the orchestrator should decide and record:

- Whether `specs/workflow_artifact_lifecycle.md`, `features/workflow_artifact_lifecycle.feature`, `design/workflow_artifact_lifecycle.md`, and `tasks/workflow_artifact_lifecycle.md` are tracked for review or explicitly kept pending.
- How `.vela/` is excluded from the review set without deleting it in this workflow unless the user asks.
- What scoped approval record format or equivalent approval gate is approved for this contract.
- Whether `SCN-018` is approved as the first implementation scenario.
