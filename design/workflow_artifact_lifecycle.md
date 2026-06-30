# Technical Design: Workflow Artifact Lifecycle

Status: Pending human approval. The related spec and feature file are pending approval, so implementation must not begin from this design yet.

This design defines how workflow hard specs, Gherkin features, approval records, cache output, and archive candidates move through the repository without recreating clean-tree blockers or losing active behavior contracts.

## Problem

Generated `specs/` and `features/` files have repeatedly become untracked clean-tree blockers. Deleting them makes the tree clean, but also discards the behavior contract that TDD, review, and QA need. Storing full artifacts only in Ancora avoids local files, but removes the contract from normal repository review and repo-only recovery.

The implementation must make intentional contract files visible, tracked, and scoped while keeping machine-local or sensitive output out of version control.

## Design Summary

| Area | Decision |
|------|----------|
| Source of truth | Repository files are canonical for full hard spec and Gherkin content. Ancora stores pointers, indexes, state, and optional drift hashes only. |
| Contract files | Active and pending contract artifacts use stable, namespaced files under `specs/` and `features/`. |
| Approval | Replace global approval semantics with scoped approval records that name exact artifact paths and approved scenario IDs. |
| Archive | Active regression contracts stay active. Only retired, superseded, or process-only artifacts move to archive with an explicit retirement reason. |
| Cleanup | Cleanup classifies each artifact as track, keep pending, archive, ignore, or delete. Active contracts are never deleted solely to satisfy a clean-tree gate. |

## Source Of Truth

Repository files are the durable collaboration surface:

- `specs/*.md` contains authoritative hard spec text.
- `features/*.feature` contains authoritative Gherkin behavior contracts.
- Approval records identify which repository artifacts and scenarios are approved.
- Tests and QA plans reference repository feature paths plus stable scenario IDs.

Ancora is an index and resume aid only. It may store artifact paths, phase, approval status, scenario IDs, requirement IDs, observation IDs, and optional checksums. If Ancora disagrees with the repository, the repository wins and the pointer is repaired or reported as stale.

## Artifact Taxonomy

| Class | Examples | Git policy | Lifecycle action |
|-------|----------|------------|------------------|
| Active contracts | Approved `specs/*.md`, approved `features/*.feature`, scoped approval records | Track | Keep discoverable in active paths while behavior remains required |
| Pending contracts | Generated spec/feature files awaiting human approval, including `specs/workflow_artifact_lifecycle.md` and `features/workflow_artifact_lifecycle.feature` | Track for review or keep explicitly pending, but never treat as implementation-ready | Block implementation until scoped human approval exists |
| Retired contracts | Superseded specs/features, replaced scenarios, obsolete approval records | Track archive record if retained | Move to archive only after explicit retirement decision |
| Local caches | `.vela/`, generated graph databases, indexes, temp analysis output | Ignore or delete | Never promote unless a reviewed project-artifact decision says so |
| Sensitive backup outputs | Backup captures, restore snapshots, real config captures, token-bearing files, private machine state | Never commit | Delete, ignore, or replace with sanitized authored examples |

## Lifecycle State Machine

| State | Entry condition | Allowed next state | Required guard |
|-------|-----------------|--------------------|----------------|
| Draft | Artifact is being generated or revised | Pending approval | Paths and SCN IDs are stable enough for review |
| Pending approval | Spec/feature exists for human review | Approved active contract or retired | No implementation; no approval marker for unrelated contracts applies |
| Approved active contract | Human approval is recorded in a scoped approval record | Implementation or retired/superseded | Contract files are tracked or intentionally exempted by reviewed policy |
| Implementation | TDD work consumes approved scenarios | Review | Tests reference approved SCN IDs and feature identity |
| Review | Implementation is validated against approved scenarios | Active regression contract or implementation revision | Verification passes and no unapproved scenarios are treated as complete |
| Active regression contract | Implemented behavior remains required | Retired/superseded | Feature remains discoverable under `features/`; spec remains discoverable under `specs/` or an active index |
| Retired/superseded | Human approves replacement or removal | Archive | Retirement record names reason, old IDs, and successor IDs or paths |
| Archive | Artifact is no longer active behavior contract | None | Archive entry preserves traceability without hiding active behavior |

## Approval Marker Strategy

The existing single `specs/.approved` marker is unsafe as a global approval mechanism. It can accidentally approve new or unrelated scenarios simply because it exists from a prior active contract.

Recommended strategy:

- Keep `specs/.approved` as a legacy marker only for the existing active scope it already represents.
- Introduce scoped approval records for new contracts at `specs/approvals/<contract-id>.approved.json`.
- Require each scoped record to include exact spec path, feature paths, approved scenario references, requirement IDs, approval timestamp, and optional file hashes.
- Store scenario references as `<feature-path>#<SCN-ID>`, not raw `SCN-NNN` values alone.
- Allow subset approval by listing only approved scenario references.
- Treat any contract file not named by a scoped approval record as pending, even when `specs/.approved` exists.

Example record shape:

```json
{
  "contract_id": "workflow_artifact_lifecycle",
  "status": "approved",
  "spec_path": "specs/workflow_artifact_lifecycle.md",
  "feature_paths": ["features/workflow_artifact_lifecycle.feature"],
  "approved_requirements": ["REQ-011", "REQ-012", "REQ-013", "REQ-014", "REQ-015", "REQ-016", "REQ-017", "REQ-018", "REQ-019", "REQ-020"],
  "approved_scenarios": [
    "features/workflow_artifact_lifecycle.feature#SCN-012",
    "features/workflow_artifact_lifecycle.feature#SCN-013",
    "features/workflow_artifact_lifecycle.feature#SCN-014",
    "features/workflow_artifact_lifecycle.feature#SCN-015",
    "features/workflow_artifact_lifecycle.feature#SCN-016",
    "features/workflow_artifact_lifecycle.feature#SCN-017",
    "features/workflow_artifact_lifecycle.feature#SCN-018",
    "features/workflow_artifact_lifecycle.feature#SCN-019",
    "features/workflow_artifact_lifecycle.feature#SCN-020",
    "features/workflow_artifact_lifecycle.feature#SCN-021",
    "features/workflow_artifact_lifecycle.feature#SCN-022",
    "features/workflow_artifact_lifecycle.feature#SCN-023",
    "features/workflow_artifact_lifecycle.feature#SCN-024"
  ]
}
```

The implementation gate should fail closed when approval scope is ambiguous.

## Git Hygiene Policy

- Track intentional active and pending contract files so review and repo recovery can see them.
- Do not update approval records until explicit human approval is received.
- Do not delete active or approved contract files to satisfy a clean-tree requirement.
- Ignore or remove `.vela/` and equivalent generated cache directories when they are local state.
- Never commit backup captures, restore snapshots, real user config captures, tokens, private machine state, or unsanitized local paths.
- If a generated artifact is intentionally promoted, record a project-artifact decision explaining why it is stable, reviewable, and safe.

## Archive Design

Active contracts remain in active locations:

- Active specs stay under `specs/` or in a documented active-spec index.
- Active Gherkin behavior contracts stay under `features/`.
- Implemented feature files are not archived only because implementation completed.

Archive candidates:

- Superseded hard specs or feature scenarios.
- Process-only planning artifacts no longer needed for active regression behavior.
- Retired approval records after a replacement approval exists.

Archive path options:

| Option | Pros | Cons |
|--------|------|------|
| `archive/workflow-artifacts/<contract-id>/` | Visible, neutral, keeps all retired workflow artifacts together | Adds a new top-level directory |
| `specs/archive/` and `features/archive/` | Keeps artifacts near original type | Splits one retirement decision across directories |
| `archive/contracts/<contract-id>/` | Short and general | Less explicit about workflow/process artifacts |

Recommended path: `archive/workflow-artifacts/<contract-id>/<YYYYMMDD>-<reason>/`.

Each archive entry should include a small retirement note naming original paths, retired scenario IDs, successor paths or IDs when applicable, approval reference, and reason. Active contracts must never move to archive as generic cleanup.

## TDD Integration

Implementation agents consume approved feature scenarios as the behavior backlog:

- Read approved scoped records to determine implementation-ready scenarios.
- Use stable `@SCN-NNN` tags from Gherkin as the trace key.
- Include the SCN ID and feature identity in test names, subtests, metadata, or equivalent trace output.
- Do not renumber approved scenarios because of reordering or insertion.
- Treat pending scenarios as unavailable for implementation until human approval is recorded.

## QA Workflow Integration

QA planning enumerates active repository feature files and scenario tags directly. It should not reconstruct full Gherkin text from Ancora.

Test framework choice follows behavior shape:

- Use Playwright only for browser-observable behavior.
- Use Go tests, TUI tests, CLI integration tests, or other project-appropriate checks for non-browser workflow behavior.
- Report failures with feature path plus SCN ID so failures remain traceable after workflow archive or resume operations.

## Migration Plan

1. Classify current repository artifacts as active contract, pending contract, archive candidate, local cache, or sensitive output.
2. Preserve existing active contract files and the legacy marker without rewriting them as part of this change.
3. Treat `specs/workflow_artifact_lifecycle.md` and `features/workflow_artifact_lifecycle.feature` as pending contract files until human approval.
4. Add scoped approval support for new contracts before using the artifact-lifecycle scenarios for implementation.
5. Backfill or configure the legacy active contract scope so `specs/.approved` no longer acts as global approval for all future files.
6. Update cleanup guidance to classify files as track, keep pending, archive, ignore, or delete instead of deleting generated contracts by default.
7. Update ignore rules or cleanup steps for `.vela/` and equivalent cache outputs without ignoring active `specs/` or `features/` contracts.
8. Add archive handling only after active-versus-retired classification exists.

## Verification Strategy

Before implementation:

- Confirm the new spec, feature, and design are still pending human approval.
- Confirm the approval gate rejects this contract until a scoped approval record names `SCN-012` through `SCN-024`.
- Confirm existing active contract files are unchanged.
- Confirm cache and sensitive-output paths are excluded from review candidates.

During implementation review:

- Verify every test or QA item references feature path plus SCN ID.
- Verify active contract files remain in active paths after implementation completes.
- Verify archive operations require an explicit retirement reason.
- Verify Ancora observations contain pointers and state only, not authoritative full artifact content.

## Risks And Open Questions

- Legacy tooling may continue treating `specs/.approved` as a global approval marker until the gate is updated.
- Pending contract files being tracked may be mistaken for approved contracts unless approval records are the only implementation gate.
- Scenario IDs can collide across feature files if tools store raw `SCN-NNN` values without feature identity.
- Archive classification mistakes can hide active regression behavior.
- Sensitive backup outputs require conservative detection; uncertain files should fail closed for human review.
- Open question: should the legacy active contract receive a backfilled scoped approval record immediately, or should the first implementation only fence `specs/.approved` to its known legacy scope?

## Decisions And Rejected Alternatives

| Decision | Rationale |
|----------|-----------|
| Use repository files as canonical artifacts | Keeps contracts reviewable, recoverable, and directly consumable by TDD and QA |
| Use Ancora only for pointers/state | Prevents memory state from replacing reviewed repository artifacts |
| Add scoped approval records | Prevents one marker from approving unrelated pending contracts |
| Keep active features active after implementation | They are living regression contracts, not disposable generated docs |
| Archive only retired or process-only artifacts | Avoids losing active behavior while preserving history when behavior is superseded |
| Reject Ancora-only storage | Hides full contracts from normal review and repo-only recovery |
| Reject deleting generated contracts to force a clean tree | Produces cleanliness by losing the approved or reviewable contract |
| Reject tracking all generated output | Cache, graph, backup, and sensitive files are noisy or unsafe unless deliberately promoted |
