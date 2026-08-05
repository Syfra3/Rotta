# Hard Spec: Workflow Ergonomics

## Objectives
- Make each full feature workflow an isolated, resumable, feature-scoped unit without writing a submission artifact into its initiating checkout.
- Replace contradictory workflow references with one deterministic authority, evidence location, baseline comparison, and role-ownership model.
- Reduce avoidable conversation, exploration, and repeated-suite cost without weakening approval, evidence, or recovery guarantees.
- Make selected OpenCode MCP installation observable and safe without claiming host capabilities OpenCode does not expose.

## Non-Goals
- Define, migrate, execute, or tune language-specific/generic quality gates. That is owned by `feature/language-agnostic-quality-gates`.
- Change application production code, tests, commits, publication, or the behavior of unselected hosts.
- Recover an unverifiable legacy workflow by inference, override malformed authority, or delete unknown state automatically.
- Guarantee OpenCode can discover MCP tools when its documented CLI exposes only server connection status.

## Adversarial Pre-Mortem
- Failure mode 1: a feature is created from a moving branch or an initiating checkout, then sibling worktrees share root `.rotta` state or an old approval; the wrong contract is implemented or reviewed.
- Failure mode 2: a bare “approved” reply is applied after contract/review drift, or a broad override survives a new baseline; unreviewed work advances despite a valid-looking record.
- Failure mode 3: JSONC, XDG/override selection, or an MCP health result is guessed; the installer overwrites the wrong config, rolls back unrelated success, or reports OpenCode discovery that it did not observe.

## Hidden Assumptions
- Git can resolve an immutable explicit base SHA and create a distinct feature branch/worktree before any submission write.
- The selected OpenCode version exposes a schema and a documented resolution/diagnostic path sufficient to validate the configuration it consumes; otherwise installation can stop visibly.
- Full workflow command/test output can be retained in the feature worktree even when the host prunes conversation tool output; installer transaction evidence has a separate host-local retention boundary before a worktree exists.
- The language-agnostic quality-gates feature will expose its documented policy and current review-evidence fingerprints before a gate-targeted override is used.

## Edge Case Sweep
- Concurrent requests with the same slug, a stale `.rotta/current`, an interrupted checkpoint, a rebased branch, a missing policy file, or a malformed approval must not merge state or silently restart from a different base.
- A reply containing multiple approval intents, an acknowledgement after a newer prompt/session restart, a clock-expired override, or a changed baseline/contract/policy must not advance anything.
- Both `opencode.json` and `opencode.jsonc`, project/override/global sources, unreadable files, a changed file during rollback, unavailable binaries, and a server that starts but exposes no tools must produce distinct observable status.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Reuse the initiating checkout or a matching existing worktree | It cannot establish exclusive ownership or prevent dirty/checkpoint state from another feature. |
| Keep root `.rotta` logs and infer scope from Git status | It recreates authority drift and loses changed files after checkpoint commits. |
| Treat any affirmative conversational text as approval | It permits stale and cross-feature authorization. |
| Run every approved scenario as an independent full-suite/checkpoint loop | It repeats Stock Chef-style overhead without adding traceability for tightly related scenarios. |
| Assume `~/.config/opencode/opencode.json` and strict JSON | OpenCode supports JSONC and documented override/config precedence; guessing can modify an inactive file. |
| Roll back an entire installation when one MCP fails | It destroys unrelated successful selected-host/capability work. |

## Summary
Rotta will use an immutable-base feature worktree and one feature-local manifest as the sole active workflow unit. Durable contracts, approval, and policy are bootstrapped and baselined inside that worktree; feature workflow mutable state and evidence remain under `.rotta/current/`. A compact, fingerprint-bound confirmation protocol and one-use actor-less override envelope protect human agency without permitting stale authority. Bounded capsules, execution slices, host output controls, and schema-valid OpenCode installation reduce workflow cost while retaining feature-local evidence and deterministic recovery.

## Explicit Decisions and Defaults
- Feature ID is the validated feature slug; branch is `feature/<feature-id>`. The worktree is created from the resolved, explicit 40-hex base SHA, not a symbolic branch name.
- There is exactly one active manifest at `.rotta/current/manifest.yaml` per feature worktree. A different feature ID or canonical worktree in that path is a collision, never a reuse.
- The default local archive retention is 30 days, matching the existing lifecycle default. Expired archives are listed for an explicit human cleanup action; active contracts are never cleanup candidates.
- A compact acknowledgement is exactly one trimmed, case-insensitive token: `x`, `yes`, `agree`, `approved`, or `approve`.
- Default compact execution uses at most three related scenarios per slice; strict per-scenario checkpointing is an explicit manifest/approval selection and never silently changes.
- OpenCode context profile defaults are: automatic compaction enabled; tool-output pruning enabled; a 10,000-token compaction reserve/buffer; and at most 120 retained tool-output lines or 12,288 bytes. The installer must map these semantics only through keys accepted by the target version's current schema.
- Recorded default step budgets are exploration 8, spec 12, orchestration 16, review 16, and implementation 24. A user-configured value must be positive, bounded by the documented host maximum, recorded in current state, and cannot turn a budget stop into success.
- Chat receives a command/evidence summary of at most 80 lines and 8 KiB. Where both host and summary limits apply, the lower limit wins. Full output is retained feature-locally; a failure summary includes exit/timeout status, a diagnostic excerpt, truncation metadata, and the evidence path.

## Data, State, and Ownership

| Artifact | Scope and owner | Required identity / authority |
|----------|-----------------|-------------------------------|
| `specs/<feature-id>_hard_spec.md`, `features/<feature-id>.feature` | Tracked durable contract; Spec role writes only these assigned artifacts | Feature ID and contract fingerprints |
| `.rotta/quality-gates.yaml` | Tracked durable policy, bootstrapped in the feature worktree | Policy fingerprint; policy semantics are owned by the quality-gates feature |
| `specs/approvals/<feature-id>.yaml` | Tracked durable approval; orchestrator only | `rotta.feature-approval/v2`, feature ID, canonical paths, structured scenarios, fingerprints, pending/confirmed baseline reference; no actor identity or hard-coded contract ID |
| `.rotta/current/manifest.yaml` | One mutable feature-local manifest; orchestrator only | `rotta.workflow-manifest/v1`, feature ID, canonical worktree, branch, base SHA, policy/contract/approval references and fingerprints, checkpoint mode |
| `.rotta/current/state.yaml`, `tdd-log.md`, `review-evidence.yaml`, `evidence/`, `capsules/`, `overrides/` | Feature-local runtime state/evidence; respective role writes evidence, orchestrator writes lifecycle/acceptance state | Manifest feature ID and baseline/contract/policy fingerprint binding |
| `${XDG_STATE_HOME:-$HOME/.local/state}/rotta/installer-transactions/<transaction-id>/` | Non-authoritative host-local installer transaction evidence; installer only | Selected host/capability, resolved config source, transaction status, redacted command/config hashes, scoped backup references, retention expiry; never a feature lifecycle input |
| `.rotta/archive/<feature-id>/<baseline-sha>/` | Feature-local terminal runtime archive; orchestrator archives, human explicitly cleans | Copy/move of a terminal current directory only; never active authority |
| `rotta.lifecycle/v1` | Canonical versioned lifecycle model owned by source/runtime code | Sole definition of legal lifecycle states, transitions, and role ownership; generated policy is a non-authoritative projection |

`state.yaml` must record phase, current scenario/slice, completed and remaining scenarios, checkpoint SHAs, safe resume point, last action, blocked reason, evidence references, and manifest fingerprint. It cannot authorize a contract by itself. An approval record uses one structured schema in both writer and validator; scenario entries contain exactly `feature_path`, `scenario_id`, and `requirement_ids`. The baseline artifact commit contains the approved contract and a pending baseline reference; an orchestrator-owned confirmation commit records that immutable baseline SHA, avoiding a self-referential commit claim.

Installer transaction evidence is retained for 30 days under its transaction directory and may be explicitly cleaned only by transaction ID after the installer confirms the path is a direct child of the host-local transaction root. It records statuses and redacted metadata, not feature contracts or current workflow evidence. It is never copied into an initiating checkout, feature worktree, archive, approval, or lifecycle state and cannot authorize, resume, or complete a feature.

## Invariants
- The initiating checkout receives no spec, feature, policy, approval, runtime state, evidence, code, test, or commit write for the new submission.
- Only the orchestrator creates/mutates manifest, lifecycle state, approval/baseline confirmation, checkpoint acceptance, archive, and completion state. Roles return their assigned artifacts/evidence only.
- Feature workflow runtime state/evidence never lives at root `.rotta/tdd-log.md`, `.rotta/state.yaml`, root review evidence, or a shared home/host config directory. Installer transaction evidence is the sole non-authoritative host-local exception and is outside every initiating checkout.
- Every active operation validates feature ID, canonical worktree, attached feature branch, explicit base/baseline, and required fingerprints; drift invalidates continuation rather than being merged.
- Review changed files are always measured from `git diff --name-status <confirmed-baseline>...HEAD`; an empty working-tree diff is never evidence of an empty feature diff.
- A waiver/override cannot authorize malformed or inconsistent authority, unknown destructive cleanup, or a worktree identity mismatch.
- Complete workflow-execution evidence is feature-local. Conversation summaries and host truncation never become the evidence source of truth; installer transaction evidence is host-local, non-authoritative, and cannot become feature evidence.

## Requirements

### REQ-081: Establish the canonical isolated feature lifecycle
**Description:** A new full workflow must create and own one clean isolated feature worktree before any submission write, bootstrap its durable policy/contract/approval only there, and retain all active mutable state under that worktree's `.rotta/current/`.

**Acceptance Criteria:**
- Before creation, Rotta validates a clean initiating worktree, a valid feature ID, the selected base branch, and resolves/records its explicit base SHA. It then creates `feature/<feature-id>` and its exclusively owned sibling worktree at that SHA.
- No submission artifact is written to the initiating checkout on success or failure. Worktree/branch/path collisions, a dirty initiator, detached HEAD, or an unresolved base stop before any submission write and report a non-destructive recovery action.
- Bootstrap creates/validates one manifest atomically in the feature worktree, copies or generates the canonical policy there, and writes contracts/approval only in that worktree. The approved contract/policy/approval baseline is committed before Phase 3.
- Manifest validation rejects a feature ID, canonical worktree, branch, base SHA, policy, contract, approval, or checkpoint-mode mismatch. Separate worktrees have separate `.rotta/current/` directories and cannot read, write, lock, or accept each other's mutable state.
- Resume reloads and validates manifest/state/fingerprints and the recorded Git identity before acting. It resumes only the recorded feature/slice; conflicting or incomplete state halts without discarding changes and offers repair, handoff, or archive when identity permits.
- Terminal archive moves only the verified current directory to `.rotta/archive/<feature-id>/<baseline-sha>/`; the active spec, feature, approval, policy, branch, and worktree remain. Human-requested cleanup validates exact ownership/eligibility and never removes an initiating checkout or an unknown path.

**Edge Cases:**
- Two simultaneous requests use the same feature ID; only the exclusive Git creation winner may continue.
- A process dies after a checkpoint commit but before state persistence, or a user changes branch/worktree during a slice.
- An archive destination already exists, a prior manifest is malformed, or only ignored runtime files are dirty.

**Out of Scope:**
- Concurrent writers in one worktree, automatic stashing/reset/rebase, automatic publication, or recovery of unidentifiable legacy state.

### REQ-082: Make policy, authority, and changed-file resolution deterministic
**Description:** The installer, generated instructions, lifecycle, review integration, and evidence must use one feature-local path model and a baseline-to-HEAD comparison, while retiring contradictory root/legacy references.

**Acceptance Criteria:**
- The only active TDD/evidence roots are `.rotta/current/tdd-log.md`, `.rotta/current/review-evidence.yaml`, and `.rotta/current/evidence/`; the only active policy path is `.rotta/quality-gates.yaml` in the recorded feature worktree.
- Bootstrap verifies the policy exists and records its fingerprint before delegation. A missing, unreadable, or fingerprint-drifted policy blocks review/continuation with remediation; it is not silently read from the initiating checkout or a sibling worktree.
- The changed-file resolver records baseline SHA, HEAD SHA, exact `git diff --name-status <baseline>...HEAD` output, and its derived canonical paths. It is used after every checkpoint and by review; plain `git diff --name-only` is not a review-scope source.
- Generated assets, runtime code, fixtures, and verification retire active references to root `.rotta/tdd-log.md`, root `.rotta/state.yaml`, root review evidence, `specs/.approved`, `assets/config/state-machine.yaml`, `.rotta/state-machine.yaml`, and other legacy state-machine paths. Detection may report such files as legacy, but they cannot authorize, satisfy evidence, expand scope, or be silently migrated/deleted.
- Policy/gate/evidence resolution is deterministic from manifest-bound paths and fingerprints. This requirement consumes the quality-gates interface and does not enumerate gate categories, commands, thresholds, discovery, or language behavior.

**Edge Cases:**
- A checkpoint commit leaves the working tree clean although baseline-to-HEAD contains changed, renamed, or deleted files.
- The installed project had a v1 policy or root TDD log before a feature worktree is created.
- A generated host instruction still names a legacy path after a successful installer run.

**Out of Scope:**
- Defining generic gate contents or migrating obsolete quality-gate semantics.

### REQ-083: Bind compact human acknowledgements to the displayed action
**Description:** A compact affirmative reply may authorize exactly one currently displayed action only when it is bound to the current feature and unchanged contract/review snapshot.

**Acceptance Criteria:**
- Before accepting a compact acknowledgement, Rotta displays one pending action with `prompt_id`, action type, feature ID, contract fingerprint, expiry/replacement condition, and—when final—`reviewed_commit` and evidence/policy fingerprints. It records that pending prompt in current state.
- Only an exact acknowledgement token may authorize the sole pending action. It is consumed once and only if the manifest/contract/baseline remain valid; a new prompt, session restart, action replacement, feature change, contract/policy drift, or final-snapshot change invalidates the old prompt.
- A message containing multiple intents, unsupported text, more than one pending approval action, a stale prompt, or a mismatched feature/fingerprint does not advance state and reports why.
- Contract approval creates the feature-scoped approval/baseline through the orchestrator only. Final approval is permitted only from `final_human_review` when the current committed snapshot equals `reviewed_commit` and the displayed pending action names that exact snapshot; it then transitions to `complete` without persisting an actor identity.

**Edge Cases:**
- “yes” arrives after a later approval prompt, after restart, after a rebase, or while two features await approval.
- A reviewer replies `approve and archive`, or approves a contract whose displayed scenario set changed.
- Objective review passed, but final-review state/evidence persistence failed.

**Out of Scope:**
- User accounts, signatures, inferred approval from silence, or approval of a different feature through a global marker.

### REQ-084: Preserve human agency with bounded overrides and safe recovery
**Description:** Rotta must offer actor-less, auditable, narrowly scoped overrides for eligible policy/gate/process rules while preserving non-waivable integrity safeguards and usable recovery paths.

**Acceptance Criteria:**
- An override is a feature-local `.rotta/current/overrides/<override-id>.yaml` record containing format/version, feature ID, exact rule/gate ID, one action/resource scope, non-empty reason, authorization action/prompt ID, confirmed baseline SHA, contract fingerprint, policy/evidence fingerprint when applicable, issued/expiry timestamps, `uses_remaining: 1`, and status. It contains no actor/reviewer identity.
- The orchestrator accepts an override only after its displayed authorization action is acknowledged and all bindings match. It consumes the record atomically for one evaluated operation, preserves it as evidence, and invalidates it on expiry, consumption, baseline/contract/policy/evidence drift, malformed fields, or scope mismatch.
- For a gate exception, the envelope references an already persisted non-passing gate outcome and delegates gate status/readiness interpretation to the language-agnostic quality-gates evaluator; it never rewrites an outcome to pass or redefines gate logic.
- The following are non-waivable: malformed/inconsistent approval or manifest authority, unknown/destructive cleanup targets, and incorrect/missing worktree identity. Rotta refuses the override and gives an explicit safe alternative: repair-and-resume, evidence/branch handoff, or verified terminal archive where ownership is proven.
- A blocked non-waivable condition never triggers destructive cleanup or indefinite silent blocking: its output identifies the failed invariant, preserved paths, and the next human-directed recovery action.

**Edge Cases:**
- Two override records target one action, an override expires during evaluation, or the system clock cannot produce a valid timestamp.
- A gate failure is recorded for a different policy fingerprint, or a process override attempts to cover multiple future slices.
- An archive request names traversal, an unknown path, or a worktree whose manifest resolves elsewhere.

**Out of Scope:**
- Identity capture, blanket/project-wide overrides, overriding integrity invariants, or a second quality-gate waiver implementation.

### REQ-085: Bound host context while retaining complete local evidence
**Description:** Generated OpenCode configuration and Rotta execution output must use supported compaction/pruning, materially bounded tool output, and recorded role-specific limits without hiding material failure evidence. Installer transaction evidence remains separately host-local before a feature worktree exists.

**Acceptance Criteria:**
- For a schema-supported OpenCode target, generated configuration enables automatic compaction/pruning, applies the 10,000-token reserve/buffer and the 120-line/12,288-byte tool-output limits, and records default budgets of exploration 8, spec 12, orchestration 16, review 16, and implementation 24. Unsupported key names are not guessed; the target schema mapping must validate or installation reports blocked.
- Every workflow-execution command that can influence lifecycle, checkpoint, review, or recovery writes full stdout/stderr, command, working directory, timestamps, exit/timeout result, and content hash under `.rotta/current/evidence/` before it emits a chat summary. Installer commands write equivalent redacted transaction evidence only under the host-local installer transaction directory.
- A summary is bounded to 80 lines/8 KiB, and the lower of the summary and host limits wins. If output is truncated/pruned, it explicitly reports line/byte totals, exit status, failure excerpt, full-evidence or transaction-evidence path, and evidence hash; an absent/truncated chat transcript cannot be treated as a passing result.
- A role reaching its recorded or valid user-configured step budget records a bounded resumable stop with current manifest/slice/evidence references and does not declare success, skip validation, or receive the full prior transcript on resume.
- Installer transaction evidence records the resolved binary path used for each executed command, selected host/capability, write/schema/resolution/discovery status, and scoped rollback result. It is retained/cleaned only under its host-local transaction root and never copied into an initiating checkout or used as lifecycle authority.
- Installation output states that OpenCode configuration changes require an OpenCode restart before they can affect a running session.

**Edge Cases:**
- A command emits a short success line after megabytes of errors, or a tool output is pruned during a later compaction.
- Workflow or installer evidence serialization fails after a command exits successfully.
- The current OpenCode schema supports a different compaction field shape than an earlier version.

**Out of Scope:**
- Increasing model context windows, storing full evidence in chat, or treating context pruning as evidence deletion.

### REQ-086: Use bounded Exploration Capsules only above a local uncertainty threshold
**Description:** Exploration is an explicit bounded task only when local inspection cannot establish a safe change scope; its capsule is the only exploration context implementation receives.

**Acceptance Criteria:**
- Rotta launches a capsule only when, after at most eight focused local reads/searches, either a required invariant/owner remains unresolved, the likely change spans more than two top-level components, or dependency evidence shows more than five direct affected modules. Otherwise it delegates directly with the current scenario/slice.
- A capsule at `.rotta/current/capsules/<capsule-id>.md` is at most 120 lines and 12 KiB and contains objective, in/out scope, no more than 12 files and 20 symbols, invariants, at most five test commands, risks, unresolved blockers, manifest fingerprint, and capsule fingerprint.
- Implementation receives only the capsule path plus the current approved scenario or slice and required evidence, never the complete exploration transcript or previous role transcript.
- A capsule that reaches a bound records its unresolved blocker and stops. Resume reuses it only when its manifest/contract/policy fingerprints still match; otherwise it is invalidated and a new bounded capsule or human recovery is required.
- State/evidence link each execution slice to its capsule ID/fingerprint (or an explicit `none-required` decision) for traceability.

**Edge Cases:**
- Graph/dependency tooling is unavailable, a capsule names a deleted file, or a sixth test command would be needed.
- Scope becomes cross-component after implementation starts or the capsule becomes stale after contract change.

**Out of Scope:**
- Unbounded autonomous reconnaissance, hidden transcript forwarding, or using a capsule as lifecycle authority.

### REQ-087: Execute related approved scenarios as compact, traceable slices
**Description:** Related approved scenarios may share one execution slice to reduce repeated full-suite/checkpoint work while preserving per-scenario TDD and evidence; strict per-scenario checkpoints remain selectable.

**Acceptance Criteria:**
- A compact slice contains one to three approved, ordered scenarios, one component/capsule scope, and no more than 12 expected changed paths. The manifest records `checkpoint_mode: compact_slice` or `strict_per_scenario` before execution.
- Within a compact slice, each scenario retains an individual Red/Green/Refactor record and focused test result. After every scenario is green, Rotta runs the resolved full validation/checkpoint once for the slice, records all scenario IDs and evidence, and creates one slice checkpoint.
- In strict mode, every slice has one scenario and retains the existing full validation/checkpoint boundary. A user/orchestrator must select strict mode explicitly; compact mode cannot silently coalesce a strict scenario.
- A failure, scope drift, unexpected change, failed focused test, or failed final slice validation stops at the affected scenario/slice, preserves all local evidence and changes, creates no partial slice checkpoint, and provides resume/handoff/recovery guidance.
- For nine compatible scenarios, default compact slicing permits at most three full-suite/checkpoint executions instead of nine while keeping a scenario-to-focused-test-to-slice-checkpoint trace.

**Edge Cases:**
- The second scenario fails after the first is green, a requested fourth scenario is related, or a slice exceeds its expected-path bound.
- The final full suite passes but state/checkpoint persistence fails.
- A user switches mode after a slice begins.

**Out of Scope:**
- Parallel scenario execution, skipping per-scenario TDD/focused tests, or reducing the quality-gates feature's required validation semantics.

### REQ-088: Eliminate lifecycle instruction duplication and ownership conflicts
**Description:** Generated surfaces must share one concise non-authoritative policy, designate exactly one lifecycle authority, and prove that roles or skill modes cannot reintroduce contradictory duties.

**Acceptance Criteria:**
- The source/runtime-owned `rotta.lifecycle/v1` canonical lifecycle model is the sole authority for legal phase transitions and lifecycle ownership. A shared concise policy is rendered from that model; role-specific instructions contain only their delegated inputs, permitted outputs, limits, and an authority reference.
- Only the orchestrator instruction may authorize approval records, baseline confirmation, lifecycle state mutation, checkpoints/commits, archives, recovery decisions, or completion. Spec, implementation, review, and their mode/host variants cannot claim any of those actions.
- Generated OpenCode, Claude, Codex, embedded assets, installer copy, and fixtures reference the same canonical paths and policy fingerprint; role/skill divergence is a generation failure.
- Regression verification compares generated role instructions to the shared policy and rejects retired paths, `assets/config/state-machine.yaml` or `.rotta/state-machine.yaml` as active authority, duplicated lifecycle authorities, direct phase advancement, root evidence paths, and embedded generic gate definitions.

**Edge Cases:**
- A host lacks subagents/slash commands, an old cached instruction remains on disk, or a role is directly invoked after timeout.

**Out of Scope:**
- Identical host UI syntax or duplicating the quality-gates policy into role prose.

### REQ-089: Install selected OpenCode MCPs through effective, schema-valid configuration
**Description:** Ancora, Vela, and Context7 installation for OpenCode must resolve the active JSON/JSONC configuration using documented precedence, alter selected managed entries only, and report each observable installation stage without unsupported host-health claims.

**Acceptance Criteria:**
- Before writing, the installer opens a host-local transaction evidence directory and resolves/reports candidate/effective config source, format, and precedence considering documented global XDG configuration, `OPENCODE_CONFIG` override, project config, and JSON/JSONC support. If multiple files cannot be deterministically resolved by the current documented OpenCode mechanism, it writes none and reports an ambiguous effective config.
- The installer uses the current official OpenCode schema for the resolved version, validates the parsed candidate before write, preserves JSONC-compatible content/format, and writes only the selected host's Rotta-managed agent/MCP entries. It uses local MCP command arrays and serializes portable bare command names; resolved absolute binary paths are used only to execute installer/preflight commands.
- Per selected host/capability status distinguishes: `preflight_command_resolved`, `written`, `schema_valid`, `opencode_resolved`, and `tools_discovered`; the transaction evidence records each status and resolved command path. `opencode_resolved` may use documented `opencode mcp list` status; `tools_discovered` is labelled direct-server discovery unless OpenCode itself exposes tool enumeration. Unobservable stages are `not_observable`, never success.
- Each host/capability transaction has a scoped backup/patch plan. A Vela/Ancora/Context7 failure restores only its own known managed changes for that host, leaves unrelated selected-host/capability success intact, and refuses rollback if concurrent modification prevents a safe scoped restore.
- Result/remediation explicitly distinguishes file written, schema valid, OpenCode resolved, direct tools discovered, command unavailable, disabled, connection failed, needs authentication, and not observable. It never claims that file text or an installer-side server probe proves OpenCode tool discovery. Transaction evidence is retained/cleaned only at its host-local scoped path and cannot be copied into, or act for, a feature lifecycle.

**Edge Cases:**
- Both `opencode.json` and `opencode.jsonc` exist; `OPENCODE_CONFIG` points elsewhere; XDG is customized; or the effective file is malformed/unreadable.
- A selected binary is found by the installer but unavailable to an OpenCode GUI process, an MCP connects but lacks expected tools, or one selected server needs authentication.
- Vela fails after Ancora or Context7 succeeds, a managed config entry is ambiguous/user-owned, or the config changes between write and rollback.

**Out of Scope:**
- Editing unselected hosts, rewriting user-owned MCP entries, installing a host-specific PATH, guaranteeing desktop-host discovery, or rolling back packages.

### REQ-090: Roll out compatibly and make rollback/recovery explicit
**Description:** The migration must make new workflows deterministic without copying the separate quality-gate feature, and must preserve a safe way to stop, inspect, archive, or restore selected host configuration.

**Acceptance Criteria:**
- Rollout first adds schema/path/role conformance checks, then enables the canonical lifecycle for new feature worktrees, then offers human-directed migration/recovery for existing workflows. Legacy artifacts are reported and retired from active behavior before deletion is considered.
- Feature-worktree migration never promotes an old root log, global approval, retired state-machine asset, or mismatched approval schema into authority. A workflow that cannot prove one manifest/base/contract/policy/approval binding is handed off or restarted in a new feature worktree.
- OpenCode rollback uses the affected selected-host/capability transaction backup only; its host-local transaction evidence is retained/cleaned within that transaction scope. Workflow rollback preserves contracts/evidence and offers archive/handoff rather than returning to legacy authority.
- Verification covers isolated Git worktrees, resumes/archives, approval and override drift, policy/evidence paths, output bounds, capsule/slice limits, generated-role conformance, and JSON/JSONC/OpenCode transaction states without requiring unsupported live-host assertions.

**Edge Cases:**
- The separate quality-gates branch is not yet merged, a legacy file is ignored by Git, or a host is restarted halfway through migration.

**Out of Scope:**
- Automatic merge ordering, historical state repair by heuristics, or implementation of generic quality-gate execution.

## Compatibility and Migration
The only essential dependency on `feature/language-agnostic-quality-gates` is its published interface: `.rotta/quality-gates.yaml` in the current generic format and `.rotta/current/review-evidence.yaml` carrying the recorded baseline, snapshot, and policy/configuration fingerprint used by its evaluator. This feature consumes those identities for policy binding and gate-override envelopes; it must not duplicate gate categories, discovery, commands, thresholds, status calculation, waiver interpretation, or readiness rules. Until that interface exists, gate-targeted overrides and dependent review continuation are blocked with remediation, while lifecycle-only work remains independently testable.

Legacy root logs/state, `specs/.approved`, `assets/config/state-machine.yaml`, `.rotta/state-machine.yaml`, other legacy state-machine references, hard-coded approval IDs, differing approval filename derivations, and v1/v2 mixed approval readers are detected as retired inputs. They are never fallback authority. The source/runtime-owned `rotta.lifecycle/v1` model replaces the legacy state-machine asset. The new writer and validator share `rotta.feature-approval/v2`, derive `specs/approvals/<feature-id>.yaml` from the manifest feature ID, and require the same canonical contract fingerprints.

## Performance and Context Budgets
- Local pre-capsule inspection: at most 8 focused actions; capsule: 120 lines/12 KiB, 12 files, 20 symbols, 5 test commands.
- Compact slice: 1–3 scenarios and at most 12 expected paths; nine compatible scenarios: at most 3 full-suite/checkpoint cycles. Strict mode remains 1 scenario/cycle.
- Host config: exploration 8, spec 12, orchestration 16, review 16, and implementation 24 steps/invocation; compaction reserve/buffer 10,000 tokens; tool output 120 lines/12,288 bytes.
- Chat: 80 lines/8 KiB summary per captured command; the lower summary/host bound wins. Full workflow evidence is retained under `.rotta/current/evidence/`; no lifecycle decision may rely on the summary alone. Installer transaction evidence is retained separately under its host-local transaction path and is non-authoritative.

## Verification Strategy
- Unit/schema tests: manifest/state/approval/override parsing, fingerprint drift, one-use/expiry, acknowledgement token and ambiguity handling, and retired-reference detection.
- Git-backed integration tests: clean explicit-base creation, no initiating-checkout writes, parallel worktree isolation, resume/archive/recovery, and baseline-to-HEAD changed files after one or more checkpoint commits.
- Execution tests: thresholded capsule creation/bounds/staleness, compact slice focused evidence plus one full checkpoint, strict mode, failure preservation, role-budget stop, and complete-evidence/truncated-summary linkage.
- Generated-surface tests: every host/role maps to the source/runtime `rotta.lifecycle/v1` model and the same path policy; no role creates a lifecycle conflict, activates the retired state-machine asset, or embeds generic quality-gate policy.
- OpenCode fixture tests: XDG/override resolution, JSON and JSONC effective selection, schema validation failure, selected-host targeting, host-local transaction evidence/retention, per-capability status, direct-vs-host discovery labelling, and scoped rollback after Vela failure. Live tests may assert only documented `opencode mcp list` connection/status behavior.

## Rollout and Rollback
1. Ship conformance/report-only checks for legacy references, schema validity, and generated-role drift.
2. Enable the canonical manifest lifecycle for newly created feature worktrees; do not silently adopt existing state.
3. Enable compact slices only with an explicit recorded mode; retain strict mode throughout rollout.
4. Offer human-directed handoff/archive/restart for old or inconsistent workflows.

On rollback, restore only the affected selected-host/capability configuration from its scoped backup and retain its non-authoritative host-local transaction evidence only within that transaction scope. Preserve feature contracts, approval records, `.rotta/current` evidence, and archives; stop with handoff/recovery guidance rather than reactivating legacy authority, copying installer evidence into an initiating checkout, or deleting unknown files.

## Open Questions
- None. The existing 30-day archive default and the current documented OpenCode capability patterns establish the stated operational defaults. Where a target OpenCode schema or effective-config precedence cannot be observed through its documented mechanism, the required behavior is a visible blocked/not-observable result, not an inferred implementation choice.

## Trade-offs
- Immutable worktree/base and fail-closed authority checks add startup work, but prevent cross-feature mutation and stale approval.
- Compact slices reduce repeated full-suite/checkpoint cost, but restrict grouping and leave strict mode available for higher-isolation work.
- Full local evidence plus bounded chat summaries increases local disk use, but makes compaction/truncation safe and auditable.
- Conservative OpenCode resolution and scoped rollback can leave an installation incomplete, but avoids changing an inactive/user-owned config or undoing unrelated success.

## Risk Level
critical — Justification: this feature changes feature creation, approval authority, recovery, checkpoint scope, human exception handling, generated host configuration, and installer rollback. A partial or inconsistent implementation could authorize stale work, corrupt user configuration, or strand evidence needed for safe recovery.
