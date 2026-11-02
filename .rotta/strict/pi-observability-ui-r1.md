# Rotta Pi local observability and interaction — execution contract

Revision: 1
Status: PROPOSED — implementation requires explicit approval of this revision and its SHA-256 digest.
Workspace: `/home/geen/Documents/personal/Rotta`
Mode: Strict (multi-component Pi UI, local history/usage, behavioral questions and installer-managed assets).

## Outcome and scope

In the interactive Pi host, provide a coherent Rotta-owned, Pi-theme-aware interface for: live context/model/cost status, scoped child-role activity, local session/model/cache/tool statistics, inspectable worktree changes and lazy diffs, compact read-only explorer handoffs, current work-record progress/TODOs, bounded attention/completion notifications, opt-in prompt navigation, and quiet-tool improvements. Improve `rotta_question` presentation and reliability while retaining exact governance bindings. The existing `/rotta:lazygit` remains a separate user-invoked Git interface. Changes may touch `assets/pi/`, `assets/core/` and `assets/agents/` only where needed for approved workflow text, Pi managed-asset declarations under `internal/installer/`, focused tests, and Pi documentation in `README.md`.

Implement in coherent reviewed slices behind this one approved contract: (1) host usage/delegate activity and themed status, (2) worktree diff and work-record activity, (3) question UI and notification behavior, (4) opt-in prompt navigation and explorer/compact-UI refinements. Each slice must retain a usable Pi integration and focused checks before the next. Do not interpret a passing UI mock as a live-host verification.

## Behavior and ownership

1. **Theme and composition:** Use only Pi's active theme roles and its TUI sizing APIs, including narrow-terminal fallbacks. Retain one Rotta startup header and one owner per live status/widget/shortcut; do not replace the user's global Pi theme or take over other extensions' bars. Non-TUI, RPC and headless behavior must continue to function without rendering assumptions. No new Pi UI, asset name, tool name, documentation for end users, or output carries another product's name. Preserve the previously requested source-linked comparison report as research provenance.
2. **Status and local stats:** Read session-model usage from Pi's authoritative responses where available; show model, effort, context pressure (snapshot), input/output/reasoning and cache read/write, cost only when reported, and elapsed session/child activity. Label unsupported/unavailable fields, missing context limits, and child usage separately; never infer token usage from elapsed time or add overlapping child time to parent elapsed. Report per-session and per-role aggregates without storing prompt/code/tool output in an analytics database. On-demand views and any local cache use user-local storage, bounded retention, and explicit deletion; no network or daemon.
3. **Children and work activity:** Display only current Rotta child role/status/timing/model evidence from the actual delegate lifecycle, with a bounded recent list and cleanup on session close/reload. A child does not write or advance the parent work record. Display work ID, phase, next step and blocker only when they can be read from the current `.rotta/work/` continuity record; otherwise say unavailable. A UI TODO does not create an independent authority ledger.
4. **Changes:** Show a bounded, read-only on-demand worktree/status and lazily loaded diff using shell-free Git, with tracked/untracked/staged states distinguished. Tool-observed edits can be attributed to a parent or child only with explicit provenance; external edits and uncertain authorship say unknown. Never claim a complete view of a child or repository from Pi's edit events alone. No staging, committing, pushing, patch writing or automatic LazyGit launches from this viewer.
5. **Explorer and compact tools:** Keep the existing read-only exploration role and Vela boundaries; improve its small return packet with exact paths, freshness, confidence, unknowns and next source action. Extend Rotta's existing compact tool rendering/detail overlay rather than register competing bash/edit shadow tools. Preserve full bounded diagnostics behind explicit expansion.
6. **Questions:** `rotta_prepare_question` and `rotta_question` remain the governance authority. Strict approval and exact-operation consent remain single closed-option decisions with session/workspace/artifact/revision/digest revalidation after display, cancellation and stale-answer safe stop, and no free-text authorization. Reuse UI patterns only in a Rotta-owned presenter; if an enhanced TUI is unavailable, fall back to Pi's native `select`. Multiple related *non-binding* clarifications may share one view only when independent and explicitly prepared; dependent clarifications remain sequential, each derived after its predecessor. Never batch an approval, consent or policy change with another answer. An invalid/stale answer reports a bounded reason category without printing secrets or being silently converted into approval. Abort listeners, UI components, one-time bindings and session-specific caches are cleaned up.
7. **Notifications and history:** Notify once for an actually completed run or a user-owned decision, not for every child/tool update; dedupe on session/turn identity and honor Pi mode/terminal support. Prompt history is opt-in, local-only, off by default, has bounded retention and deletion, and does not record secrets by default or become a work/approval record. Prefer Pi's own session/navigation APIs where sufficient instead of keeping a redundant prompt database.

## Non-goals and guards

- No wholesale copying or installation of another project's extensions; no new independent agent delegator or broadening role tool permissions.
- No remote telemetry, network exporter, daemon, automatic self-editing workflow policy, external account, provider billing lookup, or global theme configuration change.
- No live install/reload into `~/.pi`, package-manager action, Git operation, publication, commit or push without a separate explicit operational request. Preserve current uncommitted user work, managed-file ownership checks and existing installed-only customizations.
- Current local test fixtures and the existing research report are baselines; they are not evidence that live Pi instrumentation or attribution works.

## Acceptance checks

AC1. Same Pi theme switch and narrow/wide widths render all new surfaces with bounded rows and correct theme roles; startup header and existing details remain reachable; non-TUI modes do not open UI.
AC2. Session stats distinguish reported from missing token/cache/cost fields, do not double count parent and child responses, and reset/update correctly on session change and shutdown.
AC3. Running, failed, cancelled and completed child roles appear with bounded status, observed-vs-requested model distinctions, and no raw child transcript leak or unauthorized child tools.
AC4. Work-record activity labels unavailable evidence and does not mutate `.rotta/work`; changes view shows on-demand tracked, staged and untracked states, bounds diff content, and says unknown when authorship cannot be proven.
AC5. Ready, stale, cancelled and headless question cases retain current fail-closed bindings. Independent clarification batches collect only their prepared choices; Strict approval/operation consent remain individually bound, with no custom answer path. Repeated question calls release listeners and session-scoped state.
AC6. Completion/attention notifications are deduplicated and absent in noninteractive modes; history defaults off and supports an explicit local clear operation without recording prompts to a new database by default.
AC7. Pi installed bundle remains Rotta-owned and free of other product references; research comparison report remains unchanged. Existing compact tool, LazyGit, installer and Pi question regressions pass.
AC8. Focused Pi tests and installer/CLI tests pass against the final diff. Any unverified runtime hook or unavailable host usage is reported as such; a manual live Pi smoke is separate operational authorization.

## Behavioral examples

```gherkin
Scenario: The user inspects an active Rotta run
  Given a child reviewer is running and Pi reports no cache usage for its model
  When the user opens Rotta's status and activity view
  Then the view uses the current Pi theme and shows reviewer running and cache unavailable
  And it does not count parent usage again as child usage

Scenario: A source edit has unknown authorship
  Given a changed file was not tied to an observed Pi edit
  When the user opens the on-demand changes view
  Then its bounded diff is available and the author is marked unknown
  And opening it does not stage or modify the file

Scenario: An answer is stale after a contract edit
  Given a Strict approval shows the contract's path, revision and digest
  When the contract changes before the user selects Approve
  Then the selection cannot approve the changed contract
  And the user sees a bounded stale-binding reason

Scenario: Related independent clarifications are collected
  Given two independent, non-binding clarifications were explicitly prepared
  When the user answers both in a themed Pi view
  Then both answers retain their own question identity and cancellation status
  And no approval or operation consent is bundled with them
```

## Approval gate

Read back this exact path, revision and SHA-256 digest and request the user's explicit approval before implementation. Any material change to scope, permission, data retention, UI behavior or question authority requires a new revision and renewed approval. This contract does not itself authorize a live Pi install.
