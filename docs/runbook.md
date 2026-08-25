# DDAE Exporter Runbook

This runbook covers the container, Kubernetes and VM/systemd deployment
profiles for the DDAE-2 implementation. It is not evidence that any profile has run in an
authorized environment. Dell DDAE 1.5.0, Kafka, OpenSearch and alert-delivery
validation remain release blockers.

## Prerequisites

- When alerts are enabled, use one exporter instance and one writable state
  database per DDAE target. Never share `state.dir` between concurrently
  running processes. Resource-only mode requires neither Kafka nor writable state.
- A dedicated least-privilege DDAE identity authorized only for the seven GET
  operations listed in the root `README.md`; the token request is the sole POST.
- An isolated Kafka topic whose consumer performs idempotent OpenSearch upsert
  by the Kafka record key. Brokers must support TLS and `acks=all`. A hard
  publish timeout can leave broker acceptance uncertain; the exporter retains
  that outbox record and may replay the same key.
- PEM CA roots for the DDAE and Kafka identities. Certificate and hostname
  verification are enabled by default. Guarded insecure mode requires both a
  global acknowledgement and one target flag, is diagnostic only, and cannot
  produce release or production-support evidence.
- A private Prometheus path protected by an external mTLS reverse proxy or
  service mesh before using a non-loopback listener.
- Capacity for `KAFKA_OUTBOX_MAX_BYTES` plus database overhead on a local or
  ReadWriteOnce filesystem that supports fsync and file locks.

## Configuration and secrets

The full strict YAML contract, precedence and defaults are in the root
`README.md`. Select the file with `--config` (preferred) or
`DDAE_EXPORTER_CONFIG_FILE`. Supply DDAE credentials and any Kafka password at
runtime through approved file paths. Direct secret environment values remain a
compatibility interface; YAML never accepts plaintext secret values.
Do not place secret values in ConfigMaps, environment files, command arguments,
container image layers, logs or Harness evidence.

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
`state.db` while alerts are disabled.

## Startup and normal operation

`GET /healthz` reports process liveness. `GET /readyz` evaluates only enabled
pipelines: current resource snapshot for resources, healthy auth/list/detail/
state/outbox for alerts, and their conjunction in dual mode. `/metrics` never
triggers DDAE requests.

Monitor at minimum:

- `ddae_monitoring_enabled{pipeline="resources|alerts"}` to distinguish
  intentional disablement from failure;
- `ddae_up`, `ddae_snapshot_age_seconds` and
  `ddae_collector_success{collector=...}`;
- `ddae_alert_list_complete`, `ddae_alert_pipeline_ready` and
  `ddae_alert_detail_deferred`;
- `ddae_kafka_publish_success`, `ddae_kafka_buffered_events` and
  `ddae_kafka_events_failed_total{reason=...}`.

Alert payloads are confidential and appear only in the approved Kafka topic.
They must not be copied into incident tickets or command output without the
applicable data-handling authorization.

## Troubleshooting

| Symptom | Safe check | Recovery |
|---|---|---|
| Authentication failures | Confirm credential files exist and are readable; inspect only the bounded `auth` class | Correct/rotate the dedicated identity; never print token or password values |
| TLS failures | Check CA mount and hostname against the certificate | Replace the approved CA/certificate; guarded insecure diagnosis requires explicit approval and its output is never release evidence |
| One collector fails | Inspect its bounded failure class and snapshot age | Validate only its fixed GET route/shape; do not add fallback routes or generic decoding |
| Alert list incomplete | Compare the sanitized count relationship in the authorized environment | Stop release, confirm enumeration behavior, and amend the specification if pagination is required |
| Kafka unavailable or publish timeout | Watch buffered count, fixed `kafka_timeout` failures and disk capacity | Restore broker/TLS/auth; retained events replay automatically in order. Do not delete an uncertain record; the consumer's idempotent key handles a possible duplicate |
| Outbox full | Stop new alert acceptance, preserve `state.db`, and restore Kafka | Drain the existing outbox; do not delete, truncate or replace the database unless an incident owner explicitly accepts data loss |
| State database will not open | Check single-process ownership, filesystem locks, permissions and capacity | Restore the last known-good state volume or escalate; do not start statelessly |

Logs expose component and bounded failure class, not raw server errors. A
failure that requires response-body inspection must be reproduced with
authorized, separately controlled diagnostic tooling.

## Upgrade, rollback and recovery

1. Back up the state volume while the exporter is stopped, or use an atomic
   volume snapshot supported by the platform.
2. Record the binary/image digest, configuration revision and event schema.
3. Stop the old instance and allow its configured shutdown grace period.
4. Start exactly one new instance against the existing state volume.
5. Confirm liveness, readiness, snapshot age and Kafka buffered count.

Use Kubernetes `Recreate`, not a rolling overlap, when alerts are enabled
because bbolt enforces one writer. For rollback, stop the new process and
restore the previous immutable binary/image and compatible environment
configuration. A DDAE-1 binary ignores the new YAML, so restore its environment
settings explicitly. Do not downgrade after an unknown state-schema migration;
v1 currently uses the versioned `*-v1` buckets.

## Verification status

Local tests and build gates may be run with `./HARNESS/harness.sh`. The
`integration` and `e2e` stages intentionally return blocked until Peter Chu
authorizes the required non-production environments. A blocked stage is not a
pass, waiver or release approval.

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
