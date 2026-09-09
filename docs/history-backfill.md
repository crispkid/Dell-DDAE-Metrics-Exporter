# Bounded history backfill

DDAE-10 adds optional background recovery for serviceability logs and Insights
queries. Both switches default to false. Enable the corresponding parent pipeline
first. Complete YAML examples include both sections:
`monitoring.serviceability_logs.backfill` and `monitoring.queries.backfill`.

```yaml
backfill:
  enabled: false
  lookback: 24h
  overlap: 2m
  interval: 30s
  cycle_timeout: 20s
  rescan_interval: 1h
  max_pages_per_cycle: 4
  detail_max_per_cycle: 25
  detail_concurrency: 2
  max_pending_records: 10000
```

Environment overrides use `SERVICEABILITY_LOG_BACKFILL_` or `QUERY_BACKFILL_`
plus the uppercase leaf key. Environment settings take precedence over YAML.
Changing only the polling interval preserves progress. Changing overlap rejects the
old window fingerprint: restore its original value or provision a separate state
directory deliberately; never silently reuse progress with different bounds. Changing lookback explicitly
starts a new bounded sweep; previously queued events and dedup checkpoints remain.
The lookback range is 1–720 hours and cannot exceed the pipeline retention;
overlap is 1 second–1 hour and cannot exceed lookback. Interval is 5 seconds–1 hour.
Require client request_timeout < cycle_timeout < interval and
interval <= rescan_interval <= lookback. Pages: 1–32; details: 1–1000;
concurrency: 1–8 and <= detail budget; pending records: 1000–100000.
Query max_history_records must be >=1000; it does not increase the server limit.

The worker first walks a fixed recent window, then overlaps incremental windows.
Periodic rolling-lookback rescans can discover late arrivals still inside that
window. Logs use inclusive update-time filters; Insights uses typed start/end dates.
Full pages are split by time. Duplicate boundary IDs use the existing durable
store's deduplication. A full one-second interval blocks progress rather than
silently dropping records. A completed window is not a transactional snapshot,
proof of permanent audit completeness, or recovery of already deleted history.
`ddae_query_history_complete` remains 0. Filtered log windows never drive absence
reconciliation or set global log-list completeness.

## Load and state

Foreground and backfill have separate request/detail budgets and query sessions.
Per cycle, the combined detail ceiling is foreground detail_max_per_cycle plus
backfill detail_max_per_cycle; combined concurrency is the sum of the two values.
Each backfill cycle permits at most max_pages_per_cycle logical list GETs, plus
query scope checks for those pages. Authentication and retries also consume requests,
bounded by the existing client retry settings and the backfill cycle deadline.
Response byte limits remain the parent pipeline limits. Scrapes read cached metrics.

`STATE_DIR/history-backfill.db` schema 1 contains source identity hashes, time
windows and pending IDs/markers, never SQL/user/raw responses. It is separate from
existing event databases. Combined logical progress is capped at 32 MiB and pending
windows at 4096 per pipeline. Each event-store transaction commits before its
progress acknowledgement; crash replay can repeat delivery but cannot advance past
an uncommitted event. Terminal query aggregates share foreground atomic deduplication.
Kafka uses existing topics and event schemas; query events remain independently
optional. Backfill does not configure or validate a Kafka destination.

## Metrics and recovery

`ddae_history_backfill_` gauges: enabled, success, pending_windows, pending_records,
last_completed_timestamp_seconds, incomplete, blocked. Only enabled pipelines appear;
labels are limited to `pipeline=queries|serviceability_logs` (at most 14 series).
Never-completed timestamps are omitted. These measure bounded recovery only.

Healthy pending work can be ready; a failed cycle, stale worker (>3 intervals),
saturated timestamp, retention gap, capacity or state failure makes recovery unready.
Logs with a valid truncated foreground list can be ready when both foreground and
backfill are healthy. `/healthz` remains process liveness. Overview metrics continue
independently of backfill failures.

For an HTTP/detail failure, restore source access and let retained IDs retry. For
capacity, restore event delivery or increase the approved configured capacity.
For an expired or saturated window, review the gap and change lookback deliberately
to begin a new bounded sweep; it does not repair excluded history. An unavailable
record (including 404) stays pending. Restore corrupt progress from a verified backup;
never delete or auto-reset the file to hide a failure. Source mismatch requires the
original source or a separately provisioned state directory. Changing interval does
not reset checkpoints. To roll back, stop the exporter, disable both backfill
switches and retain all state files before starting the previous version. The old
version ignores the separate progress DB.

## Verification limits

Synthetic acceptance tests cover bounded splitting, 720/1474 record datasets,
replay, stale markers, corruption, capacity, cancellation and readiness. Demo tests
are separately reported in `plans/DDAE-10.md`. Diagnostic TLS bypass does not prove
trusted TLS operation. Real Kafka/OpenSearch, full Harness and independent review
remain separate handoff gates; no production release is claimed.
