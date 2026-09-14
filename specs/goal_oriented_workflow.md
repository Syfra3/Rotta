# Goal-oriented Fast/Strict workflow

## Execution scope

User requested implementation of the discussed workflow repair and a new release after establishing `feat/goal-oriented-workflow` at baseline `80216635a9c4ade716d3856299e068f5aabc9934`. This artifact records that requested scope; it is not a fabricated native approval receipt or an instruction to migrate existing application approvals.

The proposal was developed as REQ-097–REQ-108. This compact execution contract carries the accepted direction into the clean worktree without replacing historical `specs/hard_spec.md` or creating child approval gates.

## Outcome and acceptance

| Requirement | Delivered behavior |
|---|---|
| REQ-097 | Fast by default; Strict based on material behavior/risk, not product keywords or file counts. |
| REQ-098 | One scoped execution approval with optional examples in the same packet; no recursive child approvals for internal decomposition. Material changes and existing explicit child gates remain respected. |
| REQ-099 | Stable, evidence-backed blocker/advisory/evidence-gap findings; reviewer preferences cannot invent gates or scope. |
| REQ-100 | Consolidated repair and delta check; unchanged accepted scope reopens only for new material evidence. |
| REQ-101 | One autonomous root-cause recovery/final delta check after ordinary remediation; no counter-only global block, false pass or renamed-budget reset. |
| REQ-102 | Exact compatible policy loading for installed hosts, no name-based mixed-bundle fallback, preserved custom configuration with visible diagnostics. |
| REQ-103 | Meaningful focused evidence before handoff; fresh sufficient evidence reused, affected interactions checked. |
| REQ-104 | Compact acceptance/finding/recovery continuity across handoffs and resumes; coordinate overlapping edits. |
| REQ-105 | User-facing outcome/progress first; detailed governance metrics remain compact evidence, not repeated ceremony. |
| REQ-106 | Canonical policy plus host installer generation and consumers updated together; no new runtime controller. |
| REQ-107 | Focused deterministic installation/consumer tests, policy regression checks, and explicitly labeled agent decision exercises. |
| REQ-108 | Distinguish initial/delta calls, dispatches and recovery; no fabricated speedup or telemetry gate. |

For an unchanged packet, the agent-turn review allowance is one initial review, one ordinary repair/delta check, and at most one root-cause recovery/final delta check. A genuine blocker remains a blocker. Continue independent authorized work and ask only for a material choice, required authority/evidence or a bounded further attempt with a new hypothesis. Specification-only requests do not authorize implementation.

## Boundaries

- Source changes: embedded core/roles, compatible host generation and owned-upgrade logic, CLI/TUI loading diagnostics, README and focused tests/evidence.
- Use explicit loading to resolve duplicate discovery without deleting other-host or user-owned skills. Exact legacy prompt upgrades need intact ownership evidence; custom loaders are preserved and diagnosed.
- OpenCode uses its effective absolute XDG bundle, Claude its installed home bundle, Codex its embedded single-file sections.
- This is agent-turn convergence policy, not runtime-enforced orchestration. Deterministic installer tests prove generation/ownership behavior; decision exercises do not prove live multi-agent execution.
- No model-routing change, new service, application behavior change, automatic index, live configuration installation, or retroactive application approval migration.
- Publication follows the existing GitHub PR/CI/Release Please process under the user's requested release outcome. Installation into a live coding-agent configuration remains separately requested.

## Behavioral acceptance cases

| Case | Input | Required decision |
|---|---|---|
| A01 | Low-risk fix, sufficient evidence, no findings | Fast, one initial review, complete without approval prompt. |
| A02 | Strict change with resolved material decisions | One combined approval packet; implementation request plus approval advances. |
| A03 | Approved scope split into three slices; P3 suggests child gates | Retain existing approval; advice creates no gate. |
| A04 | Two related initial blockers | Consolidated repair, focused closure and regression check. |
| A05 | Delta review suggests unrelated hardening | Advisory absent new material evidence; advance. |
| A06 | First correction leaves root cause unresolved | One automatic focused recovery and final delta check. |
| A07 | Final check blocks A; authorized B is independent | Preserve A, continue B, report A's concrete missing decision/evidence. |
| A08 | Rename/resume exhausted packet | Preserve counts/findings; no automatic fresh loop. |
| A09 | Repair changes approved monetary semantics | Narrow material amendment, preserve unaffected approval. |
| A10 | OpenCode sees conflicting skill copies | Read exact selected core/roles or report the precise conflict; no fallback/deletion. |
| A11 | Policy changes mid-session | Affected checkpoint/rebaseline and restart as needed; no approval/count reset. |
| A12 | Optional integrations fail, source evidence sufficient | Source fallback and progress with stated limitation. |
| A13 | Concurrent edit invalidates some evidence | Coordinate affected edits and refresh impacted evidence only. |
| A14 | Old application contract explicitly requires child approvals | Honor until explicitly amended. |
| A15 | Last delta finds a security regression | Hold affected work; never downgrade the defect to fit budget. |
| A16 | User requested only a spec | Deliver spec; no unsolicited implementation or review hierarchy. |

## Verification and release

Run focused installer/CLI/TUI tests, then repository `make verify` for release readiness. Obtain one independent Astra review of the coherent diff and acceptance; remediate material issues under the same scope. Record actual decision-exercise and test results without claiming live host performance. Publish through the repository's normal release automation and verify tag, binary assets and checksum availability.
