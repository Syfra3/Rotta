# PR #49 policy/simplification Strict contract

**Baseline:** `/home/geen/Documents/personal/Rotta-pr49-policy`, branch `feature/token-efficient-workflow`, clean at `2ccbf4f018542efd08f7634b291cfcf61bcf6533`.

**Approved scope:** policy/routing eligibility layer only. One feature-contract decision covers the explicitly enumerated scenarios below and authorizes edits to `assets/core/rotta-core.md`, relevant `assets/agents/rotta-*.md`, affected workflow specifications, rendering/policy tests, remediation lifecycle implementation/tests, and one final managed-asset hash refresh.

**Approval:** `Approve exact contract`

## Requirements

- Advisory Fast/Strict child budget: 2; up to 4 only for one isolated remediation plus one fresh final review; deep maximum 4; stop and report on exhaustion.
- Durable handoffs only for Strict approval, resume/recovery, an explicit operation, or isolated remediation. Ordinary in-session implementation-to-review evidence is ephemeral.
- One manifest/binding refresh per feature unless scope, baseline, or policy changes; refresh managed-asset hashes once at final verification.
- Replace two automatic remediation cycles with one remediation followed by a fresh review; no generic continuations.
- Preserve handoff integrity; only policy/routing eligibility changes.

## Exclusions

No host controller/adapter or claimed native enforcement; no Vela, RTK, installer, or TUI changes; no commit, push, rebase, merge, or PR publication. No changes beyond the authorized policy-layer assets, workflow specifications, rendering/policy tests, remediation lifecycle implementation/tests, and one final managed-asset hash refresh.

## Acceptance and verification

Approved scenarios:

1. Feature-level approval and advisory child-session budgets apply, with no generic continuation.
2. Durable handoffs are selective; ordinary implementation-to-review evidence is ephemeral.
3. One feature-level binding/manifest refresh and one final managed-asset hash refresh occur.
4. One automatic remediation is followed by a fresh review lifecycle.

Implementation verification: `go test ./internal/workflow ./internal/installer`, `go vet ./internal/workflow ./internal/installer`, and `git diff --check`.
