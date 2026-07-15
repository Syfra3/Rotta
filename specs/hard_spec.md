# Hard Spec: Unified Rotta Workflow Authority and Lifecycle

## Adversarial Pre-Mortem
- Failure mode 1: A generated host instruction, skill, fixture, or implementation fallback still reads a legacy marker, so an unrelated or unapproved scenario starts despite the new record being absent or invalid.
- Failure mode 2: A phase agent creates state, commits a checkpoint, or transitions lifecycle state directly; concurrent or resumed hosts then disagree about which commit and phase are authoritative.
- Failure mode 3: Review passes at commit A, code changes to commit B, and a stale final-human-review signal is treated as approval of B, allowing unreviewed code to become complete.
- Failure mode 4: Review behavior remains partly embedded in prompts (thresholds, function names, commands, or parser assumptions), so an edited `.rotta/quality-gates.yaml` is ignored or only partially applied.
- Failure mode 5: A directly invoked Claude phase skill bypasses the orchestrator, clean-boundary check, approval validation, or scenario sequencing.

## Hidden Assumptions
- Git commits are available for the recorded feature worktree, and the orchestrator can identify the checked-out commit before every lifecycle decision.
- The feature slug is already validated and uniquely identifies the approval record at `specs/approvals/<feature-slug>.yaml`.
- A durable current-submission/lifecycle artifact exists or will be defined under `.rotta/` and can reference the feature slug, recorded worktree, confirmed baseline, phase, and checkpoints without becoming an alternate approval authority.
- The approval record schema can express structured approved scenarios, contract fingerprints, a pending baseline reference, an orchestrator-owned baseline-confirmation record, phase/review status, and `reviewed_commit` without recording a reviewer identity.
- `.rotta/quality-gates.yaml` can be extended into a complete executable review-gate configuration, including every evidence command and any parser/target data required to evaluate it.
- CI can install or otherwise invoke Claude Code for compatibility verification; an interactive installer need not have `claude` on `PATH`.
- Existing workflow artifacts and history are intentionally disposable for this feature: no legacy approval/state migration or compatibility behavior is required.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Retain `specs/.approved` as a read-only fallback | A fallback is still an authority path and can approve or block the wrong feature. |
| Let each phase agent checkpoint its own work | It violates single-writer lifecycle authority and makes cross-host resume races likely. |
| Mark Phase 4 pass as `complete` | It conflates objective evidence with explicit final human acceptance. |
| Hardcode common gates while allowing config overrides | It makes configuration non-authoritative and produces silent host drift. |
| Keep user-invocable phase skills as shortcuts | A shortcut can enter a later phase without enforcing the orchestrator-owned state transition. |
| Store a baseline commit's own SHA in the baseline artifact | A Git commit cannot contain its own SHA; it creates an unsatisfiable self-reference. |

## Summary
Replace Rotta's fragmented workflow authorities with one feature-scoped approval authority and an orchestrator-owned lifecycle that behaves identically across generated host instructions. `specs/approvals/<feature-slug>.yaml` is authoritative only when its structured approved-scenario references and contract fingerprints validate against an immutable baseline confirmed by an orchestrator-owned follow-up commit; the baseline artifact itself carries a pending baseline reference rather than an impossible self-reference. Legacy approval/state mechanisms are removed, not migrated. Only the orchestrator may create or transition approval, current-submission, lifecycle, and checkpoint state; phase roles are constrained to their artifacts and evidence. Objective review success enters a durable `final_human_review` state tied to the reviewed commit, and explicit human approval alone completes that same snapshot. The review configuration file becomes the exclusive source for all objective-gate thresholds, applicability, and commands. Claude-facing entrypoints route through orchestration, while compatibility verification is performed in CI and records the Claude version.

## Requirements

### REQ-001: Establish the Sole Approval Authority
**Description:** For every new or resumed full workflow, the only artifact that can authorize approved scenarios is the authoritative confirmation form of `specs/approvals/<feature-slug>.yaml`. Its `approved_scenarios` entries are structured objects containing `feature_path`, `scenario_id`, and `requirement_ids`; each entry is a claim about the immutable approved contract, not a display reference. The baseline artifact commit contains that contract and an approval record with a pending baseline reference. A subsequent orchestrator-owned confirmation commit records the baseline artifact commit SHA; that confirmation record is authoritative and validates the immutable baseline, avoiding an impossible self-referential baseline SHA.

**Acceptance Criteria:**
- The orchestrator resolves the feature slug and loads only the corresponding feature-scoped approval record when deciding whether a workflow may enter or resume Phase 3 or Phase 4.
- The record includes, at minimum, feature identity, structured `approved_scenarios`, contract fingerprints, baseline-reference state, lifecycle status, and `reviewed_commit` when applicable. Each `approved_scenarios` object contains exactly the authoritative `feature_path`, `scenario_id`, and `requirement_ids` fields.
- The orchestrator creates the immutable baseline artifact commit with a pending baseline reference, then creates the confirmation commit that records that baseline artifact commit SHA. Only the orchestrator may create either commit or make the confirmation record authoritative.
- Before acting, the orchestrator verifies that the confirmed baseline SHA exists, is reachable in the recorded feature worktree history, and names the immutable baseline artifact commit; it verifies that baseline's approval record is pending rather than self-referential and that the baseline contract matches the confirmation record's fingerprints and approved scenario scope.
- For every `approved_scenarios` entry, validation fails closed unless `feature_path` is a canonical repository-relative path to the named feature file; `scenario_id` resolves to exactly one scenario ID in that named file; the resolved scenario's requirement tags exactly match `requirement_ids`; and the named feature contract is covered by the record's contract fingerprint.
- Validation fails closed if a structured entry is duplicated within the record, if any active feature record globally duplicates its scenario ID, or if an entry uses a line number, display name, or opaque `path#SCN` string as authoritative scenario identity. Those values may be diagnostic-only and cannot authorize a scenario.
- A missing, malformed, mismatched, uncommitted, unreachable, or contract-drifted record fails closed before implementation, review, or completion and reports the precise failed condition.
- No other artifact, host-local state, memory pointer, report, or human conversational claim can substitute for a missing or invalid record.

**Edge Cases:**
- Two feature worktrees have different valid records and overlapping scenario IDs.
- A scenario ID appears more than once in one record, appears in two active feature records, appears zero times in the named feature file, or appears more than once in that file.
- A structured entry names a non-canonical, absolute, traversal-containing, renamed, missing, or fingerprint-uncovered feature path; supplies no requirement IDs; or supplies requirement IDs that differ from the resolved scenario tags.
- A record presents a scenario title, source line, or `features/example.feature#SCN-001` as its approval reference, including where that text would otherwise resolve unambiguously.
- A record path exists but its slug, branch/worktree identity, baseline, or fingerprints name another feature.
- A baseline commit exists locally but has been pruned, replaced, or is not reachable from the recorded branch.
- The baseline artifact commit is present but its corresponding confirmation record remains pending, the confirmation commit is absent or not orchestrator-owned, or the confirmation SHA points to itself, a later mutable record, or a different immutable baseline.
- A human approves only a scenario subset; only that subset is eligible.

**Out of Scope:**
- Remote publication of the baseline commit.
- Inferring approval from tests, reports, timestamps, Git messages, or an agent transcript.
- Treating line numbers, display names, opaque path-and-ID strings, or a baseline artifact's own commit SHA as approval authority.

### REQ-002: Remove All Legacy Workflow Authority Without Migration
**Description:** Rotta must remove legacy workflow state and `specs/.approved` from templates, fixtures, implementation, generated instructions, tests, and runtime behavior. There is no read path, write path, migration, fencing, reporting-as-legacy, or compatibility fallback for them. A workflow without a valid feature-scoped record starts fresh through normal Phase 1/2 flow.

**Acceptance Criteria:**
- The repository's active templates, embedded assets, installer output, workflow implementation, fixtures, and tests contain no behavior that reads, writes, creates, validates, reports, or relies on `specs/.approved` or any retired legacy workflow-state artifact.
- State-machine and generated-host contracts list only the canonical feature-scoped authority and current lifecycle artifacts.
- Resume discovers no valid feature-scoped record: it does not inspect legacy files and routes to a fresh workflow rather than attempting migration or recovery.
- Installation and generated artifacts do not recreate legacy files.
- Automated verification demonstrates that legacy files, if manually present, have no effect on approval, phase selection, review, or completion.

**Edge Cases:**
- A repository contains both a stale legacy marker and no feature record.
- A repository contains a legacy marker whose scenario IDs match the requested feature.
- An upgrade runs in a project with ignored legacy artifacts or historical commits containing them.

**Out of Scope:**
- Backfilling, translating, archiving, or preserving legacy approval/state contents.
- Supporting workflows begun under the removed lifecycle; they must start fresh.

### REQ-003: Make the Orchestrator the Exclusive Lifecycle Writer
**Description:** The orchestrator alone owns lifecycle decisions and writes: creation and mutation of approval records, current-submission state, phase transitions, scenario acceptance, checkpoint metadata, lifecycle archives, and commits that persist those lifecycle boundaries. Spec, implementation, and review roles may not advance lifecycle state or approvals.

**Acceptance Criteria:**
- The orchestrator is the only role instructed and implemented to create an approval record, create its pending baseline artifact and authoritative confirmation record, record `reviewed_commit`, transition a phase, accept a scenario result, commit a baseline/checkpoint, or clean/checkpoint a scenario boundary.
- Spec Mode writes only `specs/hard_spec.md` and approved Gherkin contract artifacts when delegated; it does not create approvals, baselines, current state, or commits.
- Implementation Mode writes only its explicitly owned implementation/test and TDD evidence artifacts, reports scenario evidence and changed paths, and stops after its assigned scenario; it does not select the next scenario, transition state, approve, commit, clean, or write completion/lifecycle state.
- Review Mode writes only explicitly owned review evidence artifacts and returns pass/fail/escalate evidence; it does not mutate approval/current/lifecycle state, commit, or mark complete.
- The orchestrator validates an agent report against the approved scope and required evidence before accepting it and making the next state/checkpoint action.
- Generated instructions and host adapters preserve these ownership rules across OpenCode, Claude Code, and Codex.

**Edge Cases:**
- An implementation or review agent is run directly, retries after a timeout, or returns evidence after the orchestrator has moved on.
- A phase agent can write broadly due to host permissions; instructions and validation still prevent its output becoming lifecycle authority.
- Current-state persistence fails after evidence succeeds; the orchestrator does not falsely advance and reports recoverable state ambiguity.

**Out of Scope:**
- Delegating lifecycle ownership to a host-native task runner, memory service, or phase agent.
- Giving phase roles authority to resolve approval or checkpoint conflicts.

### REQ-004: Define Durable Lifecycle States and Final Human Review
**Description:** The canonical state machine must include an explicit durable `final_human_review` state. A Phase 4 objective pass records the exact implementation `reviewed_commit` and transitions to that state. Only explicit human approval of that recorded snapshot transitions it to `complete`; no reviewer identity field is stored.

**Acceptance Criteria:**
- The state model has explicit non-terminal review and `final_human_review` states plus terminal `complete`, with legal transitions owned only by the orchestrator.
- On a Phase 4 pass, the orchestrator verifies the feature worktree is at a committed snapshot, records that SHA as `reviewed_commit`, and transitions atomically/durably to `final_human_review`.
- `complete` is reachable only from `final_human_review` after explicit human approval; objective gate pass alone never completes a feature.
- The approval record does not include a reviewer identity field.
- Before accepting final approval and on resume, the orchestrator compares the recorded `reviewed_commit` to the current approved implementation snapshot. Any later code change invalidates final approval, clears or supersedes the final-review eligibility, and returns the feature to review before completion is possible.
- Contract changes remain governed by the baseline/approval validity rules and cannot be accepted as a mere later-code-change review.

**Edge Cases:**
- The human approval arrives after a new checkpoint, manual commit, amendment, rebase, or dirty code change.
- Phase 4 passes but recording `reviewed_commit` or transitioning state fails.
- A feature resumes in `final_human_review` from another host with a detached HEAD or changed branch tip.
- A review fails after a prior final-human-review eligibility; the stale `reviewed_commit` cannot complete the feature.

**Out of Scope:**
- Capturing reviewer names, identities, signatures, or external approval-system integration.
- Automatic final approval or automatic publication after review pass.

### REQ-005: Route User-Invocable Claude Phase Entry Points Through Orchestration
**Description:** Any Claude skill, command, or natural-language entrypoint that a user can invoke for a phase must route to the Rotta orchestrator. It must not directly execute Spec, implementation, or review behavior. The orchestrator validates current workspace authority and delegates only the legal next phase.

**Acceptance Criteria:**
- Claude-facing user-invocable phase surfaces are wrappers/routers to the orchestrator, or are removed as direct phase entrypoints.
- A user request for a later phase with missing approval, invalid state, or an earlier required phase is stopped/routed by the orchestrator; it cannot cause the phase role to execute directly.
- Orchestrator-delegated phase agents remain non-user-invocable and receive only the phase task permitted by canonical state.
- Equivalent host instructions preserve the same no-bypass behavior even where host command primitives differ.
- Tests verify both normal user feature requests and explicit phase-like invocations pass through the orchestrator decision point.

**Edge Cases:**
- A user invokes a legacy skill name cached by the host.
- A host lacks hidden subagents or slash commands.
- An agent is unavailable after the orchestrator validates the phase; no substitute direct phase execution occurs.

**Out of Scope:**
- Guaranteeing identical host UI labels, autocomplete, or command syntax.

### REQ-006: Make Quality-Gates Configuration Fully Authoritative
**Description:** Review Mode must derive every objective review gate's names, order, enabled/applicable status, thresholds, critical-function list, evidence commands, targets, parsing rules, severity, and remediation outcome exclusively from `.rotta/quality-gates.yaml`. No gate name, threshold, critical function, command, runner, parser, or fallback value may be hardcoded in implementation or generated instructions.

**Acceptance Criteria:**
- The configuration schema supplies sufficient data to execute and evaluate every enabled objective gate, including all evidence commands and output interpretation required by that gate.
- Review iterates configured gates and executes only their configured evidence commands/targets; it does not invent a runner, compare branch, package target, parser, threshold, or gate-specific exception.
- A missing, unreadable, malformed, incomplete-for-enabled-gates, or internally inconsistent configuration stops review with a configuration error; no embedded defaults are used.
- Threshold changes, disabled gates, severity/remediation changes, command changes, and critical-function changes take effect without modifying review code or instructions.
- Critical-function coverage/applicability uses only the configured list. An empty list skips that sub-gate, marks it `not_applicable` in review evidence, and does not fail merely because no functions are named.
- Review evidence records the resolved configuration identity/version or fingerprint and the configured command results sufficiently to audit the decision.

**Edge Cases:**
- A configured command is empty, unsafe, exits unsuccessfully, cannot be parsed, or references no changed target.
- A critical-function list is absent versus explicitly empty.
- A non-Go project or a changed feature has no applicable module for a configured gate.
- Configuration changes after Phase 3 but before review or between a failed review and remediation.

**Out of Scope:**
- A universal fixed default quality policy outside the checked-in/generated configuration file.
- Silently substituting tool-specific behavior when the configured tool is unavailable.

### REQ-007: Preserve Autonomous, Clean-Boundary TDD Under Orchestrator Control
**Description:** Phase 3 begins only after approved Gherkin scenarios exist in a valid feature-scoped confirmation record and its confirmed immutable baseline. It is an autonomous, sequential scenario loop: each already-approved scenario starts from a clean worktree, follows strict Red/Green/Refactor, reports/checkpoints its evidence, and only the orchestrator accepts the result, commits/checkpoints or safely handles the boundary, and begins the next scenario.

**Acceptance Criteria:**
- The orchestrator does not delegate Phase 3 until approved Gherkin scenarios and a valid matching baseline are present.
- Before every scenario delegation, the orchestrator verifies the recorded worktree identity and an empty non-ignored `git status --short`; it stops non-destructively on failure.
- Each implementation task receives one already-approved scenario only, performs strict Red/Green/Refactor and required traceability/evidence, reports results, then stops without selecting another scenario or changing lifecycle state.
- The orchestrator validates the report, required evidence, approved scope, and boundary cleanliness; it alone accepts, commits/checkpoints, updates current state, and cleans only when safely authorized before launching the next approved scenario.
- A successful checkpoint automatically continues to the next already-approved scenario without a conversational continuation/commit gate; failure, ambiguity, contract drift, invalid approval, or required-gate failure halts without bypass.
- The loop never bypasses approval, current-state validation, clean-worktree checks, or configured quality gates.

**Edge Cases:**
- An ignored local artifact changes while tracked/non-ignored paths remain clean.
- A scenario has only test/configuration/instruction changes.
- A checkpoint commit succeeds but state update fails, or state update succeeds but the process dies before the clean-boundary check.
- Another process changes the worktree during delegation.

**Out of Scope:**
- Parallel scenario execution in one worktree.
- An implementation agent autonomously committing, cleaning, accepting evidence, or starting a new scenario.

### REQ-008: Verify Claude Compatibility Only in CI
**Description:** Claude compatibility is a CI verification obligation, not an installer precondition. CI must execute the supported compatibility verification and record `claude --version` with its result. The installer may generate Claude artifacts without requiring a local Claude executable.

**Acceptance Criteria:**
- The installer does not fail, degrade a successful artifact installation, or require a local `claude` executable solely to claim that generated Claude integration was installed.
- CI has an explicit Claude compatibility job/step that runs the defined verification against generated Claude artifacts and records the output of `claude --version` in durable CI evidence/logs.
- A CI compatibility claim fails when the verification cannot run or the version cannot be recorded; it does not report unverified Claude support as verified.
- Generated instructions distinguish installation capability from CI-verified compatibility and do not imply installer-time runtime validation.

**Edge Cases:**
- CI lacks network access or the Claude executable.
- `claude --version` exits non-zero, emits unexpected output, or the compatibility test fails after version capture.
- A local installer runs on a machine with an unsupported or absent Claude binary.

**Out of Scope:**
- Installing Claude Code, managing user credentials, or guaranteeing every Claude release.

### REQ-009: Apply One Canonical Contract Across Hosts and Resumes
**Description:** The canonical state machine, ownership model, approval validation, final-review behavior, quality-gate policy, and clean-boundary TDD rules must be rendered consistently in state-machine configuration, embedded templates, installer-generated host instructions, OpenCode assets, Claude assets, Codex instructions, implementation/review guidance, fixtures, and verification. Workspace artifacts—not host-local configuration or Ancora—remain durable workflow truth.

**Acceptance Criteria:**
- All supported host surfaces direct new/resumed workflows to shared workspace authority and the orchestrator before phase work.
- Resume validates the feature-scoped record, baseline, lifecycle state, recorded worktree, and relevant commit before acting; it never reconstructs authority from memory or host-local state.
- Ancora remains pointer/status-only and cannot authorize, transition, or recover a missing approval/lifecycle record.
- Cross-host continuation preserves the same legal state transitions, approval gates, no-legacy rule, and final-human-review semantics.
- Repository tests/fixtures assert the canonical rules and fail if any generated surface reintroduces direct phase execution, legacy authority, hardcoded quality-gate details, or Phase-4-to-complete transition.

**Edge Cases:**
- An old generated host asset remains on disk while the workspace contains new canonical state.
- A stale Ancora pointer references a deleted worktree or old baseline.
- Two hosts attempt resume simultaneously; only durable orchestrator-controlled state may be accepted, and conflicts fail closed rather than merging decisions.

**Out of Scope:**
- Supporting a host-specific workflow that intentionally differs from the canonical lifecycle.
- Using memory services as a distributed lock or source of approval truth.

## Open Questions
- None. The settled decisions define the authority, ownership, lifecycle, quality-gate, Claude verification, and autonomous-TDD boundaries sufficiently for Gherkin design.

## Trade-offs
- Removing all legacy paths intentionally breaks resumption of old workflows; this eliminates ambiguous authority rather than carrying permanent compatibility complexity.
- Single-writer orchestration adds validation and state-management work, but prevents agents and hosts from independently advancing a feature.
- `final_human_review` adds a mandatory explicit acceptance step after objective pass, trading speed for a reviewable, commit-bound decision.
- Fully data-driven review gates require a richer, validated configuration schema, but prevent prompt/code drift and project-specific hidden defaults.
- Structured scenario references and global active-record uniqueness reject otherwise convenient shorthand and require active-record discovery, but prevent ambiguous, stale, or cross-feature scenario authorization.
- Confirming the immutable baseline in a second orchestrator-owned commit adds one lifecycle boundary, but avoids the impossible requirement for a commit to record its own SHA while retaining a verifiable baseline.
- Autonomous scenario progression reduces interactive pauses while retaining hard stops at evidence, approval, identity, and clean-boundary failures.
- CI-only Claude validation avoids installer coupling to a locally installed CLI, but requires reliable CI evidence to make compatibility claims.

## Risk Level
critical — Justification: This change governs approval authority, Git commits, cross-host resumption, lifecycle completion, and the safeguards that prevent unapproved work from being implemented or marked complete. A partial migration or inconsistent generated instruction can silently bypass human approval, use stale review evidence, or make lifecycle state irreconcilable.
