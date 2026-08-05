# Hard Spec: Language-Agnostic PR-Readiness Quality Gates

## Adversarial Pre-Mortem
- Failure mode 1: command detection guesses `make test`, `npm test`, or a security command from a file name, reports a clean review, and leaves the actual project checks unrun.
- Failure mode 2: a waiver is stored as an undifferentiated pass or is reused after a commit/configuration change, allowing a different implementation to inherit an exception it was never granted.
- Failure mode 3: PR handoff trusts a caller-provided “Phase 4 passed” flag or a stale report, so an unreviewed or later-mutated commit is presented as ready.

## Hidden Assumptions
- Project-controlled metadata and conventions can provide an unambiguous, executable command for a gate, or their absence can be detected before a readiness decision is made.
- The current submission manifest and state identify the feature worktree, approved implementation snapshot, baseline comparison, and the only active TDD log.
- A deterministic fingerprint can identify the exact resolved quality-gates configuration and plan used for a review.
- Gate commands run in the recorded feature worktree against a committed review snapshot, with captured exit status and output available as evidence.
- A human may authorize an exception without Rotta storing that person’s identity.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Keep `rotta.quality-gates/v1` and add exceptions | v1 encodes Go-specific gates, unresolved placeholders, and no executable generic-plan/evidence/waiver semantics. Compatibility would preserve ambiguous behavior. |
| Require users to choose a language or profile in the TUI | Conflicts with language-agnostic operation and makes review behavior depend on a user-selected label rather than project evidence. |
| Ship one fixed command for every gate | Projects use different package managers, task runners, and metadata; a fixed command would either fail unnecessarily or silently test the wrong thing. |
| Treat unavailable commands as passed or not applicable | A required generic readiness gate would be bypassed without evidence. |
| Convert a waiver to `pass` | Erases the exception, prevents auditing, and makes the final PR state misleading. |
| Accept a handoff request’s supplied status, SHA, or paths | Caller input is not durable review evidence and can be stale or forged. |

## Summary
Replace the invalid Go-specific quality-gate configuration with a language-agnostic Phase 4 system that always evaluates build, tests, changed-file scope, static analysis, dependency checks, and security checks. Rotta must deterministically discover each gate command only from project conventions and metadata, persist the resolved plan and command evidence, fail closed when discovery is incomplete, and calculate PR readiness from that persisted evidence. Durable, commit- and configuration-bound human waivers may produce `ready_with_waivers` but never turn a failed/blocked gate into a pass. The installer TUI configures only generic threshold defaults, while lifecycle and PR handoff consume trusted persisted review state.

## Explicit Invariants
- The only generic Phase 4 gate categories are `build`, `tests`, `changed_file_scope`, `static_analysis`, `dependency_checks`, and `security_checks`; each is required in every review.
- Coverage, mutation, complexity, critical/named-function, language selection, and language-profile behavior are not Phase 4 quality gates in this feature.
- A command may be run only when deterministic discovery can identify its project-controlled source and resolution rule. Rotta never guesses, synthesizes, or silently substitutes a command.
- A gate status is one of `passed`, `failed`, `blocked`, or `waived`; `waived` is not `passed`.
- The only overall PR readiness states are `ready`, `ready_with_waivers`, `not_ready`, and `blocked`.
- A readiness state, waiver, review evidence, `reviewed_commit`, and PR handoff are valid only for the same recorded snapshot SHA and configuration fingerprint.
- The active TDD evidence path is `.rotta/current/tdd-log.md`; root `.rotta/tdd-log.md`, archived logs, and unrelated logs cannot satisfy or expand current review scope.
- Neither reviewer identity nor any substitute identity field is persisted in a waiver or review record.

## Requirements

### REQ-073: Provide a generic executable quality-gates configuration
**Description:** Rotta must define and install the current generic quality-gates configuration format. Its schema must declare the six required generic gate categories, their order, supported discovery inputs/rules, threshold defaults where applicable, and the evidence/waiver policy needed for review; it must contain no language-specific gate category or command.

**Acceptance Criteria:**
- A newly generated quality-gates configuration declares the current format and exactly one enabled required entry for each invariant gate category.
- Configuration validation rejects missing, duplicate, disabled, unknown, or reordered-without-explicit-order required categories, malformed threshold values, and unsupported discovery rules before command execution.
- An unsupported configuration is rejected with an explicit message that it is unsupported, is not migrated automatically, and identifies the generated/configuration remediation path.
- Validation failure persists a blocked review result and remediation; it does not use unsupported configuration data, embedded Go commands, or an inferred default configuration.
- The installed template and active project configuration use the same contract.

**Edge Cases:**
- A repository has an unsupported configuration plus an otherwise-valid stale review-evidence artifact.
- A user edits a generic category name to a former Go-specific gate.
- A config omits a numeric threshold for a category whose selected policy requires one.

**Out of Scope:**
- Automatic conversion, compatibility execution, or preservation of obsolete format semantics.
- Adding language-specific coverage, mutation, complexity, or named-function gates to the generic configuration.

### REQ-074: Deterministically resolve all generic gate plans without language selection
**Description:** For the recorded review snapshot, Rotta must resolve one executable plan for every required generic category from declared project conventions/metadata using documented deterministic precedence. The resolver may construct a conventional invocation only when the selected metadata/rule explicitly authorizes it; it must not use a language/profile setting or expose one in UX.

**Acceptance Criteria:**
- Resolution considers only supported project-controlled evidence such as declared package scripts, task-runner targets, lockfiles/manifests, repository-local tool configuration, and explicitly configured discovery rules.
- For every category, the persisted plan records category/gate ID, resolved command or built-in changed-file comparison, working directory, metadata source path, discovery rule, target/snapshot inputs, and a plan/configuration fingerprint.
- Given unchanged repository metadata, configuration, baseline, and review snapshot, repeated resolution produces the same plan and command order.
- If no permitted command can be resolved, if candidates of the same precedence conflict, if the command/tool is unavailable, or if a command is unsafe/invalid under the execution policy, that gate and the overall review become `blocked` with actionable remediation. Rotta does not run a guessed fallback.
- Changed-file scope compares the recorded review snapshot to the recorded baseline/approved scope and records the comparison inputs and result; it is not derived from an arbitrary caller path list or the host’s uncommitted diff.
- The TUI and generated instructions do not offer language or profile selection.

**Edge Cases:**
- A repository contains both `package.json` scripts and Make targets for the same category.
- A monorepo has multiple manifests with conflicting scripts.
- Metadata is modified after plan resolution, a lockfile is absent, or a resolved executable is missing from `PATH`.
- A documentation-only change has no source files, but the required generic categories still require an explicit resolved execution or a blocked outcome.

**Out of Scope:**
- Installing missing build, test, static-analysis, dependency, or security tools.
- Heuristic source-code inspection to infer commands that are not declared by a supported convention.

### REQ-075: Evaluate plans and persist reproducible review evidence
**Description:** Phase 4 must execute the resolved generic plan against the recorded committed snapshot and write the complete feature-scoped evidence artifact to `.rotta/current/review-evidence.yaml`. The evaluator, rather than instructions alone, is authoritative for producing gate and overall status.

**Acceptance Criteria:**
- Evaluation loads the active current submission, validates its approved scope and `.rotta/current/tdd-log.md` scenario evidence, and never uses root or archived TDD logs as active evidence.
- Before command execution, evaluation verifies that the recorded worktree, committed snapshot, baseline, and configuration fingerprint still match the resolved plan; mismatch produces `blocked` evidence.
- Evidence records schema version, submission/feature scope, baseline SHA, snapshot SHA, configuration fingerprint, plan fingerprint, timestamp, gate order, per-gate category/ID/status/remediation, resolved command details, exit status, captured output or a bounded artifact reference, measurements/threshold results where applicable, and a concrete remediation for every non-passed gate.
- The evidence artifact records `blocked` results even when resolution/configuration prevents a command from running; missing evidence can never be interpreted as success.
- Re-evaluation creates evidence for the current snapshot/configuration only; stale evidence cannot be reused after either changes.
- Failure to write complete evidence prevents a readiness state, lifecycle advance, and PR handoff.

**Edge Cases:**
- A command times out, produces unparsable output, or is interrupted.
- A command succeeds but evidence serialization fails.
- Concurrent mutation of the worktree, baseline, config, or current submission occurs during evaluation.

**Out of Scope:**
- Uploading command output to a third-party evidence service.
- Treating a conversational Judge report as a substitute for the persisted artifact.

### REQ-076: Derive PR readiness and durable waivers correctly
**Description:** Rotta must derive overall readiness exclusively from persisted evidence and valid durable waivers. A waiver is an exception record for a non-passed gate, not a status conversion.

**Acceptance Criteria:**
- `ready` is emitted only when all six required gates are `passed` and no valid waiver is applied.
- `ready_with_waivers` is emitted only when every gate is `passed` or validly `waived` and at least one gate is `waived`.
- `not_ready` is emitted when one or more gates are `failed` and no blocked condition prevents evaluation; `blocked` is emitted whenever configuration, resolution, integrity, or execution availability blocks any required gate.
- A durable waiver records gate ID(s), non-empty reason, snapshot SHA, configuration fingerprint, timestamp, scope, and optional expiry, and records no reviewer identity.
- A waiver applies only to the exact matching snapshot/configuration, named gate(s), and declared scope; an expired, malformed, mismatched, or duplicate/conflicting waiver is invalid and produces an actionable blocked result rather than a pass.
- Evidence retains the underlying failed/blocked outcome and records the applied waiver separately with gate status `waived`; it never overwrites the outcome to `passed`.
- A waiver may be applied only after the failed or blocked gate outcome is durably recorded for the matching review; it cannot excuse missing review evidence, invalid configuration, unknown gate IDs, plan-integrity failure, or a changed snapshot/configuration.

**Edge Cases:**
- Multiple waivers name overlapping gates with different reasons or expiry.
- A waiver is written after evidence generation, or a new waiver is added for a previously failed review.
- The machine clock is unavailable or cannot produce a valid timestamp.

**Out of Scope:**
- Reviewer accounts, signatures, identity capture, external ticket-system synchronization, or automatic waiver approval.

### REQ-077: Make final-review lifecycle commit-bound and complete
**Description:** A `ready` or `ready_with_waivers` Phase 4 decision must durably bind the evidence snapshot to `reviewed_commit` and transition the current submission to `final_human_review`; a non-ready decision must not enter that state.

**Acceptance Criteria:**
- On a ready state, the orchestrator atomically persists/links the qualifying evidence, records its snapshot SHA as `reviewed_commit`, and transitions to `final_human_review` without marking the feature complete.
- The transition records the overall readiness state and the evidence/configuration/plan fingerprints required to validate it on resume.
- `not_ready` and `blocked` preserve remediation and remain/re-enter review or remediation; neither may set or retain final-review eligibility for a different snapshot.
- On resume or final human approval, Rotta verifies the current approved implementation snapshot and evidence fingerprint against `reviewed_commit`; any amendment, rebase, checkpoint, configuration change, or evidence mismatch invalidates eligibility and requires another review.
- Explicit final human approval is still required to reach `complete`, including for `ready_with_waivers`; no reviewer identity is stored.

**Edge Cases:**
- Evidence is ready but state persistence fails partway through the transition.
- A previously ready snapshot is superseded by a failed review of a later snapshot.
- A feature is resumed in `final_human_review` from another supported host.

**Out of Scope:**
- Automatic final approval, commit, push, PR creation, or merge.

### REQ-078: Generate PR handoff only from trusted ready evidence
**Description:** Manual GitHub PR handoff must load and verify the recorded current-submission lifecycle and persisted review evidence; it must not accept caller-supplied readiness, commit, branch, or reviewed-path assertions as authority.

**Acceptance Criteria:**
- Handoff is available only when persisted evidence has overall state `ready` or `ready_with_waivers`, the evidence snapshot/configuration/plan fingerprints match current durable state, and the worktree’s approved snapshot matches `reviewed_commit`.
- Handoff refuses `not_ready`, `blocked`, missing, stale, malformed, or mismatched evidence and reports the failed/blocked metrics and remediation instead of publication commands.
- When eligible, output identifies the readiness state and any waived gate IDs/reasons, prints the recorded absolute worktree path and `git status --short`, and derives any optional reviewed paths, branch, base branch, remote, and web URL from trusted recorded state/repository inspection.
- Caller-supplied handoff fields are treated as display/request inputs only and cannot override persisted evidence or cause a command to be generated.
- Handoff remains manual: it does not execute Git/GitHub commands, push, create a PR, merge, or use credentials.

**Edge Cases:**
- A caller supplies a passing status while no evidence exists.
- The evidence is ready but the branch tip was rebased after review.
- GitHub remotes are absent or ambiguous.

**Out of Scope:**
- Automated publication or resolving ambiguous remote ownership.

### REQ-079: Make the installer TUI meaningful and language-neutral
**Description:** The installer TUI must make a real selection only for generic threshold defaults and must accurately disclose the generated configuration path, generic-gate detection/readiness model, and blocked metrics. It must remove the inert “review later” behavior and all language-specific threshold/profile text.

**Acceptance Criteria:**
- The quality-gate screen describes the six generic categories and states that commands are detected from project conventions/metadata during review, not selected by language.
- For the selected project path, the screen presents each generic category's detection/readiness preview: the resolved command/source when detected, or its blocked category/metric and remediation when it cannot be resolved. This preview is informational and does not execute Phase 4 or create readiness evidence.
- Every selectable default option changes only the generated threshold values/policy values it names; selection is reflected in the generated `.rotta/quality-gates.yaml` and confirmation summary.
- The screen displays the generated configuration path `.rotta/quality-gates.yaml`, the count/list of blocked preview metrics, and explains that unresolved required commands result in `blocked` readiness with remediation.
- The screen identifies the persisted review-evidence location and the four possible readiness states, including that waivers remain visible as `ready_with_waivers`.
- The UI contains no coverage, mutation, complexity, named-function, Go-specific, language, or profile choice; it does not offer an option whose behavior is identical to another option.

**Edge Cases:**
- A user goes back and changes a threshold selection before installation.
- Installation fails after selection but before config generation.
- Terminal width truncates descriptions; the selected policy and config path remain discoverable.

**Out of Scope:**
- Interactive command editing, runtime language detection, or executing Phase 4 from the installer.

### REQ-080: Propagate the canonical contract across generated surfaces
**Description:** The generic schema, discovery, evaluator/evidence, waiver, lifecycle, handoff, and TUI semantics must be consistently represented in embedded assets, installer output, host instructions, state-machine/workflow logic, and verification fixtures so generated hosts cannot revive obsolete Phase 4 behavior.

**Acceptance Criteria:**
- All generated review guidance directs reviewers to the executable evaluator and `.rotta/current/review-evidence.yaml`, required generic categories, fail-closed discovery, waiver semantics, and evidence-derived readiness.
- No generated asset, installer test fixture, state-machine declaration, or TUI copy claims v1 support, root TDD-log authority, Go-only commands, coverage/mutation/complexity/named-function gates, or untrusted handoff eligibility.
- Supported host surfaces preserve the same readiness states, waiver semantics, final-human-review requirement, and evidence freshness checks.
- Focused regression tests cover config validation, deterministic discovery, evaluator evidence, waiver validity, TDD-log scope, lifecycle transition, trusted handoff, and TUI output/selection behavior.

**Edge Cases:**
- An old generated host asset remains on disk while the workspace has current configuration.
- Installation is retried into a repository containing an unsupported configuration or legacy root evidence.
- Ancora is unavailable; workspace artifacts remain the sole authority and no readiness rule is bypassed.

**Out of Scope:**
- Retroactively editing third-party host installations that are not being installed or updated.

## Open Questions
- None. The resolved decisions establish mandatory categories, discovery behavior, failure semantics, waiver fields, readiness states, lifecycle binding, and UI boundaries sufficiently for implementation.

## Trade-offs
- Fail-closed command detection can block projects whose conventions are not yet supported, but prevents false PR readiness.
- Persisting full command outcomes increases local artifact size and may require bounded output references, but makes a readiness decision reproducible.
- Commit/configuration-bound waivers require reauthorization after legitimate changes, but prevent exception leakage to a different review target.
- Removing language-specific metrics reduces specialized quality coverage, but keeps Phase 4 portable and makes its generic guarantees executable.

## Risk Level
critical — Justification: this changes the authority for PR readiness, command execution, durable exceptions, lifecycle progression, and publication guidance. A false pass, stale waiver, or stale handoff could directly misrepresent an unreviewed commit as ready for PR.
