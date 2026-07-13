# Hard Spec: Autonomous Phase 3 Scenario Checkpoints

## Adversarial Pre-Mortem
- Failure mode 1: Rotta commits an incomplete, failed, or cross-scenario diff because it treats a passing targeted test as sufficient proof, leaving a misleading checkpoint that the next scenario builds on.
- Failure mode 2: A pre-existing user edit or an untracked non-ignored file is included in an automatic commit, or is silently discarded while trying to restore a clean worktree boundary.
- Failure mode 3: Automation bypasses explicit contract approval, advances despite stale workflow state, pushes a branch, or treats scenario-level validation as a replacement for the Phase 4 review gate.

## Hidden Assumptions
- The approved contract identifies the ordered scenario set to implement, and the implementation agent can report the paths it changed for the scenario it just completed.
- A newly created dedicated feature branch and isolated worktree can contain the approved, tracked contract baseline while beginning with no non-ignored working-tree changes.
- Required scenario validation and the active objective quality checks can produce a pass/fail result before a checkpoint is created.
- Git can create a local commit in the isolated worktree; failure to do so is observable and leaves the workflow stopped rather than partially advanced.
- Ignored runtime state may be updated without violating the clean-worktree boundary because it is excluded from the non-ignored Git status and never becomes scenario commit content.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Keep a human “commit and continue” prompt after every scenario | Repeats a mechanical decision after the approved contract and objective checks have already established the allowed scenario boundary. |
| Accumulate all scenario changes into one end-of-workflow commit | Loses scenario-to-commit traceability and means a later failure leaves more than one scenario's work in an ambiguous dirty state. |
| Automatically stash, revert, or delete unexpected changes | Can hide or destroy user work and cannot prove that the resulting commit contains only the current scenario's work. |
| Automatically push each checkpoint | Publishes incomplete feature work and removes the human's final control over remote publication. |
| Stop automatically for sensitive paths | The explicitly selected policy is to apply the same validation and scope checks to every scenario without a separate security/auth/payments/infra/migrations/secrets stop. |

## Summary
After explicit human approval of a hard spec and its Gherkin contract, Rotta Phase 3 must run the approved scenarios autonomously in order. Each scenario retains strict Red → Green → Refactor and must pass its required tests and active objective validation before Rotta creates exactly one local, scenario-scoped commit containing only that scenario agent's expected changes. Phase 3 must start on a newly created dedicated feature branch in a clean isolated worktree; after a successful checkpoint, Rotta records pointer-only workflow state, confirms the non-ignored worktree is clean, and starts the next approved scenario. Unexpected changes, untracked non-ignored files, validation failures, or commit failures halt without cleanup by destruction, commit, push, or progression. The flow preserves explicit Gherkin approval, workspace source-of-truth rules, and the later review gate; remote push remains a single manual human action after workflow and review completion.

## Requirements

### REQ-021: Establish an Isolated Feature-Branch Baseline for Phase 3
**Description:** Before any approved scenario is implemented, Rotta must create a new dedicated feature branch and a clean isolated worktree that contains the approved workspace contract baseline.
**Acceptance Criteria:**
- Phase 3 creates a new feature branch and a separate isolated worktree for the approved workflow before launching the first scenario agent.
- The isolated worktree contains the approved hard spec and Gherkin feature contract as workspace source-of-truth artifacts.
- Before the first scenario begins, `git status --short` in that worktree has no non-ignored changes.
- Rotta reports the branch and isolated worktree it selected or created before scenario execution begins.
- This feature does not add a protected-branch or default-branch eligibility check; the dedicated-new-branch rule is the required branch safeguard.
**Edge Cases:**
- The requested branch name already exists.
- The intended worktree path already exists or cannot be created.
- Approved contract artifacts are absent, untracked, or differ from the approved scope.
**Out of Scope:**
- Choosing a remote name, publishing the branch, or changing repository branch-protection settings.

### REQ-022: Require Explicit Scoped Approval Before Autonomous Execution
**Description:** Autonomous Phase 3 execution must begin only for scenarios explicitly approved in the relevant hard spec and Gherkin contract.
**Acceptance Criteria:**
- Rotta refuses to launch the autonomous loop when explicit human approval for the relevant contract scope is absent, ambiguous, or does not include the next scenario.
- Rotta derives the execution order and eligible scenario set from the approved workspace contract artifacts, not host-local instructions or memory content.
- A prior approval for a different spec or feature does not approve this loop's scenarios.
- The loop preserves the strict Red → Green → Refactor requirement for every approved scenario.
**Edge Cases:**
- The human approves only a subset of scenarios in a feature file.
- The contract changes after approval but before Phase 3 starts.
- Workflow state points to a scenario no longer included in the approved scope.
**Out of Scope:**
- Replacing explicit human approval with inferred, remembered, or agent-generated approval.

### REQ-023: Create Exactly One Validated Local Commit Per Scenario
**Description:** When one approved scenario finishes its strict TDD cycle and passes all required validation, Rotta must create one and only one local commit containing that scenario's expected changes.
**Acceptance Criteria:**
- Rotta does not create a scenario commit until the scenario's Red, Green, and Refactor evidence is complete, its required tests pass, and active objective validation passes.
- The commit contains only non-ignored paths in the expected change set for the just-completed scenario; it does not include changes from a prior, future, or unrelated scenario.
- Rotta creates exactly one local commit for each successfully completed approved scenario, records its identifier in workflow state, and does not create a second checkpoint for the same scenario.
- If the local commit operation fails, Rotta reports the failure and halts without marking the scenario checkpointed or beginning another scenario.
- Scenario commits do not add AI-generated, generated-by, or co-author attribution.
**Edge Cases:**
- A scenario changes both production and test files that are within its expected scope.
- The scenario produces only an expected test, configuration, or documentation change.
- A retry occurs after a commit command fails before a commit identifier is recorded.
**Out of Scope:**
- Squashing, rebasing, amending, or combining scenario commits.

### REQ-024: Halt Without Mutating Unexpected or Untracked Non-Ignored Changes
**Description:** Rotta must fail closed when the scenario worktree contains non-ignored changes outside the current scenario agent's expected change set, including untracked non-ignored files.
**Acceptance Criteria:**
- Before committing, Rotta compares all non-ignored modified, deleted, renamed, and untracked paths with the current scenario's expected change set.
- If any non-ignored path is outside that set, Rotta halts, identifies the unexpected paths, creates no scenario commit, and does not launch the next scenario.
- If any untracked non-ignored path exists, Rotta halts even when no tracked file is unexpected.
- On a halt, Rotta does not automatically stash, discard, revert, delete, add, or commit the unexpected change.
- Ignored local runtime artifacts do not block the loop and are excluded from scenario commits.
**Edge Cases:**
- A user edits a tracked file while the scenario agent is running.
- A tool writes a new non-ignored report, backup, cache, or temporary file.
- A scenario agent modifies an expected file and an unrelated untracked file appears concurrently.
**Out of Scope:**
- Automatic conflict resolution or automatic classification of ambiguous user changes as scenario work.

### REQ-025: Restore the Clean Boundary and Advance Durable Workflow State
**Description:** After a successful scenario checkpoint, Rotta must preserve a clean non-ignored worktree boundary, update durable workflow state, and continue with the next approved scenario only when one remains.
**Acceptance Criteria:**
- After a successful local scenario commit, `git status --short` in the isolated worktree is empty except for explicitly ignored local artifacts before the next scenario begins.
- Rotta updates workspace workflow state with the completed scenario, local commit identifier, remaining approved scenarios, and next scenario when applicable.
- Ancora, when enabled, stores only a compact pointer/status index for this transition; the workspace contract and workflow artifacts remain authoritative.
- Rotta starts the next approved scenario automatically only after the preceding checkpoint and clean-boundary checks succeed.
- When no approved scenarios remain, Rotta advances to the existing Phase 4 review gate rather than declaring final human approval.
**Edge Cases:**
- Writing ignored runtime state changes the filesystem after the commit.
- State persistence fails after the commit but before the next scenario begins.
- The final scenario passes and no next scenario exists.
**Out of Scope:**
- Treating a successful scenario checkpoint as a substitute for the metrics-based or human review process.

### REQ-026: Apply Validation and Scope Rules Without a Sensitive-Scope Stop
**Description:** A scenario that changes security, authentication, payments, infrastructure, migrations, secrets, or another sensitive area must follow the same autonomous checkpoint rule when it satisfies the approved scope and all required validation.
**Acceptance Criteria:**
- Rotta does not halt solely because an expected scenario-scoped path is classified as security, auth, payments, infra, migrations, or secrets.
- Such a scenario still must meet explicit approval, strict TDD, required tests, active objective validation, expected-diff, and clean-boundary requirements before it can be committed.
- Unexpected changes in sensitive areas halt under the same unexpected-change rule as all other paths.
- The absence of a sensitive-scope stop does not weaken the Phase 4 review gate.
**Edge Cases:**
- A scenario's expected diff includes a migration and its matching test.
- A sensitive path is modified but was not expected for the current scenario.
- A sensitive scenario fails one required objective validation check.
**Out of Scope:**
- Adding a sensitive-scope approval prompt, special exception process, or automatic push policy.

### REQ-027: Prohibit Automatic Remote Publication
**Description:** Autonomous scenario checkpointing must never push to a remote; a human manually pushes only once after the workflow and review are complete.
**Acceptance Criteria:**
- The autonomous Phase 3 loop performs no `git push` or equivalent remote-publication action.
- A successful local scenario commit does not cause remote branch creation, remote update, tag publication, pull-request creation, or merge action.
- After Phase 4 review completes, Rotta reports that a human may perform one manual push and does not perform it itself.
- A failed scenario, validation halt, or review failure never triggers a push.
**Edge Cases:**
- The dedicated feature branch has an existing remote tracking configuration.
- A host or Git hook offers to publish a branch after commit.
- The human resumes the workflow after local checkpoints but before review completion.
**Out of Scope:**
- Designing remote permissions, CI workflows, pull requests, merge queues, or release automation.

## Open Questions
- None.

## Trade-offs
- One local commit per validated scenario creates a longer history, but provides precise scenario-to-commit traceability and a clean recovery point after every slice.
- A fail-closed expected-diff check can interrupt automation for harmless-looking files, but avoids silently committing or destroying user and tool output.
- Dedicated worktrees add setup overhead, but isolate the autonomous loop from a contributor's ordinary checkout and remove the need for a separate protected/default-branch rule.
- Removing the sensitive-scope stop accelerates a uniformly validated loop, but makes the ordinary approval, TDD, objective-validation, scope, and later review gates the only safeguards for those paths.

## Risk Level
high — Justification: This feature changes the authority boundary around local Git commits and workflow progression. Errors can commit unintended work, lose an explicit approval or review gate, strand a workflow after a partial checkpoint, or publish incomplete work if the no-push boundary is not enforced.
