# Local retention and expiry verification

The optional Claude overlay uses **12-hour** log/trace retention, deliberately
shorter than the approved 24-hour maximum to leave normal cleanup margins.
Prometheus keeps seven days. This is not the lab's long-retention/monthly ledger.

## Effective settings (cached Loki 3.1 / Tempo 2.5)

| Store | Retention and cleanup settings |
| --- | --- |
| Loki | retention, old-sample admission and query lookback 12h; maximum head chunk age 2h; idle chunks 10m; flush check 1m; WAL checkpoint 5m; compaction and retention application 10m; delete delay 10m; downloaded index cache TTL 1h/resync 1m |
| Tempo | block retention 12h; maximum head block duration 10m; completed ingester block timeout 10m; compacted block retention 10m; compaction cycle 1m; blocklist polling 1m |
| Prometheus | `--storage.tsdb.retention.time=7d` |

The configured healthy-service budgets leave substantial margin below 24h. A
nominal retention value alone is **not** a physical-erasure deadline: compactor
startup, flushing, index shipping, WAL replay, caches and processing queues matter.
Setting `23h` with Loki's former default two-hour deletion delay was insufficient;
that configuration is no longer used.

## What was actually tested

All probes use cached images, unique disposable networks/containers and synthetic
records; no private history, live stack, host clock changes or downloads.

* `python3 observability/verify_retention.py`: an aged log is first accepted and
  queryable with admission/retention disabled **only in the test fixture**. It is
  flushed to a known on-disk chunk and its TSDB index sealed. A restart enables
  scaled retention/cleanup and the matching read window. The real compactor removes
  that exact physical chunk and the query becomes empty. This tests both read
  expiry and actual chunk deletion, not rejection of an event that was never stored.
* `python3 observability/verify_tempo_expiry.py`: an aged trace is queryable (200)
  and four block files, including Parquet, are observed before retention changes.
  Real compaction with shortened fixture intervals removes the known block files;
  trace lookup becomes 404 and the block directory is empty.
* A separate production-window Loki admission probe accepted a current sample and
  rejected a 13-hour-old sample. Both production configurations were validated by
  their actual pinned binaries.

Fixture windows/intervals are shortened to exercise the same backend algorithms
without waiting twelve hours; this is not an elapsed 24-hour production soak test.
Before/after evidence and earlier failed attempts remain in the work record's
`.rotta/evidence/latest-lab-parity/` references. Each final probe verifies removal
of its own containers and network. Temporary test directories are also cleaned.

## Limits and recovery

These tests establish deletion of observed chunks/blocks and configured read
expiry. They do **not** establish forensic erasure of every WAL/index/cache byte,
backups, filesystem snapshots or external copies. A stopped host, failed compactor,
unbounded queue, or restored snapshot can retain bytes beyond any nominal budget;
no unconditional wall-clock physical deletion guarantee is claimed during faults.
The read window is not evidence that every physical copy has been erased.

Keep compaction/storage healthy and investigate failed retention/flush jobs before
relying on expiry. After a prolonged outage or restore, recheck store/WAL/cache
state and let retention drain before resuming collection. Do not silently delete
existing volumes or reset exporter checkpoints to repair a retention failure.
Any real cleanup/reload/activation remains a separately scoped operator action.
The exporter checkpoint is dedicated, access-restricted correlation/dedup state;
resetting it can recount recent records. It is not an extra monthly cost ledger.
