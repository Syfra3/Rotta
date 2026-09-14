# Goal-oriented workflow verification

## Scope and evidence limits

Acceptance: `specs/goal_oriented_workflow.md` (REQ-097–REQ-108, A01–A16).
Baseline: `80216635a9c4ade716d3856299e068f5aabc9934`.

This change repairs generated host loading and agent-turn workflow instructions. It does not introduce a runtime-enforced controller, prove live host permission/overlay behavior, or claim a measured percentage speedup.

## Deterministic checks

- Focused `go test ./internal/installer ./cmd/rotta ./internal/tui`: passed (61 top-level tests reported by the focused runner).
- `GOTOOLCHAIN=go1.25.0 make verify`: passed; formatting, golangci-lint (0 issues), race-enabled suite (621 tests including subtests), and build.
- `git diff --check`: passed.
- Initial verification under this machine's experimental Go 1.27 failed in the pinned linter's export-data importer, before identifying a source defect. Verification used the project's Go 1.25 toolchain without changing repository toolchain requirements or suppressing checks.

Regression coverage includes absolute source paths with default/custom XDG roots and spaces; duplicate skills preserved; all generated roles; owned legacy upgrades and rollback; no-op stability; custom-loader preservation and visible CLI/TUI warnings; Claude and Codex source binding; generated core convergence policy and absence of previously contradictory gate text.

## Independent review

Model: `openai/gpt-6-astra`, verified from assistant message metadata.
Review session: `ses_f61df750bffe1Cy0b0xlJ8NRkk`.

Result: **no blockers** across the coherent source/installer/consumer/test diff. The reviewer reused focused test evidence; the full verification result above completed separately. Optional advice was to freeze historical asset fixtures rather than render current unbound assets in migration tests. This remains advisory: exact legacy prompt matching, intact manifest ownership, rollback and idempotence are exercised.

## Retained agent decision exercise

Model: `openai/gpt-6-astra`, verified from assistant message metadata.
Exercise session: `ses_f61df7500ffebf6ntp7SKQiMVC`.

Method: an independent agent read only the new core/orchestrator/review policies and sixteen scenario inputs, without reading the expected-outcome table or tests. It returned routing decisions without executing implementation or operations. The following is a summary of its observed answers, **a simulation, not a live multi-agent workflow or timing benchmark**.

| Case | Observed decision | Acceptance |
|---|---|---|
| A01 | Cosmetic finance change is Fast; passing initial review completes without another prompt. | Matched |
| A02 | Changed payment semantics use one combined Strict approval packet before implementation. | Matched |
| A03 | Three internal slices retain approval; P3 child-gate preference remains advisory. | Matched |
| A04 | Consolidate related blockers, repair, then delta-check closure and regressions. | Matched |
| A05 | Unrelated optional hardening does not reopen accepted scope. | Matched |
| A06 | One automatic root-cause recovery and final delta check; ask only if a material boundary changes. | Matched |
| A07 | Preserve blocked A, continue independent approved B, report A's concrete next decision. | Matched |
| A08 | Renaming/resuming retains exhausted counts and finding history. | Matched |
| A09 | Changed monetary semantics require a narrow amended approval. | Matched |
| A10 | Reject mixed provenance, load exact parent bundle and rebaseline affected work; no fallback. | Matched |
| A11 | Policy drift holds affected work for rebaseline, preserving applicable approval/evidence and counts. | Matched |
| A12 | Optional integration failures use source fallback without new gates. | Matched |
| A13 | Coordinate shared edits and refresh only impacted evidence. | Matched |
| A14 | Existing explicit child gates remain binding until amended. | Matched |
| A15 | A last-check security regression stays blocking, even when the allowance is exhausted. | Matched |
| A16 | A specification request ends with the specification, not unsolicited implementation/review gates. | Matched |

## Operational follow-up

Release through normal PR/CI/Release Please automation. Installing the updated binary alone does not rewrite an active coding agent's configuration: run the selected-host installer, inspect source-loading diagnostics, restart the host and verify actual policy paths. Existing custom loaders are intentionally preserved and may need user reconciliation. Other-host skills are not deleted.
