# Hard Spec: Scoped Rotta Workflow Lifecycle

## Adversarial Pre-Mortem
- Failure mode 1: a new task reads an old global approval marker or TDD log and is blocked by unrelated scenarios from another feature or worktree.
- Failure mode 2: cleanup deletes active workflow state after an interrupted session, leaving Ancora unable to reconstruct the full contract or evidence needed to resume.
- Failure mode 3: an archive silently becomes an active queue again, or old archives are judged as current work, recreating cross-task contamination.

## Hidden Assumptions
- Requirements and Gherkin feature documentation are durable project artifacts and may be committed, while execution state is local workflow data.
- Concurrent work uses separate Git worktrees; `.rotta/current/` therefore belongs to one active submission in one worktree.
- Ancora is long-term memory and can retain compact resume records, pointers, and evidence references, but is not a replacement for the full workspace contract or execution files.
- The user can explicitly choose to abandon or cancel a submission; an interrupted process is not terminal.
- A completed, abandoned, or cancelled lifecycle no longer needs to participate in future Judge scope.
- The existing global `specs/.approved` and global `.rotta/tdd-log.md` artifacts may exist from older Rotta versions and must not silently control a new submission.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Keep one global approval marker and TDD log | Cross-contaminates unrelated tasks and requires reconstructing old evidence. |
| Store the complete workflow history only in Ancora | Memory availability is optional and full artifacts cannot be safely reconstructed from compact state. |
| Delete `.rotta/current/` at every session close | An interrupted session loses resumable state and forces duplicate work. |
| Keep every execution log permanently in the repository | Pollutes the project with transient workflow data and turns historical logs into accidental gates. |
| Permanently retain every local archive | Prevents clutter during long-running projects and gives old execution data no defined lifecycle. |

## Summary
Rotta must isolate execution state to an explicitly identified current submission in the active Git worktree. Durable requirements/specifications and Gherkin feature documentation remain project artifacts, while manifest, phase state, TDD evidence, and Judge reports live in ignored `.rotta/current/` files. Ancora stores a compact long-term resume index with pointers and evidence references. Judge evaluates only the current submission manifest and never treats global legacy approval or TDD files as active scope. On terminal completion, abandonment, or cancellation, Rotta archives the current execution files locally for a bounded retention period and then deletes them; archived submissions are never active review scope.

## Requirements

### REQ-032: Create an isolated current submission
**Description:** Starting work on a requirement creates one identifiable submission manifest and execution state for the active worktree.
**Acceptance Criteria:**
- `.rotta/current/manifest.yaml` identifies the submission, feature documentation paths, scenario IDs, worktree, and lifecycle status.
- `.rotta/current/state.yaml` identifies the current phase, completed work, remaining work, last action, and safe resume point.
- A new submission does not read or inherit scenario scope from global legacy approval or TDD files.
- The submission manifest is the only source used to determine the active Judge scope.
**Edge Cases:**
- A current submission already exists when a new requirement is requested.
- The manifest is missing, malformed, or names a missing feature file.
- Two developers work concurrently in separate Git worktrees.
**Out of Scope:**
- Supporting concurrent writers in the same Git worktree.
- Changing the durable requirement and Gherkin documentation model.

### REQ-033: Preserve resumable interrupted state
**Description:** Rotta must preserve enough local state to resume an interrupted non-terminal submission without recreating documents or inventing progress.
**Acceptance Criteria:**
- Interrupted or in-progress submissions retain `manifest.yaml`, `state.yaml`, `tdd-log.md`, and any current Judge report.
- Resume reads the current submission files before acting.
- Resume identifies completed, remaining, and blocked work from state rather than from conversation todos.
- Ancora, when available, receives only a compact pointer/status record containing submission identity, phase, status, scenario progress, last test/action, file references, and evidence references.
- If Ancora is unavailable, the same local files remain sufficient for resume.
**Edge Cases:**
- Ancora is unavailable, stale, or contains a pointer to deleted local files.
- The process stops during implementation or review.
- The worktree has uncommitted changes from the active submission.
**Out of Scope:**
- Reconstructing full specs, Gherkin, TDD logs, or reports from Ancora alone.

### REQ-034: Scope Judge review to the current submission
**Description:** Judge must review only the scenarios and evidence named by the current submission manifest.
**Acceptance Criteria:**
- Judge loads scenario IDs from `.rotta/current/manifest.yaml`.
- Judge validates TDD evidence only for those scenario IDs.
- Judge does not scan global `specs/.approved`, global `.rotta/tdd-log.md`, unrelated feature files, or archived submissions as active scope.
- Legacy global lifecycle artifacts may produce a warning but cannot block or expand current review scope.
- Missing evidence for unrelated historical scenarios cannot fail the current submission.
**Edge Cases:**
- Legacy global markers list scenarios from another feature.
- The current manifest references a scenario with missing evidence.
- An archive contains the same scenario ID as the current feature.
**Out of Scope:**
- Automatic migration of legacy global markers into current submission scope.

### REQ-035: Archive terminal execution state without repository pollution
**Description:** Terminal lifecycle state must leave the active queue while remaining temporarily recoverable without being committed.
**Acceptance Criteria:**
- `completed`, `abandoned`, and `cancelled` are terminal statuses; `interrupted` and `in_progress` are not.
- Terminal transition moves `.rotta/current/` to ignored `.rotta/archive/<submission-id>/`.
- A terminal submission is removed from active Judge scope.
- Archives are retained for a configurable bounded period, then deleted by cleanup; they are never automatically treated as abandoned work or active scope.
- Durable requirement/specification and Gherkin files are not deleted by lifecycle cleanup.
- Completion cleanup occurs only after required feature changes are safely committed; interruption never triggers terminal cleanup.
**Edge Cases:**
- The user closes a session without choosing a terminal status.
- Archive destination already exists.
- Cleanup runs while a submission is being resumed.
- The user explicitly requests immediate deletion of a terminal archive.
**Out of Scope:**
- Committing `.rotta/current/` or `.rotta/archive/` by default.

### REQ-036: Keep long-term memory compact and referential
**Description:** Ancora integration must preserve resumability without making memory the authoritative document store.
**Acceptance Criteria:**
- Ancora records submission ID, status, phase, scenario progress, last test/action, local state pointer, and evidence references.
- Ancora does not receive full hard specs, Gherkin files, TDD logs, or Judge reports as lifecycle state.
- Ancora records terminal summaries separately from active resume pointers.
- If local files are removed after retention cleanup, Ancora clearly indicates that full execution artifacts are unavailable rather than fabricating recovery content.
**Edge Cases:**
- Ancora MCP is unavailable during a phase transition.
- A local pointer is stale after worktree removal.
- The user asks to recall a completed submission.
**Out of Scope:**
- Guaranteeing recovery of deleted full execution artifacts.

## Open Questions
- None. Archive retention duration and immediate cleanup behavior are configurable policy, not scope decisions; defaults must be documented and must never expand Judge scope.

## Trade-offs
- Ignored local execution files preserve interruption recovery without polluting Git, but recovery is worktree-local.
- Compact Ancora pointers support long-term recall, but cannot replace deleted full evidence.
- Bounded archives protect against accidental cleanup while requiring a retention policy and cleanup command.
- Ignoring legacy global markers avoids false blocking, but users must explicitly migrate old work if they want to resume it.

## Risk Level
high — Justification: this changes workflow state ownership, resume behavior, cleanup, and review gates. A scope bug can either delete recoverable work or allow unrelated evidence to influence quality decisions.
