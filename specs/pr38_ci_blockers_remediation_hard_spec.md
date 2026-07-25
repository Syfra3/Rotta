# Hard Spec: PR #38 CI Blocker Remediation

## Adversarial Pre-Mortem
- Failure mode 1: A failed selected-backup restore followed by a failed rollback reports success or omits one recovery location, leaving an operator unable to recover safely.
- Failure mode 2: Workflow tests create a commit by inheriting a developer or CI runner's global Git identity, so they pass locally but fail in a clean CI environment.
- Failure mode 3: The remediation broadens into review-quality-gate policy work or rewrites the active unified-workflow-authority artifacts, invalidating the active feature authority fingerprints.

## Hidden Assumptions
- `internal/installer/backup.go.tmp` contains the intended rollback-failure branch and differs from `internal/installer/backup.go` only by that stranded behavior.
- Workflow tests that create commits can configure an identity in the temporary repository before committing; no product runtime behavior needs a Git identity.
- `go test ./...` and `make verify` are the required regression commands; the latter retains its current quality-gate composition.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Set `user.name` and `user.email` globally in CI | It mutates the host environment, masks non-hermetic tests, and violates the approved scope. |
| Suppress rollback errors or always report rollback success | It can mislead an operator after a partial restore and removes the locations needed for manual recovery. |
| Modify `specs/hard_spec.md` or `features/unified-workflow-authority.feature` | Those active authority artifacts have fingerprints and are expressly out of scope. |
| Redesign `make verify` or quality gates | The blocker is test-repository identity and stranded recovery behavior, not quality-gate policy. |

## Summary
Remediate only the PR #38 CI blockers: restore the rollback-failure reporting branch currently stranded in `internal/installer/backup.go.tmp` and remove that stray temporary file; make workflow-test temporary Git repositories self-contained by configuring test-only identity locally through test helpers; and prove the existing verification commands pass without any global Git identity. The work must not alter production CI configuration, quality-gate design, or the active unified-workflow-authority hard-spec, feature, or approval artifacts, and implementation changes must remain uncommitted for human review.

## Requirements

### REQ-070: Preserve Both Recovery Locations When Restore and Rollback Fail
**Description:** If applying a selected backup fails and restoring the pre-restore safety backup also fails, restoration returns an error that identifies the selected backup location and the pre-restore safety backup location, reports rollback failure, and does not state or imply that rollback succeeded. The behavior must match the intended branch preserved in `internal/installer/backup.go.tmp`. After restoration, the stray `internal/installer/backup.go.tmp` file must not remain in the repository.

**Acceptance Criteria:**
- A selected-backup restore failure followed by a rollback failure returns an error that identifies both the selected backup directory and the generated pre-restore safety-backup directory.
- That error communicates that rollback failed and contains no success claim about restoration or rollback.
- A restore failure whose rollback succeeds continues to report successful rollback as distinct behavior.
- `internal/installer/backup.go.tmp` is deleted after its intended behavior is restored to the tracked Go source.

**Edge Cases:**
- The selected backup and generated safety backup have similar timestamp-derived names but are distinct paths.
- The rollback fails because its manifest cannot be read or parsed after the selected restore has already failed.
- The restoration result still exposes the pre-restore backup directory for manual recovery even when the rollback fails.

**Out of Scope:**
- Changes to backup layout, manifest schema, backup retention, or normal successful restore behavior.
- New recovery policy beyond restoring the stranded rollback-failure behavior.

### REQ-071: Make Workflow-Test Git Repositories Hermetic
**Description:** Workflow tests that create commits in temporary Git repositories must configure a test-only `user.name` and `user.email` as repository-local configuration through test helpers. Their correctness must not depend on a Git identity from the host, global Git configuration, system Git configuration, or CI setup. Tests that deliberately remove a repository-local identity to exercise the missing-identity path must retain that intentional condition.

**Acceptance Criteria:**
- Every workflow-test temporary repository that commits has local test-only identity before its first commit.
- The workflow test suite can run when host/global Git identity is unavailable; no global Git configuration is written or required.
- A test that deliberately removes local identity still observes the expected missing-identity behavior rather than being silently repaired by a helper.
- Test-only Git identity configuration is confined to test helpers and temporary repositories.

**Edge Cases:**
- A test creates more than one temporary repository or a linked worktree and commits in more than one repository.
- A test creates a repository in a subtest or table-driven case.
- Global and system Git configuration are unavailable or explicitly isolated while local repository configuration remains usable.

**Out of Scope:**
- Configuring a Git identity in GitHub Actions, developer machines, production code, or project-level Git configuration.
- Changing workflow runtime Git identity requirements.

### REQ-072: Verify the Narrow Remediation Without Redesign or Commit
**Description:** Validate the remediation using the existing repository commands without altering CI, Make targets, or quality-gate policy. Leave all implementation changes uncommitted for human review. The new contract artifacts are isolated from active unified-workflow-authority artifacts and do not modify their fingerprints.

**Acceptance Criteria:**
- `go test ./...` passes in the remediation worktree.
- `make verify` passes using its existing definition.
- No production CI configuration, Make target, or quality-gate redesign is introduced.
- The active `specs/hard_spec.md`, `features/unified-workflow-authority.feature`, and existing unified-workflow-authority approval artifacts are unchanged.
- Implementation changes remain uncommitted when the work is handed to a human reviewer.

**Edge Cases:**
- Verification runs in an environment without a preconfigured global Git identity.
- A generated or temporary test artifact must not be mistaken for an implementation commit.
- The remediation's contract files coexist with active authority artifacts without being substituted for them.

**Out of Scope:**
- Committing, pushing, creating or updating a pull request, or requesting final review.
- Quality-gate, workflow-authority, or CI-policy redesign.

## Open Questions
- None. The approved scope, intended rollback branch, required verification commands, and no-commit handoff condition are explicit.

## Trade-offs
- Local test identity adds explicit fixture setup, but it makes CI behavior deterministic and avoids host mutation.
- The rollback-failure error preserves the currently intended diagnostic branch rather than expanding error aggregation semantics beyond this remediation.
- Verification is intentionally limited to existing commands; broader quality-gate changes remain separately governed.

## Risk Level
medium — Justification: the source change affects failure recovery diagnostics and the test changes span workflow repository setup, but the permitted behavior is narrow, observable, and covered by existing verification commands.
