# Hard Spec: OpenCode-Only Rotta Workflow and Local Graph Evidence

## Adversarial Pre-Mortem
- Failure mode 1: Version-prefixed or non-OpenCode code, commands, assets, or durable records remain reachable after removal. A user can then start an obsolete workflow or two incompatible lifecycle authorities can act on one submission.
- Failure mode 2: A graph operation silently uses an endpoint, proxy, or cached claim rather than the local `vela` executable. Contract and review decisions are then supported by stale, incomplete, or non-local evidence.
- Failure mode 3: A resume accepts partial durable state, a worker advances its own phase, or a remote branch moves between publication verification and cleanup. The system can falsely claim progress or remove the only recoverable worktree.

## Hidden Assumptions
- `vela`, `git`, and the Go toolchain can be discovered locally when their respective operations are requested; unavailable tools produce a bounded blocked or uncertain result rather than an invented success.
- The OpenCode user configuration root is `~/.config/opencode`, and Rotta can identify files it manages without overwriting unrelated user configuration.
- A submission ID, selected repository, immutable baseline commit, feature branch, remote name, and fully qualified remote ref can be recorded before an operation that depends on them.
- The repository is a Go module for which `go test ./...`, `go vet ./...`, and Go coverage are meaningful quality adapters.
- Human decisions are available for contract approval, graph indexing consent, and worktree cleanup consent.

## Edge Case Sweep
- Concurrent resumes and delayed workers can replay an old authorization; transition persistence must compare the expected durable status and revision and reject stale writes.
- A local graph can be absent, indexed for another commit, incomplete, or fail during indexing. It remains advisory uncertainty and must never become a fabricated graph result.
- A durable record can be absent, malformed, outside the selected repository, or inconsistent with the approved contract, worktree, or commit. Resume must stop before dispatching work.
- A Go command can be missing, return non-zero, produce unreadable coverage data, or observe no applicable packages. Each adapter must report its actual outcome and must not translate execution failure to pass or not-applicable.
- A remote ref can resolve to the reviewed commit initially and a different commit after cleanup consent. Cleanup must not occur in that case.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Preserve version-prefixed names or records as compatibility fallback | It retains a second lifecycle surface and contradicts complete removal with no migration. |
| Offer a host selector or install into multiple AI-host configurations | The supported surface is OpenCode only; choices create unsupported paths and legacy assets. |
| Use a Vela service, MCP endpoint, proxy, or remote graph storage | Local-only evidence requires direct local CLI execution and no remote endpoint. |
| Treat graph evidence as a quality or approval gate | Graph availability and completeness are advisory and cannot define behavior or advance lifecycle state. |
| Keep a generic analyzer policy | This Go repository requires real, reproducible `go test`, `go vet`, and coverage adapters. |
| Treat `git push` success or ancestry as publication verification | Archive safety requires the recorded remote ref itself to resolve exactly to the reviewed commit twice. |
| Delete assets based only on a text search | Names and strings do not prove ownership or reachability; semantic inventory is required first. |

## Summary
Rotta shall be a clean, non-versioned, OpenCode-only workflow. The normal `rotta` invocation presents a minimal terminal UI with **Install**, **Status**, and **Quit**; installation manages only the user's OpenCode configuration. Workflow authority, durable state, contracts, quality evidence, graph evidence, and lifecycle operations use the non-versioned `.rotta/workflow` namespace. All retired version-prefixed and non-OpenCode workflow surfaces are removed without migration after a semantic inventory proves their disposition. The orchestrator alone authorizes phase changes; workers return bounded evidence. Local `vela` CLI indexing and querying are consent-gated, advisory, and prohibited from using remote endpoints. Review executes real local Go test, vet, and coverage adapters. Git lifecycle operations use the local `git` CLI and exact recorded remote/ref verification; publication never implies merge or acceptance.

## Goals
- Provide one OpenCode-only Rotta installation and workflow surface with no version-prefixed naming.
- Preserve explicit approval gates, orchestrator-only lifecycle authority, fail-closed resume, bounded evidence, and local-only graph evidence.
- Make removal of obsolete workflow assets safe through semantic inventory and reachability verification.
- Provide actual local Vela, Go quality, and Git lifecycle integrations rather than simulated evidence.

## Non-Goals
- Migration, import, interpretation, dual-read, compatibility fallback, or cleanup of retired durable records or contracts.
- Claude, Codex, or any other non-OpenCode host integration, host selector, asset, installer option, runtime adapter, or configuration.
- Any remote Vela endpoint, source/graph upload, proxy, tunnel, hosted graph service, or graph-driven approval.
- Automatic Vela indexing, automatic cleanup, automatic merge, pull-request creation, branch/ref deletion, or acceptance of published work.
- Deleting unrelated product functionality because its name, content, or documentation mentions a retired host or version.

## Actors and Authority
| Actor | May do | Must not do |
|-------|--------|-------------|
| Human | Request new work or resume; approve an unchanged contract; grant or decline indexing and cleanup consent; control Git credentials and external merge/acceptance. | Advance durable state by assertion alone or treat publication/cleanup consent as merge or acceptance. |
| Orchestrator | Select baseline; dispatch bounded tasks; evaluate returned evidence; authorize legal transitions. | Execute commands, edit artifacts, manage worktrees, commit, publish, merge, or clean up directly. |
| Spec worker | Write the delegated hard spec and return bounded findings. | Write approval or lifecycle records, approve a contract, or authorize a transition. |
| Operational worker | Execute one authorized local operation and atomically persist its authorized result. | Choose lifecycle state, edit contract/source scope, merge, or operate on unrecorded remotes, refs, or worktrees. |
| Vela worker | Run permitted local `vela` CLI preflight, indexing, and bounded queries; return advisory evidence. | Use a remote endpoint, index without consent, define requirements, or advance lifecycle state. |
| Quality worker | Run the required Go adapters and report exact evidence for the candidate commit. | Claim a tool was run when it was not, waive a required failure, or advance lifecycle state. |

## Lifecycle and Durable Artifacts
- The lifecycle is **Draft → Contract → TDD → Review → Archive**. Indexing is a delegated Draft capability, not a phase.
- The only durable workflow root is `.rotta/workflow`. Submission ledgers are `.rotta/workflow/submissions/<submission-id>.yaml`; the quality policy is `.rotta/workflow/quality-policy.yaml`; inventory, evidence, and archive metadata reside beneath `.rotta/workflow` and are repository-relative.
- A new request may create only an initial Draft ledger after a fresh submission identity and baseline commit are validated. A resume/recovery request must derive all state from valid matching durable artifacts and otherwise fails closed. Neither absence of state nor legacy artifacts imply a new request.
- The behavioral contract is `specs/hard_spec.md` plus approved feature contracts. The ledger records operational facts only and cannot amend or substitute for a contract.
- Every durable transition includes submission ID, expected status, target status, revision, selected baseline, authorized scope, evidence pointers, actor authorization, and resulting revision. Persistence is atomic or reports no transition.
- Evidence is bounded, tied to the relevant immutable commit, redacted of credentials/secrets, and contains outcomes, uncertainty, affected scope, and a safe next action. Raw graph dumps and raw command logs are not durable evidence.

## Requirements

### REQ-001: Remove Version-Prefixed and Legacy Workflow Surfaces
**Description:** The delivered repository shall contain no reachable version-prefixed Rotta workflow naming in source, CLI, durable namespace, contracts, feature artifacts, approval artifacts, runtime selection, installer options, tests, fixtures, or assets. It shall remove the retired workflow rather than migrate or interpret it.

**Acceptance Criteria:**
- Active Rotta source, public CLI help, durable paths, contract references, feature filenames, approval records, runtime selectors, tests, fixtures, and managed assets use the non-versioned workflow terminology only.
- No active command accepts a version-prefixed workflow subcommand, flag, namespace, or compatibility mode.
- Retired version-prefixed contract, feature, and approval artifacts are removed; they are neither read nor translated by the replacement workflow.
- Any historical Git content is irrelevant to runtime behavior and does not require alteration.

**Edge Cases:**
- A stale retired ledger or approval file is present when a user starts or resumes work.
- A test fixture or documentation file contains a retired term but has no workflow role.

**Out of Scope:**
- Migrating, preserving, or making use of retired workflow state.

### REQ-002: Inventory Before Deleting Workflow Assets
**Description:** Before the first deletion of an in-scope retired workflow asset, the implementation shall create a durable semantic removal inventory under `.rotta/workflow` and reconcile every discovered candidate.

**Acceptance Criteria:**
- The inventory records each discovered candidate exactly once with repository-relative identity, category (`workflow_code`, `test`, `fixture`, `skill_or_agent_asset`, `configuration`, `installer_or_runtime_integration`, or `compatibility_shim`), semantic role, invocation/reachability evidence, classification (`retired_only`, `mixed`, or `unrelated`), disposition, replacement/retention rationale, and post-removal verification plan.
- A candidate is deleted only when classified `retired_only` with evidence that it exclusively selects, installs, invokes, validates, or preserves the retired workflow. A mixed asset is separated before deletion; an unrelated asset is retained.
- The inventory reconciles every discovered candidate and records a fingerprint referenced by the submission ledger before deletion begins.
- Post-removal verification proves no reachable retired installer, selector, compatibility path, workflow command, or managed non-OpenCode integration remains.

**Edge Cases:**
- One installer contains both obsolete workflow handling and unrelated product behavior.
- A host name occurs in unrelated documentation or metadata.
- A fixture is reachable only through a compatibility test.

**Out of Scope:**
- Text-search-only deletion decisions or deletion of unrelated product functionality.

### REQ-003: Provide the Minimal Rotta CLI and TUI
**Description:** Invoking `rotta` with no arguments shall open a simple interactive terminal UI whose only top-level choices are Install, Status, and Quit. The non-interactive public commands are `rotta install`, `rotta status`, and `rotta version`/`rotta --version`.

**Acceptance Criteria:**
- The no-argument UI displays exactly Install, Status, and Quit as its top-level actions; Quit exits without changing files.
- `rotta install` performs the OpenCode-only installation defined in REQ-004; `rotta status` reports bounded local installation and durable-workflow status without mutation.
- The CLI rejects retired lifecycle commands and backup/restore commands rather than routing them to compatibility behavior.
- Help and errors name only the supported commands and OpenCode integration.

**Edge Cases:**
- The UI runs without an existing user configuration directory.
- Status finds no installed assets, a partial managed installation, or malformed durable workflow state.

**Out of Scope:**
- A target chooser, project-path wizard, host-specific setup screens, or workflow phase advancement directly from the public CLI.

### REQ-004: Install into OpenCode User Configuration Only
**Description:** Installation shall manage only Rotta-owned OpenCode assets under `~/.config/opencode`. It shall expose no host target, host selector, optional host integration, or installation path for Claude, Codex, or any other host.

**Acceptance Criteria:**
- Installation writes only explicitly managed Rotta files below `~/.config/opencode` and records the managed file set for Status and safe replacement.
- Installation preserves unrelated user configuration; a conflicting unmanaged file stops installation with a bounded conflict report and safe next action rather than overwrite it.
- Installation does not create, modify, select, validate, or advertise any configuration, instruction, skill, agent, MCP asset, or runtime integration for a non-OpenCode host.
- Installer options contain no target selection, project installation, selective phase installation, backup/restore, Ancora, Context7, or host-specific Vela setup option.

**Edge Cases:**
- `~/.config/opencode` is missing, unreadable, a symlink outside the expected user directory, or contains a partially managed prior installation.
- A managed asset needs replacement after its content no longer matches its recorded fingerprint.

**Out of Scope:**
- Installing OpenCode itself, managing credentials, or modifying repository-local host configuration.

### REQ-005: Make the Orchestrator the Sole Lifecycle Authority
**Description:** Only the orchestrator may authorize a Draft, Contract, TDD, Review, or Archive transition. Delegated workers execute one bounded authorized task and return evidence; they cannot decide their own result or next phase.

**Acceptance Criteria:**
- A transition request includes submission ID, expected status and durable revision, target status, authorized scope, evidence references, and orchestrator authorization; stale, illegal, incomplete, or worker-originated requests are rejected.
- The orchestrator dispatches operations and evaluates returned compact evidence but does not itself run commands, edit files, manage worktrees, commit, publish, or delete.
- Every transition is atomically persisted with actor attribution and evidence references, or durable status remains unchanged.
- Explicit human approval of an unchanged contract is required before TDD, and the approved snapshot plus approved scenario IDs are committed and recorded before TDD dispatch.

**Edge Cases:**
- Two resumes attempt a transition at the same revision.
- A delayed worker retries after another transition succeeded.
- The contract changes after the human review but before it is recorded.

**Out of Scope:**
- Worker self-approval, conversational approval, or memory-service lifecycle authority.

### REQ-006: Fail Closed on Durable Resume
**Description:** Durable submission state shall be compact, commit-bound, and stored only below `.rotta/workflow`. Resume/recovery validates phase-appropriate records before any lifecycle operation; uncertainty stops safely.

**Acceptance Criteria:**
- A ledger records submission identity, selected full baseline commit, status, revision, contract fingerprint/commit when established, approved scenarios, recorded worktree identity, implementation/reviewed/published commits when established, remote/ref verification facts, evidence pointers, and archive outcome.
- A new request never overwrites, adopts, normalizes, or repairs an existing ledger, contract, evidence record, or inventory for the same identity.
- Resume validates matching ledger, contract fingerprint, approved scope, relevant commits, recorded worktree identity, quality-policy fingerprint during Review, and remote/ref facts during Archive before dispatching work.
- Missing, malformed, conflicting, unreachable, or repository-escaping durable state reports the failed precondition and safe next action, starts no worker, and creates no substitute state.

**Edge Cases:**
- A process stops after an operation but before ledger persistence.
- A worktree moved, became dirty, points to another branch, or has a different HEAD.
- A record points to an evidence file that no longer exists.

**Out of Scope:**
- Automatic recovery, destructive repair, or reconstructing authority from host caches or conversational memory.

### REQ-007: Use Consent-Gated Local `vela` CLI Evidence
**Description:** Graph preflight, indexing, and queries shall execute the local `vela` CLI directly. The workflow shall not configure or connect to an endpoint, MCP server, proxy, tunnel, remote service, or remote graph storage. Indexing is performed only after explicit human consent.

**Acceptance Criteria:**
- Before Draft exploration, the Vela worker checks that the local executable is available and determines whether a local graph is present, complete, and tied to the selected full baseline commit.
- If indexing or re-indexing is required, the worker requests and durably records explicit consent before invoking the local `vela` CLI. Declined, unavailable, stale, incomplete, or failed indexing yields advisory uncertainty and does not fabricate graph findings.
- Each Vela invocation records command purpose, selected baseline, local executable identity, local graph-storage location, exit/result classification, and bounded redacted evidence. It makes no network connection and sends no source or graph payload outside the host.
- Draft uses at most five Vela invocations and returns one packet containing at most ten unique modules/symbols, ten dependency/impact risks, and five architectural constraints, with confidence and uncertainty. TDD and Review query only a stated structural question for affected scope.
- Vela findings cannot create requirements, approve contracts, satisfy a required Go quality gate, or advance lifecycle state.

**Edge Cases:**
- `vela` is absent, exits non-zero, indexes another commit, reports incomplete coverage, or returns more findings than packet bounds permit.
- A requested query would require a sixth Draft invocation or exceed an evidence bound.

**Out of Scope:**
- Automatic indexing, broad repeated exploration, remote graph operations, or graph output as an approval authority.

### REQ-008: Run Real Go Quality Adapters
**Description:** Review shall execute real local Go quality adapters for the exact candidate commit: tests with `go test ./...`, static analysis with `go vet ./...`, and coverage generated by `go test -coverprofile=<temporary-local-file> ./...` and evaluated with `go tool cover`. Their outcomes are recorded as bounded evidence.

**Acceptance Criteria:**
- The committed quality policy at `.rotta/workflow/quality-policy.yaml` names the three adapters, their command argv, applicable changed-path rules, required/advisory gate, thresholds, and normalized result schema.
- Each adapter executes in the recorded worktree at the candidate commit and reports command identity, candidate commit, applicability inputs, exit classification, normalized findings/metrics, redacted evidence pointers, and exactly one outcome: `pass`, `fail`, `blocked`, or `not_applicable`.
- Required applicable `go test` or `go vet` non-zero exits fail Review; missing executables, execution errors, incomplete output, malformed coverage profile, or inability to parse required evidence block Review.
- Coverage reports the measured percentage derived from the generated profile and applies the committed threshold. A required applicable threshold breach fails Review; missing or unreadable coverage evidence blocks Review.
- A required adapter is `not_applicable` only when its committed changed-path predicate is false. Tool absence, execution failure, malformed output, and threshold breach are never `not_applicable`.
- Archive is eligible only when all applicable required adapter outcomes pass; a required failure returns to TDD and a required blocked result remains Review.

**Edge Cases:**
- The repository has no changed Go paths, no testable packages, a package-level test failure, vet diagnostics, or a zero/partial coverage profile.
- The candidate changes between adapter runs or the policy changes after a review attempt.

**Out of Scope:**
- Remote analyzers, implicit language presets beyond these Go adapters, waiver by worker assertion, or raw tool-log retention.

### REQ-009: Use Local Git CLI with Explicit Remote and Ref
**Description:** Lifecycle operations requiring Git shall invoke the local `git` CLI against the recorded repository/worktree. A submission records one explicit remote name and fully qualified remote ref before publication; no default remote/ref inference is permitted.

**Acceptance Criteria:**
- Worktree observation, commit verification, contract commit creation, feature commit creation, publication, remote-ref resolution, and recorded-worktree removal are delegated bounded operations using local `git` CLI commands.
- Each publication operation names the recorded remote and fully qualified ref, verifies that the ref resolves exactly to the full `reviewed_commit`, and records the observed commit. Push success, branch name equality, and ancestor containment are insufficient.
- No operation merges, rebases onto a base branch, creates a pull request, accepts a change, deletes a branch/ref, or changes the recorded remote/ref implicitly.
- Archive requests cleanup consent only after initial exact remote/ref verification. Immediately after consent and before deletion, it re-verifies the same remote/ref; only an exact match permits removal of the exact recorded worktree and archive persistence.
- A mismatch, remote failure, declined consent, missing consent, or removal failure retains the worktree and records the specific pending, failed, or `publication_changed` outcome.

**Edge Cases:**
- A remote ref advances after initial verification, credentials fail, the ref is missing, or the reviewed commit exists only as an ancestor.
- A worktree is unrecorded, not clean, outside the selected repository, or already missing during Archive.

**Out of Scope:**
- Implicit remote selection, merge/acceptance automation, pull-request automation, or deletion of unrecorded local or remote resources.

### REQ-010: Preserve Contract, Evidence, and Archive Verification
**Description:** The hard spec and approved feature contracts remain the behavioral source of truth. Evidence, ledgers, graph packets, quality reports, and archive records are operational facts only and must be bounded and exact.

**Acceptance Criteria:**
- Every TDD and Review task traces to approved requirement and scenario IDs from an unchanged approved contract; changed contract content requires new explicit approval.
- Reports distinguish baseline, contract, implementation, reviewed, and published commits even when values coincide.
- Archive metadata retains the reviewed/published commit, both remote/ref observations, cleanup decision/outcome, worktree identity, and evidence references after successful cleanup; contract artifacts and durable metadata remain.
- Errors report submission, phase, expected versus observed durable state/commit where known, failed precondition, bounded evidence reference, uncertainty, and safe next action without secrets.

**Edge Cases:**
- A report is missing after a ledger points to it.
- Multiple retries create evidence for one authorized task.
- A stale graph result recommends scope absent from the approved contract.

**Out of Scope:**
- Using reports, graph output, Git state, or human conversation as an implicit contract amendment or lifecycle authorization.

## Open Questions
- None. The requested constraints define the supported host, durable root, CLI surface, graph execution model, quality adapters, and Git publication/cleanup semantics sufficiently for implementation.

## Trade-offs
- Complete removal without migration strands retired state, but avoids dual authority and compatibility risk.
- OpenCode-only installation narrows host reach, but eliminates unsupported selectors and host-specific maintenance paths.
- Consent-gated local indexing and bounded graph evidence can leave uncertainty, but prevent surprise work, remote exposure, and graph-driven authority.
- Real Go adapter execution is less generic than a configurable analyzer abstraction, but provides reproducible quality evidence for this Go repository.
- Exact remote/ref checks and retained worktrees delay cleanup, but protect the reviewed snapshot when branches move.
- Semantic inventory slows deletion, but prevents removal of mixed or unrelated functionality.

## Risk Level
critical — Justification: This replacement removes workflow surfaces while changing durable authority, host installation, graph execution, quality evidence, and Git cleanup boundaries. An incomplete removal, unsafe resume, false graph/quality evidence, or changed remote ref could permit unauthorized work or destroy the only recoverable worktree.
