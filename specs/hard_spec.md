# Hard Spec: Vela v2 — OpenCode-Only Rotta Workflow and Local Graph Intelligence

## Adversarial Pre-Mortem
- Failure mode 1: A specialist worker, a stale host instruction, or a prior lifecycle artifact advances a submission without an orchestrator-authorized transition, or a partial v1 removal leaves an old runtime selector able to do so. The ledger then claims a phase or commit that the approved contract and actual worktree do not support.
- Failure mode 2: A missing, stale, partial, or incorrectly based Vela graph is silently treated as fresh, or an unspecified quality tool result is treated as passing. Draft or review conclusions then omit affected modules, boundaries, or required quality failures.
- Failure mode 3: The branch published to a remote changes after initial verification, but cleanup still deletes the persistent worktree. The reviewed snapshot is no longer locally recoverable and the ledger falsely presents the feature as archived.
- Failure mode 4: A missing, malformed, or conflicting durable v2 record is mistaken for a first submission. A recovery then overwrites or recreates lifecycle authority, starts work against an unknown scope, or lets an Ancora pointer select a phase.

## Hidden Assumptions
- The orchestrator can select and identify a Git baseline commit before it requests graph preflight or exploration.
- The project can retain a stable, repository-local submission ledger and contract artifacts after a feature worktree is removed.
- A local Vela service can either identify the commit on which its graph was extracted or report that this cannot be established.
- Humans are available to answer the two explicit consent decisions: extraction/re-indexing and worktree cleanup.
- The configured quality tools can produce evidence for applicable changed code, or can report an unavailable/not-applicable condition without inventing a passing result.
- A remote ref can be queried after publication to determine the commit it resolves to; a successful push message alone is insufficient verification.
- A pre-deletion inventory can distinguish a legacy-only workflow/integration asset from a mixed or unrelated product asset using invocation and reachability evidence, not a name or string match.
- The installed v2 workflow can provide a repository-local, language-agnostic quality policy that names its analyzers, parsing contract, applicability rules, and gates.
- Each request explicitly declares whether it is a `NEW` submission or a `RESUME/RECOVERY`; request mode is never inferred from absent artifacts, a legacy marker, or Ancora content.

## Edge Case Sweep
- Concurrent resume attempts, a delayed worker result, or a persistence retry can replay a transition. Versioned, expected-state transition requests must reject stale writes rather than merge them.
- A graph may exist but have no trustworthy base-commit metadata, cover only part of the repository, or fail while re-indexing. Each case must be visible as uncertainty; none may become a fabricated graph finding.
- The cached worktree can be moved, deleted, dirty, on the wrong branch, or replaced by a path outside the selected repository. No worker may substitute another worktree or delete it without the recorded identity and the required cleanup confirmation.
- A quality command can be unavailable, return partial results, or find an issue after the code was committed. The review outcome and evidence must distinguish failure, blocked analysis, and not-applicable checks.
- A remote branch can contain an ancestor reviewed commit but resolve at its tip to a different commit. That is a publication mismatch for archive purposes, even if the reviewed commit exists somewhere on the remote.
- A configured analyzer can exit successfully with malformed, incomplete, or commit-mismatched output. That is blocked evidence for an applicable required check, not a pass or not-applicable result.
- A legacy asset can share a file, command, configuration key, or documentation term with active unrelated functionality. It must be retained or separated until deletion can be proved safe.
- A caller can label a request `NEW` even though the requested identity has a malformed or conflicting v2 artifact. That identity is not fresh and must not be overwritten or silently reclassified.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Retain legacy approval/lifecycle markers as a read-only fallback | A fallback is a second authority path and defeats the clean v2 replacement. |
| Let workers update lifecycle state after returning evidence | It permits a worker to decide the result of its own work and creates races across resumes. |
| Automatically re-index whenever Vela is unavailable or stale | Graph extraction can be expensive or surprising; the human must consent before it starts. |
| Make Vela a mandatory gate for Contract, TDD, or Review | Local graph evidence can be unavailable or incomplete and must remain advisory rather than define required behavior. |
| Query Vela broadly before every TDD scenario | Repeated broad queries add cost and noise; exploration belongs in Draft and TDD queries must be targeted only when blocked. |
| Treat a successful push as merge, acceptance, or archive authorization | Publication neither proves merge/acceptance nor permits cleanup until the exact remote commit and human cleanup decision are verified. |
| Keep a sibling-worktree convention as the durable location policy | It is brittle across resumes and checkouts; v2 targets a recorded persistent cache-worktree identity instead. |
| Hard-code a SonarQube, Go, or other language preset for Review | V2 must work through an explicit project quality policy, with independently testable commands/adapters and result parsing rather than a branded or language-specific integration. |
| Delete all files that mention a retired host or v1 marker | Text matches cannot establish workflow reachability or ownership and would erase unrelated product behavior. |
| Retain a non-OpenCode v2 installer or runtime selector “just in case” | It creates an unsupported second integration path and undermines the OpenCode-only replacement boundary. |
| Infer `NEW` from a missing ledger or let Ancora repair a resume | Absence can mean loss or corruption rather than first use; inference would turn advisory or incomplete data into lifecycle authority. |

## Summary
Vela v2 is an OpenCode-only clean rewrite of Rotta's workflow and integration surface. It replaces legacy/v1 workflow code, tests, fixtures, skills/agent assets, configuration, installer/runtime integrations, and compatibility shims that serve the replaced workflow, using an inventory to protect unrelated product functionality. The orchestrator-led lifecycle remains Draft, Contract, TDD, Review, and Archive; an explicitly requested `NEW` submission may establish its initial Draft contract/submission ledger state from no prior v2 state, while `RESUME/RECOVERY` requires phase-appropriate valid durable state and otherwise fails closed. Approved contract artifacts define behavior, and a compact ledger records only operational facts and authorized transitions. Vela is local-only advisory evidence: bounded Draft exploration informs Contract, while TDD and Review use targeted queries only when warranted. Review is language-agnostic and follows a versioned project quality-policy configuration. Archive may remove the recorded worktree only after two exact remote-ref checks—one before, and one immediately after, human cleanup confirmation.

## Goals
- Replace all in-scope legacy/v1 workflow and integration behavior with one OpenCode-only v2 workflow; no legacy compatibility, migration, fallback, dual-read path, host selector, or runtime adapter remains in that scope.
- Make the orchestrator accountable for every phase decision and the sole authority that authorizes lifecycle transitions while keeping specialist workers narrowly delegated.
- Make approved contract artifacts the behavioral source of truth; allow explicit first-time Draft initialization only for `NEW`; and retain enough durable ledger facts to resume, audit, publish, and archive a submission safely.
- Add useful local Vela preflight, exploration, and targeted validation without making graph availability or graph output an approval authority.
- Preserve Red-Green-Refactor and scenario traceability while allowing deliberately bounded approved scenario batches.
- Archive only an exact, remotely verified reviewed snapshot and only after an explicit human decision about local worktree cleanup followed immediately by a second exact remote-ref verification.

## Non-Goals
- Backward compatibility, migration, import, interpretation, or cleanup of pre-v2 lifecycle/approval artifacts.
- Supporting v2 through Claude, Codex, or any other non-OpenCode host integration; retaining their v1 workflow installers, runtime selection, compatibility shims, skills, agent assets, or configuration within the replaced workflow/integration scope.
- A hosted Vela service, remote graph upload, graph-derived behavioral requirements, or graph-driven phase advancement.
- Automatic extraction/re-indexing, automatic cleanup, automatic merge, pull-request creation, or treating publication as acceptance.
- A dependency on SonarQube, a Go preset, or any other particular analyzer, language, framework, or coverage tool.
- Removing an unrelated product feature, test, fixture, configuration, installer, runtime, or documentation solely because it contains a v1, legacy, host, or non-OpenCode word.
- Allowing any specialist worker, Ancora memory entry, host-local cache, or remote branch to approve scope or authorize a lifecycle transition.
- Inferring a `NEW` request from missing, invalid, or conflicting state, or converting a `RESUME/RECOVERY` request into `NEW` without a new explicit request and a fresh identity.
- Defining a user-interface, editor integration, or a replacement for the project's Git credentials and remote access policy.

## Actors and Authority
| Actor | May do | Must not do |
|-------|--------|-------------|
| Human | Supply a draft; explicitly request `NEW` or `RESUME/RECOVERY`; explicitly approve the unchanged Contract; consent to or decline requested extraction/re-indexing; consent to or decline requested worktree cleanup; make a new explicit decision when `publication_changed` is recorded; control credentials and any external merge/acceptance decision. | Directly alter the durable lifecycle status by conversational assertion alone, have a cleanup decision treated as merge/acceptance, or have a `RESUME/RECOVERY` request inferred to be `NEW`. |
| Orchestrator | Select the baseline; delegate bounded work; evaluate worker evidence against the contract; authorize every legal state transition; request lifecycle persistence; route failed review to TDD; request the required human decisions. | Inspect code, run commands/tests, edit contract/source artifacts, manage worktrees, commit, push, publish, remove worktrees, or clean/archive directly. |
| Spec + Gherkin worker | Produce the delegated hard spec and Gherkin contract and return bounded evidence. | Approve a contract, create ledger state, choose a transition, commit, or start TDD. |
| `rotta-index` | Check local graph availability, base-commit freshness, and completeness; request human consent before extraction/re-indexing; return a bounded preflight result. | Re-index without consent, define behavior, edit the contract, or advance the lifecycle. |
| `rotta-explore` | In Draft, return one bounded exploration packet for the selected baseline. | Re-run broad exploration during Contract, define required behavior, or advance the lifecycle. |
| TDD worker | Implement only its authorized approved scenario batch, preserve per-scenario Red-Green-Refactor evidence, and ask Vela targeted structural questions only when blocked. | Select new scenarios, authorize transitions, commit/push, manage worktrees, or query Vela automatically for every scenario. |
| Review worker | Return actual quality, test/static/coverage, and targeted Vela boundary/impact evidence with pass, fail, blocked, or not-applicable outcomes. | Approve lifecycle state, publish, or treat Vela as a behavioral gate. |
| `rotta-ops` / lifecycle persistence worker | Execute one orchestrator-authorized bounded operational task; validate preconditions; atomically persist an authorized transition; manage the recorded worktree; create/verify authorized commits or publication; remove the worktree only in Archive. | Choose transitions, change scope, modify source/contract content, merge, accept, or use credentials outside the authorized remote/ref operation. |
| Ancora (if enabled) | Retain cross-session context and pointers to durable artifacts. | Be the lifecycle authority, sole durable truth, approval source, lock, or recovery mechanism. |

## Lifecycle

The five user-facing phases are **Draft → Contract → TDD → Review → Archive**. `rotta-index` is a delegated preflight capability, not a sixth user-facing phase. Every request explicitly identifies `NEW` or `RESUME/RECOVERY`; neither absence of artifacts nor any legacy/Ancora data may select that mode. The orchestrator alone authorizes transitions; a lifecycle persistence worker may only validate and atomically persist the already-authorized expected transition.

| Phase/status | Entry condition | Permitted outcome |
|--------------|-----------------|-------------------|
| Draft | For `NEW` only: a human explicitly requests `NEW`, supplies a draft, the orchestrator selects `base_commit`, and the identity has no prior v2 contract/ledger state; authorized initialization may create the initial Draft contract/submission ledger state. For `RESUME/RECOVERY`: the recorded Draft state is entered only after its phase-appropriate durable record validates. Index preflight and one bounded exploration packet are requested. | Contract when Draft findings, including Vela uncertainty, are returned; stopped when a required human decision is pending. |
| Contract | The spec worker writes the hard spec and Gherkin contract from the Draft packet. | TDD only after explicit human approval of the unchanged contract and an ops-created, ledger-recorded `contract_commit`; otherwise remain in Contract. |
| TDD | The approved contract, exact `contract_commit`, recorded worktree identity, and authorized scenario batch are valid. | Review after all approved scenarios have traceable accepted evidence; remain/return to TDD on batch failure or a failed Review. |
| Review | All approved TDD work is recorded at an implementation snapshot. | Archive only after required quality evidence passes and that exact snapshot is recorded as `reviewed_commit`; return to TDD on denied/failed quality gates. Blocked review remains Review until evidence can be obtained or the workflow is explicitly otherwise resolved. |
| Archive | The intended remote ref is verified to resolve exactly to `reviewed_commit`. | `archived` only after human cleanup confirmation, an immediate second exact verification of that remote ref, successful delegated worktree removal, and archive metadata persistence. A declined, missing, failed, or `publication_changed` cleanup outcome remains Archive with the worktree retained. |

## Data and Artifacts
- **Behavioral contract:** `specs/hard_spec.md` and the feature Gherkin contract(s), including unique scenario and requirement identifiers. They define required behavior and approved scope.
- **Submission ledger:** a compact v2 record at `.rotta/v2/submissions/<submission-id>.yaml`. An explicitly authorized `NEW` request may create the initial Draft contract/submission ledger state from no prior v2 contract/ledger state; it records the new identity, supplied draft, Draft status, ledger version, and selected `base_commit`, while contract fingerprint/pointers, `contract_commit`, approved scenario IDs, and downstream lifecycle facts remain unestablished. It is not an approved behavioral contract and cannot authorize a later phase. Once those facts are established, the ledger contains at least `submission_id`, feature identity, status, ledger version, selected `base_commit`, contract fingerprint/pointers, `contract_commit`, persistent worktree identity, approved scenario IDs, implementation/reviewed/published commit fields, remote/ref verification facts, ordered transition/evidence pointers, and archive/cleanup outcome. It records facts and authorized status; it cannot make a contract valid or change its behavioral requirements.
- **Worktree identity:** a durable record of the persistent cache-worktree path, repository identity, branch/ref, and expected commit. V2 does not depend on or recreate a sibling-worktree convention.
- **Vela preflight result:** selected base commit; availability; freshness; completeness; consent request/decision where applicable; resulting graph base commit if known; and explicit uncertainty/fallback reason.
- **Exploration packet:** selected base commit; bounded affected modules; dependency/impact risks; architectural constraints; confidence per finding; graph status; uncertainty; and evidence pointers. It contains no lifecycle decision.
- **Quality-policy configuration:** the committed `.rotta/v2/quality-policy.yaml` document using the `rotta-quality-policy/v1` contract in REQ-111. Review records its content fingerprint with the reviewed candidate; a changed policy requires a new complete Review for the changed candidate.
- **Worker evidence:** a bounded structured result with submission/task ID, ledger version or authorization token, phase, requested scope, outcome, base/relevant commit, changed or inspected module references where applicable, evidence pointers, uncertainties, and remediation/next-action recommendation. Evidence must not contain credentials or secret values.
- **Quality report:** the applicable quality dimensions, findings and severities, test/static/coverage evidence, Vela impact/boundary evidence or uncertainty, outcome, and evidence pointers tied to the reviewed commit.
- **Removal inventory:** a versioned, submission-scoped evidence artifact referenced by the ledger before any in-scope legacy deletion. It records the discovery and reachability inputs used to establish its candidate set and an explicit reconciliation from every discovered in-scope candidate to exactly one inventory entry. Each candidate identifies its repository-relative path or non-file asset, `asset_category` (`workflow_code`, `test`, `fixture`, `skill_or_agent_asset`, `configuration`, `installer_or_runtime_integration`, or `compatibility_shim`), semantic role, invocation/reachability evidence, classification (`legacy_only`, `mixed`, or `unrelated`), proposed action, v2 replacement or retention rationale, affected tests/fixtures, and required post-removal verification evidence. A missing required field, unreconciled discovered candidate, uncertain classification, or unrecorded required category makes the inventory incomplete.
- **Archive metadata:** immutable compact facts that retain the exact reviewed/published commit, initial and post-confirmation remote/ref verification results, `publication_changed` where applicable, cleanup decision/outcome, worktree identity, and evidence pointers after worktree deletion. Contract artifacts and the remote branch are retained.

## Requirements

### REQ-101: Replace the Legacy Lifecycle with V2 Only
**Description:** Vela v2 shall use only the five-phase lifecycle and v2 artifacts defined by this specification. An explicitly requested `NEW` submission may establish only its initial Draft state under REQ-104; a `RESUME/RECOVERY` submission derives state only from the phase-appropriate valid v2 durable artifacts. Within the replaced workflow/integration scope, pre-v2 approval and lifecycle paths are neither read, written, migrated, reported as valid state, nor used as fallback authority. The replacement scope includes legacy/v1 workflow code, tests, fixtures, skills/agent assets, configuration, installer/runtime integrations, and compatibility shims that select, install, execute, validate, or preserve the old workflow; it does not include unrelated product functionality merely because it uses similar words or host names.

**Acceptance Criteria:**
- An explicitly requested `NEW` v2 submission may initialize only a Draft contract/submission ledger state from no prior v2 contract/ledger state as REQ-104 defines; `RESUME/RECOVERY` derives its state only from valid phase-appropriate v2 durable contract/ledger artifacts.
- A legacy marker or approval artifact, whether present, stale, malformed, or apparently matching the feature, does not authorize Draft, Contract, TDD, Review, Archive, or completion.
- Active v2 templates, instructions, fixtures, tests, installer/runtime behavior, and configuration do not create, select, depend on, or validate a legacy lifecycle/approval path.
- The in-scope legacy removal is planned and verified according to REQ-118 before the v2 rewrite is accepted.

**Edge Cases:**
- A checkout contains a legacy marker alongside no v2 ledger.
- A legacy artifact names the same scenario IDs or commit as a v2 submission.
- Historical commits retain legacy artifacts after v2 is installed.

**Out of Scope:**
- Translating, preserving, or resuming any legacy workflow.

### REQ-102: Keep Contract Artifacts as Behavioral Source of Truth
**Description:** The hard spec and Gherkin contract define behavior and approved scenario scope. The ledger, worker reports, graph packets, memory pointers, and branch state provide operational facts only and may not add, remove, or reinterpret contractual behavior.

**Acceptance Criteria:**
- Every implementation and review task is traceable to one or more approved requirement and scenario IDs in the immutable approved contract.
- Contract approval identifies an unchanged contract fingerprint and the exact `contract_commit`; changed contract content requires a new Contract approval before TDD may continue.
- A ledger entry, Vela result, worker claim, or human conversational statement cannot substitute for a missing or invalid contract artifact.

**Edge Cases:**
- A ledger points at a contract file whose content has changed since approval.
- A Vela packet recommends scope absent from the approved Gherkin.
- A scenario ID exists in a report but not exactly once in the approved contract.

**Out of Scope:**
- Using operational evidence as an implicit contract amendment.

### REQ-103: Make the Orchestrator the Sole Transition Authorizer
**Description:** The orchestrator is accountable end-to-end and is the only actor allowed to authorize a lifecycle transition. It delegates all operational execution to bounded specialist tasks and may ask a lifecycle persistence worker to atomically validate and record an already-authorized transition. Its own permitted action surface is limited to dispatching bounded tasks, evaluating compact returned worker evidence with any required durable human decision, and authorizing a legal transition; it is never an executor.

**Acceptance Criteria:**
- A transition request identifies the submission, expected current status, target status, ledger version, authorized scope, and evidence references; persistence rejects a stale, illegal, incomplete, or worker-originated transition.
- Spec, index, explore, TDD, review, and ops workers return bounded evidence and cannot independently select a next phase or alter approved scope.
- Every transition authorization links compact worker evidence for the completed delegated task(s), the expected ledger version and status, and any required durable human decision. Direct inspection, direct tool output, an unbounded conversation, an Ancora pointer, or a worker recommendation without the required evidence cannot authorize a transition.
- Actor-attributed operational audit evidence distinguishes `dispatch`, `evidence_evaluation`, and `transition_authorization` from command execution, test execution, source/contract/artifact writes, worktree operations, commits, pushes/publication, and cleanup. When one of the latter operations is needed, the orchestrator dispatches a bounded worker task and waits for its evidence; it emits no such operational event and causes no operational effect directly.
- A failed persistence attempt leaves no falsely advanced durable status and reports a recoverable ambiguity with the expected and observed ledger version.

**Edge Cases:**
- A timed-out worker retries an old transition after a resume.
- Two orchestrator processes race to accept different evidence for the same submission.
- A worker has filesystem access sufficient to alter a ledger directly.
- A transition has a valid human approval but lacks the compact evidence required for its delegated operational precondition.

**Out of Scope:**
- Giving host-native workers or memory services independent lifecycle authority.

### REQ-104: Persist a Compact, Commit-Bound Submission Ledger
**Description:** Each v2 submission shall have one durable, compact ledger under the v2 namespace. `NEW` is an explicit first-initialization request, not a recovery path: only it may establish an initial Draft contract/submission ledger state from no prior v2 state. Together, the phase-appropriate valid contract artifacts and matching ledger preserve the operational facts needed to resume safely while separating behavioral authority from lifecycle facts. Ancora, if enabled, is an advisory pointer service only: it cannot supply, select, repair, override, or recover lifecycle state.

**Acceptance Criteria:**
- The ledger records the selected `base_commit` and status and, once each fact is established for the lifecycle phase, the contract fingerprint and `contract_commit`, worktree identity, approved scenarios, implementation/reviewed/published commit fields, evidence pointers, and archive result.
- A recorded commit field is a full immutable commit identifier and is verified against the recorded repository/worktree or remote before the field is used for a lifecycle decision.
- Ledger writes are atomic with their authorized transition or fail without claiming the transition occurred.
- Only an explicitly requested `NEW` submission with a fresh identity, supplied draft, and selected `base_commit` may initialize an initial Draft contract/submission ledger from no prior v2 contract/ledger state. It records no approved scope, contract fingerprint, `contract_commit`, or later lifecycle fact, and it does not infer a phase beyond Draft.
- `NEW` never overwrites, adopts, normalizes, or repairs an existing v2 contract/ledger artifact. A valid, malformed, incomplete, or conflicting artifact for the requested identity means the identity is not fresh; initialization stops with a safe next action and requires a different fresh identity or explicit resolution outside this lifecycle operation.
- `RESUME/RECOVERY` derives the submission identity, phase, approved scope, and lifecycle facts only from valid matching phase-appropriate contract artifacts and a valid submission ledger. It must not synthesize a missing ledger/contract, fill a required durable field, select an alternate submission, reclassify itself as `NEW`, or repair a lifecycle fact from Ancora content.
- Ancora, if available, may retain a pointer to the ledger but a missing, stale, or conflicting pointer has no lifecycle effect. A pointer that disagrees with the requested submission ID, ledger version/status, contract fingerprint, or durable artifact location is ignored as advisory evidence and reported as stale or conflicting; it cannot alter the ledger or schedule work.
- When a `RESUME/RECOVERY` request has missing, invalid, or conflicting required durable contract/ledger state and cannot unambiguously establish lifecycle state, it fails closed: it reports the inability to establish state and safe next action, starts no lifecycle operation (including Draft initialization or worker dispatch), and does not use Ancora to recover, recreate, or replace authority.

**Edge Cases:**
- The process fails between an operational action and ledger persistence.
- The ledger is missing, malformed, points outside the repository, or names an unreachable commit.
- A resume finds a worktree whose HEAD differs from the recorded commit.
- Ancora returns two incompatible pointers, or a pointer names an Archive status while the matching durable ledger records Contract.
- A request explicitly says `NEW`, but the requested identity has an invalid, partial, or conflicting v2 contract/ledger artifact.

**Out of Scope:**
- Storing source code, full tool logs, credentials, or a second behavioral contract in the ledger; inferring `NEW` from absent/broken durable state or reusing a non-fresh identity as a recovery shortcut.

### REQ-105: Enforce Contract Approval Before TDD
**Description:** Contract is a human approval gate. After the Spec + Gherkin worker returns its artifacts, the orchestrator must obtain explicit approval of the unchanged contract before it authorizes an ops task to create and record the approved `contract_commit` and before it enters TDD.

**Acceptance Criteria:**
- The human approval identifies the unchanged contract snapshot; approval of a different or later snapshot is not reusable.
- The Contract-to-TDD transition is authorized only after `contract_commit` is successfully created/verified and atomically recorded with the approved scenario scope.
- A denied, absent, ambiguous, or stale approval leaves the submission in Contract and starts no TDD task.
- The Spec + Gherkin worker does not create approval records, baseline/state records, or commits.

**Edge Cases:**
- The contract changes after the human reads it but before the commit is recorded.
- Commit creation succeeds but the ledger write fails.
- Human approval covers a scenario subset; only that exact approved subset is eligible for TDD.

**Out of Scope:**
- Implicit approval inferred from a message, test run, commit message, or branch name.

### REQ-106: Use a Persistent, Recorded Cache Worktree
**Description:** V2 shall use a persistent cache-worktree policy rather than a sibling-worktree convention. The selected worktree is identified in the ledger and reused across legal phases until Archive cleanup succeeds.

**Acceptance Criteria:**
- Before a delegated task that needs a worktree, ops validates its recorded repository identity, path, branch/ref, and expected commit rather than selecting a convenient substitute.
- The worktree remains retained through Contract, TDD, Review, publication verification, and any pending cleanup decision.
- A missing, moved, dirty, mismatched, or unvalidated recorded worktree stops the affected operation without automatic replacement, deletion, or use of another checkout.
- Worktree creation, validation, reuse, and removal are delegated operations; the orchestrator only authorizes them from evidence.

**Edge Cases:**
- A persistent cache root is unavailable after a host resume.
- Two submissions request the same worktree identity.
- The worktree is detached, points at another feature branch, or contains non-ignored changes.

**Out of Scope:**
- Preserving the current sibling-worktree layout or deleting an unrecorded checkout.

### REQ-107: Bound the `rotta-ops` Role and Split It When Necessary
**Description:** `rotta-ops` is the preferred small operational worker for recorded worktree operations, authorized commit/publication verification, cleanup, archive metadata, and authorized lifecycle persistence. Its task scope must remain bounded and must split before it spans unrelated operational trust boundaries.

**Acceptance Criteria:**
- An ops task names one submission, one recorded worktree, an expected ledger version/status, explicit permitted operations, and the exact evidence it must return.
- One task may operate on no more than one worktree, one remote/ref publication target, and one resulting ledger/archive write. It may perform the ordered Archive cleanup sequence—including its initial and post-confirmation exact verification pair—only when all actions concern that same submission, worktree, and remote/ref.
- The role must be split into separate delegated tasks when work would span more than one worktree, more than one remote/ref action, an unrecorded repository, source/contract editing, or an operation unrelated to the submission's authorized lifecycle step.
- Ops must not merge, create acceptance records, create pull requests, alter source/contract content, or delete branches/remotes.

**Edge Cases:**
- A task request combines cleanup of two feature worktrees.
- A publication action requests a different remote or ref than the ledger records.
- Archive needs a retry after cleanup failed.

**Out of Scope:**
- A general-purpose repository administration worker.

### REQ-108: Run Consent-Gated Local Vela Index Preflight
**Description:** Before Vela exploration for a selected baseline, the orchestrator delegates `rotta-index`. It checks local graph availability, exact-base freshness, and completeness, tying every result to the selected `base_commit`. It must request human consent before extraction or re-indexing.

**Acceptance Criteria:**
- Preflight classifies the graph as `fresh` only when local graph evidence establishes the exact selected `base_commit` and no incompleteness is reported.
- No local graph, unresolvable/mismatched base commit, failed graph access, or unavailable metadata is reported as missing, stale, or unavailable with explicit uncertainty rather than as fresh.
- If graph extraction or re-indexing is needed, `rotta-index` asks the human and records the decision before beginning that operation; it never starts automatically.
- On consent, the result records the resulting graph base commit and any incompleteness. On decline or failure, Draft continues with a clearly marked fallback/uncertainty result unless another non-Vela requirement independently blocks it.

**Edge Cases:**
- The graph has a matching commit but extraction reported omitted files or an error.
- The selected baseline changes while consent is pending.
- Re-indexing completes for a different commit, times out, or produces no base-commit metadata.

**Out of Scope:**
- Remote graph indexing, automatic graph refresh, or treating graph freshness as human approval.

### REQ-109: Produce One Bounded Draft Exploration Packet
**Description:** In Draft, `rotta-explore` uses the preflight result to produce one bounded packet for the selected base commit. A Vela query is any graph-query invocation used by Draft preflight or exploration; the combined Draft activity may make at most five. The packet informs the Contract worker and records uncertainty when graph evidence cannot support a conclusion.

**Acceptance Criteria:**
- The packet contains at most 10 unique affected modules or symbols, at most 10 dependency/impact risks, and at most 5 architectural constraints, plus confidence, base commit, graph status, uncertainty, and evidence references.
- The packet records the count and purpose of each of its at most 5 Vela queries; it contains neither raw graph dumps nor raw tool logs.
- Contract consumes the Draft packet as advisory input and may convert supported findings into explicit requirements; it does not repeat broad repository exploration.
- When Vela is stale, missing, incomplete, or unavailable, the packet explicitly identifies the fallback and unknown structural areas rather than inventing affected modules or confidence.
- If a bound prevents the packet from representing a finding, the packet records the capped/truncated scope and resulting uncertainty; it does not silently omit the finding, exceed a bound, or issue additional broad queries.
- Vela findings cannot independently create required behavior, approve a scenario, or change a lifecycle status.

**Edge Cases:**
- The feature has no graph-resolvable affected modules.
- The graph exposes a dependency path that conflicts with an assumption in the draft.
- The packet is returned for a base commit different from the ledger-selected baseline.
- A sixth query or an eleventh module, symbol, or risk appears necessary to investigate the draft.

**Out of Scope:**
- Repeated broad exploration during Contract or using Vela as a contract approval engine.

### REQ-110: Preserve Red-Green-Refactor with Bounded Scenario Batches
**Description:** TDD may receive a bounded batch of up to three already-approved scenarios to reduce orchestration overhead, but each scenario preserves separate Red-Green-Refactor traceability and acceptance evidence. Vela is optional and targeted only when the TDD worker is blocked by a structural question.

**Acceptance Criteria:**
- The orchestrator authorizes only scenario IDs present in the approved contract and records each batch's exact IDs; a batch contains one to three scenarios.
- The TDD worker reports Red, Green, and Refactor evidence separately for every scenario and stops after its authorized batch.
- A Vela query during TDD names the blocking structural question, relevant module/symbol boundary, base/relevant commit, result or uncertainty, and is absent when no structural block exists.
- The orchestrator accepts evidence per scenario, not merely per batch, and does not authorize another batch if an approved scenario lacks required traceability or the worktree boundary is invalid.

**Edge Cases:**
- One scenario in a three-scenario batch fails while earlier scenarios succeeded.
- A worker requests a fourth scenario, an unapproved scenario, or broad exploration unrelated to a stated block.
- Vela is unavailable when a targeted TDD question is asked.

**Out of Scope:**
- Parallel execution of scenarios in one worktree or automatic Vela queries for every scenario.

### REQ-111: Enforce a Configured, Language-Agnostic Quality Policy in Review
**Description:** Review must use a committed `.rotta/v2/quality-policy.yaml` conforming to `rotta-quality-policy/v1`; no SonarQube, Go, or other language/tool preset is implicit. The policy is the sole configuration authority for review analyzers and gates. It is required to contain a non-empty `policy_id` and `policy_version`; a unique `analyzers` list; and a `dimensions` mapping for each canonical dimension: `code_smells`, `duplication`, `complexity`, `maintainability`, `good_practices`, `security`, `risk`, `tests`, `static_analysis`, and `coverage`. An analyzer specifies a unique ID, exactly one shell-free `command` argv invocation or named `adapter` with its adapter configuration, supported dimensions, an `applies_when` predicate, and a parser declaration. The only valid predicates are `all` and `changed_path_matches_any` with a non-empty list of repository-relative glob patterns evaluated against the recorded base-to-candidate changed paths; no analyzer output or absent tool may influence applicability. The parser declaration names `quality-result/v1` and maps command/adapter output into its normalized result schema. Each dimension specifies its analyzer, `gate` (`required` or `advisory`), `applies_when` predicate or inherited analyzer predicate, severity policy, and metric thresholds. Severity policy supplies a total ordered severity scale and an explicit blocking severity for the dimension; every metric threshold supplies a metric ID, one of `lt`, `lte`, `eq`, `neq`, `gte`, or `gt`, a numeric or boolean target, and a unit where applicable.

**Acceptance Criteria:**
- The normalized `quality-result/v1` for each configured analyzer contains its analyzer ID, policy ID/version/fingerprint, exact candidate commit, evaluated applicability inputs and rationale, one result for every configured dimension it supports, outcome, findings, measured metrics, threshold comparison, execution status, and redacted evidence pointers. A finding contains a stable rule/finding ID, dimension, severity, message, and location or an explicit location-unavailable rationale. Outcome is exactly `pass`, `fail`, `blocked`, or `not_applicable`.
- Every canonical dimension is represented in the report for the exact implementation snapshot proposed as `reviewed_commit`; it is tied to valid analyzer evidence, a policy-authorized `not_applicable` result, or a `blocked` result. A missing policy dimension, duplicate/unknown analyzer or dimension, malformed policy, unreadable/partial/unparseable output, candidate-commit mismatch, unavailable command/adapter, non-zero analyzer execution without a complete parseable result, or missing required evidence makes every affected applicable required dimension `blocked`.
- `not_applicable` is permitted only when its configured predicate evaluates false from recorded changed-path facts, with those facts and rationale in evidence. Tool absence, execution failure, parse failure, incomplete output, or an unmet threshold must never become `not_applicable`.
- An applicable dimension is `fail` only when valid complete normalized evidence identifies a finding at or above its configured blocking severity or a configured metric threshold breach; it is `pass` only when valid complete evidence identifies no such breach. A finding with a severity absent from the configured ordered scale, or a metric with an undeclared/ill-typed comparator or target, is malformed evidence and blocks its applicable dimension. Required applicable `fail` routes Review to TDD; required applicable `blocked` keeps Review blocked. Advisory failures and blocks are reported but affect lifecycle only if the policy explicitly marks that dimension required.
- Review may record `reviewed_commit` and enter Archive only when every applicable required dimension is `pass` and every other dimension has one of the four explicit outcomes. The report includes the policy path, policy ID/version/fingerprint, resolved command or adapter identity, parser/schema version, invocation/execution evidence pointer, normalized-result evidence pointer, applicability decision, finding/threshold summary, and redacted evidence pointers; it does not embed raw tool logs.
- Tests, static analysis, and coverage are evaluated through the same policy contract when their predicates apply; unavailable required evidence is blocked rather than treated as passing.

**Edge Cases:**
- A quality tool exits successfully but emits unreadable or partial results.
- A change has no applicable coverage target but has applicable static analysis.
- A security finding, excessive complexity, or duplication crosses the configured severity or metric threshold.
- A policy is changed after a review attempt; the changed candidate and policy fingerprint receive a complete new Review.

**Out of Scope:**
- Provisioning, licensing, or communicating with a SonarQube server, or supplying a default analyzer, language, severity scale, threshold, or policy waiver outside `quality-policy.yaml`.

### REQ-112: Use Targeted Vela Review Validation Without Making It a Gate Authority
**Description:** Review shall request targeted Vela impact and boundary validation for changed modules when graph evidence is available. The results improve risk reporting but do not define behavior or independently pass, fail, or advance the lifecycle.

**Acceptance Criteria:**
- The review evidence identifies changed modules, the targeted impact/boundary question, graph base commit/status, findings, and uncertainty.
- If Vela is stale, incomplete, missing, or unavailable, review records the uncertainty and continues to determine quality from non-Vela required evidence; it must not report a Vela-backed validation that did not occur.
- The orchestrator transitions Review only after it evaluates the complete review result; Vela output alone cannot route to Archive or TDD.

**Edge Cases:**
- A changed module is absent from an otherwise fresh graph.
- Targeted Vela evidence finds an unexpected boundary but all quality checks pass.
- The graph's base commit predates the implementation snapshot.

**Out of Scope:**
- Broad re-exploration during Review or automatic graph re-indexing to make a review pass.

### REQ-113: Route Failed Review Back to TDD
**Description:** A denied or failed required Review gate returns the submission to TDD through an orchestrator-authorized, atomically persisted transition. Review evidence remains durable and traceable; it cannot be overwritten as a pass.

**Acceptance Criteria:**
- The ledger records the failed review outcome, reviewed candidate commit, quality evidence pointers, and TDD remediation route.
- No publication verification, cleanup request, worktree deletion, or Archive transition occurs for a failed or blocked review.
- After TDD remediation, Review evaluates the new implementation snapshot; a prior pass or failure cannot be reused for a changed commit.

**Edge Cases:**
- Review fails after an earlier attempt passed for a different commit.
- The transition persistence fails after review evidence is produced.
- A failed quality gate is later reclassified only with new auditable evidence.

**Out of Scope:**
- Waiving required quality gates by a worker assertion.

### REQ-114: Archive Only the Exact Reviewed and Published Snapshot
**Description:** Archive is strictly ordered: required Review passes; the exact `reviewed_commit` is published and verified on the ledger-recorded remote/ref; the human is asked to confirm cleanup; immediately after that confirmation and before any deletion, ops re-verifies that exact remote ref; only then may ops remove the recorded worktree and persist archive metadata. Publication is neither merge nor acceptance and the branch is retained.

**Acceptance Criteria:**
- Before cleanup is requested, remote verification establishes that the intended recorded remote ref resolves exactly to `reviewed_commit`; a push result or ancestor match is insufficient.
- A remote/ref mismatch, unreachable remote, unverified publication, or changed reviewed snapshot prevents cleanup and Archive completion and records the mismatch.
- Only after successful exact publication verification does the orchestrator request explicit human cleanup confirmation.
- Immediately after an explicit cleanup confirmation, and with no deletion before it, ops re-queries the same ledger-recorded remote/ref. It may remove the worktree only when that re-verification resolves exactly to the same `reviewed_commit`.
- If post-confirmation re-verification differs, is unreachable, or cannot be established, ops persists a `publication_changed` Archive outcome with the original and observed remote/ref/commit facts and evidence pointer, does not remove the worktree, leaves the submission in Archive rather than `archived`, and requests a new explicit human decision. A later cleanup attempt must restart from exact remote verification, obtain a new cleanup confirmation, and perform the post-confirmation re-verification again.
- On a successful post-confirmation re-verification, ops removes only the recorded worktree; only after successful removal may it atomically record `archived` metadata. It retains the branch, remote ref, contract artifacts, ledger, and evidence pointers.
- On cleanup decline, no response, or removal failure, the submission remains Archive with durable pending/declined/failed cleanup evidence and the worktree is retained.

**Edge Cases:**
- The remote ref advances after initial verification but before cleanup.
- The remote becomes unreachable immediately after human cleanup confirmation.
- The reviewed commit is merged elsewhere but the intended feature ref does not resolve to it.
- The worktree disappears before cleanup or deletion succeeds but archive metadata persistence fails.

**Out of Scope:**
- Merging, pull-request creation, branch deletion, automatic acceptance, or remote cleanup.

### REQ-115: Fail Closed, Preserve Evidence, and Support Safe Resume
**Description:** Invalid artifacts, unavailable dependencies, concurrency conflicts, and operation failures shall stop the affected action without fabricating success, losing evidence, deleting data, or silently selecting substitutes. Resume uses durable v2 artifacts and verifies current facts again.

**Acceptance Criteria:**
- Errors report the submission, phase, expected versus observed state/commit where available, failed precondition, evidence pointer, and safe next action without exposing secrets.
- A stale worker result, replayed authorization, illegal transition, or ledger version conflict is rejected with no phase advancement.
- `RESUME/RECOVERY` revalidates phase-appropriate durable state before acting: the initial Draft contract/submission ledger identity and selected baseline for Draft; and the contract fingerprint, approved scope, ledger version, selected baseline, worktree identity, relevant commit, quality-policy fingerprint where Review is involved, and any required remote verification for later phases.
- Missing, invalid, or conflicting required durable contract/ledger state on `RESUME/RECOVERY` reports a safe next action and starts no lifecycle operation, Draft initialization, transition, or worker task. Ancora pointers may be reported as advisory diagnostics only and cannot recreate, repair, select, or replace durable authority.
- Vela unavailability is reported as advisory uncertainty; contract, quality, worktree, publication, and cleanup errors fail according to their own required preconditions.

**Edge Cases:**
- A process stops during cleanup, during commit verification, or after remote publication but before ledger persistence.
- A ledger/evidence pointer refers to a deleted local report.
- A remote outage occurs after a previously verified publication.
- An explicit `NEW` request targets an identity with a malformed ledger or a contract/ledger conflict, and a caller attempts to use Ancora to make that identity appear fresh.

**Out of Scope:**
- Automatic conflict resolution, destructive recovery, silently recreating lost artifacts, or silently converting a `RESUME/RECOVERY` request into `NEW`.

### REQ-116: Constrain Security, Privacy, and Operational Risk
**Description:** Vela usage and operational delegation must minimize authority and prevent untrusted evidence, paths, or credentials from changing behavior or causing destructive actions. Vela graph extraction, storage, and querying are local-only: graph/source data is handled by a process or graph endpoint on the selected host and never sent to a remote Vela service.

**Acceptance Criteria:**
- Vela extraction, graph storage, and every graph query use a local Vela executor or a local endpoint: a process on the selected host, a Unix-domain socket, or a loopback endpoint served on that host. A DNS, non-loopback, proxy, tunnel, or otherwise remote Vela endpoint is not local and is rejected before graph/source input is read or transmitted.
- For each Vela operation, bounded evidence records the local executor or endpoint classification, local graph-storage location, operation purpose, and the result or uncertainty. Instrumented operation evidence must show no attempted non-loopback Vela connection and no graph or source payload sent outside the selected host.
- Vela graph inputs/results are advisory evidence rather than executable instruction.
- All ledger paths, artifact pointers, worktree paths, branch/ref names, and commit identifiers are validated against the selected submission/repository before use.
- Worker evidence and durable metadata exclude credentials, access tokens, and secret values; remote credentials remain human-controlled.
- Cleanup is limited to the exact recorded worktree after all Archive preconditions and explicit consent, and no worker may delete a branch, remote ref, or unrecorded path.

**Edge Cases:**
- A graph result or ledger contains a traversal path, malformed ref, or command-like text.
- A worker receives more filesystem or Git permissions than its delegated responsibility requires.
- Evidence collection encounters a secret in tool output.
- A Vela configuration names an HTTPS endpoint, a forwarded local port backed by a remote service, or a proxy that would receive graph data.

**Out of Scope:**
- Managing credential storage, changing project-wide Git authorization, or trusting external graph providers.

### REQ-117: Provide Auditable, Bounded Observability
**Description:** Every delegated action and lifecycle decision must leave compact, correlated evidence sufficient for a human to understand what happened, why it happened, what is uncertain, and what can happen next without converting logs or memory into authority.

**Acceptance Criteria:**
- The ledger links each transition to the authorization context, actor role, relevant commit/base commit, bounded outcome, and evidence pointer.
- Preflight, exploration, TDD, quality, publication, cleanup, removal, and archive reports clearly label confidence, uncertainty, pass/fail/blocked/not-applicable state, and affected scope. Exploration reports include counts and purposes but no raw graph dump or raw tool log.
- A human can distinguish an approved contract commit, implementation commit, reviewed commit, and remotely published commit even when some are identical.
- Reports identify no-op/declined decisions and failed attempts rather than omitting them.

**Edge Cases:**
- Several retries produce multiple evidence reports for one authorized action.
- The same commit plays contract, implementation, review, and publication roles.
- A durable report is missing while the ledger still references it.

**Out of Scope:**
- Replacing durable artifacts with full transcript retention or using observability records as approval authority.

### REQ-118: Remove In-Scope Legacy Assets Through an Inventory and Semantic Verification
**Description:** Before deleting any legacy/v1 workflow or integration asset, the v2 implementation shall create and validate the removal inventory defined in Data and Artifacts. The inventory makes removal semantic: it distinguishes assets exclusively serving the replaced workflow/integration from mixed or unrelated assets, rather than inferring removability from a forbidden word, file name, or host label. Its reconciled candidate set covers every discovered relevant in-scope legacy/v1 workflow-code, test, fixture, skill/agent asset, configuration, installer/runtime integration, and compatibility-shim asset.

**Acceptance Criteria:**
- Before its first in-scope deletion, the inventory identifies every discovered in-scope candidate exactly once and records its `asset_category`, path/asset, semantic role, invocation or reachability evidence, `legacy_only`/`mixed`/`unrelated` classification, proposed disposition, v2 replacement or retention rationale, affected tests/fixtures, and post-removal evidence plan. It records discovery/reachability inputs and reconciles every discovered relevant candidate to an entry; its category set includes each applicable workflow-code, test, fixture, skill/agent asset, configuration, installer/runtime integration, and compatibility-shim candidate. The ledger references the completed inventory, its fingerprint, and its evidence.
- An asset is deleted only when the inventory classifies it `legacy_only` and its semantic evidence shows that it exclusively selects, installs, invokes, validates, or preserves the replaced workflow/integration, or when a listed obsolete in-scope integration is replaced by its OpenCode v2 equivalent. A `mixed` asset is first separated so its active unrelated behavior remains; an `unrelated` asset is retained.
- Post-removal evidence proves, through entrypoint/runtime-selection and reachability analysis or equivalent behavioral execution, that no reachable legacy execution path remains; that no obsolete installer/runtime target is selectable; and that no compatibility flag, v1 schema, legacy fixture, or test exercising only removed behavior remains live.
- The verification reconciles every inventory disposition to the repository state. It demonstrates that legacy tests/fixtures were removed or replaced and that tests covering v2 behavior are present and run as the applicable quality policy requires. A generic forbidden-string search may assist discovery but is never the sole deletion or post-removal proof.
- If inventory evidence is incomplete—including a missing required field, unreconciled candidate, absent applicable category, missing inventory fingerprint/reference, or uncertain classification—an asset's classification is uncertain, a mixed asset cannot be safely separated, or post-removal verification finds a remaining legacy path, deletion/acceptance stops before a deletion task starts with bounded evidence and a safe remediation recommendation; unrelated functionality is not deleted to force a clean result.

**Edge Cases:**
- One installer dispatches both a v1 workflow target and an unrelated product target.
- A legacy flag is parsed but normally unreachable, or a fixture is loaded only by an obsolete compatibility test.
- A non-OpenCode word occurs in an unrelated product document, integration, or test with no workflow/integration invocation path.

**Out of Scope:**
- Deleting unrelated product functionality, preserving v1 artifacts for migration, or treating text search as a complete reachability, ownership, or safety analysis.

### REQ-119: Support the V2 Workflow Through OpenCode Only
**Description:** OpenCode is the sole supported installation and runtime integration for Vela v2. The v2 installer/runtime surface shall provision and select only the OpenCode v2 workflow assets. Non-OpenCode v1 host integrations—including their skills/agent assets, configuration, installer/runtime dispatch, and compatibility shims—are intentionally excluded when they belong to the replaced workflow/integration scope and are removed under REQ-118.

**Acceptance Criteria:**
- A supported v2 installation selects and provisions the OpenCode v2 integration and exposes no alternative host-specific v2 workflow target, runtime selector, adapter, fallback, or compatibility mode.
- A request to select a non-OpenCode v2 workflow integration is rejected as unsupported before activation, without installing, selecting, or partially activating a non-OpenCode workflow path.
- Installation/runtime verification demonstrates the active OpenCode entrypoint and demonstrates that legacy/non-OpenCode in-scope selectors, installers, adapters, and compatibility flags cannot select or execute a workflow.
- The removal inventory explicitly identifies the in-scope non-OpenCode assets and any retained item with a semantic unrelated-function rationale; neither a host name nor a non-OpenCode word alone establishes removal scope.

**Edge Cases:**
- A stale configuration requests a retired host after OpenCode v2 is installed.
- An installer receives an unsupported host selector alongside valid OpenCode configuration.
- An unrelated product feature includes documentation or metadata for another host but has no Vela workflow installation or runtime role.

**Out of Scope:**
- Supporting, migrating, installing, adapting, or validating Vela v2 through Claude, Codex, or any other non-OpenCode host; removing unrelated product capabilities based on host terminology alone.

## Open Questions
- None. The binding decisions and the configuration, inventory, bounded-evidence, and lifecycle contracts above are sufficient to create a testable contract. Projects choose analyzer commands or adapters only by committing a valid `rotta-quality-policy/v1`; they may not introduce another lifecycle authority, a non-OpenCode v2 integration, or an unstated quality gate.

## Trade-offs
- A persistent cache worktree and durable ledger add operational state, but they make resume, exact-commit verification, and cleanup safer than an ephemeral sibling-worktree convention.
- Human consent before re-indexing and cleanup adds pauses, but prevents unexpected expensive extraction and destructive worktree removal.
- Treating Vela as advisory can leave structural uncertainty, but avoids making a local/index-dependent service an approval or behavior authority.
- Batches of up to three scenarios reduce orchestration overhead, while per-scenario Red-Green-Refactor evidence preserves diagnoseable TDD boundaries.
- Exact remote-ref verification can delay Archive when a branch advances, but prevents archiving a snapshot different from the one actually reviewed.
- Replacing rather than migrating legacy state intentionally strands old workflow artifacts, but removes ambiguous dual-authority behavior.
- A complete, language-agnostic quality-policy schema adds configuration work, but makes Review outcomes reproducible without coupling V2 to a vendor or language.
- Inventory and semantic verification make legacy removal slower than a string-based purge, but avoid deleting unrelated product behavior while proving obsolete paths are gone.
- OpenCode-only support narrows host reach, but eliminates unsupported installer/runtime selection and compatibility branches from the v2 workflow.

## Risk Level
critical — Justification: Vela v2 changes the project’s authority model, lifecycle ordering, commit/publication handling, worktree deletion boundary, and security posture for graph-derived evidence. A partial replacement, stale graph claim, unauthorized transition, or mistaken publication verification could start unapproved work, misstate review quality, or delete the only recorded worktree for an unarchived submission.
