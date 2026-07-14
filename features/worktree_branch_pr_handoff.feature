Feature: Isolated feature worktree and manual pull-request handoff
  Rotta users need every implementation submission isolated from their ordinary checkout so concurrent agents cannot contaminate protected branches and reviewed work remains under human publication control.

  Background:
    Given workspace artifacts are the source of truth for the submission
    And the protected branch names are "main", "master", "develop", "release/*", and "hotfix/*"

  @REQ-037 @REQ-038 @SCN-241
  Scenario: Create an isolated feature worktree before Phase 2 writes a contract
    Given the initiating Git worktree has no non-ignored changes
    And the configured or repository-default integration branch resolves successfully
    And the submission slug is a valid lowercase slug
    When Rotta prepares a new implementation submission
    Then Rotta creates "feature/<validated-slug>" from the selected integration branch
    And Rotta creates the prescribed isolated sibling worktree for that branch
    And Rotta reports the absolute worktree path, base branch, and feature branch
    And Phase 2 writes the durable contract only from that isolated worktree

  @REQ-037 @REQ-044 @SCN-242
  Scenario Outline: Reject an unsafe starting condition without falling back to the initiating worktree
    Given the new submission has the unsafe condition "<condition>"
    When Rotta prepares the submission worktree
    Then Rotta reports the failed validation
    And Rotta does not write a durable submission artifact or code in the initiating worktree
    And Rotta does not start Phase 2 or Phase 3
    And Rotta does not select an alternative branch or worktree

    Examples:
      | condition |
      | the initiating worktree has a non-ignored change |
      | the current checkout is not a Git repository |
      | HEAD is detached |
      | the integration branch cannot be resolved |
      | branch or worktree creation fails |

  @REQ-038 @REQ-044 @SCN-243
  Scenario Outline: Reject an invalid or unavailable feature branch
    Given the initiating Git worktree is clean
    And the selected integration branch is available
    And the submission value "<submission-value>" is invalid or unavailable because "<reason>"
    When Rotta prepares the submission worktree
    Then Rotta reports the invalid slug or branch conflict
    And Rotta does not create or reuse a feature branch
    And Rotta does not write in a protected branch, the base branch, or detached HEAD

    Examples:
      | submission-value | reason |
      | Feature Name | it contains uppercase letters and whitespace |
      | ../escape | it contains a path traversal separator |
      | release-fix | feature/release-fix already exists locally |

  @REQ-039 @REQ-044 @SCN-244
  Scenario: Reject a colliding sibling worktree path
    Given a valid slug and a clean initiating Git worktree
    And an existing file, directory, or symlink occupies "../<repository>-<slug>"
    When Rotta prepares the submission worktree
    Then Rotta reports the path collision
    And Rotta does not delete, move, or reuse the existing path
    And Rotta does not start Phase 2 or Phase 3 in the initiating worktree

  @REQ-039 @SCN-245
  Scenario: Allow concurrent submissions only with independent worktree ownership
    Given one active submission owns "feature/alpha" and its sibling worktree
    And another clean initiating worktree requests the distinct valid slug "beta"
    When Rotta prepares the second submission
    Then Rotta creates "feature/beta" in its distinct prescribed sibling worktree
    And each submission has a different branch, canonical path, and worktree state
    And neither submission writes in the other's worktree

  @REQ-040 @REQ-041 @SCN-246
  Scenario: Halt when a Phase 3 subagent boundary loses feature-worktree identity
    Given Phase 3 is running from the recorded isolated feature worktree
    And a subagent returns after the checked-out branch changed to the base branch
    When Rotta validates the post-subagent boundary
    Then Rotta reports the branch identity failure
    And Rotta does not commit, push, merge, rebase, or reset
    And Rotta does not launch the next subagent or fall back to the initiating worktree

  @REQ-040 @REQ-041 @SCN-247
  Scenario: Commit a validated scenario only on the recorded feature branch
    Given the recorded isolated worktree is attached to "feature/<validated-slug>"
    And an approved Phase 3 scenario has passed its required validation
    And the worktree has only the expected non-ignored scenario changes
    When Rotta creates the scenario checkpoint
    Then Rotta creates the local commit on "feature/<validated-slug>"
    And Rotta does not commit on a protected branch, the base branch, or detached HEAD
    And Rotta does not push, merge, rebase, reset, tag, or create a pull request

  @REQ-042 @REQ-043 @SCN-248
  Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
    Given Phase 4 has passed for the recorded feature worktree
    And a GitHub-capable push remote is resolved unambiguously
    When Rotta presents the completion handoff
    Then Rotta prints commands to enter the absolute worktree path and inspect "git status --short"
    And Rotta prints reviewed-path-only commit commands when reviewed outstanding changes exist
    And Rotta prints a push command for "feature/<validated-slug>"
    And Rotta prints "gh pr create --base <base-branch> --head feature/<validated-slug>"
    And Rotta offers the GitHub web UI alternative
    And Rotta discloses the active host's command/delegation limitations and user-controlled credentials
    And Rotta does not push, create a pull request, merge, or directly modify the base branch

  @REQ-042 @REQ-044 @SCN-249
  Scenario: Preserve the feature worktree when manual PR creation fails
    Given Phase 4 has passed and Rotta has presented the manual handoff
    And the user reports that the push or "gh pr create" command failed
    When Rotta receives the failure result
    Then Rotta reports the manual command failure and safe inspection guidance
    And Rotta preserves the isolated feature worktree and feature branch
    And Rotta does not retry automatically, create a PR by another mechanism, merge, or fall back to a direct base-branch change

  @REQ-042 @REQ-043 @SCN-250
  Scenario: Block guessed PR publication when no GitHub remote is unambiguous
    Given Phase 4 has passed for the recorded feature worktree
    And no configured submission remote or exactly one GitHub-capable remote can be resolved
    When Rotta prepares the manual handoff
    Then Rotta reports that remote selection requires user resolution
    And Rotta does not print a guessed push command
    And Rotta does not push, create a pull request, or merge
