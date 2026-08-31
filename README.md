# Dell DDAE Metrics Exporter

Dell DDAE Metrics Exporter is a read-only monitoring service for Dell Data Domain
Active Enterprise (DDAE) 1.5.0. It exports system status as Prometheus metrics and
can publish serviceability alerts and Serviceability Logs to Kafka.

| Item | Value |
|---|---|
| Current release candidate | [`v1.0.0-rc3`](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/releases/tag/v1.0.0-rc3) |
| DDAE target version | 1.5.0 |
| Runtime | Go 1.26.6 |
| License | Apache License 2.0 |
| Validation scope | Local unit, component, security, build, and documentation checks |

RC3 is a prerelease. Authenticated DDAE, Kafka, OpenSearch, deployment E2E, and
independent review remain external release gates.

## Contents

- [Overview](#overview)
- [DDAE API compatibility](#ddae-api-compatibility)
- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Prometheus metrics](#prometheus-metrics)
- [Kafka events](#kafka-events)
- [Deployment](#deployment)
- [Operations](#operations)
- [Development](#development)
- [License](#license)

## Overview

### Features

| Feature | Behavior |
|---|---|
| Resource monitoring | Collects API availability, cluster state, Coordinator and Worker configuration, node state and capacity, system lock state, and readiness. |
| Prometheus output | Serves the latest collected data on `/metrics`. |
| Alert collection | Reads the serviceability issue list and fetches bounded detail records. |
| Alert delivery | Stores alert events in a local bbolt outbox before publishing them to Kafka. |
| Serviceability Logs | Reads Serviceability Event list and detail records and publishes new or changed records to a separate Kafka topic. |
| Independent pipelines | `resources`, `alerts`, and `serviceability_logs` have separate switches and schedules. |
| Independent state | Alerts use `state.db`; Serviceability Logs use `serviceability-logs.db`. |
| YAML configuration | Uses a versioned, typed, size-limited YAML file. Environment variables can override YAML values. |
| TLS | Verifies DDAE and Kafka certificates and hostnames by default. Custom CA files and Kafka mTLS are supported. |
| Kafka authentication | Supports `PLAIN`, `SCRAM-SHA-256`, and `SCRAM-SHA-512`. |
| Deployment | Includes release archives, Docker, Kubernetes, and Linux/systemd instructions. |

### Data flow

```text
DDAE Management API
  |
  +-- resources ------------> current snapshot ------------> Prometheus /metrics
  |
  +-- serviceability issues -> alert outbox ---------------> Kafka alert topic
  |
  +-- serviceability events -> serviceability log outbox --> Kafka log topic
```

### Monitoring modes

At least one pipeline must be enabled.

| Mode | `resources` | `alerts` | `serviceability_logs` | Kafka required |
|---|---:|---:|---:|---:|
| Default | `true` | `true` | `false` | Yes |
| Resources only | `true` | `false` | `false` | No |
| Alerts only | `false` | `true` | `false` | Yes |
| Serviceability Logs only | `false` | `false` | `true` | Yes |
| All pipelines | `true` | `true` | `true` | Yes |

Enabled pipelines participate in `/readyz`. The
`ddae_monitoring_enabled{pipeline}` metric always reports the state of all three
pipelines.

## DDAE API compatibility

### API operations

The OAuth token request uses a fixed path. All monitoring operations use fixed,
read-only `GET` suffixes.

| Operation | Prefix setting | Default request | Purpose |
|---|---|---|---|
| Authentication | Fixed path | `POST /auth/realms/ddae/protocol/openid-connect/token` | Obtains a `dv-admin-rest` bearer token. |
| Ping | `ping_prefix` | `GET /ping` | Checks Management API availability. |
| Clusters | `api_prefix` | `GET /v1/ddae-clusters` | Reads cluster state and Coordinator/Worker configuration. |
| Nodes | `api_prefix` | `GET /v1/infrastructure-nodes` | Reads node state, capacity, allocatable resources, and conditions. |
| System lock | `api_prefix` | `GET /v1/system-lock` | Reads appliance lock state. |
| System shutdown | `api_prefix` | `GET /v1/system-shutdown` | Reads control-plane and node readiness. |
| Alert list | `api_prefix` | `GET /v1/serviceability-issues` | Lists serviceability issue IDs. |
| Alert detail | `api_prefix` | `GET /v1/serviceability-issues/{id}` | Reads one validated alert detail record. |
| Log list | `api_prefix` | `GET /v1/serviceability-events` | Lists Serviceability Event IDs. |
| Log detail | `api_prefix` | `GET /v1/serviceability-events/{id}` | Reads one validated Serviceability Log record. |

Use a dedicated DDAE identity with read-only access to these operations. DDAE
connections require TLS 1.2 or later.

### Path prefixes

Ping and the remaining Management API operations have separate prefix settings:

| YAML key | Environment variable | Default | Applies to |
|---|---|---|---|
| `ddae.paths.ping_prefix` | `DDAE_PING_PATH_PREFIX` | Empty string | `/ping` |
| `ddae.paths.api_prefix` | `DDAE_API_PATH_PREFIX` | `/v1` | All other Management API suffixes |

Common configurations:

| Route layout | `ping_prefix` | `api_prefix` | Ping result | API result example |
|---|---|---|---|---|
| Current default | `""` | `/v1` | `/ping` | `/v1/ddae-clusters` |
| Preserve v1.0.0-rc2 routes | `/rest/v1` | `/rest/v1` | `/rest/v1/ping` | `/rest/v1/ddae-clusters` |
| Dell PDF examples | `/rest` | `/rest/v1` | `/rest/ping` | `/rest/v1/ddae-clusters` |

A prefix may be empty. A non-empty prefix must meet all of these rules:

- Maximum length: 128 bytes.
- Starts with one `/` and does not end with `/`.
- Each segment contains only `A-Z`, `a-z`, `0-9`, `.`, `_`, `~`, or `-`.
- Does not contain empty segments, repeated slashes, `.` segments, or `..` segments.
- Does not contain whitespace, Unicode, percent encoding, query strings, fragments,
  schemes, authorities, user information, backslashes, or control characters.

The exporter concatenates the validated prefix and compiled suffix exactly. It
does not perform path normalization, runtime discovery, or alternate-path
fallback. A route failure is returned as an error without exposing the configured
prefix in metrics or error text.

## Requirements

### Runtime requirements

| Scope | Requirement |
|---|---|
| All modes | DDAE 1.5.0 HTTPS origin, such as `https://ddae.example.com`. |
| All modes | Dedicated DDAE read-only username, password, and `dv-admin-rest` client secret. |
| Private DDAE CA | PEM CA bundle. |
| Alerts | Kafka brokers, dedicated alert topic, TLS, and `acks=all`. |
| Serviceability Logs | Separate Kafka topic and ACL. |
| Kafka SASL | SASL username and password. |
| Kafka mTLS | Client certificate and private key. |
| Kafka pipelines | Writable persistent directory with file-lock and fsync support. |
| Prometheus | Network path to TCP port 9469. |
| Runtime host | Synchronized system clock. |

Resources-only mode does not require Kafka or persistent state.

### Source build requirements

| Tool | Version or use |
|---|---|
| Git | Downloads the repository. |
| Go | Language version `1.26.0`, toolchain `go1.26.6`. |
| Go module access | Required when dependencies are not present in the local cache. |

## Quick start

The following procedure installs the release archive on Linux amd64. Replace the
archive name for macOS or arm64.

### 1. Download and verify the release

Download the archive from the
[`v1.0.0-rc3` release](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/releases/tag/v1.0.0-rc3).

```bash
ARCHIVE=dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64.tar.gz
shasum -a 256 "$ARCHIVE"
tar -xzf "$ARCHIVE"
cd dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64
```

Compare the SHA-256 output with the value shown in the release notes.

Each platform archive contains:

| File | Purpose |
|---|---|
| `ddae-exporter` | Exporter executable. |
| `config.example.yaml` | Complete YAML example. |
| `README.md` | Installation and configuration reference. |
| `RUNBOOK.md` | Operations, recovery, and rollback procedures. |
| `LICENSE` | Apache License 2.0. |

### 2. Create credential files

Each `*_file` setting points to a regular file containing one raw value. Do not
add a key name, quotes, YAML, or JSON.

| YAML key | Runtime path example | Example content |
|---|---|---|
| `ddae.credentials.username_file` | `/secure/runtime/ddae-username` | `ddae-exporter-reader` |
| `ddae.credentials.password_file` | `/secure/runtime/ddae-password` | `REPLACE_WITH_DDAE_READ_ONLY_PASSWORD` |
| `ddae.credentials.client_secret_file` | `/secure/runtime/ddae-client-secret` | `REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET` |
| `kafka.sasl.password_file` | `/secure/runtime/kafka-password` | `REPLACE_WITH_KAFKA_SASL_PASSWORD` |

Example file contents:

`/secure/runtime/ddae-username`

```text
ddae-exporter-reader
```

`/secure/runtime/ddae-password`

```text
REPLACE_WITH_DDAE_READ_ONLY_PASSWORD
```

`/secure/runtime/ddae-client-secret`

```text
REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET
```

`/secure/runtime/kafka-password`

```text
REPLACE_WITH_KAFKA_SASL_PASSWORD
```

Install files supplied by a secret manager:

```bash
sudo install -d -o ddae-exporter -g ddae-exporter -m 0700 /secure/runtime
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-username /secure/runtime/ddae-username
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-password /secure/runtime/ddae-password
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-client-secret /secure/runtime/ddae-client-secret
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/kafka-password /secure/runtime/kafka-password
```

For local development, create the files interactively:

```bash
install -d -m 0700 secrets
umask 077
printf 'DDAE username: '
IFS= read -r ddae_username_value
printf 'DDAE password: '
IFS= read -r -s ddae_password_value
printf '\nDDAE dv-admin-rest client secret: '
IFS= read -r -s ddae_client_secret_value
printf '\nKafka SASL password (press Enter when unused): '
IFS= read -r -s kafka_password_value
printf '\n'
printf '%s' "$ddae_username_value" > secrets/ddae-username
printf '%s' "$ddae_password_value" > secrets/ddae-password
printf '%s' "$ddae_client_secret_value" > secrets/ddae-client-secret
printf '%s' "$kafka_password_value" > secrets/kafka-password
chmod 0600 secrets/ddae-username secrets/ddae-password \
  secrets/ddae-client-secret secrets/kafka-password
unset ddae_username_value ddae_password_value ddae_client_secret_value kafka_password_value
```

Credential file rules:

| Item | Requirement |
|---|---|
| File type | Regular file |
| Parent directory mode | `0700` |
| File mode | `0600` |
| Owner | Exporter runtime account |
| Encoding | Valid UTF-8 |
| Content | One non-empty value |
| Maximum size | 64 KiB |
| YAML value | Absolute path readable by the Exporter |

The reader removes one trailing `LF`, `CRLF`, or `CR` sequence. The `secrets/`
directory is excluded by `.gitignore`.

### 3. Create a configuration file

Copy the example:

```bash
cp config.example.yaml config.yaml
chmod 0600 config.yaml
```

For a resources-only first run, use this minimal configuration and replace every
path and endpoint:

```yaml
version: 1

monitoring:
  resources:
    enabled: true
  alerts:
    enabled: false
  serviceability_logs:
    enabled: false

ddae:
  base_url: https://ddae.example.com
  paths:
    ping_prefix: ""
    api_prefix: /v1
  credentials:
    username_file: /absolute/path/secrets/ddae-username
    password_file: /absolute/path/secrets/ddae-password
    client_secret_file: /absolute/path/secrets/ddae-client-secret
```

Use the [configuration reference](#configuration) for alerts, Serviceability
Logs, Kafka, custom CA files, and runtime limits.

### 4. Start the exporter

```bash
./ddae-exporter --config ./config.yaml
```

The configuration file may also be selected by environment variable:

```bash
DDAE_EXPORTER_CONFIG_FILE=/absolute/path/config.yaml ./ddae-exporter
```

### 5. Verify the process

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

| Check | Expected result |
|---|---|
| `/healthz` | Returns HTTP 200 after the HTTP server starts. |
| `/readyz` | Returns HTTP 200 after all enabled pipelines become ready; otherwise returns 503. |
| `/metrics` | Contains `ddae_build_info`, three `ddae_monitoring_enabled` series, and metrics for enabled pipelines. |

## Configuration

### File selection and precedence

YAML files use `version: 1`, UTF-8 encoding, one document, and a maximum size of
1 MiB. Unknown keys, incorrect types, and out-of-range values are rejected before
the HTTP server and background workers start.

Configuration precedence, highest first:

1. Individual environment variables.
2. YAML selected by `--config`.
3. YAML selected by `DDAE_EXPORTER_CONFIG_FILE`.
4. Program defaults.

An explicitly empty path-prefix environment variable is a valid override.

### Complete annotated YAML

Every setting in this example includes an inline description. Replace `.invalid`,
`replace-with-*`, and `/secure/runtime/*` values before deployment.

```yaml
version: 1 # Configuration schema version; currently fixed at integer 1.

monitoring: # Configuration for the three monitoring pipelines.
  resources: # Prometheus resource metrics.
    enabled: true # Enables the ping, clusters, nodes, lock, and power collectors.
    interval: 30s # Resource collection interval in Go duration syntax.
    stale_after: 120s # Snapshot expiry; must be greater than resources.interval.
  alerts: # DDAE alert-to-Kafka pipeline.
    enabled: true # Enables alert list/detail collection, outbox, and Kafka producer.
    interval: 30s # Alert list collection interval.
    list_response_max_bytes: 8388608 # Alert list body limit in bytes; range 1–67108864.
    detail: # Per-alert detail limits.
      response_max_bytes: 1048576 # Detail body limit in bytes; range 1–67108864.
      refresh_interval: 10m # Existing-alert refresh interval; must be at least alerts.interval.
      max_per_cycle: 200 # Maximum detail records per cycle; range 1–10000.
      concurrency: 4 # Concurrent detail requests; range 1–128 and no greater than max_per_cycle.
  serviceability_logs: # Serviceability Logs-to-Kafka pipeline.
    enabled: false # Enables its worker, producer, and serviceability-logs.db.
    interval: 30s # Serviceability Log list collection interval.
    list_response_max_bytes: 8388608 # Log list body limit in bytes; range 1–67108864.
    detail: # Per-Log detail limits.
      response_max_bytes: 1048576 # Detail body limit in bytes; range 1–67108864.
      refresh_interval: 10m # Existing-Log refresh interval; must be at least serviceability_logs.interval.
      max_per_cycle: 200 # Maximum detail records per cycle; range 1–10000.
      concurrency: 4 # Concurrent detail requests; range 1–128 and no greater than max_per_cycle.

server: # Exporter HTTP server.
  listen_address: 127.0.0.1:9469 # Listener; use 0.0.0.0:9469 in containers and Kubernetes.
  shutdown_grace_period: 15s # Total time to stop workers, sync state, and close HTTP.

security: # Global acknowledgement for TLS diagnostic mode.
  allow_insecure_tls: false # Must also be true before a target may disable TLS verification.

ddae: # DDAE Management API client.
  base_url: https://ddae.example.invalid # HTTPS origin; optional port, no path, query, or user information.
  paths: # Independent prefixes for Ping and the remaining Management API.
    ping_prefix: "" # Empty by default; produces GET /ping.
    api_prefix: /v1 # Default for other APIs; for example, GET /v1/ddae-clusters.
  source_instance: replace-with-stable-appliance-name # Stable Kafka source identity; required for a Kafka pipeline.
  credentials: # DDAE OAuth password-grant credential files.
    username_file: /secure/runtime/ddae-username # Contains only the DDAE read-only username.
    password_file: /secure/runtime/ddae-password # Contains only the DDAE read-only password.
    client_secret_file: /secure/runtime/ddae-client-secret # Contains only the dv-admin-rest client secret.
  tls: # DDAE HTTPS trust settings.
    ca_file: /etc/ddae-exporter/trust/ddae-ca.pem # Additional PEM CA bundle; remove when using system roots.
    insecure_skip_verify: false # Disables DDAE certificate/hostname checks only with allow_insecure_tls.
  request_timeout: 5s # Per-token or API request deadline; must be less than cycle_timeout.
  cycle_timeout: 20s # Aggregate cycle deadline; must be less than each enabled pipeline interval.
  response_max_bytes: 4194304 # General DDAE response limit in bytes; range 1–67108864.
  retry_max: 2 # Retries after the initial safe request; range 0–10.

kafka: # Shared connection settings for Alerts and Serviceability Logs.
  brokers: # Bootstrap broker list; 1–64 entries when a Kafka pipeline is enabled.
    - kafka.example.invalid:9093 # Broker in host:port form; add list items for more brokers.
  topic: ddae-serviceability-alerts # Dedicated alert topic; maximum 249 bytes.
  serviceability_logs_topic: ddae-serviceability-logs # Dedicated Log topic; must differ from the alert topic.
  client_id: ddae-exporter # Kafka client ID; 1–128 bytes.
  tls: # Kafka TLS and optional mTLS.
    ca_file: /etc/ddae-exporter/trust/kafka-ca.pem # Additional PEM CA bundle; remove when using system roots.
    # client_cert_file: /secure/runtime/kafka-client.crt # Optional mTLS client certificate PEM.
    # client_key_file: /secure/runtime/kafka-client.key # Optional paired mTLS private key.
    insecure_skip_verify: false # Disables Kafka certificate/hostname checks only with allow_insecure_tls.
  sasl: # Optional Kafka SASL; this example uses SCRAM-SHA-512.
    mechanism: SCRAM-SHA-512 # PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512.
    username: replace-with-runtime-identity # Kafka SASL username.
    password_file: /secure/runtime/kafka-password # Contains only the Kafka SASL password.
  publish_timeout: 10s # Per-record acknowledgement deadline; minimum 1s.

state: # Local persistent state for Alerts and Serviceability Logs.
  dir: /var/lib/ddae-exporter # Writable absolute directory used by Kafka pipelines.
  outbox_max_bytes: 1073741824 # Alert outbox payload limit in bytes.
  outbox_max_events: 100000 # Alert outbox record limit; range 1–10000000.
  checkpoint_retention: 720h # Retention after an alert disappears from a complete list.
  checkpoint_max_alerts: 100000 # Alert checkpoint limit; range 1–10000000.
  serviceability_logs_outbox_max_bytes: 1073741824 # Log outbox payload limit in bytes.
  serviceability_logs_outbox_max_events: 100000 # Log outbox record limit; range 1–10000000.
  serviceability_logs_checkpoint_retention: 720h # Retention after a Log disappears from a complete list.
  serviceability_logs_checkpoint_max_records: 100000 # Log checkpoint limit; range 1–10000000.

logging: # Application logging.
  level: info # debug, info, warn, or error.
  format: json # json or text.
```

### Configuration reference

#### Monitoring, HTTP, and DDAE

| YAML key | Default or requirement | Description |
|---|---|---|
| `monitoring.resources.enabled` | `true` | Enables resource collectors and resource metrics. |
| `monitoring.resources.interval` | `30s` | Resource collection interval. |
| `monitoring.resources.stale_after` | `120s` | Maximum age of the current resource snapshot. |
| `monitoring.alerts.enabled` | `true` | Enables alert list/detail collection, Kafka, and alert state. |
| `monitoring.alerts.interval` | `30s` | Alert list collection interval. |
| `monitoring.serviceability_logs.enabled` | `false` | Enables Serviceability Log collection, its Kafka producer, and its state file. |
| `monitoring.serviceability_logs.interval` | `30s` | Serviceability Log list collection interval. |
| `server.listen_address` | `127.0.0.1:9469` | HTTP listener in `host:port` format. |
| `server.shutdown_grace_period` | `15s` | Total graceful shutdown budget. |
| `ddae.base_url` | Required | One HTTPS origin without path, query, fragment, or user information. |
| `ddae.paths.ping_prefix` | Empty string | Prefix concatenated with `/ping`. |
| `ddae.paths.api_prefix` | `/v1` | Prefix concatenated with all other Management API suffixes. |
| `ddae.source_instance` | Required for Kafka | Stable event source identity, 1–128 UTF-8 bytes. |
| `ddae.credentials.username_file` | Required | DDAE username file. |
| `ddae.credentials.password_file` | Required | DDAE password file. |
| `ddae.credentials.client_secret_file` | Required | `dv-admin-rest` client secret file. |
| `ddae.tls.ca_file` | System roots | Additional PEM CA bundle. |
| `ddae.request_timeout` | `5s` | Deadline for one token or DDAE request. |
| `ddae.cycle_timeout` | `20s` | Aggregate deadline for one collection cycle. |
| `ddae.response_max_bytes` | `4194304` | General response body limit; range `1–67108864` bytes. |
| `ddae.retry_max` | `2` | Retry count after the initial safe request; range `0–10`. |

Duration values use Go syntax such as `500ms`, `5s`, `10m`, and `720h`.

Required timing relationships:

```text
ddae.request_timeout < ddae.cycle_timeout < each enabled pipeline interval
monitoring.resources.interval < monitoring.resources.stale_after
alerts.detail.refresh_interval >= alerts.interval
serviceability_logs.detail.refresh_interval >= serviceability_logs.interval
```

#### Alert detail limits

| YAML key | Default | Range or rule |
|---|---:|---|
| `monitoring.alerts.list_response_max_bytes` | `8388608` | `1–67108864` bytes |
| `monitoring.alerts.detail.response_max_bytes` | `1048576` | `1–67108864` bytes |
| `monitoring.alerts.detail.refresh_interval` | `10m` | At least `alerts.interval` |
| `monitoring.alerts.detail.max_per_cycle` | `200` | `1–10000` records |
| `monitoring.alerts.detail.concurrency` | `4` | `1–128`, not greater than `max_per_cycle` |

Records above `max_per_cycle` are deferred in deterministic order. New or
changed records and refresh-due records use separate queues. When both queues
contain work, the scheduler preserves capacity for each queue.

#### Serviceability Log detail limits

| YAML key | Default | Range or rule |
|---|---:|---|
| `monitoring.serviceability_logs.list_response_max_bytes` | `8388608` | `1–67108864` bytes |
| `monitoring.serviceability_logs.detail.response_max_bytes` | `1048576` | `1–67108864` bytes |
| `monitoring.serviceability_logs.detail.refresh_interval` | `10m` | At least `serviceability_logs.interval` |
| `monitoring.serviceability_logs.detail.max_per_cycle` | `200` | `1–10000` records |
| `monitoring.serviceability_logs.detail.concurrency` | `4` | `1–128`, not greater than `max_per_cycle` |

An incomplete list may still provide validated IDs for bounded detail processing.
Only a complete list is used for absence retention. The pipeline does not emit a
deletion event when a record disappears from the list.

#### Kafka and persistent state

| YAML key | Default or requirement | Description |
|---|---|---|
| `kafka.brokers` | Required for Kafka | List of 1–64 TLS broker addresses. |
| `kafka.topic` | Required for alerts | Alert topic, maximum 249 bytes. |
| `kafka.serviceability_logs_topic` | `ddae-serviceability-logs` | Log topic; must differ from the alert topic when both pipelines are enabled. |
| `kafka.client_id` | `ddae-exporter` | Kafka client ID, 1–128 bytes. |
| `kafka.tls.ca_file` | System roots | Additional broker CA bundle. |
| `kafka.tls.client_cert_file` | Optional | mTLS client certificate. |
| `kafka.tls.client_key_file` | Optional | mTLS private key paired with the client certificate. |
| `kafka.sasl.mechanism` | Optional | `PLAIN`, `SCRAM-SHA-256`, or `SCRAM-SHA-512`. |
| `kafka.sasl.username` | Required with SASL | Kafka SASL username. |
| `kafka.sasl.password_file` | Required with SASL | Kafka SASL password file. |
| `kafka.publish_timeout` | `10s` | Hard deadline for one broker acknowledgement; minimum `1s`. |
| `state.dir` | `/var/lib/ddae-exporter` | Writable absolute directory for Kafka pipeline state. |
| `state.outbox_max_bytes` | `1073741824` | Alert outbox byte limit. |
| `state.outbox_max_events` | `100000` | Alert outbox record limit. |
| `state.checkpoint_retention` | `720h` | Alert checkpoint retention. |
| `state.checkpoint_max_alerts` | `100000` | Alert checkpoint limit. |
| `state.serviceability_logs_outbox_max_bytes` | `1073741824` | Log outbox byte limit. |
| `state.serviceability_logs_outbox_max_events` | `100000` | Log outbox record limit. |
| `state.serviceability_logs_checkpoint_retention` | `720h` | Log checkpoint retention. |
| `state.serviceability_logs_checkpoint_max_records` | `100000` | Log checkpoint limit. |

Kafka delivery uses `acks=all`, an idempotent producer, per-key ordering, and a
durable at-least-once outbox. A publish timeout leaves the record in the outbox
because the broker result is uncertain. Consumers must perform idempotent upserts
using the Kafka record key.

When an outbox reaches its byte or record limit, the exporter retains existing
records, stops accepting new records for that pipeline, and reports the pipeline
as not ready.

#### TLS verification

| YAML key | Default | Effect |
|---|---:|---|
| `security.allow_insecure_tls` | `false` | Global acknowledgement required before any target may disable verification. |
| `ddae.tls.insecure_skip_verify` | `false` | Disables DDAE certificate and hostname checks only when the global acknowledgement is also `true`. |
| `kafka.tls.insecure_skip_verify` | `false` | Disables Kafka certificate and hostname checks only when the global acknowledgement is also `true`. |

DDAE and Kafka settings are independent. Normal deployments keep all three
values `false` and install the correct CA bundle. A target cannot combine
`ca_file` with its effective insecure mode. TLS 1.2 remains the minimum version.

#### Environment variables

Environment variables preserve compatibility with environment-only deployments.
Direct secret values are supported for existing deployments; file-based secrets
remain the preferred interface.

| Group | Environment variables |
|---|---|
| Pipeline switches | `DDAE_RESOURCE_MONITORING_ENABLED`, `DDAE_ALERT_MONITORING_ENABLED`, `DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED` |
| Pipeline intervals | `DDAE_COLLECTION_INTERVAL`, `DDAE_RESOURCE_COLLECTION_INTERVAL`, `DDAE_ALERT_COLLECTION_INTERVAL`, `DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL`, `DDAE_STALE_AFTER` |
| Alert limits | `ALERT_LIST_RESPONSE_MAX_BYTES`, `ALERT_DETAIL_RESPONSE_MAX_BYTES`, `ALERT_DETAIL_REFRESH_INTERVAL`, `ALERT_DETAIL_MAX_PER_CYCLE`, `ALERT_DETAIL_CONCURRENCY` |
| Log limits | `SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES`, `SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES`, `SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL`, `SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE`, `SERVICEABILITY_LOG_DETAIL_CONCURRENCY` |
| DDAE | `DDAE_BASE_URL`, `DDAE_PING_PATH_PREFIX`, `DDAE_API_PATH_PREFIX`, `DDAE_SOURCE_INSTANCE`, `DDAE_CA_FILE`, `DDAE_TLS_INSECURE_SKIP_VERIFY`, `DDAE_REQUEST_TIMEOUT`, `DDAE_CYCLE_TIMEOUT`, `DDAE_RESPONSE_MAX_BYTES`, `DDAE_RETRY_MAX` |
| DDAE credentials | `DDAE_USERNAME`, `DDAE_PASSWORD`, `DDAE_CLIENT_SECRET`, and their corresponding `*_FILE` variables |
| Kafka | `KAFKA_BROKERS`, `KAFKA_TOPIC`, `KAFKA_SERVICEABILITY_LOG_TOPIC`, `KAFKA_CLIENT_ID`, `KAFKA_PUBLISH_TIMEOUT` |
| Kafka TLS | `KAFKA_CA_FILE`, `KAFKA_CLIENT_CERT_FILE`, `KAFKA_CLIENT_KEY_FILE`, `KAFKA_TLS_INSECURE_SKIP_VERIFY` |
| Kafka SASL | `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD`, `KAFKA_SASL_PASSWORD_FILE` |
| State | `STATE_DIR`, `KAFKA_OUTBOX_MAX_BYTES`, `KAFKA_OUTBOX_MAX_EVENTS`, `CHECKPOINT_RETENTION`, `CHECKPOINT_MAX_ALERTS` |
| Log state | `SERVICEABILITY_LOG_OUTBOX_MAX_BYTES`, `SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS`, `SERVICEABILITY_LOG_CHECKPOINT_RETENTION`, `SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS` |
| Server and logging | `EXPORTER_LISTEN_ADDRESS`, `SHUTDOWN_GRACE_PERIOD`, `LOG_LEVEL`, `LOG_FORMAT` |
| TLS acknowledgement | `ALLOW_INSECURE_TLS` |

## Prometheus metrics

The default listener is `127.0.0.1:9469`.

| Endpoint | Behavior |
|---|---|
| `GET /metrics` | Prometheus exposition and collector diagnostics. Maximum five concurrent requests; nine-second handler timeout. |
| `GET /healthz` | Process liveness. |
| `GET /readyz` | Readiness of all enabled pipelines. Returns 200 when ready and 503 otherwise. |

### Units

| Unit | Meaning |
|---|---|
| `boolean` | `0` or `1` |
| `one-hot boolean` | One series in a fixed state set is `1`; the remaining state series are `0` |
| `seconds` | Seconds; fractional values are allowed |
| `bytes` | Bytes, not KiB or MiB |
| `CPU cores` | CPU cores; fractional values are allowed |
| `nodes`, `alerts`, `events`, `logs`, `records` | Non-negative integer counts |
| `counter` | Non-negative value that increases during one process lifetime |

### Common metrics

| Metric | Type | Unit | Value range and meaning |
|---|---|---|---|
| `ddae_build_info{version,go_version}` | gauge | none | Always `1`; labels identify the exporter and Go versions. |
| `ddae_monitoring_enabled{pipeline}` | gauge | boolean | `0` or `1`; always reports `resources`, `alerts`, and `serviceability_logs`. |
| `ddae_collector_success{collector}` | gauge | boolean | `1` when the latest collection for that collector succeeded. |
| `ddae_collector_duration_seconds{collector}` | gauge | seconds | Non-negative duration of the latest collection. |

### Resource metrics

| Metric | Type | Unit | Value range and meaning |
|---|---|---|---|
| `ddae_up` | gauge | boolean | `1` when all required resource collectors succeeded and the snapshot is fresh. |
| `ddae_snapshot_age_seconds` | gauge | seconds | Non-negative age of the latest complete resource snapshot. |
| `ddae_management_api_up` | gauge | boolean | `1` when the Ping collector completes a valid authenticated request. |
| `ddae_cluster_state_info{cluster,state}` | gauge | one-hot boolean | `state` is `available` or `unknown`. |
| `ddae_cluster_coordinator_configured_cpu_cores{cluster}` | gauge | CPU cores | Non-negative finite configured CPU. |
| `ddae_cluster_coordinator_configured_memory_bytes{cluster}` | gauge | bytes | Non-negative configured memory. |
| `ddae_cluster_worker_configured_cpu_cores{cluster}` | gauge | CPU cores | Non-negative finite worker CPU configuration. |
| `ddae_cluster_worker_configured_memory_bytes{cluster}` | gauge | bytes | Non-negative worker memory configuration. |
| `ddae_node_state_info{node,state}` | gauge | one-hot boolean | Reports one fixed normalized node state. |
| `ddae_node_ready{node}` | gauge | boolean | `1` when normalized node state is `ready`. |
| `ddae_node_capacity_cpu_cores{node}` | gauge | CPU cores | Non-negative finite total capacity. |
| `ddae_node_capacity_memory_bytes{node}` | gauge | bytes | Non-negative total memory capacity. |
| `ddae_node_capacity_ephemeral_storage_bytes{node}` | gauge | bytes | Non-negative total ephemeral-storage capacity. |
| `ddae_node_allocatable_cpu_cores{node}` | gauge | CPU cores | Non-negative finite allocatable CPU. |
| `ddae_node_allocatable_memory_bytes{node}` | gauge | bytes | Non-negative allocatable memory. |
| `ddae_node_allocatable_ephemeral_storage_bytes{node}` | gauge | bytes | Non-negative allocatable ephemeral storage. |
| `ddae_node_condition{node,condition}` | gauge | boolean | `condition` is `disk_pressure` or `memory_pressure`; `1` means pressure is present. |
| `ddae_system_locked` | gauge | boolean | `1` when the appliance is locked. |
| `ddae_control_plane_ready` | gauge | boolean | `1` when control-plane nodes are ready. |
| `ddae_nodes_ready` | gauge | nodes | Integer from `0` to `ddae_nodes_total`. |
| `ddae_nodes_total` | gauge | nodes | Non-negative integer. |

Missing CPU, memory, or storage fields omit the corresponding series. Present
quantities must parse as Kubernetes quantities and produce non-negative finite
values.

Node state values:

| State | Meaning |
|---|---|
| `maintenance_mode` | Node is in maintenance mode. |
| `scheduling_disabled` | Scheduling is disabled. |
| `not_ready` | Node is not ready. |
| `ready` | Node is ready. |
| `restarting` | Node is restarting. |
| `shutting_down` | Node is shutting down. |
| `powered_off` | Node is powered off. |
| `powering_on` | Node is powering on. |
| `unknown` | Source state does not match a fixed known value. |

### Alert metrics

| Metric | Type | Unit | Value range and meaning |
|---|---|---|---|
| `ddae_alert_list_complete` | gauge | boolean | `1` when the latest list structure and count relationship are complete. |
| `ddae_alert_detail_deferred` | gauge | alerts | Number of alert details deferred by `max_per_cycle`. |
| `ddae_alert_pipeline_ready` | gauge | boolean | `1` when list/detail collection, persistent state, and outbox capacity are healthy. |
| `ddae_kafka_publish_success` | gauge | boolean | `1` when the latest required Kafka batch was acknowledged. |
| `ddae_kafka_publish_duration_seconds` | gauge | seconds | Non-negative publish duration. |
| `ddae_kafka_events_published_total` | counter | events | Number of acknowledged alert events. |
| `ddae_kafka_events_failed_total{reason}` | counter | events | Failed alert events grouped by fixed failure class. |
| `ddae_kafka_buffered_events` | gauge | events | `0` to `state.outbox_max_events`; the byte limit may be reached first. |

### Serviceability Log metrics

| Metric | Type | Unit | Value range and meaning |
|---|---|---|---|
| `ddae_serviceability_log_list_complete` | gauge | boolean | `1` when the latest list passes structure and count completeness checks. |
| `ddae_serviceability_log_detail_deferred` | gauge | logs | Number of log details deferred by `max_per_cycle`. |
| `ddae_serviceability_log_pipeline_ready` | gauge | boolean | `1` when list/detail collection, state, and capacity are healthy. |
| `ddae_serviceability_log_kafka_publish_success` | gauge | boolean | `1` when the latest required Log publish was acknowledged. |
| `ddae_serviceability_log_kafka_publish_duration_seconds` | gauge | seconds | Non-negative publish duration. |
| `ddae_serviceability_log_records_published_total` | counter | records | Number of acknowledged Log records. |
| `ddae_serviceability_log_records_failed_total{reason}` | counter | records | Failed Log records grouped by fixed failure class. |
| `ddae_serviceability_log_buffered_records` | gauge | records | `0` to `state.serviceability_logs_outbox_max_events`; the byte limit may be reached first. |

### Fixed label values

| Label | Values |
|---|---|
| `pipeline` | `resources`, `alerts`, `serviceability_logs` |
| `collector` | `ping`, `clusters`, `nodes`, `lock`, `power`, `alert_list`, `alert_detail`, `serviceability_log_list`, `serviceability_log_detail` |
| Cluster `state` | `available`, `unknown` |
| Node `state` | `maintenance_mode`, `scheduling_disabled`, `not_ready`, `ready`, `restarting`, `shutting_down`, `powered_off`, `powering_on`, `unknown` |
| `condition` | `disk_pressure`, `memory_pressure` |
| `reason` | `auth`, `tls`, `timeout`, `transport`, `http`, `decode`, `validation`, `kafka_auth`, `kafka_timeout`, `kafka_rejected`, `buffer_full`, `internal` |

Deployment dimensions such as site, environment, and region belong in Prometheus
service discovery or relabeling configuration.

## Kafka events

### Alert event

Each alert record uses schema version `1.0`.

| Envelope field | JSON type | Value or source |
|---|---|---|
| `schema_version` | string | Always `1.0`. |
| `event_type` | string | Always `ddae.serviceability_alert.upsert`. |
| `source_system` | string | Always `dell_ddae`. |
| `source_instance` | string | Stable configured appliance identity, 1–128 UTF-8 bytes. |
| `alert_id` | string | 1–256 ASCII characters from the validated list ID. |
| `content_hash_sha256` | string | 64-character lowercase SHA-256 of canonical normalized `alert` JSON. |
| `observed_at` | string | UTC RFC 3339 observation time. |
| `alert` | object | Allowed typed alert fields. |

Alert payload fields:

| Field | JSON type | Range or limit |
|---|---|---|
| `severity` | string | `critical`, `error`, `warning`, `info`, `normal`, or `unknown` |
| `acknowledged` | boolean | `true` or `false` |
| `occurrence_count` | integer | Non-negative |
| `created_at`, `updated_at` | string | UTC RFC 3339 timestamp |
| `clear_type` | string | Maximum 64 UTF-8 bytes |
| `auto_clear_timeout_raw` | integer | Non-negative raw value; Dell 1.5.0 does not define its unit |
| `app_name`, `component`, `namespace` | string | Maximum 256 UTF-8 bytes each |
| `message` | string | Maximum 8192 UTF-8 bytes |
| `reason` | string | Maximum 4096 UTF-8 bytes |
| `remedies` | array of strings | Maximum 32 items, 2048 UTF-8 bytes each |
| `resource_id`, `related` | string | Maximum 512 UTF-8 bytes |
| `symptom_id` | string | Maximum 256 UTF-8 bytes |
| `related_events` | array of typed objects | Maximum 100 one-level objects |

Alert JSON template:

```json
{
  "schema_version": "1.0",
  "event_type": "ddae.serviceability_alert.upsert",
  "source_system": "dell_ddae",
  "source_instance": "fixture-site",
  "alert_id": "alert-1",
  "content_hash_sha256": "49ccb1aaa6bbe2aecdfddba289a1ac720e153a14b1e59fe14380086233d513ea",
  "observed_at": "2026-08-24T03:00:00Z",
  "alert": {
    "severity": "warning",
    "acknowledged": false,
    "occurrence_count": 2,
    "created_at": "2026-08-24T02:00:00Z",
    "message": "synthetic operator message",
    "remedies": [
      "inspect synthetic node"
    ]
  }
}
```

Kafka record contract:

| Item | Value |
|---|---|
| Topic | `kafka.topic` |
| Key | Lowercase SHA-256 of `source_instance + NUL + alert_id` |
| Headers | `content-type=application/json`, `ddae-schema-version=1.0` |
| Maximum event size | 256 KiB |
| Delivery | Durable outbox, `acks=all`, idempotent producer, at-least-once replay |

Fields absent from the DDAE response are omitted. Source fields outside the typed
allowlist are not copied to the event.

### Serviceability Log event

Serviceability Logs use an independent topic, record identity, producer, and
state file.

| Envelope field | JSON type | Value or source |
|---|---|---|
| `schema_version` | string | Always `1.0`. |
| `event_type` | string | Always `ddae.serviceability_log.upsert`. |
| `source_system` | string | Always `dell_ddae`. |
| `source_instance` | string | Stable configured appliance identity, 1–128 UTF-8 bytes. |
| `log_id` | string | Validated list/detail ID, 1–256 UTF-8 bytes. |
| `content_hash_sha256` | string | 64-character lowercase SHA-256 of canonical normalized `log` JSON. |
| `observed_at` | string | UTC RFC 3339 observation time. |
| `log` | object | Allowed typed Log fields. |

Log payload fields:

| Field | JSON type | Range or limit |
|---|---|---|
| `severity` | string | `critical`, `error`, `warning`, `info`, `normal`, or `unknown` |
| `acknowledged` | boolean | `true` or `false` |
| `occurrence_count` | integer | Non-negative |
| `created_at`, `updated_at` | string | UTC RFC 3339 timestamp |
| `app_name`, `component`, `namespace` | string | Maximum 256 UTF-8 bytes each |
| `message` | string | Maximum 8192 UTF-8 bytes |
| `reason` | string | Maximum 4096 UTF-8 bytes |
| `remedies` | array of strings | Maximum 32 items, 2048 UTF-8 bytes each |
| `resource_id`, `related` | string | Maximum 512 UTF-8 bytes |
| `symptom_id` | string | Maximum 256 UTF-8 bytes |

Serviceability Log JSON template:

```json
{
  "schema_version": "1.0",
  "event_type": "ddae.serviceability_log.upsert",
  "source_system": "dell_ddae",
  "source_instance": "fixture-site",
  "log_id": "log-1",
  "content_hash_sha256": "a51e004808c177b20fc11b89fdfc0823f10e9dccf5a4a11481158a802fd0d17a",
  "observed_at": "2026-08-24T03:00:00Z",
  "log": {
    "severity": "warning",
    "acknowledged": false,
    "occurrence_count": 2,
    "created_at": "2026-08-24T02:00:00Z",
    "message": "synthetic serviceability log",
    "remedies": [
      "inspect synthetic node"
    ]
  }
}
```

Kafka record contract:

| Item | Value |
|---|---|
| Topic | `kafka.serviceability_logs_topic`; default `ddae-serviceability-logs` |
| Key | Lowercase SHA-256 of `source_instance + NUL + serviceability_log + NUL + log_id` |
| Headers | `content-type=application/json`, `ddae-schema-version=1.0`, `ddae-record-kind=serviceability_log` |
| Maximum event size | 256 KiB |
| Delivery | Independent durable outbox, `acks=all`, idempotent producer, per-key ordering, at-least-once replay |

Optional fields absent from the source are omitted. `labels`, `links`, and unknown
source fields are not included. An invalid present value fails that record and
retains the existing checkpoint.

## Deployment

### Build from source

```bash
git clone https://github.com/crispkid/Dell-DDAE-Metrics-Exporter.git
cd Dell-DDAE-Metrics-Exporter
git checkout v1.0.0-rc3
go mod download
go test ./...
./scripts/build.sh
```

The executable is written to `bin/ddae-exporter`.

Build with release metadata:

```bash
VERSION=1.0.0-rc3 \
REVISION=REPLACE_WITH_GIT_SHA \
BUILD_DATE=REPLACE_WITH_UTC_RFC3339 \
./scripts/build.sh
```

### Docker

Build the image from the repository root:

```bash
docker build \
  --build-arg VERSION=1.0.0-rc3 \
  --build-arg REVISION=REPLACE_WITH_GIT_SHA \
  --build-arg BUILD_DATE=REPLACE_WITH_UTC_RFC3339 \
  --tag ddae-exporter:v1.0.0-rc3 \
  .
```

The final image uses `scratch`, UID/GID `65532`, and `/ddae-exporter` as its
entrypoint.

Create host directories:

```bash
sudo install -d -m 0700 -o 65532 -g 65532 /srv/ddae-exporter/state
sudo install -d -m 0700 -o 65532 -g 65532 /etc/ddae-exporter/secrets
sudo install -d -m 0755 -o root -g root /etc/ddae-exporter/trust
sudo install -o root -g root -m 0600 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
```

Use these paths inside the container:

| Data | Container path |
|---|---|
| YAML | `/etc/ddae-exporter/config.yaml` |
| Credentials | `/run/secrets/...` |
| CA bundles | `/run/trust/...` |
| Persistent state | `/var/lib/ddae-exporter` |

Set `server.listen_address` to `0.0.0.0:9469`, update all paths in the YAML, and
start the container:

```bash
docker run --rm \
  --name ddae-exporter \
  --read-only \
  --user 65532:65532 \
  --publish 127.0.0.1:9469:9469 \
  --mount type=bind,source=/etc/ddae-exporter/config.yaml,target=/etc/ddae-exporter/config.yaml,readonly \
  --mount type=bind,source=/etc/ddae-exporter/secrets,target=/run/secrets,readonly \
  --mount type=bind,source=/etc/ddae-exporter/trust,target=/run/trust,readonly \
  --mount type=bind,source=/srv/ddae-exporter/state,target=/var/lib/ddae-exporter \
  ddae-exporter:v1.0.0-rc3 --config /etc/ddae-exporter/config.yaml
```

### Kubernetes

Deployment files:

| File | Contents |
|---|---|
| [`deploy/kubernetes/configmap.yaml`](deploy/kubernetes/configmap.yaml) | Non-secret YAML example. |
| [`deploy/kubernetes/deployment.yaml`](deploy/kubernetes/deployment.yaml) | ServiceAccount, 3 GiB RWO PVC, single-replica `Recreate` Deployment, and ClusterIP Service. |
| [`deploy/kubernetes/networkpolicy.yaml`](deploy/kubernetes/networkpolicy.yaml) | Default-deny ingress and egress policy. |

Before deployment:

1. Replace the image reference with an immutable image digest.
2. Update DDAE, Kafka, monitoring mode, and runtime limits in the ConfigMap.
3. Select a storage class that supports RWO, fsync, and file locking.
4. Add exact DNS, DDAE, Kafka, and Prometheus rules to the NetworkPolicy.
5. Provide controlled Prometheus access through a service mesh or mTLS proxy.

Create Secrets from controlled files:

```bash
kubectl -n monitoring create secret generic ddae-exporter-credentials \
  --from-file=username=/secure/runtime/ddae-username \
  --from-file=password=/secure/runtime/ddae-password \
  --from-file=client-secret=/secure/runtime/ddae-client-secret

kubectl -n monitoring create secret generic ddae-exporter-kafka \
  --from-file=password=/secure/runtime/kafka-password

kubectl -n monitoring create secret generic ddae-exporter-trust \
  --from-file=ddae-ca.pem=/secure/runtime/ddae-ca.pem \
  --from-file=kafka-ca.pem=/secure/runtime/kafka-ca.pem
```

Apply the resources:

```bash
kubectl -n monitoring apply -f deploy/kubernetes/configmap.yaml
kubectl -n monitoring apply -f /secure/runtime/ddae-exporter-allow-policy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/networkpolicy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/deployment.yaml
```

Check the deployment:

```bash
kubectl -n monitoring rollout status deployment/ddae-exporter
kubectl -n monitoring get pod,service -l app.kubernetes.io/name=ddae-exporter
kubectl -n monitoring get pvc ddae-exporter-state
kubectl -n monitoring logs deployment/ddae-exporter
```

The Deployment uses one replica and `Recreate` to maintain one bbolt writer.

### Linux and systemd

The repository includes
[`deploy/systemd/ddae-exporter.service`](deploy/systemd/ddae-exporter.service).
The unit passes secrets with `LoadCredential=`.

Create the service account and directories:

```bash
sudo useradd --system \
  --home-dir /var/lib/ddae-exporter \
  --shell /usr/sbin/nologin \
  ddae-exporter

sudo install -o root -g root -m 0755 bin/ddae-exporter /usr/local/bin/ddae-exporter
sudo install -d -o root -g ddae-exporter -m 0750 /etc/ddae-exporter
sudo install -d -o root -g root -m 0700 /etc/ddae-exporter/secrets
sudo install -d -o root -g root -m 0755 /etc/ddae-exporter/trust
sudo install -d -o ddae-exporter -g ddae-exporter -m 0700 /var/lib/ddae-exporter
```

Install the configuration and unit:

```bash
sudo install -o root -g ddae-exporter -m 0640 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
sudo install -o root -g root -m 0644 \
  deploy/systemd/ddae-exporter.service /etc/systemd/system/ddae-exporter.service
```

Install systemd credential source files:

```bash
sudo install -o root -g root -m 0600 /secure/runtime/ddae-username \
  /etc/ddae-exporter/secrets/ddae-username
sudo install -o root -g root -m 0600 /secure/runtime/ddae-password \
  /etc/ddae-exporter/secrets/ddae-password
sudo install -o root -g root -m 0600 /secure/runtime/ddae-client-secret \
  /etc/ddae-exporter/secrets/ddae-client-secret
sudo install -o root -g root -m 0600 /secure/runtime/kafka-password \
  /etc/ddae-exporter/secrets/kafka-password
```

Update `/etc/ddae-exporter/config.yaml`, install CA bundles under
`/etc/ddae-exporter/trust/`, then start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ddae-exporter
sudo systemctl status ddae-exporter
sudo journalctl -u ddae-exporter
```

### Prometheus

Local scrape configuration:

```yaml
scrape_configs:
  - job_name: dell-ddae
    scrape_interval: 30s
    static_configs:
      - targets: [127.0.0.1:9469]
        labels:
          site: example-site
          environment: production
```

Remote scrape through an mTLS proxy:

```yaml
scrape_configs:
  - job_name: dell-ddae
    scheme: https
    tls_config:
      ca_file: /etc/prometheus/trust/exporter-ca.pem
      cert_file: /etc/prometheus/certs/prometheus.pem
      key_file: /etc/prometheus/certs/prometheus-key.pem
      min_version: TLS12
    static_configs:
      - targets: [ddae-exporter-proxy.example.invalid:9443]
        labels:
          site: example-site
          environment: production
```

## Operations

### Startup checks

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

| Area | Metrics or evidence |
|---|---|
| Process | `/healthz`, `ddae_build_info` |
| Enabled mode | `ddae_monitoring_enabled` |
| Resources | `ddae_up`, `ddae_collector_success`, `ddae_snapshot_age_seconds` |
| Alerts | `ddae_alert_list_complete`, `ddae_alert_pipeline_ready`, `ddae_alert_detail_deferred` |
| Alert delivery | `ddae_kafka_publish_success`, `ddae_kafka_buffered_events`, `ddae_kafka_events_failed_total` |
| Logs | `ddae_serviceability_log_list_complete`, `ddae_serviceability_log_pipeline_ready`, `ddae_serviceability_log_detail_deferred` |
| Log delivery | `ddae_serviceability_log_kafka_publish_success`, `ddae_serviceability_log_buffered_records`, `ddae_serviceability_log_records_failed_total` |

### Path migration from RC2

RC2 used `/rest/v1` for both Ping and the remaining API operations. Preserve that
route layout during an RC3 upgrade with:

```yaml
ddae:
  paths:
    ping_prefix: /rest/v1
    api_prefix: /rest/v1
```

Change the prefixes only after confirming the routes exposed by the target DDAE
gateway. Restart the exporter after a configuration change.

### State and recovery

`state.db` and `serviceability-logs.db` each require one writer. Stop the exporter
before copying, restoring, or moving state files. Preserve both databases during
Kafka outages so pending records can replay after service recovery.

Detailed incident response, outbox recovery, and rollback procedures are in
[`docs/runbook.md`](docs/runbook.md).

## Development

### Commands

| Purpose | Command |
|---|---|
| Lint and vet | `./scripts/stage-lint.sh` |
| Race-enabled tests | `./scripts/stage-test.sh` |
| Coverage gate | `./scripts/stage-coverage.sh` |
| Build | `./scripts/build.sh` |
| Security policy | `./scripts/security-policy.sh` |
| CI policy | `./scripts/stage-ci-policy.sh` |
| Supply-chain artifacts | `./scripts/supply-chain.sh` |

### RC3 local validation

| Check | Result |
|---|---|
| Lint, vet, formatting | Passed |
| Race-enabled unit/component tests | Passed |
| Coverage | 81.5%; threshold 80% |
| Build | Passed |
| Security policy | Passed |
| `govulncheck` v1.7.0 | `No vulnerabilities found.` |
| Reproducible build and CycloneDX SBOM | Passed |
| External authenticated integration and E2E | Retained as later release gates |

## License

Licensed under the [Apache License 2.0](LICENSE). Dell and related product names
are trademarks of their respective owners. The project name identifies the target
system and does not imply endorsement.
