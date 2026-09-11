# DDAE Exporter Runbook

This runbook covers the container, Kubernetes and VM/systemd deployment
profiles for the DDAE-5 local implementation. It is not evidence that any profile has run in an
authorized environment. Dell DDAE 1.5.0, Kafka, OpenSearch and alert-delivery
validation remain release blockers.

## Prerequisites

- When alerts or Serviceability Logs are enabled, use one exporter instance and
  one writable state directory per DDAE target. The exporter maintains separate
  `state.db` and `serviceability-logs.db` files. Never share `state.dir` between
  concurrently running processes.
- A dedicated least-privilege DDAE identity authorized only for the nine GET
  operations listed in the root `README.md`. By default these use `/ping` and
  `/v1/*`; the token request remains the sole fixed POST.
- An isolated alert Kafka topic and a separate Serviceability Logs topic whose
  consumers perform idempotent OpenSearch upsert by Kafka record key. Brokers
  must support TLS and `acks=all`. A hard
  publish timeout can leave broker acceptance uncertain; the exporter retains
  that outbox record and may replay the same key.
- PEM CA roots for the DDAE and Kafka identities. Certificate and hostname
  verification are enabled by default. Guarded insecure mode requires both a
  global acknowledgement and one target flag, is diagnostic only, and cannot
  produce release or production-support evidence.
- A private Prometheus path protected by an external mTLS reverse proxy or
  service mesh before using a non-loopback listener.
- Capacity for both enabled outbox byte limits plus database overhead on a local or
  ReadWriteOnce filesystem that supports fsync and file locks.

## Configuration and secrets

The root `README.md` summarizes the important settings, precedence and path
compatibility. The complete committed YAML example is
`deploy/systemd/config.example.yaml`. Select the file with `--config`
(preferred) or `DDAE_EXPORTER_CONFIG_FILE`. Supply DDAE credentials and any
Kafka password at runtime through approved file paths. Direct secret
environment values remain a compatibility interface; YAML never accepts
plaintext secret values.
Do not place secret values in ConfigMaps, environment files, command arguments,
container image layers, logs or Harness evidence.

Kafka SASL credentials apply whenever alerts, Serviceability Logs or query events
are enabled, including query-only events. Set `kafka.sasl.mechanism` to `PLAIN`,
`SCRAM-SHA-256` or `SCRAM-SHA-512`, supply `kafka.sasl.username` and a readable
`kafka.sasl.password_file`, and retain the broker's TLS configuration. Environment
overrides are `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME` and
`KAFKA_SASL_PASSWORD_FILE` (or the direct password alternative, not both).
Missing/invalid required credentials fail configuration loading before startup;
correct the named setting and restart. Query metrics-only/resources-only profiles
do not read Kafka credentials. After replacing a binary or changing credentials,
restart and verify readiness plus query publish/backlog metrics as described in
the README. Preserve all outbox files during authentication recovery.

Set `ddae.paths.ping_prefix` and `ddae.paths.api_prefix` for the actual gateway
before rollout. Omitted settings use an empty Ping prefix and `/v1` API prefix,
producing `/ping` and `/v1/*`. A gateway that still requires v1.0.0-rc2 routes
uses `/rest/v1` for both settings. The Dell PDF Ping-operation form uses
`ping_prefix: /rest` with `api_prefix: /rest/v1`. Prefix selection is
deterministic: the exporter does not probe or retry an alternate namespace.

Create Kubernetes Secrets out of band with these keys:

- `ddae-exporter-credentials`: `username`, `password`, `client-secret`;
- `ddae-exporter-kafka`: `password`;
- `ddae-exporter-trust`: `ddae-ca.pem`, `kafka-ca.pem`.

Replace every `.invalid` endpoint in `deploy/kubernetes/configmap.yaml`, add
site-specific NetworkPolicies, and replace the image tag with an approved
immutable image digest. Apply the ConfigMap, workload and site policies only in
the authorized namespace. The committed NetworkPolicy is default-deny by
design; it must not be removed merely to make connectivity work.

For systemd, create the non-login `ddae-exporter` user, install the static
binary at `/usr/local/bin/ddae-exporter`, copy
`deploy/systemd/config.example.yaml` to `/etc/ddae-exporter/config.yaml`, and
install the four credential source files
mode `0400`. Install the unit, run `systemctl daemon-reload`, then enable it.
Keep the listener on loopback and place the Prometheus route behind an mTLS
reverse proxy. In resource-only mode, disable alerts in YAML and remove the
Kafka credential lines in a site-specific unit override. Preserve an existing
`state.db` while alerts are disabled and preserve `serviceability-logs.db`
while Serviceability Logs are disabled.

## Startup and normal operation

`GET /healthz` reports process liveness. `GET /readyz` evaluates only enabled
pipelines: current resource snapshot for resources, healthy auth/list/detail/
state/outbox for alerts, healthy complete list/detail/dedicated state for
Serviceability Logs, and their conjunction when combined. `/metrics` never
triggers DDAE requests. Its handler admits at most five concurrent requests and
returns a bounded service-unavailable response for excess work; each admitted
handler has a nine-second timeout. `/healthz` and `/readyz` remain outside that
limit and stay responsive during scrape overload.

Monitor at minimum:

- `ddae_monitoring_enabled{pipeline="resources|alerts|serviceability_logs"}` to distinguish
  intentional disablement from failure;
- `ddae_up`, `ddae_snapshot_age_seconds` and
  `ddae_collector_success{collector=...}`;
- `ddae_alert_list_complete`, `ddae_alert_pipeline_ready` and
  `ddae_alert_detail_deferred`;
- `ddae_kafka_publish_success`, `ddae_kafka_buffered_events` and
  `ddae_kafka_events_failed_total{reason=...}`.
- `ddae_serviceability_log_list_complete`,
  `ddae_serviceability_log_pipeline_ready`,
  `ddae_serviceability_log_detail_deferred`,
  `ddae_serviceability_log_kafka_publish_success` and
  `ddae_serviceability_log_buffered_records`.

Alert and Serviceability Log payloads are confidential and appear only in their
approved Kafka topics and protected durable state. They must not be copied into
incident tickets or command output without the applicable data-handling
authorization.

## Troubleshooting

| Symptom | Safe check | Recovery |
|---|---|---|
| Authentication failures | Confirm credential files exist and are readable; inspect only the bounded `auth` class | Correct/rotate the dedicated identity; never print token or password values |
| TLS failures | Check CA mount and hostname against the certificate | Replace the approved CA/certificate; guarded insecure diagnosis requires explicit approval and its output is never release evidence |
| Management API returns 404 | Compare the configured Ping/API prefixes with the gateway's documented route namespace and sanitized ingress/service observations | Correct `ddae.paths.ping_prefix` or `ddae.paths.api_prefix` and restart; do not add fallback routes, change fixed suffixes or treat a 401 as response-schema proof |
| Node collector reports a decode failure | Confirm the response is either a Dell 1.5.0 `results` envelope or the supported legacy array; inspect only sanitized field types and names | Correct the API/gateway response contract; do not log the raw node inventory or coerce malformed values to zero |
| One collector fails | Inspect its bounded failure class and snapshot age | Validate only its fixed GET route/shape; do not add fallback routes or generic decoding |
| Alert list incomplete | Compare the sanitized count relationship in the authorized environment | Stop release, confirm enumeration behavior, and amend the specification if pagination is required |
| Serviceability Log list incomplete | Observe the fixed completeness/deferred metrics and sanitized counts in an authorized environment | Keep the logs pipeline not ready, retain safe IDs for bounded processing, and confirm Dell list semantics before release |
| Kafka unavailable or publish timeout | Watch buffered count, fixed `kafka_timeout` failures and disk capacity | Restore broker/TLS/auth; retained events replay automatically in order. Do not delete an uncertain record; the consumer's idempotent key handles a possible duplicate |
| Outbox full | Stop new alert acceptance, preserve `state.db`, and restore Kafka | Drain the existing outbox; do not delete, truncate or replace the database unless an incident owner explicitly accepts data loss |
| Logs outbox full | Preserve `serviceability-logs.db` and its dedicated topic path | Restore Kafka and drain log records; alert state/capacity remains independent |
| State database will not open | Check single-process ownership, filesystem locks, permissions and capacity; retain the bounded startup failure class | Keep the original database unchanged and escalate for controlled offline repair; restore a protected copy only when its event-loss impact is understood, and do not start statelessly |

Logs expose component and bounded failure class, not raw server errors. A
failure that requires response-body inspection must be reproduced with
authorized, separately controlled diagnostic tooling.

Node decoding supports the documented `results` envelope, integer CPU,
`ephemeralStorage` and object conditions, plus the prior bare-array, quantity
string, `ephemeral-storage` and condition-array forms. Serviceability lists
remain indexes for their `/{id}` detail operations. If `totalRecords` exceeds
the unique valid returned IDs, do not treat the list as complete and do not add
undocumented pagination parameters.

## Upgrade, rollback and recovery

1. Back up the state volume while the exporter is stopped, or use an atomic
   volume snapshot supported by the platform.
2. Record the binary/image digest, configuration revision and event schema.
   For an RC2 gateway, add `ping_prefix: /rest/v1` and
   `api_prefix: /rest/v1` before starting the DDAE-5 binary.
3. Stop the old instance and allow its configured shutdown grace period.
4. Start exactly one new instance against the existing state volume.
5. Confirm liveness, readiness, snapshot age and Kafka buffered count.

Keep protected copies of both state files when their pipelines are enabled.
The new Serviceability Logs pipeline defaults to disabled; provision its
dedicated topic/ACL and capacity before first enable. Disabling only
`monitoring.serviceability_logs.enabled` is the non-destructive rollback and
retains queued log records for later replay.

On first DDAE-3 open, a valid unversioned database is validated and receives
the current schema marker transactionally. Current and legacy databases have
their derivable event-count and byte-count caches recomputed from validated
records. Unsupported future versions and primary/cross-record corruption fail
closed without deleting records or checkpoints. Keep the stopped-writer backup
until normal Kafka draining and alert readiness are confirmed.

Use Kubernetes `Recreate`, not a rolling overlap, when alerts are enabled
because bbolt enforces one writer. For rollback, stop the new process and
first restore both prefixes to the previous route namespace. If configuration
rollback is insufficient, restore the previous immutable binary/image and its
compatible configuration after a clean stop. A DDAE-1 binary ignores the new
YAML, so restore its environment settings explicitly. The DDAE-3 schema marker is extra metadata that DDAE-2
ignores, so the current database remains backward-readable; preserve all
outbox records and stop the only writer before either upgrade or rollback.
`serviceability-logs.db` uses an independent schema marker, primary-integrity
scan and derived-counter repair; corruption or capacity in that file cannot
modify or consume alert state.

## Verification status

Run local tests and build gates with `./scripts/stage-lint.sh`,
`./scripts/stage-test.sh`, `./scripts/stage-coverage.sh`, and
`./scripts/build.sh`. The `integration` and `e2e` stages intentionally return
blocked until Peter Chu authorizes the required non-production environments. A
blocked stage is not a pass, waiver or release approval.

The self-hosted `authorized-integration` CI job accepts only an explicit
`workflow_dispatch` opt-in on `refs/heads/main`; it checks out the dispatched
commit's exact SHA without persisting GitHub credentials. Repository
environment protection and runner lifecycle remain external controls and need
separate authorized attestation.

An authorized E2E runner must expose verified-HTTPS endpoints through these
nine non-secret runtime variables before setting `DDAE_E2E_ENABLED=1`:

```text
DDAE_E2E_CONTAINER_RESOURCE_URL
DDAE_E2E_CONTAINER_ALERT_URL
DDAE_E2E_CONTAINER_DUAL_URL
DDAE_E2E_KUBERNETES_RESOURCE_URL
DDAE_E2E_KUBERNETES_ALERT_URL
DDAE_E2E_KUBERNETES_DUAL_URL
DDAE_E2E_SYSTEMD_RESOURCE_URL
DDAE_E2E_SYSTEMD_ALERT_URL
DDAE_E2E_SYSTEMD_DUAL_URL
```

Use `DDAE_E2E_CA_FILE` and the optional paired
`DDAE_E2E_CLIENT_CERT_FILE`/`DDAE_E2E_CLIENT_KEY_FILE` for the external mTLS
boundary. The E2E client deliberately has no insecure option; evidence from an
exporter instance running with either target's insecure TLS mode is ineligible.

## Query monitoring operations

Use [Query monitoring](query-monitoring.md) for independent credentials, role scope,
Insights/identity-provider egress, durable query state and Kafka recovery. Existing
resource/alert/log settings are unchanged.

### State integrity and recovery

`query-events.db` schema 1 is validated before startup retention pruning. Only an
absent database is initialized; existing empty/truncated files, schema/source
mismatches, malformed checkpoints, inconsistent pending count/bytes, and invalid
persisted events stop startup with bounded errors. Replay validates the event
allowlist and source again before returning bytes to the Kafka publisher.
Extra fields, including SQL text, are rejected rather than silently stripped.
Keep the original DB for investigation; automatic reset, repair and migration are
not part of this operation. Startup logs remain redacted and do not dump DB payloads.

Before restoring, stop the only writer and retain all state files, including
`history-backfill.db`. Confirm source identity, backup completeness, permissions
and the configured retention before coordinating a known-good backup restore.
Restoring older state can replay previously delivered events and restore earlier
cumulative values; consumers must retain their existing deduplication behavior.
Verify query collection/scope metrics, `/readyz` and the pending count after recovery.

Retention can legitimately remove a checkpoint while keeping an older pending
event. Such events remain eligible for delivery. Terminal pending events within
the persisted retention floor require their matching final checkpoint. Multiple
nonterminal historical events and metrics-only checkpoint updates remain valid;
their hashes need not all equal the latest checkpoint. Completed counters and
histograms are cumulative and are not rebuilt from the remaining outbox.

### Freshness, capacity and topic checks

Resource readiness uses each required family's `CollectedAt`, presence and
collector success, not just the latest cycle completion. Inspect individual
collector success and `ddae_management_api_up` when `/readyz` is not ready.
Represented null node pressure makes node collection incomplete while retaining
other usable fields. Alert capacity includes `CHECKPOINT_MAX_ALERTS`: equality
is full even with no buffered events. Only eligible absent, non-pending checkpoints
are removed by retention; preserve pending data while resolving capacity pressure.

All Kafka event topic names use 1–249 ASCII characters from `[A-Za-z0-9._-]`;
`.` and `..` are rejected during configuration loading. Keep enabled event topics
distinct and confirm broker ACLs after correcting a name. Alert/Log detail attempts
rotate within their existing class quotas even when a request fails. Live query
and backfill detail validation use response time for the five-second future-time
tolerance while retaining request-start observation order.

## Bounded history recovery

See [history backfill](history-backfill.md) for opt-in settings, combined foreground/background request budgets, retention gaps, readiness, state recovery and rollback. Preserve history-backfill.db together with the existing event databases.
