# Hard Spec: Isolated Worktree, Feature Branch, and Manual PR Handoff

## Adversarial Pre-Mortem
- Failure mode 1: Rotta writes a Phase 2 contract or Phase 3 change in the contributor's ordinary checkout before isolation, then creates a worktree too late; the protected/default branch is left dirty or receives a commit.
- Failure mode 2: concurrent agents derive the same slug or follow a symlink/colliding sibling path, so they share a worktree, overwrite workflow state, or create competing branches.
- Failure mode 3: an automation convenience path pushes, creates a PR, or merges after review, bypassing the user's cross-host credentials, repository policy, and final inspection.

## Hidden Assumptions
- The initiating checkout is a Git worktree whose non-ignored status can be inspected, and Git can create a sibling worktree from a locally resolvable integration branch.
- A configured integration branch, when present, is authoritative; otherwise the repository's remote default branch is available and resolvable. The selected base must not be a protected branch for writing, but may be read as the branch from which the feature branch is created.
- A submission slug is supplied by the request/workflow and can be validated before any filesystem or Git mutation. This contract fixes its accepted syntax as lowercase ASCII `[a-z0-9]+(?:-[a-z0-9]+)*`.
- Workspace contracts and workflow state are authoritative. Ancora is unavailable in this drafting session and must not prevent Phase 2 or later work when workspace artifacts exist.
- OpenCode can consume named agents/skills, Claude Code uses its adapted skills/settings surface, and Codex uses an adapted `AGENTS.md` plus natural-language Rotta invocations; Codex does not receive OpenCode-style named sub-agents, skill directories, or slash commands.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Create the feature worktree only when Phase 3 starts | Phase 2 would already have written durable contract artifacts in the ordinary checkout, violating the isolation boundary and allowing a dirty protected/base checkout. |
| Let an agent reuse an existing branch or worktree when its name/path matches | Reuse cannot establish exclusive ownership, can inherit unreviewed changes or stale state, and is unsafe under concurrent submissions. |
| Automatically push, open, or merge a PR after Phase 4 | Cross-host credentials, remote policy, CI, review, and merge authority belong to the user; automation must not publish or merge reviewed work. |

## Summary
Before a new implementation submission enters Phase 2 or Phase 3, Rotta must fail closed unless it creates an exclusively owned sibling Git worktree on a new `feature/<validated-slug>` branch based on the configured or repository-default integration branch. The ordinary/base checkout is read-only for the submission and must be clean. Phase 2, Phase 3, and their subagents operate only from the isolated worktree and repeatedly prove its branch and clean-boundary ownership. Local commits are allowed only on that feature branch. After a passing Phase 4, Rotta stops at a manual, host-neutral handoff: it prints fully resolved commands for the user to inspect, optionally commit reviewed outstanding work, push, and create a PR, while never pushing, creating a PR, merging, rebasing, or resetting itself.

## Requirements

### REQ-037: Establish Isolation Before Durable Submission Work
**Description:** Rotta must create and select the isolated feature worktree before Phase 2 or Phase 3 writes any durable workflow artifact, production/test code, or commit for a new implementation submission.
**Acceptance Criteria:**
- Phase 1 may perform read-only discovery, but may not write submission specs, features, `.rotta/` state, reports, code, tests, or commits in the initiating/base worktree.
- Before the first Phase 2 or Phase 3 write, Rotta validates the initiating/base worktree is a Git worktree and that `git status --short` contains no non-ignored entries.
- Rotta creates the submission worktree at the validated sibling path and reports its absolute path, selected base branch, and feature branch before launching a Phase 2 or Phase 3 subagent.
- All durable submission artifacts, including this submission's specs, features, workflow state, reports, code, tests, and commits, are written from the isolated worktree.
- A failure at any validation or creation step stops the submission before durable writes or fallback use of the initiating worktree.
**Edge Cases:**
- The command is launched outside a Git repository, from a nested path, or in detached HEAD.
- The initiating worktree has staged, unstaged, deleted, renamed, or untracked non-ignored changes.
- Ignored local artifacts exist but `git status --short --ignored` shows no non-ignored change; these do not make the base dirty.
**Out of Scope:**
- Cleaning, stashing, committing, or discarding a user's base-worktree changes.
- Migrating artifacts that an earlier workflow already wrote in a base worktree.

### REQ-038: Select a Safe Base and Create a Valid Feature Branch
**Description:** Each submission must use a newly created `feature/<validated-slug>` branch based on a safe integration branch and must never write on a protected branch or detached HEAD.
**Acceptance Criteria:**
- Rotta selects the configured integration branch when configured; otherwise it resolves the repository default branch. The resolved branch is the submission base branch.
- The base branch must resolve to an existing local or remotely resolvable branch tip. Rotta records the resolved branch name and starting commit before creation.
- The slug must match `[a-z0-9]+(?:-[a-z0-9]+)*`; empty strings, uppercase letters, whitespace, path separators, dot segments, control characters, Git revision syntax, and values that fail Git branch validation are rejected.
- Rotta creates exactly the new local branch `feature/<validated-slug>` at the selected base commit.
- Rotta rejects detached HEAD and rejects any attempt to create, select, or commit on `main`, `master`, `develop`, `release/*`, `hotfix/*`, or the selected base branch. Protected-name matching is case-sensitive because Git branch names are case-sensitive.
- If the feature branch already exists locally, is checked out by any listed worktree, or cannot be created, Rotta stops without reusing it or falling back to another branch/worktree.
**Edge Cases:**
- A configured base is missing, ambiguous, protected only by a local naming convention, or differs from the remote default.
- A slug is valid text but the resulting feature branch conflicts with an existing ref.
- Git branch/worktree creation fails because of permissions, hooks, locks, filesystem failure, or a concurrent ref update.
**Out of Scope:**
- Changing server-side branch protection, fetching/remoting arbitrary branches, or selecting a release/hotfix workflow.

### REQ-039: Validate Collision-Safe Sibling Worktree Ownership
**Description:** Rotta must give each submission an exclusively named direct sibling worktree and must reject duplicate active ownership.
**Acceptance Criteria:**
- The target path is exactly `../<repository>-<slug>`, where `<repository>` is the validated basename of the Git repository's top-level directory and `<slug>` is the accepted submission slug.
- Rotta canonicalizes the parent and candidate path before mutation and verifies the candidate is a direct child of the canonical parent; it must not traverse or accept `..`, separators, or symlink substitution.
- Any existing filesystem entry at the candidate path, including a directory, file, broken symlink, or symlink, is a collision and stops the submission without deleting, moving, or reusing it.
- Rotta inspects `git worktree list --porcelain` (or an equivalent authoritative Git worktree listing) and rejects an active worktree with the target path or feature branch.
- Concurrent submissions may proceed only after each independently passes branch and path ownership validation with distinct slugs, branches, and sibling paths.
**Edge Cases:**
- Two agents race after validation; only the Git operation that successfully obtains branch/worktree ownership may continue, and the loser reports the creation failure.
- The repository directory basename cannot safely form the prescribed sibling name.
- A stale Git administrative worktree entry or a path collision remains after a manually removed worktree.
**Out of Scope:**
- Sharing a worktree, a feature branch, or `.rotta/current/` state between agents.

### REQ-040: Enforce Worktree and Branch Identity at Subagent Boundaries
**Description:** Phase 2 and Phase 3 must run only in the submission worktree and validate its ownership, branch, and required cleanliness at every subagent boundary.
**Acceptance Criteria:**
- Before and after every Phase 2 or Phase 3 subagent invocation, Rotta verifies the current directory resolves to the recorded isolated worktree, HEAD is attached to the recorded `feature/<slug>` branch, and no protected/base branch is checked out.
- Before a Phase 2 subagent starts, the new isolated worktree has no non-ignored changes. Phase 2 may then create the scoped durable contract artifacts in that worktree.
- Before each Phase 3 scenario subagent starts, the isolated worktree has no non-ignored changes except explicitly ignored local artifacts; after it returns, Rotta validates the same branch/worktree identity before applying the approved checkpoint policy.
- A boundary validation failure, unexpected non-ignored change, detached HEAD, branch switch, missing worktree, or worktree-list ownership mismatch halts the workflow and identifies the failed check. Rotta does not repair, stash, reset, rebase, or redirect the task to the initiating worktree.
- Phase 4 receives the recorded submission worktree/feature branch as its review context; it does not authorize branch mutation or publication.
**Edge Cases:**
- A user or another agent changes branches, deletes the worktree, or writes a file while a subagent is running.
- An ignored runtime artifact changes between checks.
- The host launches a subagent with a different working directory than requested.
**Out of Scope:**
- Allowing concurrent writers in the same worktree.

### REQ-041: Restrict Git Mutation and Commit Operations
**Description:** Rotta may create local commits only while attached to the recorded feature branch and must never mutate protected/base branch history or publish changes.
**Acceptance Criteria:**
- Before every commit, Rotta proves HEAD is attached to the recorded feature branch in the recorded isolated worktree.
- Rotta never commits directly on `main`, `master`, `develop`, `release/*`, `hotfix/*`, the selected base branch, or detached HEAD.
- Rotta never pushes, merges, rebases, resets, force-pushes, tags, or creates a pull request during Phases 2–4, including on error, retry, cleanup, or recovery paths.
- A prohibited or failed Git operation is reported and leaves the feature worktree and branch intact for user inspection; Rotta does not compensate by mutating the base/protected branch.
**Edge Cases:**
- Git aliases, hooks, host integrations, or prompts offer to publish or rewrite history.
- A commit command succeeds but subsequent workflow-state persistence fails.
- The feature branch has an existing upstream due to external user action.
**Out of Scope:**
- Configuring remotes, repository permissions, branch protection rules, merge queues, or release tags.

### REQ-042: Provide a Manual PR Handoff Only After Passing Phase 4
**Description:** After Phase 4 passes, Rotta must provide a manual, user-controlled PR handoff and must not perform any remote or PR action.
**Acceptance Criteria:**
- Rotta issues no handoff for a failed, incomplete, or unreviewed Phase 4 result; it reports the applicable remediation/blocked state instead.
- For a passing Phase 4 result, Rotta prints the absolute isolated-worktree path and fully resolved, user-executable commands in this order: `cd <absolute-worktree-path>`, `git status --short`, an optional `git add <reviewed-path>...` and `git commit -m "<reviewed summary>"` only when reviewed outstanding non-ignored changes exist, `git push -u <resolved-github-remote> feature/<slug>`, and `gh pr create --base <base-branch> --head feature/<slug>`.
- The optional commit commands must name only the reviewed outstanding paths; Rotta must not use `git add .` or `git add -A` in the handoff.
- Rotta resolves `<resolved-github-remote>` from the configured submission remote, or, if none is configured, from exactly one GitHub-capable remote. If it cannot resolve one unambiguously, it reports that PR handoff is blocked and prints no guessed push command.
- Rotta also offers the GitHub web UI alternative: open the repository's GitHub compare/new-pull-request page, select `feature/<slug>` as the compare branch and `<base-branch>` as the base branch, then create the PR manually.
- If the user reports or supplies a failure from the push or `gh pr create` command, Rotta reports that failure, preserves the feature branch/worktree unchanged, and offers inspection/remediation guidance only; it does not retry automatically, fall back to direct merge, create a PR by another mechanism, or mutate the base branch.
**Edge Cases:**
- `gh` is not installed, unauthenticated, targets the wrong GitHub host, or lacks PR permission.
- Push is rejected by remote policy, network failure, a non-fast-forward update, or an unavailable remote.
- Reviewed changes remain after Phase 4, no reviewed changes remain, or unreviewed changes are present.
**Out of Scope:**
- Automatic PR creation, PR editing, reviewer assignment, CI triggering, merge queue enrollment, merge, or direct deployment.

### REQ-043: Preserve Cross-Host Policy and Disclose Limitations
**Description:** This isolation and handoff policy must have the same safety semantics for OpenCode, Claude Code, and Codex while clearly disclosing each host's command/delegation limitations.
**Acceptance Criteria:**
- OpenCode, Claude Code, and Codex follow the same base-cleanliness, branch/path validation, subagent-boundary, no-publication, and manual-handoff rules.
- OpenCode uses its native named-agent/skill surface; Claude Code uses its installed adapted skill/settings surface; Codex receives the policy in `AGENTS.md` and uses documented natural-language Rotta invocations rather than OpenCode-style named subagents, skill directories, or slash commands.
- A host that cannot launch a delegated subagent must still perform the boundary checks around its equivalent sequential role invocation; it must not skip isolation or run Phase 2/3 in the base worktree.
- Handoff output identifies the active host and any relevant limitation, including that GitHub CLI authentication, browser access, remote credentials, and PR/merge permissions are user-controlled and not supplied by Rotta.
**Edge Cases:**
- A workflow starts on one supported host and resumes on another from the same isolated worktree.
- A host lacks `gh`, browser access, named agents, or a slash-command surface.
- Host-local configuration conflicts with the recorded workspace submission state.
**Out of Scope:**
- Making host-specific credential stores, GUIs, PR permissions, or command surfaces identical.

### REQ-044: Fail Closed and Preserve Evidence
**Description:** Any invalid repository, branch, slug, path, worktree, cleanliness, or handoff condition must fail visibly without destructive cleanup or unsafe fallback.
**Acceptance Criteria:**
- Error reports name the failed validation category, attempted branch/path when safe to disclose, current workflow phase, and a safe user recovery action.
- Failure never causes Rotta to write durable submission artifacts or code in the initiating/base worktree, reuse a duplicate branch/worktree, or silently choose an alternative slug, path, base branch, remote, or protected branch.
- Failure preserves pre-existing user files, existing worktrees/branches, and any successfully created isolated feature worktree for inspection; Rotta does not delete, reset, stash, rebase, or commit them automatically.
- Ancora unavailability is reported as a non-blocking degradation; workspace artifacts remain authoritative and the isolation/approval gates remain in force.
**Edge Cases:**
- Git is absent, the repository is corrupt, a lock is held, filesystem access is denied, or a command times out.
- The error occurs after branch creation but before worktree creation, or after worktree creation but before Phase 2 begins.
- A base worktree becomes dirty after initial validation but before the Git creation operation.
**Out of Scope:**
- Automatic repair of Git repositories, locks, remote credentials, or filesystem permissions.

## Open Questions
- None. The draft's stated manual handoff and isolation decisions, together with the validation rules fixed in this contract, resolve the operational choices needed for implementation.

## Trade-offs
- Creating the worktree before Phase 2 adds setup and Git failure points, but prevents contracts and code from contaminating the ordinary checkout.
- Strict rejection of existing branches/paths sacrifices convenient resume-by-reuse, but establishes exclusive ownership and makes concurrent behavior auditable.
- Manual publication adds user steps and requires a configured GitHub remote/credentials, but preserves cross-host authority boundaries and prevents accidental remote mutation.
- A narrow lowercase slug format reduces naming flexibility, but prevents path traversal, ambiguous branch syntax, and collision-prone worktree names.

## Risk Level
high — Justification: this feature changes the boundary for every submission's filesystem and Git mutations. A defect can contaminate a protected branch, lose work through unsafe recovery, collide concurrent agents, or publish/merge changes without human authority.
