# Hard Spec: Workflow Artifact Lifecycle

## Adversarial Pre-Mortem
- Failure mode 1: Generated `specs/` and `features/` files remain untracked, so implementation agents refuse to continue because the working tree is not clean; deleting those files to unblock work silently discards the approved behavior contract.
- Failure mode 2: Ancora becomes the only place where the full spec or Gherkin content exists, making the workflow unrecoverable from the repository and preventing normal code review, TDD, and QA planning from using stable contract files.
- Failure mode 3: Archive cleanup treats active regression contracts as completed change-process debris, moving or deleting living feature files and causing later refactors to lose the scenarios they must preserve.

## Hidden Assumptions
- Contributors can distinguish active behavior contracts from temporary change-process artifacts during cleanup, archive, and review.
- The repository is the durable collaboration surface for specs and Gherkin contracts; Ancora is available for state lookup but must not be required to reconstruct full artifact content.
- Existing approval gates rely on a clean working tree and `specs/.approved`, so generated contract files must be tracked or intentionally ignored before implementation begins.
- Scenario IDs are stable enough for tests, QA plans, and implementation notes to reference without being renumbered during ordinary edits.
- Some generated outputs, cache files, backups, or config captures may contain sensitive or machine-local data and therefore cannot be promoted automatically to tracked workflow artifacts.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Delete generated spec and feature files whenever they block a clean tree | It creates a clean tree by losing the approved contract and breaks traceability from scenario to test to behavior. |
| Store full specs and features only in Ancora | It makes memory state the source of truth, hides contract changes from code review, and prevents repo-only recovery. |
| Archive all generated artifacts after implementation completes | Active feature files are living regression contracts; archiving them would obscure behavior that must continue to work. |
| Track every generated file under the workflow directories | Local caches, graph databases, backup outputs, and config captures may be noisy, machine-specific, or sensitive. |

## Summary
Rotta must treat repository files as the source of truth for approved workflow contracts. Markdown hard specs and Gherkin feature files are version-controlled artifacts that remain active while they describe behavior that must keep working. Ancora stores only compact state and pointers to those files. Approval gates must prevent implementation before human approval while also avoiding the recurring untracked-artifact blocker. Archive behavior must retire only superseded or completed process artifacts and must never delete or hide active regression contracts.

## Invariants
- Full hard spec and Gherkin content lives in repository files, not only in Ancora.
- Active `features/*.feature` files are behavior contracts, not disposable generated documentation.
- Active scenario IDs remain stable after approval unless a human explicitly approves a migration that preserves traceability.
- The workflow never treats deletion of approved spec or feature files as the normal way to obtain a clean working tree.
- Local caches, generated graph state, backups, and sensitive config captures are excluded unless intentionally promoted through a reviewed, sanitized project-artifact decision.

## Requirements

### REQ-011: Repo-Tracked Contract Files Are the Source of Truth
**Description:** Approved hard specs and Gherkin feature contracts must exist as repository files and must be tracked by version control when they are active.
**Acceptance Criteria:**
- Active hard specs live under `specs/` using stable, descriptive filenames.
- Active Gherkin behavior contracts live under `features/` using stable, descriptive filenames.
- The repository file content is the authoritative source for spec and feature text.
- A workflow cannot rely on Ancora observations as the only copy of full hard spec or feature content.
- New workflow-policy artifacts must use namespaced files, such as `specs/workflow_artifact_lifecycle.md` and `features/workflow_artifact_lifecycle.feature`, when an existing active contract already occupies `specs/hard_spec.md` or `features/installer_recovery.feature`.
**Edge Cases:**
- Multiple active specs and features exist at the same time.
- A generated contract file exists locally but has not been added to version control.
- A legacy generic spec filename exists and must remain untouched for an active contract.
**Out of Scope:**
- Rewriting existing approved installer recovery contract content as part of this workflow-policy spec.

### REQ-012: Gherkin Features Are Living Behavior Contracts
**Description:** `features/*.feature` files must describe observable behavior that remains active for as long as the behavior must keep working.
**Acceptance Criteria:**
- Feature files are written as Gherkin behavior contracts with `Feature`, optional `Background`, and concrete `Scenario` entries.
- Active feature files remain in `features/` after implementation and verification complete.
- Feature files are not described or handled as disposable generated docs.
- Removing, archiving, or superseding an active feature requires an explicit human-approved lifecycle decision.
**Edge Cases:**
- A scenario is implemented and verified but still describes required regression behavior.
- A new workflow change extends an existing active behavior contract.
- A feature is partly superseded while other scenarios remain active.
**Out of Scope:**
- Mandating a specific Gherkin runner or test framework.

### REQ-013: Scenario IDs Are Stable and Traceable to Tests
**Description:** Every Gherkin scenario must have a stable scenario ID and every test or QA artifact derived from a scenario must reference that ID.
**Acceptance Criteria:**
- Each scenario has a `@SCN-NNN` tag and at least one `@REQ-NNN` tag.
- Scenario IDs are not renumbered after approval solely because scenarios are reordered or new scenarios are inserted.
- Scenario IDs are unique across active feature files whenever the approval marker records only raw `SCN-NNN` values.
- Tests that implement or verify a scenario reference the scenario ID in the test name, metadata, subtest name, or an equivalent traceable location.
- Test and QA traces include the feature identity or file path in addition to `SCN-NNN`.
- Scenario retirement or replacement records the old ID and the successor ID or retirement reason.
**Edge Cases:**
- A scenario is split into two narrower scenarios after review.
- A test covers multiple approved scenarios.
- A feature file is renamed while scenario IDs remain active.
**Out of Scope:**
- Requiring one test file per scenario.

### REQ-014: Ancora Stores Pointers and State Only
**Description:** Ancora must act as a compact state index for workflow artifacts, not as the sole source of full spec or feature content.
**Acceptance Criteria:**
- Ancora observations store artifact paths, phase/status, approval state, risk level, approved scenario IDs, requirement IDs, and optional hashes or checksums.
- Ancora observations may store observation IDs or state pointers needed to resume workflow phases.
- Ancora observations do not store the full Markdown spec or full Gherkin feature text for this workflow as the authoritative artifact.
- If an Ancora pointer and repository file disagree, the repository file content wins and the pointer must be repaired.
- A workflow can recover the current contract from a repository checkout without reading full artifact content from Ancora.
**Edge Cases:**
- Ancora is unavailable during implementation or verification.
- A pointer references a deleted or renamed file.
- A checksum differs because the repo file changed after approval.
**Out of Scope:**
- Banning compact summaries or state indexes in Ancora.

### REQ-015: Approval Marker Gates Implementation Without Approving Unrelated Specs
**Description:** Implementation may begin only after human approval is recorded for the relevant contract and the required working-tree cleanliness gate is satisfied.
**Acceptance Criteria:**
- Generating a new hard spec or feature contract leaves its approval state pending until explicit human approval is received.
- Spec generation does not create or update `specs/.approved` for the new contract unless human approval has been granted.
- The approval marker identifies the approved contract scope through artifact paths, scenario IDs, or equivalent unambiguous metadata.
- An existing approval marker for one active contract does not automatically approve a different pending contract.
- Before implementation begins, active contract files required by the approved scope are either tracked in version control or intentionally exempted by a reviewed ignore policy.
- A dirty tree caused by untracked active spec or feature files is resolved by tracking or approving the files, not by deleting the approved contract.
**Edge Cases:**
- `specs/.approved` already exists for a prior approved feature.
- Multiple active contracts share local scenario IDs such as `SCN-001`.
- A user approves only a subset of scenarios.
**Out of Scope:**
- Implementing the approval gate code in this spec artifact.

### REQ-016: Archive Policy Preserves Active Regression Contracts
**Description:** Archive behavior must distinguish active behavior contracts from retired, superseded, or completed change-process artifacts.
**Acceptance Criteria:**
- Active feature files remain in `features/` while they describe behavior that must continue to work.
- Active hard specs remain discoverable in `specs/` or through a documented active-spec index.
- Retired or superseded contracts may move to an archive only after an explicit lifecycle decision records why they are no longer active.
- Completed temporary process artifacts may move to an archive when they are no longer needed for active regression behavior.
- Archiving never deletes, hides, or obscures the active regression contract needed by TDD, QA, or future maintenance.
**Edge Cases:**
- A completed change process introduced behavior that remains required.
- A replacement feature covers only part of an older feature file.
- Archive folders exist for historical process artifacts while active features remain in place.
**Out of Scope:**
- Defining the exact archive folder hierarchy beyond the active-versus-retired rule.

### REQ-017: Local Cache and Generated Graph Artifacts Are Ignored or Removed
**Description:** Machine-local caches and generated graph artifacts must not be committed as workflow contract files unless intentionally promoted.
**Acceptance Criteria:**
- `.vela/` and similar graph/cache directories are ignored or removed when they are only local generated state.
- Generated databases, indexes, temporary analysis output, and cache files are not committed as spec or feature artifacts by default.
- A local/generated artifact may be tracked only after a deliberate project decision explains why it is a stable project artifact.
- Ignoring local cache artifacts must not ignore active `specs/` or `features/` contracts.
**Edge Cases:**
- A generated graph file is useful for debugging but not needed by the project.
- A cache directory contains both generated state and a hand-authored project artifact.
- A tool recreates ignored cache files during verification.
**Out of Scope:**
- Removing or changing cache-generating tools themselves.

### REQ-018: Backups and Sensitive Config Captures Are Never Workflow Artifacts
**Description:** Backup outputs and sensitive configuration captures must not be committed as workflow artifacts.
**Acceptance Criteria:**
- Backup directories, restore snapshots, and recovery outputs are excluded from spec and feature artifact commits.
- Captures of user-level config, credentials, tokens, MCP settings, local home-directory paths, or private machine state are not committed as workflow artifacts.
- If a config example is needed, it must be sanitized and intentionally authored as documentation or a fixture.
- Spec and feature files may describe security rules without embedding sensitive captured values.
**Edge Cases:**
- A failing test produces a backup or restore output under the repository.
- A user copies a real config file into `specs/`, `features/`, fixtures, or archive directories.
- A cache file includes absolute paths or secrets.
**Out of Scope:**
- Defining the full secret-scanning implementation.

### REQ-019: QA and TDD Consume Contracts Directly
**Description:** The artifact lifecycle must support strict TDD and later QA planning by making approved scenarios stable, discoverable, and traceable.
**Acceptance Criteria:**
- TDD planning uses approved feature scenarios as the behavior backlog.
- QA planning can enumerate active feature files and scenario IDs without reconstructing full content from Ancora.
- Test failures can be reported against scenario IDs and feature file paths.
- Unapproved pending specs and scenarios are not treated as implementation-ready.
- Scenario IDs remain stable across implementation, verification, and future regression planning.
**Edge Cases:**
- A QA workflow runs after the original implementation phase has been archived.
- A pending spec exists beside approved active contracts.
- A scenario is approved but implementation is intentionally deferred.
**Out of Scope:**
- Generating test code or QA plans as part of this spec task.

### REQ-020: Migration Prevents the Current Blocker-Prone Workflow
**Description:** Existing and future generated specs/features must migrate toward tracked living contracts and pointer-only state so implementation is not blocked by untracked artifacts or lost approvals.
**Acceptance Criteria:**
- When generated spec or feature files represent approved behavior, the workflow prompts for tracking or committing them rather than deleting them to clean the tree.
- When generated files are pending approval, the workflow records them as pending and avoids creating an approval marker for them.
- Existing active installer recovery artifacts remain untouched when creating a namespaced workflow-policy contract.
- The workflow can explain which files are active contracts, pending contracts, archived artifacts, local caches, or sensitive outputs.
- Cleanup guidance differentiates between files to track, files to archive, files to ignore, and files to delete.
**Edge Cases:**
- A developer starts implementation with active contract files untracked.
- A prior cleanup removed generated contract files from the repository checkout.
- Multiple pending workflow specs exist at once.
**Out of Scope:**
- Retrofitting all historical artifacts in this spec-writing step.

## Non-Goals
- Do not implement production workflow changes in this artifact.
- Do not create or update tests in this artifact.
- Do not stage, commit, or delete existing active installer recovery contracts.
- Do not create `specs/.approved` for this pending workflow artifact lifecycle contract.
- Do not make Ancora the full-content artifact store for this workflow.

## Open Questions
- None.

## Trade-offs
- Keeping active contracts tracked in the repository adds review surface, but it preserves the behavior contract and removes ambiguity for TDD and QA.
- Ancora pointer-only persistence is less convenient than storing full content in memory, but it prevents memory state from replacing reviewed repo artifacts.
- Stable scenario IDs require discipline when editing features, but they make scenario-to-test traceability reliable.
- Archiving only retired or process-only artifacts requires lifecycle classification, but it avoids losing active regression coverage.

## Risk Level
high — Justification: The policy governs approval gates, artifact retention, traceability, and sensitive-output handling. A weak lifecycle can either block implementation indefinitely, lose approved contracts, or accidentally commit local/private artifacts.
