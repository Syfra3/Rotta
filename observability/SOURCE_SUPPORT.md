# Latest lab integration: source support and limits

## OpenCode metadata source update

The repository now includes `/d/ai-cli-opencode`, separate from the older
companion below. The adapter and collector source cover model/agent usage,
completed tool outcomes, execution outcomes, newly observed session parent
relationships and compactions; model IDs are locally visible. Agent labels are
bounded and unknown tool names are not assumed to be MCP. Events and correlations
are volatile; missing, evicted or unscoped events can undercount. The location
filter and new event families still need real OpenCode v2.2.7 end-to-end validation.
The disposable collector test validates metric acceptance and label scrubbing,
not provider behavior. New source does not by itself reload the collector or
upgrade the installed plugin. A bind-mounted Grafana may reload dashboard source.
See README for upgrade recovery and counting limits. The comparison below describes
the older Pi/OpenCode companion, not the new dashboard.

Reference: the user-supplied `claude-code-telemetry-lab-20261109T213247Z-1-001`.
The three immutable JSON originals and their pinned hashes are in
`reference-dashboards/` and `test_reference_parity.py`.

## Claude: same presentation, explicit local wiring

Provisioned UIDs: `claude-code-telemetry-lab`, `claude-code-members-sessions`,
`claude-code-budget`. Panel order, grid, visualization settings, units, navigation,
other variables and PromQL/LogQL expressions are preserved. Enumerated exceptions:

* Prometheus datasource UID becomes `local-prometheus`.
* The source selector defaults to **all**: native supplies usage/cost, transcript
  supplies metadata without duplicating native usage/cost counters. `real-otel`
  matches `native|^$`. Explicit raw formatting preserves these fixed enum regexes;
  Grafana otherwise escapes the custom `.*` value and silently shows no data.
* The trace explorer uses the equivalent supported Tempo 2.5 TraceQL predicate;
  the reference's literal-string/nil regex alternative is unsupported there.
* Edit-counter expressions select the authoritative native counter, not the same
  decision twice from native and transcript sources. The session-cost log query
  also accepts sanitized native `api_cost` events.
* Context-peak panels (team 30, member 53 and overview C) query stored request logs.
  A last-value gauge after a one-shot history scan is not a range peak. The member
  overview uses mixed sources with the same local-profile label and unchanged units.
* Descriptions disclose privacy exclusions and unavailable attribution. Missing
  fields say **Not observed** (empty frames use Grafana **No data**), never an invented zero. The budget row visibly says
  **INCOMPLETE: seven-day data, not full-month totals**; it is not a monthly ledger.

`python3 observability/sync_reference_dashboards.py` deterministically regenerates
these complete local artifacts from the immutable references. Full-structure tests
allow only the enumerated wiring/privacy/coverage changes, not visual redesigns.

These are presentation parity with documented wiring differences, **not a promise
that every reference signal exists on every host**. `verify_lab.py` provisions and
queries all four dashboards on disposable cached-image backends, exercises actual
serialized Claude exporter bodies, tests a lost acknowledgement and unchanged
restart, and checks required fixture-supported panels have values. Empty unsupported
queries are not zero. It can render browser evidence with `--browser`.

| Claude family | Source and exact limitation |
| --- | --- |
| Usage/cache/cost | Native cumulative OTLP, canonical and reference-compatible pipelines; token unit maps to `claude_code_token_usage_tokens_total`. Native model-bearing metrics retain valid exact model IDs (otherwise unknown), source is main/subagent/unknown; cost is estimated, not billing. No derived usage/cost counters. Native `api_request` cost/duration metadata is projected into `api_cost` with a hashed session, never duplicated as another derived API request. |
| Model/source mix | Native usage/cost metrics retain valid exact IDs; derived context uses opus/sonnet/haiku families, not exact IDs. Absent source is **unknown**, not a guessed subagent. |
| Sessions/member | Unique transcript sessions, cumulative counters and salted log identities; a single local member is not an organization directory. Native member cost/cache aggregate under `local-user`; no emails or real member names. |
| Context | Input + cache-read + cache-creation tokens, reference family-window heuristic (including extended-window threshold), unknown window omitted. Drop = previous main request >=20k and next <half, **not proof of compaction**. Requests are globally chronological and assistant revisions coalesced before this inference. |
| Tools/edits | Typed tool-use/result ID matching, bounded names, observed transcript intervals and salted path hashes. Native `claude_code.code_edit_tool.decision` is authoritative for chart counts; transcript decisions stay separately attributed, not counted again. Success does not imply acceptance. No arguments/results/code. |
| Bash/Git | Fixed command classes and inferred commit/push/PR/branch-switch actions, anonymous branches; shell parsing is heuristic, not an authoritative Git audit. No command text or branch names. |
| API/errors | Observed request metadata, typed retry/failure records; latency is an **inferred transcript-record interval**, not measured provider latency. |
| Traces | Salted per-prompt trace IDs and linked interaction/LLM/tool spans with observed transcript intervals; not native provider-internal tracing. Incomplete prompts have only their observed interval. |
| MCP servers | Unavailable: unbounded raw server/tool names are reduced to `other`. No fabricated server counts. |
| Per-session cost | Native API events provide documented `cost_usd` per request, grouped by a hashed session, without installing the reference optional estimator. Native session hashes and salted transcript session IDs are separate namespaces, not a fabricated cross-source join. Transcript-only historical cost estimates remain unavailable. |
| Per-branch cost | Unavailable without a trustworthy cost/branch join; native totals are not assigned to guessed branches. Unattributed groups are not branch spend. |
| Monthly budget | Reference UI retained with prominent **incomplete** coverage disclosure. Seven-day retention cannot establish full-month spend or projection; no hidden longer-lived budget store. |

## Pi and OpenCode: supported equivalents, not invented parity

Provisioned companion UID: **`ai-cli-host-companions`**. It uses existing opt-in
adapters. Fake-event tests exercise their serialized outputs; backend fixtures
verify panel mappings but do not prove compatibility with a real provider run.
No new global host configuration is written by provisioning a dashboard.
Companion quantity/cost cards use reset-aware last-value differences like the lab,
not rate extrapolation: two observed runs stay **2**, not a fractional `increase()`
estimate. The explicitly named rate chart remains a per-second rate.

| Reference family | Pi | OpenCode |
| --- | --- | --- |
| Estimated cost/cache/tokens | Final assistant usage/cost; legacy `ai_cli_pi_tokens_total` unioned with canonical metrics without double counting | Completed-message candidate; canonical metrics |
| Model breakdown | Validated exact model usage/cost from existing extension | Unavailable from current candidate |
| Sessions/activity | Completed agent **runs**, not durable sessions | Unavailable |
| Tools/MCP | Completed category/status counts, not tool/server identities or edit permissions | Unavailable |
| Subagents | Final child-response counters exist, **not whole-session consumption**; excluded from aggregate main usage | Unavailable |
| Context/compaction | SDK supports additional metadata, but no verified window/compaction integration in this adapter | Unavailable |
| Logs/traces/edit decisions/Git/member attribution | Unavailable; not synthesized from usage | Unavailable |

Pi model labels preserve trimmed valid IDs (1–128 ASCII characters, alphanumeric first, then alphanumerics or `._-:/`), otherwise `unknown`. Custom/private IDs appear locally and distinct IDs create unbounded cardinality; historical `other` cannot be recovered.
Its sender retains one in-flight request and one
coalesced latest cumulative snapshot per series; failures back off and retry
while the process stays alive. Intermediate snapshots can be lost, and shutdown or
process exit during an outage loses unsent in-memory values (no durable Pi outbox).
The Pi 1.4.0 SDK documents the existing message/tool lifecycle; real Pi activation
is separate. OpenCode installed version 2.2.6 event semantics are **not yet
verified** against matching local SDK declarations. No speculative new event
families are enabled. The existing explicit-path Pi history reader is disconnected
and is not presented as completed live collection.

## Opt-in and recovery

Repository commands (not executed against real history during implementation):

* `make telemetry-claude-up` selects the optional local Loki/Tempo/collector overlay.
* `make telemetry-claude` runs the process-local Claude native opt-in wrapper.
* `CLAUDE_PROJECTS_DIR=<explicit directory> CLAUDE_STATE_DIR=<private state directory>
  make telemetry-claude-export` reads that chosen directory once. Re-run with the
  **same state** for subsequent snapshots; no implicit HOME/history scan or daemon.

Exporter state is protected by a single-writer lock, 0700 directory and 0600 salt/DB.
Only cumulative metric payloads, metadata receipts (keys only), checkpoints and pseudonymous IDs
are persisted; log/trace bodies are never staged in the database. On upgrade, legacy
pending metadata bodies are removed with SQLite secure-delete, VACUUM and WAL
checkpoint/truncation; this cannot erase external backups or filesystem snapshots. Transcript IDs use the protected state salt; native random session/request
identifiers are SHA-256 pseudonyms, while native user identity is a fixed local profile
label, never an email/account ID. Sanitized log lines include bounded event kinds and
pseudonymous event IDs because Loki otherwise collapses parallel equal-timestamp lines
when their bodies match, even if structured metadata differs. Counter contributions and transport acknowledgements are durable;
a lost acknowledgement can replay reconstructed logs/spans from still-available
source, but cannot increment counters again. Metadata retry is best effort only:
this one-shot CLI needs another invocation and its original transcript within the
12h input window; missing sources, stopped processes and aged receipts lose logs/traces.
No original transcripts are removed. Older checkpoint migration conservatively avoids recounting legacy events.

Limits are explicit backpressure: 64 MiB per scan, 10,000 files, 100,000 records,
accepted event IDs and tool correlations, 4,096 metric series and 800,000
contributions. No counted IDs are silently evicted. On capacity failure retain the
state, narrow the input or plan an explicit capacity adjustment; deleting/resetting
state can recount recent history. A partially written final JSONL line waits for
its newline. Oversized lines are discarded whole, never parsed by suffix. Input
records older than 12h or more than five minutes in the future are ignored.

Native cumulative counters carry an opaque 64-hex epoch/origin fingerprint before
identity/model scrubbing, so parallel SDK instances and models are summed rather
than overwriting each other. Derived counter epochs are likewise distinguished.
No raw instance/session/agent identifiers become metric labels. Active exporter
series expire after five minutes; historical samples keep the seven-day retention.
Origin hashes are pseudonyms, not anonymity or authorization. Clients omitting all
usable epoch/instance/session facts cannot be reliably distinguished; native
version compatibility remains a separate check.

Initial historical backfill does not reconstruct a past Prometheus billing ledger:
counter graphs describe collection-time observations and available baselines.
Request-log timelines and context maxima use stored event timestamps. Keep one
state for snapshots and avoid overlapping independently checkpointed history readers.

See `RETENTION.md` for healthy-service retention budgets, actual expiry probes and
outage/snapshot/WAL limitations. Telemetry stays local by configuration; no Affirm
or other remote exporters are configured. Loopback listeners and disabled vendor
checks are **not** a general outbound firewall. No real clients, private history,
installation, persistent stack restart, registry/image download, commit or push
are implied by passing source tests. Live activation needs separate authority.
