# Query monitoring

Query monitoring is an optional, default-off pipeline. The current adapter was
verified against Insights UI API on engine 479-e.4, not a stable Management API.
It reads query statistics and sanitized query details. It never submits or
cancels SQL, changes privileges, or exports SQL text.

## Configuration

Set `QUERY_ENABLED=true` and disable other pipelines explicitly for query-only
operation. Query-only does not require Management API credentials. YAML values
live under `monitoring.queries`; environment values take precedence. Secret-file
values use the existing secret loader; direct environment secrets override YAML
secret-file paths. Never put credentials or session cookies in YAML or arguments.

| YAML field | Environment | Default / meaning |
|---|---|---|
| enabled | QUERY_ENABLED | false |
| events_enabled | QUERY_EVENTS_ENABLED | false; Kafka query detail publishing |
| base_url | QUERY_BASE_URL | required HTTPS Insights origin |
| auth_url | QUERY_AUTH_URL | required HTTPS identity-provider origin |
| realm | QUERY_REALM | required explicit realm |
| role | QUERY_ROLE | required explicit existing role with all-query visibility |
| username_file | QUERY_USERNAME_FILE | independent username file; QUERY_USERNAME env alternative |
| password_file | QUERY_PASSWORD_FILE | independent password file; QUERY_PASSWORD env alternative |
| ca_file | QUERY_CA_FILE | system roots plus Insights CA |
| auth_ca_file | QUERY_AUTH_CA_FILE | system roots plus identity-provider CA |
| insecure_skip_verify | QUERY_TLS_INSECURE_SKIP_VERIFY | false; diagnostic only, also requires ALLOW_INSECURE_TLS=true |
| interval | QUERY_INTERVAL | 30s |
| request_timeout | QUERY_REQUEST_TIMEOUT | 5s |
| cycle_timeout | QUERY_CYCLE_TIMEOUT | 20s |
| stale_after | QUERY_STALE_AFTER | 90s |
| retry_max | QUERY_RETRY_MAX | 2; bounded GET retries |
| response_max_bytes | QUERY_RESPONSE_MAX_BYTES | 16777216; ceiling 67108864 |
| max_history_records | QUERY_MAX_HISTORY_RECORDS | 1000; maximum accepted list size, not server pagination |
| detail_max_per_cycle | QUERY_DETAIL_MAX_PER_CYCLE | 100 |
| detail_concurrency | QUERY_DETAIL_CONCURRENCY | 4; maximum32 |
| checkpoint_retention | QUERY_CHECKPOINT_RETENTION | 720h |
| checkpoint_max_records | QUERY_CHECKPOINT_MAX_RECORDS | 100000 |
| outbox_max_bytes | QUERY_OUTBOX_MAX_BYTES | 268435456; payload budget |
| outbox_max_events | QUERY_OUTBOX_MAX_EVENTS | 10000 |
| kafka_topic | QUERY_KAFKA_TOPIC | required when events_enabled=true; distinct from other topics |

`DDAE_SOURCE_INSTANCE` identifies the monitored instance; `STATE_DIR` must be an
absolute persistent directory. The query pipeline owns `query-events.db` even
when Kafka publishing is off, because deduplication and aggregates survive
restarts. Do not run two exporters on the same state file. The source identity
is pinned in the database; use a separate directory for a different instance.
Kafka transport uses existing KAFKA_BROKERS/TLS/SASL settings. Queries have an
independent producer/topic and do not alter alert or serviceability event schemas.

The engine cookie session is kept only in memory. An expired session is renewed
once after401;403 is a permission failure. All business calls are fixed GET
routes. Password submission is confined to the configured identity-provider realm.
MFA, additional login forms and unknown redirects fail closed. Updating SSO or
Insights may require adapter changes. A Management API client token is not an
Insights authentication substitute.

## Outputs and interpretation

Prometheus reports `ddae_queries_running`, `ddae_queries_queued`, the upstream
sample timestamp and collection/scope status. Gauges disappear when their sample
expires; empty or failed responses never become zero queries. `ddae_query_history_complete`
is0 because this API's full history coverage is unproven. A successful poll is
not proof of complete audit coverage.

`ddae_queries_observed_completed_total{state}` and the elapsed/execution/queued
histograms describe deduplicated terminal queries that this exporter observed.
They are not cluster-wide totals or complete workload latency distributions.
Histograms are in seconds; source detail durations are integer milliseconds.
CPU time is a per-query detail and is not elapsed wall-clock time.
No query ID, user, application source or SQL becomes a Prometheus label.

Kafka schema1.0 headers identify record kind `query`; JSON contains only:
source_instance, query_id, user, optional source, normalized state,
submitted_at, optional completed_at, observed_at, and optional
elapsed_seconds/queued_seconds/execution_seconds/cpu_seconds.
States include queued, running, planning, starting, dispatching,
waiting_for_resources, finishing, finished, failed, canceled and unknown.
Missing optional values are omitted. Unknown is never treated as terminal.

Stable binary record key is SHA256(source_instance + NUL + query_id). Kafka
publishing is at-least-once. Consumers should key their idempotent upsert by
source_instance/query_id and avoid overwriting newer observed_at with older
records. Same-query updates are ordered within a publisher; duplicate delivery
is possible across broker acknowledgement/crash. Raw SQL, IP, principal,
session properties, query plans and failure text are excluded.

## Limits, recovery and deployment

History may be truncated upstream, short-lived queries may never be observed,
and the bounded detail budget delays processing. No list disappearance is
interpreted as completion. Records older than the persistent retention floor
are not counted again; missing history remains unknown. Outbox/checkpoint capacity
or publish failures degrade readiness instead of silently dropping fresh records.
Query metrics can remain available while detail/publish health is degraded.
`/healthz` remains process liveness; `/readyz` reflects enabled pipeline readiness.

Mount query credential and CA files read-only. Permit egress from Kubernetes or
VM to both configured HTTPS origins, DNS and, when enabled, the query Kafka
brokers. Existing Management-only NetworkPolicies must be updated with the actual
Insights/IdP destinations; do not use broad allow-all egress. Standard Kubernetes
NetworkPolicy needs concrete IP/CIDR or pod selectors; hostnames alone are not rules.

Rollback by disabling QUERY_ENABLED and using the previous exporter. Retain the
query state directory; older exporters do not read it. Unknown future state schema
is rejected. Do not delete state merely to fix a transient login or broker error.
Production release still requires trusted TLS, authorized integration and full
Harness verification; diagnostic TLS-bypass observations are functional evidence.
## Complete configuration references

`deploy/systemd/config.example.yaml` and `deploy/kubernetes/configmap.yaml`
include every supported YAML setting, including all query limits and optional
Kafka mTLS certificate/key paths. The query-only example includes every query
setting but intentionally omits inactive Management and Kafka configuration.
Examples contain placeholders, not working credentials. Environment variables
still override YAML; enabling every feature is an explicit operator choice.

For systemd queries, provision the two separate query credentials and uncomment
both `LoadCredential` and both `Environment` lines in the unit. For Kubernetes,
create the `ddae-exporter-query` Secret with `username` and `password` keys before
enabling queries; its optional mount only permits startup while queries are off.
Add `insights-ca.pem` and `identity-ca.pem` to the mounted trust Secret, and permit
the actual Insights/IdP destinations in NetworkPolicy. To use Kafka mTLS, mount
both client certificate and private key and set both corresponding paths.

Set `monitoring.queries.enabled: true` for query Prometheus metrics, including
queue counts and observed queue duration. Set `events_enabled: true` as well for
Kafka query details, and configure the shared Kafka connection. Alerts, logs
and queries use separate topics. There is no separate query-queue switch.
