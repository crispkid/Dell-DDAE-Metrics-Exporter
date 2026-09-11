# Specification

## DDAE-1 Deliver a Go DDAE monitoring exporter with Prometheus and Kafka outputs

### Specification Metadata

- Specification Version: `1.0.0`
- Status: active
- Owner: `Peter Chu`
- Clarification Status: resolved
- Approval Status: Governed exclusively by the digest-bound named records in
  `DEVELOPMENT_PLAN.md`; this metadata line does not itself assert approval.
- Target implementation language: Go
- Initial Dell compatibility target: Dell Data Lakehouse System Software 1.5.0
  Management API v1

Version this specification according to `PROJECT.md`: patch for a
non-behavioral clarification, minor for backward-compatible behavior, and
major for incompatible behavior or removal. Do not reuse a released or
verified requirement or acceptance ID for different behavior.

### Executive Summary

v1 是一個單一 DDAE target 的唯讀 Go Exporter，使用 Dell Data Lakehouse System
Software 1.5.0 Management API v1。它以背景 polling 收集資料，不讓 Prometheus
scrape 直接觸發 DDAE fan-out。

| 資料範圍 | DDAE 來源 | v1 輸出 |
|---|---|---|
| Cluster 狀態 | `GET /rest/v1/ddae-clusters` | Prometheus |
| Node 狀態與容量 | `GET /rest/v1/infrastructure-nodes` | Prometheus |
| Appliance operability | `GET /rest/v1/system-lock`、`GET /rest/v1/system-shutdown` | Prometheus |
| Coordinator/Worker 配置 CPU、Memory | `GET /rest/v1/ddae-clusters` | Prometheus |
| 告警清單與詳細內容 | `GET /rest/v1/serviceability-issues`、`GET /rest/v1/serviceability-issues/{id}` | Kafka，後續由既有流程寫入 OpenSearch 並發出告警 |

v1 不收集 live CPU/Memory utilization、query performance、SQL/JMX/table data，也
不把告警內容送進 Prometheus。所有 DDAE 存取都是 GET allowlist；告警確認 `PATCH`
與其他 mutation 永遠禁止。Kafka 採 at-least-once、穩定 record key、下游
idempotent upsert 與 bounded durable outbox。

Dell 1.5.0 文件沒有個別 service health endpoint。為避免誤導，v1 將原需求中的
「服務狀態」明確收斂為 appliance lock、power、control-plane/node readiness，並以
精確的 appliance/node metric 名稱呈現；個別服務健康不屬於 v1 支援範圍。

### Source Basis

#### Dell source

- Document: *Dell Data Lakehouse System Software 1.5.0 - API Documentation*
- PDF pages: 147
- SHA-256:
  `d96dd1bbcee34b7853391a4be3913cff8244ae7f29aab10e46cbcc0ed11812d7`
- Relevant evidence:
  - REST base URL, API v1, forward compatibility guidance: PDF pp. 12-13.
  - Password-grant token acquisition and bearer authentication: PDF pp. 13-15.
  - DDAE cluster endpoints and status/configuration fields: PDF pp. 21-24 and
    schemas pp. 74-80.
  - Infrastructure node state, capacity, allocatable resources and conditions:
    PDF pp. 36-38 and schemas pp. 64-67.
  - Installed-component API exposes application name/version but no documented
    service-health field: PDF p. 37 and schema p. 67.
  - Serviceability alert list, summary and detail fields: PDF pp. 45-47 and
    schemas pp. 84-89.
  - Appliance lock and power/node readiness status, which are not explicitly
    documented as individual service health: PDF pp. 54-55 and schemas
    pp. 117-118 and 122.
  - Sensitive response risk: object-storage fields on p. 36, DDAE cluster
    configuration examples on pp. 76 and 80, serviceability alert messages,
    reasons, remedies, labels, links and identifiers on pp. 45-47 and 84-88,
    and SupportAssist fields on pp. 49-50 and 118-120.

The Dell document is an API reference, not an instruction to disable TLS or to
use administrator credentials in production. Its sample `curl -k` commands are
evidence of endpoint shape only. This specification's security requirements
govern exporter behavior.

#### Prometheus and Go sources

- [Prometheus - Writing exporters](https://prometheus.io/docs/instrumenting/writing_exporters/)
- [Prometheus - Metric and label naming](https://prometheus.io/docs/practices/naming/)
- [Prometheus Go client library](https://github.com/prometheus/client_golang)
- [Go toolchain selection](https://go.dev/doc/toolchain)

### Evidence Limits and Documented Gaps

The requested scope contains five observation areas: DDAE cluster status,
infrastructure node status/capacity, service status, configured
coordinator/worker CPU and memory, and serviceability alerts. Dell 1.5.0
documents direct Management API fields for four of them, but it does not define
an individual-service health/status endpoint or service-state model. v1
therefore defines its service-status boundary as appliance operability only:
lock, power, control-plane readiness and node readiness. These signals MUST NOT
be relabeled as individual service health. The first four
requested areas are routed to Prometheus; alert list and per-alert detail
records are routed exclusively to Kafka for an existing downstream
OpenSearch/alerting pipeline. The detail endpoint has a documented field schema,
but the document does not define Kafka event schema, list pagination/query
parameters, incremental change detection, alert identity or update/clear
lifecycle. It also does not document live CPU or memory
utilization, query latency/throughput, running or queued query counts, processed
bytes/rows, cache hit ratios, IOPS, or a Prometheus/JMX performance endpoint.
Those live-performance categories are explicitly outside this specification.
Capacity and configured resources MUST NOT be represented as live utilization.

The official document also contains these integration constraints:

- The token-call example uses `/rest/v1/ping`, while the Ping operation is
  printed as `GET /rest/ping` on p. 14.
- Several status strings, including DDAE `clusterStatus` and system shutdown
  `state`, have no complete enum in the document.
- The serviceability alert list response uses a weakly typed
  `results[*].additionalProperties` schema while the detail schema contains
  free-form messages, reasons, remedies, labels, links and identifiers.
- Error responses, token lifetime/refresh behavior, alert pagination/query
  parameters, alert update/clear semantics, rate limits and maximum response
  sizes are not documented.

v1 behavior for these constraints is defined below. Before release, sanitized
non-production integration evidence MUST confirm the actual route/shape,
authentication behavior, list completeness and list-to-detail relationship.
A mismatch is an unsupported DDAE 1.5.0 deployment, not permission to guess,
silently omit data or broaden the API allowlist.

### Problem Statement

Dell DDAE operators need Prometheus-compatible, bounded-cardinality metrics for
cluster status, infrastructure node status/capacity, appliance operability, and
configured coordinator/worker CPU and memory. Serviceability alert records must
instead be published to Kafka for an existing OpenSearch/alerting pipeline. The
official Management API returns JSON protected by bearer authentication, but it
provides neither a Prometheus exposition endpoint nor a Kafka event contract,
and alert responses can contain sensitive or free-form operational data.
Without a purpose-built exporter, neither output can receive a stable, safe and
observable contract.

### Goals

- Provide a production-oriented Go service that collects approved Dell DDAE
  observations through read-only interfaces and exposes Prometheus metrics.
- Expose exactly four selected DDAE data areas as Prometheus metrics: cluster
  status; infrastructure node status/capacity; appliance operability; and configured
  coordinator/worker CPU and memory. API reachability and exporter diagnostics
  are supporting observability rather than additional DDAE data areas.
- Collect serviceability alert list and per-alert detail records through the
  read-only Management API, normalize an approved detail-field allowlist into a
  versioned event envelope and publish them to Kafka for the existing
  downstream OpenSearch/alerting path.
- Protect credentials and sensitive DDAE response fields from metrics, labels,
  logs and retained evidence.
- Make partial failure, freshness, collection duration and target reachability
  observable without producing misleading current data.
- Preserve a stable metric contract suitable for dashboards, recording rules
  and alerts.

### Non-goals

- Mutating DDAE configuration or calling any DDAE `POST`, `PATCH`, `PUT` or
  `DELETE` operation.
- Acknowledging alerts, configuring acceleration, shutting down the appliance,
  rotating certificates, modifying resource allocation, or generating support
  bundles.
- Exporting raw DDAE responses or unapproved alert fields. Alert messages,
  remedies, identifiers, labels, links and nested events may be sent to Kafka
  only if the approved versioned event allowlist explicitly includes them.
- Exporting request IDs, Kerberos configuration, object-storage configuration,
  certificate bodies, SupportAssist credentials/PINs, contacts, or other
  personal data.
- Acting as a general reverse proxy to arbitrary URLs or accepting a scrape-time
  target supplied by an untrusted request.
- Replacing Dell administration tools, Prometheus alert rules, dashboards,
  long-term storage, or node_exporter.
- Publishing serviceability alert content as Prometheus metrics or labels.
- Owning OpenSearch indexing, downstream alert evaluation or notification;
  those remain responsibilities of the existing Kafka consumer pipeline.
- Collecting live CPU/memory utilization, query workload, query latency,
  throughput, processed bytes/rows, cache performance or JMX/SQL-table metrics.
- Collecting resource allocation, versions, installed components,
  acceleration, license, TLS, SupportAssist or serviceability log/event data.
- Claiming live utilization or query performance from configuration/capacity
  fields.

### Actors and Scenarios

- Actor `Prometheus` needs to scrape `/metrics` on a stable schedule and receive
  valid metric families within its configured timeout.
- Actor `DDAE operator` needs to see whether the Management API, clusters,
  nodes and documented appliance/control-plane readiness signals are available.
- Actor `SRE` needs bounded-cardinality cluster, node capacity, appliance-state,
  configured-resource and exporter/Kafka-pipeline-health metrics for dashboards.
- Actor `Kafka consumer` needs versioned, sanitized DDAE alert events containing
  the approved per-alert details, with a stable identity and explicit
  update/clear semantics for OpenSearch indexing and downstream alerting.
- Actor `Security operator` needs credentials, sensitive configuration and raw
  operational messages to remain absent from exporter output and logs.
- Actor `Maintainer` needs deterministic fixtures, explicit API compatibility
  and actionable errors when Dell changes optional fields or enums.

### v1 System Boundary

```text
runtime secret/config source
        |
        v
Go exporter -> DDAE token endpoint -> bearer token cache
        |
        +----> status/capacity/configuration collectors
        |          -> validated atomic snapshot
        |          -> Prometheus client_golang -> /metrics, /healthz, /readyz
        |
        +----> serviceability alert-list collector
                   -> bounded per-ID alert-detail GETs
                   -> typed detail-field allowlist -> versioned Kafka envelope
                   -> Kafka -> existing OpenSearch/alerting pipeline

bounded logs and exporter diagnostics -> Prometheus metrics and logs
```

v1 uses one configured DDAE target per exporter process and background
collection into atomic last-known snapshots. Prometheus scrapes read snapshots
only; alert polling/detail retrieval is a separate bounded background path.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-1-001 | must | The system MUST be implemented as a Go module, pin its minimum/preferred Go toolchain in `go.mod`, and use the official Prometheus Go client for exposition. | AC-DDAE-1-001 |
| REQ-DDAE-1-002 | must | The system MUST expose Prometheus metrics at `/metrics` and liveness/readiness endpoints at `/healthz` and `/readyz` on a configurable listen address. | AC-DDAE-1-002 |
| REQ-DDAE-1-003 | must | The DDAE client MUST call only an approved GET allowlist and MUST contain no product code capable of invoking a DDAE mutation endpoint. | AC-DDAE-1-003 |
| REQ-DDAE-1-004 | must | The client MUST support the documented DDAE bearer-token flow, inject credentials only at runtime, cache tokens safely, and never expose authentication material. | AC-DDAE-1-004, AC-DDAE-1-005 |
| REQ-DDAE-1-005 | must | TLS certificate and hostname verification MUST be enabled by default for DDAE and token connections; custom trust roots MUST be supported without changing the default. | AC-DDAE-1-006 |
| REQ-DDAE-1-006 | must | The exporter MUST expose exactly four DDAE data areas from Management API v1 as Prometheus metrics: cluster status; infrastructure node status/capacity; documented appliance operability; and configured coordinator/worker CPU and memory. API reachability and exporter/Kafka diagnostics MAY support those collectors. The exporter MUST NOT claim that appliance operability is individual-service health. | AC-DDAE-1-007 |
| REQ-DDAE-1-007 | must | The exporter MUST NOT collect live CPU/memory utilization, query workload/performance, resource-allocation, version, acceleration, component, license, TLS, SupportAssist, serviceability logs/events, alert summary or other data outside the five selected areas; configured or capacity quantities MUST NOT be represented as utilization, and serviceability alert list/detail records MUST be routed only to Kafka rather than exposed as Prometheus metric content. | AC-DDAE-1-008 |
| REQ-DDAE-1-008 | must | Every metric MUST have a stable name, help, Prometheus type, base unit, bounded labels, missing-value rule and source mapping before implementation. | AC-DDAE-1-009 |
| REQ-DDAE-1-009 | must | The exporter MUST decode only allowlisted fields into typed structures, ignore unknown JSON fields/enums safely, bound response size and never log or persist raw DDAE bodies. | AC-DDAE-1-010, AC-DDAE-1-011 |
| REQ-DDAE-1-010 | must | Metrics and labels MUST exclude secrets, personal data, endpoints/IP addresses, free-form text, alert/event IDs, timestamps as labels and other unbounded values; Kafka events MUST exclude every field not explicitly approved in their versioned allowlist. | AC-DDAE-1-012 |
| REQ-DDAE-1-011 | must | The exporter MUST bound request timeout, collection deadline, concurrency, retry/backoff and response size and MUST prevent overlapping collection cycles. | AC-DDAE-1-013 |
| REQ-DDAE-1-012 | must | Collector failures MUST be isolated; stale or partial data MUST be identifiable through freshness and per-collector success metrics and MUST NOT be represented as a current successful observation. | AC-DDAE-1-014, AC-DDAE-1-015 |
| REQ-DDAE-1-013 | must | Unknown or malformed optional fields MUST affect only their collector/output family unless they invalidate authentication or the entire target connection. | AC-DDAE-1-016 |
| REQ-DDAE-1-014 | must | Configuration MUST be deterministic, validate before serving, separate non-secret settings from secret inputs, include the approved Kafka producer settings, and redact secret-derived error text. | AC-DDAE-1-017 |
| REQ-DDAE-1-015 | must | Logs MUST be structured, bounded and actionable and MUST not include bearer tokens, passwords, DDAE/Kafka secrets, response bodies, Kafka event payloads or disallowed source fields. | AC-DDAE-1-018 |
| REQ-DDAE-1-016 | must | The process MUST handle shutdown signals, stop accepting work, cancel in-flight requests and exit within a configurable grace period. | AC-DDAE-1-019 |
| REQ-DDAE-1-017 | must | DDAE 1.5.0 API, Prometheus metric and Kafka event compatibility MUST be regression-tested with deterministic sanitized fixtures, including missing fields, unknown fields/enums and failures. | AC-DDAE-1-020 |
| REQ-DDAE-1-018 | must | Integration claims MUST use an authorized non-production DDAE system and isolated Kafka test broker/topic and MUST record the DDAE software/API version without retaining credentials or private data. | AC-DDAE-1-021 |
| REQ-DDAE-1-019 | should | The release SHOULD provide a reproducible static Go binary and OCI image from the same source revision, with version/build metadata exposed through an info metric. | AC-DDAE-1-022 |
| REQ-DDAE-1-020 | must | Operator documentation MUST define configuration, secret injection, supported versions, metrics, collection intervals, Prometheus scrape configuration, Kafka connectivity/event compatibility, troubleshooting and upgrade behavior. | AC-DDAE-1-023 |
| REQ-DDAE-1-021 | must | The exporter MUST discover serviceability alerts through `GET /rest/v1/serviceability-issues`, retrieve approved per-alert details through `GET /rest/v1/serviceability-issues/{id}`, normalize the approved typed detail-field allowlist into a versioned event envelope, and publish it to the configured Kafka topic without acknowledging or otherwise mutating DDAE; alert content MUST NOT be exposed as Prometheus metrics or labels. | AC-DDAE-1-024 |
| REQ-DDAE-1-022 | must | Kafka broker/topic, TLS and authentication settings MUST be runtime-injected; producer timeouts, retries, concurrency and buffering MUST be bounded; and publish failure MUST be observable without silently reporting delivery success or silently discarding an alert event. | AC-DDAE-1-025 |
| REQ-DDAE-1-023 | must | The Kafka alert contract MUST define a stable schema version, DDAE source identity, event key, occurrence/update/clear semantics, timestamps and approved fields; raw bodies and unapproved nested or free-form fields MUST be absent. | AC-DDAE-1-026, AC-DDAE-1-027 |
| REQ-DDAE-1-024 | must | Every alert-detail request MUST derive its path ID only from a validated alert-list result, encode it as one path segment, enforce approved per-cycle request/concurrency/response limits, validate that the returned alert ID matches the requested ID, and follow the approved new/update/refresh policy without fabricating or silently publishing an incomplete detail event. | AC-DDAE-1-028, AC-DDAE-1-029 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-1-001 | REQ-DDAE-1-001 | A clean supported build environment | The project is built and tested | `go.mod` identifies the module and toolchain requirements, dependencies are locked in `go.sum`, and the exporter builds using the approved Go toolchain. |
| AC-DDAE-1-002 | REQ-DDAE-1-002 | A valid exporter configuration | The process starts | `/metrics` returns a Prometheus-compatible response, `/healthz` reports process liveness, and `/readyz` reflects availability of a usable current snapshot. |
| AC-DDAE-1-003 | REQ-DDAE-1-003 | Static analysis and an HTTP test server | Every DDAE request path and method is exercised | Only approved GET paths are observable and mutation methods/paths are absent from production code. |
| AC-DDAE-1-004 | REQ-DDAE-1-004 | Valid runtime username, password and client secret | Authentication is required | The client obtains a bearer token from the configured DDAE realm token endpoint and sends it only in the Authorization header to the configured target. |
| AC-DDAE-1-005 | REQ-DDAE-1-004 | Expired or rejected credentials | A collection is attempted | The exporter reports an authentication-class failure without logging any credential/token and follows the approved token renewal policy. |
| AC-DDAE-1-006 | REQ-DDAE-1-005 | An untrusted or hostname-mismatched certificate | The exporter connects with default settings | The connection fails closed; when an approved CA file is configured, the same valid chain succeeds without disabling verification. |
| AC-DDAE-1-007 | REQ-DDAE-1-006 | Sanitized Dell 1.5.0 fixtures for cluster, node, lock and power/readiness | A collection cycle completes | Metrics cover cluster state, node state/capacity, documented appliance operability, and configured coordinator/worker CPU/memory with the specified source fields and units; no output claims individual-service health, and missing optional fields are omitted or mapped according to contract. |
| AC-DDAE-1-008 | REQ-DDAE-1-007 | Production code, configuration, fixtures, gathered metrics and emitted events | The selected input and output surfaces are inspected | No SQL/JMX/performance source or out-of-scope Management API collector exists, no out-of-scope metric family is emitted, alert content is absent from Prometheus, and configured/capacity values are named and documented without utilization semantics. |
| AC-DDAE-1-009 | REQ-DDAE-1-008 | The committed metric contract | Metrics are linted and gathered | Names, help, types, units and label sets match exactly and duplicate/conflicting series are rejected. |
| AC-DDAE-1-010 | REQ-DDAE-1-009 | A response containing documented sensitive fields plus unknown fields | It is decoded and collected | Only approved typed fields affect metrics or Kafka events; sensitive/unknown values are absent from logs, metrics and artifacts and are absent from Kafka unless explicitly approved by the event allowlist. |
| AC-DDAE-1-011 | REQ-DDAE-1-009 | An oversized or malformed response | It is read | The request is aborted at the configured limit and only the affected collector reports a bounded decode/validation failure. |
| AC-DDAE-1-012 | REQ-DDAE-1-010 | Fixtures containing IPs, messages, event IDs, timestamps, credentials and contact data | Metrics, logs and Kafka events are inspected | None of those values appears in metric labels, help, log attributes or retained evidence; a Kafka event contains only fields explicitly permitted by its approved versioned allowlist and never contains credentials or contact data. |
| AC-DDAE-1-013 | REQ-DDAE-1-011 | A slow, failing or retryable test server and concurrent scrape requests | Collection and scrapes run | Deadlines and retry limits are respected, concurrency remains bounded, no collection cycle overlaps, and workers/goroutines return. |
| AC-DDAE-1-014 | REQ-DDAE-1-012 | One failed collector and other successful collectors | `/metrics` is scraped | Successful families remain available, the failed collector reports success `0`, and the target/snapshot status follows the approved partial-failure policy. |
| AC-DDAE-1-015 | REQ-DDAE-1-012 | A previously successful snapshot followed by collection failure | The snapshot exceeds its maximum age | Metrics expose the actual snapshot age and readiness becomes false; stale values are not presented without a stale/failure signal. |
| AC-DDAE-1-016 | REQ-DDAE-1-013 | Added fields, missing optional fields and unknown status values | Fixtures are collected | The process does not crash; known safe data remains available and unknown states map only to the bounded `unknown` representation. |
| AC-DDAE-1-017 | REQ-DDAE-1-014 | Missing, conflicting or invalid settings | The process starts | It exits non-zero before serving and names the invalid setting without printing its secret value. |
| AC-DDAE-1-018 | REQ-DDAE-1-015 | Success and every defined DDAE/Kafka failure class | Logs are captured | Logs contain timestamp, level, collector/output and bounded failure class but no raw body, Kafka event payload or secret-bearing value. |
| AC-DDAE-1-019 | REQ-DDAE-1-016 | In-flight collection | SIGTERM is delivered | Requests are canceled, listeners close, no new cycle begins and the process exits within the configured grace period. |
| AC-DDAE-1-020 | REQ-DDAE-1-017 | The full sanitized fixture suite | Unit/component tests run repeatedly | Prometheus and Kafka outputs are deterministic and cover documented enums, optional/malformed fields, token failure, TLS failure, timeout, partial failure and sensitive-field rejection. |
| AC-DDAE-1-021 | REQ-DDAE-1-018 | An explicitly authorized non-production DDAE 1.5.0 environment and isolated Kafka test broker/topic | Integration tests run | The real token and selected GET endpoints and Kafka publish path succeed or record a truthful blocked/failure result, and retained evidence contains no credentials, private response body or production alert data. |
| AC-DDAE-1-022 | REQ-DDAE-1-019 | A release revision | Binary and OCI artifacts are built twice in approved clean environments | Both use the same source/version metadata and satisfy the approved reproducibility/checksum policy. |
| AC-DDAE-1-023 | REQ-DDAE-1-020 | A new operator with documented prerequisites | The runbook is followed | The exporter can be configured, scraped and connected to the isolated Kafka topic without undocumented credentials, paths, defaults, metric semantics or event compatibility rules. |
| AC-DDAE-1-024 | REQ-DDAE-1-021 | Sanitized serviceability alert-list and per-ID detail fixtures plus a Kafka test producer | Alert collection completes | The approved detail GET is made for each alert selected by the lifecycle policy; exactly the corresponding typed detail events are published with the approved envelope; no alert-content metric series is emitted; and no DDAE mutation request occurs. |
| AC-DDAE-1-025 | REQ-DDAE-1-022 | Unavailable, slow, TLS-invalid and authentication-rejecting Kafka fixtures | The exporter attempts delivery | Approved timeout, retry, concurrency and buffer limits are respected; producer failure is visible through bounded metrics/readiness/log signals; and the exporter never reports an unacknowledged publish as successful or silently drops it. |
| AC-DDAE-1-026 | REQ-DDAE-1-023 | An alert fixture containing all documented, unknown, nested, free-form and sensitive fields | A Kafka event is encoded | The event matches the exact approved schema version and field allowlist; raw response data and unapproved fields are absent. |
| AC-DDAE-1-027 | REQ-DDAE-1-023 | Duplicate, updated, cleared and post-restart alert fixtures | Events are collected and published | Event keys and occurrence/update/clear/replay behavior deterministically match the approved lifecycle and downstream idempotency contract. |
| AC-DDAE-1-028 | REQ-DDAE-1-024 | Alert-list fixtures containing valid, duplicate, oversized and path-manipulation IDs plus detail responses with matching/mismatching IDs | Detail retrieval runs | Only valid IDs are encoded as one path segment, duplicates are coalesced, request/concurrency/response limits hold, mismatched response IDs fail validation, and no arbitrary DDAE path is called. |
| AC-DDAE-1-029 | REQ-DDAE-1-024 | New, unchanged, updated, detail-failing and post-restart alerts | Repeated collection cycles run | Detail fetch, refresh, checkpoint and publish behavior matches the approved policy; an unavailable or invalid detail never becomes a fabricated or silently incomplete successful Kafka event. |

### Dell Management API v1 Read Allowlist

Only the following paths are permitted in v1. The Ping route is fixed to the
versioned path used by Dell's token-call example. Non-production integration
MUST confirm it before release; the exporter MUST fail compatibility validation
rather than fall back to an unapproved path.

| Collector | Method and path | Safe fields used | Purpose | Default |
|---|---|---|---|---|
| `ping` | `GET /rest/v1/ping` | `status` | Management API reachability | core |
| `clusters` | `GET /rest/v1/ddae-clusters` | `id`, `clusterStatus`, coordinator/worker CPU and memory quantities | DDAE status and configured resources | core |
| `nodes` | `GET /rest/v1/infrastructure-nodes` | `id`, `state`, capacity/allocatable/conditions fields | Node readiness and resource capacity | core |
| `lock` | `GET /rest/v1/system-lock` | `status` | Appliance lock state | core |
| `power` | `GET /rest/v1/system-shutdown` | `controlPlaneReady`, `nodesReady`, `totalNodes` | Power/readiness state | core |
| `alert_list` | `GET /rest/v1/serviceability-issues` | `results[*].id`; optional `results[*].updatedon`; `threshold`; `totalRecords` | Alert discovery and lifecycle input | core |
| `alert_detail` | `GET /rest/v1/serviceability-issues/{id}` | Exact v1 Kafka allowlist below | Versioned detailed alert events to Kafka | core |

All other Management API endpoints are denied unless added by an approved
specification amendment. In particular, the exporter MUST NOT call object
storage, resource allocation, system versions, acceleration, installed
components, licenses, TLS certificates, SupportAssist, support bundle,
serviceability log/event list or detail, serviceability alert summary, catalog
configuration, connector configuration, certificate body, DR configuration or
any mutation endpoint. In particular, the documented alert acknowledgement
`PATCH` operation is prohibited even though the alert detail `GET` is allowed.

The `lock` and `power` collectors expose only their documented appliance/node
signals. They satisfy the v1 appliance-operability scope but MUST NOT be
described as authoritative individual-service health.

### v1 Prometheus Metric Contract

This table is the v1 mapping for official Management API fields and bounded
exporter/Kafka-producer diagnostics. All `state`, `condition`, `collector` and
`reason` values MUST come from committed bounded sets; unrecognized values map
to `unknown` rather than becoming arbitrary new label values. No alert content
or alert count is part of the Prometheus metric contract.

| Metric | Type | Labels | Source | Semantics |
|---|---|---|---|---|
| `ddae_up` | gauge | none | Required core collectors | `1` only when the approved target-success policy is satisfied by a current snapshot; otherwise `0`. |
| `ddae_collector_success` | gauge | `collector` | Exporter | Last attempted collector cycle succeeded (`1`) or failed (`0`). |
| `ddae_collector_duration_seconds` | gauge | `collector` | Exporter | Duration of the last collector attempt in seconds. |
| `ddae_snapshot_age_seconds` | gauge | none | Exporter | Seconds since the newest successfully published required snapshot. |
| `ddae_build_info` | gauge | `version`, `go_version` | Exporter build | Constant `1`; changes only when the exporter build changes. |
| `ddae_management_api_up` | gauge | none | Ping | Documented reachability status and successful authenticated request. |
| `ddae_cluster_state_info` | gauge | `cluster`, `state` | DDAE clusters | One-hot state using `available` or `unknown`; `cluster` is Dell cluster `id`. |
| `ddae_cluster_coordinator_configured_cpu_cores` | gauge | `cluster` | DDAE clusters | Configured coordinator CPU cores, not usage. |
| `ddae_cluster_coordinator_configured_memory_bytes` | gauge | `cluster` | DDAE clusters | Configured coordinator memory converted from a validated quantity to bytes. |
| `ddae_cluster_worker_configured_cpu_cores` | gauge | `cluster` | DDAE clusters | CPU quantity returned by the cluster `worker` configuration object; no aggregate or utilization meaning is implied. |
| `ddae_cluster_worker_configured_memory_bytes` | gauge | `cluster` | DDAE clusters | Memory quantity returned by the cluster `worker` configuration object, converted to bytes; no aggregate or utilization meaning is implied. |
| `ddae_node_state_info` | gauge | `node`, `state` | Infrastructure nodes | One-hot state from the documented bounded node-state enum; `node` is Dell node `id`. |
| `ddae_node_ready` | gauge | `node` | Infrastructure nodes | `1` only for documented `Ready`; `0` for other known states. |
| `ddae_node_capacity_cpu_cores` | gauge | `node` | Infrastructure nodes | Total capacity in CPU cores, not utilization. |
| `ddae_node_capacity_memory_bytes` | gauge | `node` | Infrastructure nodes | Total memory capacity converted to bytes. |
| `ddae_node_capacity_ephemeral_storage_bytes` | gauge | `node` | Infrastructure nodes | Total ephemeral storage capacity converted to bytes. |
| `ddae_node_allocatable_cpu_cores` | gauge | `node` | Infrastructure nodes | Allocatable CPU cores, not current unused CPU. |
| `ddae_node_allocatable_memory_bytes` | gauge | `node` | Infrastructure nodes | Allocatable memory bytes, not current unused memory. |
| `ddae_node_allocatable_ephemeral_storage_bytes` | gauge | `node` | Infrastructure nodes | Allocatable ephemeral storage bytes. |
| `ddae_node_condition` | gauge | `node`, `condition` | Infrastructure nodes | Boolean condition for the fixed set `disk_pressure`, `memory_pressure`. |
| `ddae_system_locked` | gauge | none | System lock | `1` when the appliance is locked by another job. |
| `ddae_control_plane_ready` | gauge | none | System shutdown status | Boolean readiness of control-plane nodes. |
| `ddae_nodes_ready` | gauge | none | System shutdown status | Current ready-node count. |
| `ddae_nodes_total` | gauge | none | System shutdown status | Current total-node count. |
| `ddae_alert_list_complete` | gauge | none | Alert-list collector | `1` only when the returned list is structurally valid and `totalRecords` is not greater than the usable returned result count. |
| `ddae_alert_detail_deferred` | gauge | none | Alert-detail collector | Number of still-listed alert IDs deferred after the last cycle because of the configured request cap. |
| `ddae_alert_pipeline_ready` | gauge | none | Alert pipeline | `1` only when list/detail collection, persistent state and outbox-capacity requirements are satisfied. |
| `ddae_kafka_publish_success` | gauge | none | Kafka producer | Last required Kafka publish batch was acknowledged (`1`) or failed (`0`). |
| `ddae_kafka_publish_duration_seconds` | gauge | none | Kafka producer | Duration of the last required Kafka publish batch in seconds. |
| `ddae_kafka_events_published_total` | counter | none | Kafka producer | Total events acknowledged by Kafka; producer retry attempts do not increment it. |
| `ddae_kafka_events_failed_total` | counter | `reason` | Kafka producer | Total events whose approved delivery policy ended in failure, using a fixed reason set. |
| `ddae_kafka_buffered_events` | gauge | none | Kafka producer | Current number of events retained for retry under the approved bounded buffering policy. |

Prometheus target labels such as site, environment, region or deployment name
MUST be attached by Prometheus service discovery/relabeling, not copied onto
every exporter metric.

The `collector` label has the fixed v1 set `ping`, `clusters`, `nodes`, `lock`,
`power`, `alert_list` and `alert_detail`. The `reason` label has only the fixed
error classes defined under Observability. Neither set may include a URL,
exception/error text, alert ID, Kafka topic or source field.

Metric identity and normalization are fixed as follows:

- `cluster` is the non-empty Dell cluster `id`; `node` is the non-empty Dell
  node `id`. A missing or duplicate ID fails the affected collector rather than
  falling back to a mutable name or address.
- Cluster state is case-insensitively normalized to `available`; every other
  value maps to `unknown` until a later version adds a bounded value.
- Node state uses `maintenance_mode`, `scheduling_disabled`, `not_ready`,
  `ready`, `restarting`, `shutting_down`, `powered_off`, `powering_on` and
  `unknown`.
- Node conditions parse documented `True`/`False` strings case-insensitively;
  missing or invalid present values are omitted and make the collector
  incomplete. `disk_pressure=1` means pressure exists;
  `memory_pressure=1` means pressure exists.
- CPU values are cores. Memory and ephemeral-storage quantities are parsed as
  Kubernetes quantities and exposed in bytes. Missing values are omitted, not
  converted to zero.

### v1 Kafka Alert Event Contract

The exporter MUST construct a new typed JSON event from the list and detail
responses. It MUST NOT forward a complete DDAE response or use a generic
`map[string]any`. The UTF-8 encoded event MUST be no larger than 256 KiB; a
source value that violates a bound or type causes an observable validation
failure rather than truncation or a partially successful event.

| Field | JSON type | Required | v1 rule/source |
|---|---|---|---|
| `schema_version` | string | yes | Constant `1.0`. |
| `event_type` | string | yes | Constant `ddae.serviceability_alert.upsert`. |
| `source_system` | string | yes | Constant `dell_ddae`. |
| `source_instance` | string | yes | Configured stable ID, 1-128 UTF-8 bytes, excluding NUL, URL and credentials. |
| `alert_id` | string | yes | Detail `id`, 1-256 UTF-8 bytes, excluding NUL; must match the requested list ID. |
| `content_hash_sha256` | string | yes | Lowercase SHA-256 hex of the canonical normalized `alert` object. |
| `observed_at` | string | yes | Exporter observation time normalized to UTC RFC 3339. |
| `alert` | object | yes | Typed detail object defined below. |

The Kafka record key MUST be lowercase SHA-256 hex over UTF-8
`source_instance`, one NUL byte and UTF-8 `alert_id`. Required headers are
`content-type=application/json` and `ddae-schema-version=1.0`; alert content,
identifiers and credentials MUST NOT appear in other headers.

The `alert` object permits only these fields. Every Dell field is optional in
the source except that a usable event requires a matching `id`; absent optional
fields are omitted, while present invalid fields fail the event.

| Field | JSON type/bounds | Dell source and normalization |
|---|---|---|
| `severity` | enum `critical`, `error`, `warning`, `info`, `normal`, `unknown` | Lowercase mapping of `type`; other or absent values become `unknown`. |
| `acknowledged` | boolean | Strict case-insensitive conversion of documented string `acknowledged`; invalid values fail validation. |
| `occurrence_count` | integer, >= 0 | `count`. |
| `created_at` | RFC 3339 UTC string | Parsed `createdon`; an invalid present timestamp fails validation. |
| `updated_at` | RFC 3339 UTC string | Parsed `updatedon`; an invalid present timestamp fails validation. |
| `clear_type` | string, <= 64 bytes | `clearType`; source semantics remain Dell-defined and are not used as a clear signal. |
| `auto_clear_timeout_raw` | integer, >= 0 | `autoClearTimeOut`; the name deliberately carries no unit because Dell 1.5.0 does not document one. |
| `app_name` | string, <= 256 bytes | `appname`. |
| `component` | string, <= 256 bytes | `component`. |
| `namespace` | string, <= 256 bytes | `namespace`. |
| `message` | string, <= 8192 bytes | `message`; confidential free-form operational data. |
| `reason` | string, <= 4096 bytes | `reason`; confidential free-form operational data. |
| `remedies` | array, <= 32 strings of <= 2048 bytes | `remedies`. |
| `resource_id` | string, <= 512 bytes | `resourceID`. |
| `symptom_id` | string, <= 256 bytes | `symptomid`. |
| `related` | string, <= 512 bytes | `related`; no additional semantics are inferred. |
| `related_events` | array, <= 100 typed objects | `events`; each object may contain the same fields above except `related_events`, so nesting is exactly one level. |

Dell `labels` and `links`, including those inside nested `events`, are excluded
from v1 because their internal schemas are not documented. Unknown fields are
ignored. Alert payloads are confidential operational data and may appear only
in the approved Kafka topic; they MUST NOT enter Prometheus, logs or retained
test evidence.

Kafka delivery is at-least-once with producer idempotence and `acks=all`.
Records for a given key MUST retain order. The existing consumer MUST perform an
idempotent OpenSearch upsert by record key. The exporter publishes a new record
when a new alert is observed or the canonical `alert` hash changes; an unchanged
periodic refresh does not publish. v1 does not synthesize a cleared/deleted
event when an alert disappears from the list because Dell 1.5.0 defines no such
lifecycle contract.

The durable outbox has no automatic expiry and MUST replay retained records
after restart in per-key order. At either configured hard limit, the exporter
MUST reject new outbox writes, keep existing records, stop advancing affected
checkpoints, set alert-pipeline readiness to `0` and require operator recovery;
it MUST NOT evict or overwrite an unacknowledged event. Checkpoints for alerts
absent from the list are deleted after the configured retention, oldest absent
records first, while checkpoints for currently listed alerts are never removed
to satisfy the count limit.

### Alert List and Detail Retrieval

- Poll `GET /rest/v1/serviceability-issues` every 30 seconds by default on a
  configurable background schedule; Prometheus scrapes MUST NOT trigger detail
  fan-out.
- Validate and deduplicate alert IDs before constructing a detail path. An ID
  is data, not a URL/path; it MUST be encoded as exactly one path segment, and
  the returned detail object's `id` MUST match the requested ID.
- Fetch `GET /rest/v1/serviceability-issues/{id}` immediately for a new ID or a
  changed usable `updatedon` marker. Refresh every still-listed alert detail at
  least once every 10 minutes even when no usable marker exists. The defaults
  are at most 200 detail requests per cycle and four concurrent detail requests.
- When a cycle exceeds the request cap, process new/changed IDs first, then the
  oldest periodic refreshes; defer the remainder deterministically and expose
  the deferred count. No ID may starve indefinitely while it remains listed.
- If `totalRecords` is greater than the number of returned `results`, mark
  `alert_list` incomplete and readiness degraded. Process safe returned IDs but
  do not claim list completeness; release support requires integration evidence
  that the configured DDAE/query behavior can enumerate the full list.
- A missing, malformed or failed detail response is an observable alert-detail
  collection failure. The exporter MUST NOT fabricate missing detail fields or
  mark an incomplete event as successfully published.
- Build the Kafka event from a new typed structure. Do not retain or forward the
  raw list/detail response body.
- Persist the normalized event to the durable outbox before advancing its
  content-hash checkpoint. Advance delivered state only after Kafka
  acknowledgement; a restart may replay a record but MUST NOT lose it silently.

### Explicitly Excluded Performance Data

This specification deliberately excludes live CPU/memory utilization, query
concurrency/outcomes/latency, processed rows/bytes, cache performance and
per-service runtime consumption. Adding any such category requires a later
approved specification change with an authoritative source, exact field/unit
semantics, bounded labels and new acceptance coverage. Configured CPU/memory
and node capacity/allocatable quantities remain configuration/capacity metrics
and MUST NOT use names, help text or alerts that imply live utilization.

### Interfaces, Data, and Failure Behavior

#### Exporter HTTP interface

- `GET /metrics`: Prometheus exposition. The final OpenMetrics/text negotiation
  behavior is delegated to the pinned official Go client.
- `GET /healthz`: process liveness only; MUST NOT depend on DDAE availability.
- `GET /readyz`: `200` only when configuration is valid and the approved current
  snapshot policy is satisfied; otherwise `503` with a bounded plain response.
- All other paths return `404` unless added by an approved specification.
- HTTP server read-header, read, write and idle timeouts MUST be explicit.
- Debug/profiling endpoints MUST be disabled by default and MUST NOT be exposed
  on the public listener without an approved security design.

#### Configuration contract

v1 configuration uses environment variables, with `_FILE` variants for secret
values. Durations use Go duration syntax and byte limits use positive integer
bytes. Invalid, conflicting or missing required settings fail before serving.

| Setting | Required/default | Contract |
|---|---|---|
| `DDAE_BASE_URL` | required | HTTPS origin only; no user info, query or fragment. |
| `DDAE_SOURCE_INSTANCE` | required | Stable event identity, 1-128 bytes, not a URL or secret. |
| `DDAE_USERNAME` / `DDAE_USERNAME_FILE` | required secret | Dedicated read-only account. |
| `DDAE_PASSWORD` / `DDAE_PASSWORD_FILE` | required secret | Password-grant secret. |
| `DDAE_CLIENT_SECRET` / `DDAE_CLIENT_SECRET_FILE` | required secret | `dv-admin-rest` client secret. |
| `DDAE_CA_FILE` | system roots | Optional additional CA bundle; TLS verification cannot be disabled. |
| `EXPORTER_LISTEN_ADDRESS` | `127.0.0.1:9469` | HTTP listener; non-loopback exposure requires an approved private-network or external mTLS boundary. |
| `DDAE_COLLECTION_INTERVAL` | `30s` | Background status/capacity/configuration and alert-list interval. |
| `DDAE_REQUEST_TIMEOUT` | `5s` | Per DDAE/token request, bounded by cycle timeout. |
| `DDAE_CYCLE_TIMEOUT` | `20s` | Aggregate collection-cycle deadline. |
| `DDAE_RESPONSE_MAX_BYTES` | `4194304` | Default response-body limit. |
| `DDAE_RETRY_MAX` | `2` | Maximum retry attempts after the initial safe request. |
| `DDAE_STALE_AFTER` | `120s` | Maximum age for required Prometheus snapshots. |
| `ALERT_LIST_RESPONSE_MAX_BYTES` | `8388608` | Alert-list response limit. |
| `ALERT_DETAIL_RESPONSE_MAX_BYTES` | `1048576` | Per-detail response limit. |
| `ALERT_DETAIL_REFRESH_INTERVAL` | `10m` | Maximum interval between detail refreshes for a still-listed alert. |
| `ALERT_DETAIL_MAX_PER_CYCLE` | `200` | Bounded detail fan-out per cycle. |
| `ALERT_DETAIL_CONCURRENCY` | `4` | Concurrent detail requests. |
| `KAFKA_BROKERS` | required | Comma-separated configured brokers; confidential operational metadata. |
| `KAFKA_TOPIC` | required | Existing pipeline's dedicated alert topic; no default. |
| `KAFKA_CLIENT_ID` | `ddae-exporter` | Bounded client identity. |
| `KAFKA_CA_FILE` | system roots | Optional additional broker CA bundle. |
| `KAFKA_CLIENT_CERT_FILE` / `KAFKA_CLIENT_KEY_FILE` | deployment-dependent | mTLS pair; key is secret. |
| `KAFKA_SASL_MECHANISM` | unset | Only a mechanism supported by the selected reviewed Kafka client. |
| `KAFKA_SASL_USERNAME` / `KAFKA_SASL_PASSWORD_FILE` | deployment-dependent | Runtime SASL credentials; password is secret. |
| `KAFKA_PUBLISH_TIMEOUT` | `10s` | Per publish acknowledgement deadline. |
| `STATE_DIR` | `/var/lib/ddae-exporter` | Writable mode-0700 volume for checkpoint and durable outbox. |
| `KAFKA_OUTBOX_MAX_BYTES` | `1073741824` | Hard durable-outbox byte limit. |
| `KAFKA_OUTBOX_MAX_EVENTS` | `100000` | Hard durable-outbox record limit. |
| `CHECKPOINT_RETENTION` | `720h` | Retain last hash/observation for 30 days after an alert disappears. |
| `CHECKPOINT_MAX_ALERTS` | `100000` | Hard bound on retained alert checkpoints. |
| `SHUTDOWN_GRACE_PERIOD` | `15s` | Cancellation, final state sync and listener shutdown budget. |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `json` | Bounded structured logs; payload logging is never enabled. |

The collector set and output routes are compiled contracts, not runtime
switches: four data areas go to Prometheus and detailed serviceability alerts go
to Kafka. No setting may enable an out-of-scope endpoint or route alert content
into metrics.

Secret values MUST come from runtime environment variables or files mounted by
the deployment secret mechanism. Secret values MUST NOT be accepted through
ordinary command-line arguments because process listings may expose them.
Setting both a direct secret and its `_FILE` form is invalid; file values remove
at most one trailing line ending and otherwise remain unchanged.
Kafka broker and topic names are operational metadata and MUST NOT be exported
as Prometheus labels or logged at unrestricted verbosity.

#### Token behavior

- The documented token endpoint is
  `/auth/realms/ddae/protocol/openid-connect/token` with client ID
  `dv-admin-rest`, password grant, client secret, username and password.
- The token MUST be held only in memory, excluded from equality-comparable
  config dumps, and replaced atomically on renewal.
- The exporter MUST not decode a JWT and treat unverified claims as
  authorization evidence. If the token response provides a positive documented
  lifetime, refresh is scheduled at the earlier of 80% of that lifetime or 60
  seconds before expiry. Without a usable lifetime, retain the token until the
  server rejects it.
- Concurrent collectors MUST share one renewal operation rather than issuing a
  token request per collector.
- On the first `401`, perform one coalesced token renewal and retry the original
  safe request once. A second `401` is an authentication failure until the next
  scheduled cycle; no recursive or rapid retry is allowed.
- Authentication failure MUST not cause unbounded rapid retries.

#### JSON and sensitive data handling

- Use typed response structs containing only fields used by the metric
  contract. Do not retain generic maps for complete responses.
- Unknown JSON fields are ignored for forward compatibility, consistent with
  Dell's p. 13 guidance, but response byte size and nesting MUST be bounded.
- No raw response body, request Authorization header, token form body, URL user
  info or secret-bearing configuration may be logged or retained.
- Alert IDs used in a detail request MUST originate from the validated list
  response, be length/character bounded, be encoded as one URL path segment and
  never be interpreted as an absolute/relative URL or additional path.
- Strings representing Kubernetes quantities MUST be parsed strictly and
  converted to Prometheus base units. Invalid quantities omit only the affected
  metric and fail the collector according to the approved policy.
- Missing optional source fields do not become zero unless the Dell contract
  explicitly defines absence as zero. Otherwise the metric is omitted and the
  collector diagnostic indicates incomplete data.
- Unknown enum values map to the single bounded label value `unknown`; the raw
  value may appear only in a locally bounded debug log after an explicit
  security review, and is prohibited by default.
- Alert event construction MUST use a distinct typed allowlist. Prometheus-safe
  field rules do not automatically authorize a field for Kafka, and Kafka
  authorization does not make a field safe for a metric label or log.

#### Partial failure and freshness

v1 behavior is:

- `/metrics` remains a successful Prometheus response while the exporter can
  render its own diagnostics and any safe snapshot.
- Each collector publishes last-attempt success and duration.
- A failed collector does not delete unrelated current metric families.
- A prior snapshot may be served only with its real age. Once older than the
  configured stale threshold, `ddae_up` and readiness are `0`, and stale
  target families MUST be withheld rather than looking current.
- Authentication/TLS/target-unreachable failures set target availability to
  `0`. `ddae_up=1` requires current successful snapshots for `ping`, `clusters`,
  `nodes`, `lock` and `power`.
- Kafka publishing is isolated from the Prometheus snapshot: a Kafka outage
  MUST NOT falsify successfully collected cluster/node/appliance/configuration
  metrics. Kafka producer health and backlog MUST remain visible through the
  bounded diagnostic metrics.
- `/readyz` is `503` when the required Prometheus snapshot is stale, DDAE
  authentication is unusable, persistent state is unavailable, the alert list
  is known incomplete, the last required alert-detail cycle failed, or the
  durable outbox is full. A temporary Kafka outage remains ready while the
  durable outbox is healthy and has capacity.
- An alert-list success with one or more required alert-detail failures is a
  partial failure of the Kafka alert output. It MUST NOT invalidate unrelated
  Prometheus data, but the detail collector/output diagnostics MUST expose the
  failure and no incomplete event may be reported as successfully delivered.

### Quality Attributes

#### Security

- Use least-privilege read-only DDAE credentials. The document's default
  administrator example is not permission to deploy administrator credentials.
- Default-deny every DDAE path not present in the compiled collector allowlist.
- Validate scheme and host; redirects MUST be disabled or constrained so an
  Authorization header cannot cross origin.
- Disable proxy-from-environment for DDAE requests unless proxy behavior is an
  explicit approved configuration, because a proxy can observe credentials and
  private responses.
- Do not expose runtime metrics containing Go command lines, environment values
  or request URLs when those could reveal secrets.
- Authenticate to Kafka with a least-privilege producer identity restricted to
  the approved topic. Enable broker identity verification and TLS whenever the
  Kafka boundary is not protected by an equivalently approved local mechanism.
- Never log Kafka credentials, event values, serialized payloads or broker
  protocol frames. Topic and broker metadata must be treated as confidential
  operational configuration.
- The OCI image, if approved, SHOULD run as a non-root user with a read-only
  root filesystem and no unnecessary Linux capabilities.

#### Performance and reliability

- Memory and goroutine use MUST remain bounded by configuration and target
  inventory size.
- Collection MUST not overlap. If a scheduled cycle is still running, the next
  cycle is skipped or coalesced and exposed as a bounded diagnostic.
- Use one shared HTTP transport with connection reuse and explicit idle limits.
- Retry only safe GET/token operations and only for approved transient failure
  classes; use bounded exponential backoff with jitter.
- A Prometheus scrape MUST not initiate an unbounded fan-out to DDAE.
- Metric gathering MUST be race-free under concurrent scrapes.
- Kafka publishing, retry and buffering MUST be bounded independently from the
  Prometheus collection/scrape path so a slow broker cannot exhaust memory,
  goroutines or the DDAE request budget.
- Alert-detail retrieval is a bounded fan-out: duplicate IDs are coalesced and
  the maximum requests per cycle, concurrency, per-request deadline, aggregate
  deadline and response bytes are enforced even when the list is large or
  malicious.

#### Observability

- Expose build info, target availability, collector success, last collection
  duration, snapshot age, Kafka publish success/duration/counters and current
  bounded Kafka backlog.
- Error classes MUST use a fixed set such as `auth`, `tls`, `timeout`,
  `transport`, `http`, `decode`, `validation`, `kafka_auth`, `kafka_timeout`,
  `kafka_rejected`, `buffer_full` and `internal`.
- Do not use raw HTTP status text, URLs or error messages as label values.

### Compatibility and Migration

- Initial support is limited to Dell Data Lakehouse System Software 1.5.0 and
  Management API v1 until real integration evidence proves another version.
- Added unknown JSON fields/enums MUST not crash the exporter.
- A metric name, type, help, unit, label name or label meaning change is a
  public compatibility change. Incompatible changes require a major
  specification version and documented dashboard/alert migration.
- Adding a bounded label value to an already documented enum is
  backward-compatible only when existing PromQL aggregation remains valid.
- Removing a metric requires deprecation, replacement and an approved overlap
  when feasible.
- Kafka topic use, record key, headers, schema version, field names/types and
  occurrence/update/clear/replay meanings are public compatibility contracts.
  Incompatible changes require a new event schema version and an approved
  downstream OpenSearch consumer migration/overlap plan.
- Upgrading the Dell compatibility target requires new sanitized fixtures,
  integration evidence and review of every allowlisted endpoint and metric.
- Recovery is rollback to the last verified binary/config pair; configuration
  changes MUST be versioned independently from secrets.

### v1 Resolved Decision Register

以下每一項均使用固定且恰好四個頂層元素：`詳細說明`、`可能的選項`、
`最佳選項`、`最佳選項的分析說明`。可能的選項以 A、B、C……依序排列，且每個
選項各自占一行。Requester 要求整理為 v1 最終規格，故各項最佳選項均成為 v1
release-candidate behavior；這不等同於偽造 `PROJECT.md` 要求的具名 requirements
與 implementation-plan approval。

#### OQ-DDAE-1-001 - Monitoring data scope (resolved for v1)

- 詳細說明：Dell 1.5.0 Management API 足以支援 Cluster 狀態、Node 狀態與容量、
  appliance operability、Coordinator/Worker 配置 CPU 與 Memory，以及
  serviceability alerts；它沒有證實 individual-service health、live utilization 或
  query performance。Requester 已明確指定前四區送 Prometheus，告警事件送 Kafka，
  再由既有流程寫入 OpenSearch 並發出告警。
- 可能的選項：
  - A. 五個資料區全部轉成 Prometheus metrics，包括 serviceability alert counts/content。
  - B. 四個狀態／容量／配置資料區送 Prometheus，serviceability alert records 經版本化 allowlist 後送 Kafka。
  - C. 除上述五個資料區外，再加入 JMX、Kubernetes、SQL/system-table 或其他 Management API 資料來源。
- 最佳選項：**選項 B；已由 requester 選定**。
- 最佳選項的分析說明：選項 B 完全落在 Dell 1.5.0 文件已證實的只讀 Management
  API 邊界內，也符合既有 Kafka→OpenSearch→告警流程的責任分工；Prometheus 只保留
  適合低 cardinality 的狀態、容量、配置與 pipeline diagnostics。配置 CPU、Memory
  與 Node capacity 只表示 configuration/capacity，不能命名或告警為 live
  utilization。決策證據為 2026-08-24 requester 最新明確指示；正式具名
  requirements approval 仍待後續治理流程記錄。

#### OQ-DDAE-1-002 - Collection model and interval (resolved for v1)

- 詳細說明：Exporter 可以在每次 Prometheus scrape 時向 DDAE 即時取資料，也可以
  依自己的週期先收集並提供 snapshot。前者把 DDAE latency 直接放進 scrape critical
  path；後者需要明確定義資料新鮮度、失敗後舊資料與 readiness。Alert polling 與
  Kafka publishing 也必須有明確週期，但不應由 Prometheus scrape 次數觸發。
- 可能的選項：
  - A. 每次 scrape 同步收集。資料最新、模型簡單，但慢 API 會使 scrape timeout，多台 Prometheus 也會倍增 DDAE 負載。
  - B. 背景 polling 後以 atomic snapshot 提供 metrics。scrape 快且 DDAE 負載可控，但必須輸出 snapshot age 並定義 stale threshold。
  - C. 混合模式，狀態資料高頻、容量與配置資料低頻收集。可減少較穩定資料的請求，但 scheduling、錯誤語意與測試複雜度最高。
- 最佳選項：**選項 B；v1 已選定**，預設 30 秒且可配置，request deadline 必須
  小於 collection interval，並設定明確 stale threshold。
- 最佳選項的分析說明：Management API 需要 token 且包含多個 endpoint；背景收集能
  避免 Prometheus scrape 數量控制 DDAE 負載，亦可用單一 atomic snapshot 解決並行
  scrape race。相較混合模式，它足以支援 v1 且較容易證明 bounded concurrency 與
  freshness；只有實際 API 負載證據要求不同頻率時再演進成選項 C。決策負責人為
  Project Owner/SRE owner。

#### OQ-DDAE-1-003 - Authentication and token renewal (resolved for v1)

- 詳細說明：官方文件要求 username、password、`dv-admin-rest` client secret 取得
  bearer token，但未說明 token lifetime、refresh token、401 renewal 或可用的
  read-only role。Exporter 是長時間無人值守服務，不能依賴管理員帳號或無限重試。
- 可能的選項：
  - A. 建立 dedicated least-privilege DDAE user，使用官方 password grant，在記憶體 cache 並安全更新 token。
  - B. 由部署平台注入並輪替 short-lived bearer token。Exporter 不保存 password，但可靠輪替與到期處理轉移給外部平台。
  - C. 使用預設 administrator。最容易取得所有 GET 權限，但權限過大且秘密洩漏影響最大。
- 最佳選項：**選項 A；v1 已選定**，前提是 Dell/DDAE 管理者確認該帳號能以 GET-only/read-only
  權限取得所需資料；若未來有權威自動輪替機制，再增加選項 B。
- 最佳選項的分析說明：選項 A 最貼近官方支援流程，同時可將權限限制在監控範圍，
  且不需建立額外 token delivery control plane。選項 B 的秘密暴露面較小，但若
  rotation 失敗會直接中斷監控；選項 C 違反 least privilege，不適合 production。
  核准證據必須包含去識別化 token response fields、expiry/refresh behavior、401
  behavior 與 read-only role authorization；決策負責人為 Project Owner 與
  security/DDAE administrator。

#### OQ-DDAE-1-004 - One target or multiple targets per process (resolved for v1)

- 詳細說明：單一 Exporter 可以只監控一套 DDAE，也可以同時保存多套 endpoint 與
  credentials。Multi-target 會要求每個 metric 加 target 維度、隔離每套秘密與失敗，
  並在每個 Kafka event 中提供不會碰撞的 DDAE source identity；scrape-time target
  還會形成 SSRF 邊界。
- 可能的選項：
  - A. 一個 process 對應一套 DDAE。部署數量較多，但 secrets、health、cardinality 與 failure domain 最清楚。
  - B. 設定檔中的 static multi-target。process 較少，但每套 target 都需要 label、credential、timeout 與 snapshot isolation。
  - C. 使用 `/probe?target=` 動態指定目標。彈性最高，但有 SSRF、任意內網探測與 credential routing 風險。
- 最佳選項：**選項 A；v1 已選定**；透過 deployment replica 與 Prometheus target
  labels 擴充，明確排除選項 C。
- 最佳選項的分析說明：選項 A 讓 Prometheus 的 instance/job labels 自然表達 target，
  不必把 target 複製到每個 metric，也能讓單一 DDAE 故障只影響一個 process。
  其額外部署成本可由容器或 service manager 自動化，安全與操作收益高於省下的
  process 數量。決策負責人為 Project Owner/SRE owner。

#### OQ-DDAE-1-005 - Metric namespace (resolved for v1)

- 詳細說明：Metric prefix 會成為 dashboard、alert rule 與 recording rule 的長期
  公開契約。部署後再變更 prefix 會造成查詢與告警大量遷移。
- 可能的選項：
  - A. `ddae_`。簡短、具產品識別性，且與專案名稱一致。
  - B. `dell_ddae_`。Vendor 辨識更明確，但每個 PromQL 與 metric 都較冗長。
  - C. Prefix 可配置。可避開局部命名衝突，但會讓相同 Exporter 產生不相容的 dashboard 與 alert contract。
- 最佳選項：**選項 A：`ddae_`；v1 已選定**。
- 最佳選項的分析說明：`ddae_` 已足以形成單字 domain namespace，符合 v1
  metrics 並保持 PromQL 可讀性。環境、site、region 或 deployment 名稱應由
  Prometheus target relabeling 加入，而不是塞進 namespace；因此沒有必要承擔
  configurable prefix 的相容性成本。決策負責人為 Project Owner/Prometheus owner；
  v1 metric contract 使用選項 A。

#### OQ-DDAE-1-006 - API path/schema ambiguities (resolved for v1)

- 詳細說明：官方文件同時出現 `/rest/v1/ping` 與 `GET /rest/ping`，且
  `clusterStatus`、system lock status 與 system shutdown `state` 等欄位沒有完整
  enum；serviceability alert list 只有 weakly typed `results`，也未說明 pagination、
  change marker 或 list ID 與 detail response 的實際關聯。若只依任一段文字或未驗證
  shape 實作，可能造成錯誤 endpoint、漏告警、重複 detail fan-out 或狀態 mapping。
- 可能的選項：
  - A. 以 tutorial/example 為準，採 `/rest/v1/ping` 並接受文件範例狀態。
  - B. 以 operation/schema 為準，採 `/rest/ping` 並只接受已列出的狀態。
  - C. 對非 production DDAE 1.5.0 執行受控 discovery，保留去識別化 structural fixtures，並以官方 PDF 記錄產品意圖。
- 最佳選項：**選項 C；v1 已選定為 release integration gate**。
- 最佳選項的分析說明：只有 live structural evidence 能確定實際部署的 route 與 JSON
  shape，同時保留 PDF 可解釋為何支援該行為。測試必須避免將 credentials 放入 URL
  或 logs；未知狀態必須映射到 bounded `unknown`，不得成為任意 label value。決策
  負責人為 DDAE administrator 與 Requirements Approver；sanitized live discovery
  是 release integration gate。若結果不符合 v1 contract，該部署不受支援，必須修訂
  規格，不能在 runtime 猜測另一條 route 或 schema。

#### OQ-DDAE-1-007 - Partial failure HTTP semantics (resolved for v1)

- 詳細說明：當部分 DDAE endpoint 失敗時，Exporter 必須決定 `/metrics` 是否仍回傳
  2xx。非 2xx 會讓 Prometheus 將整次 scrape 視為失敗，也可能丟失原本可用的
  diagnostics 與其他 collector 資料。
- 可能的選項：
  - A. 任何 collector 失敗就回非 2xx。Prometheus `up=0` 很直接，但所有 partial metrics 與 exporter diagnostics 都不可用。
  - B. 只要能產生合法 exposition 就回 `200`，以 `ddae_up=0`、`ddae_collector_success`、snapshot age 與 `/readyz` 表達 target failure。
  - C. 非必要診斷失敗回 `200`，任一核准資料 collector 失敗回非 2xx。語意看似折衷，但會產生 Prometheus `up` 與 `ddae_up` 兩套分支規則。
- 最佳選項：**選項 B；v1 已選定**。
- 最佳選項的分析說明：選項 B 能保留故障診斷與未受影響的 appliance metrics，對多
  endpoint Exporter 最有操作價值；同時以明確 target metric 與 readiness 避免把
  partial/stale data 誤認為健康。告警規則必須同時監控 Prometheus `up` 和
  `ddae_up`，即可區分 Exporter process 與 DDAE target 故障。決策負責人為 Project
  Owner/SRE owner。

#### OQ-DDAE-1-008 - Collector scope beyond the five selected areas (resolved for v1)

- 詳細說明：Management API 還提供 resource allocation、versions、acceleration、
  components、license、TLS、SupportAssist、serviceability logs/events 與 alert
  summary；requester 已將產品限制在五個資料範圍與兩條固定輸出路由，並明確把
  serviceability alert list 及 per-ID detail 納入告警範圍。Ping 和 Exporter/Kafka
  diagnostics 只用於證明收集管線健康，不構成額外 DDAE 業務資料範圍。
- 可能的選項：
  - A. 僅啟用 ping、clusters、nodes、lock、power/readiness、serviceability alert-list 與 per-ID alert-detail GET；狀態／容量／配置送 Prometheus，detail alert events 送 Kafka。
  - B. 在選項 A 外增加 serviceability alert summary Prometheus metrics、resource allocation 或 versions collectors。
  - C. 再增加 acceleration、components、license、TLS、SupportAssist 或 serviceability log/event collectors。
- 最佳選項：**選項 A；已由 requester 選定**。
- 最佳選項的分析說明：選項 A 直接符合最新 routing 決定，形成最小 GET allowlist、
  以及可分別驗證的 Prometheus metric 與 detailed Kafka event contracts；detail GET
  提供文件明示的完整告警 schema，而不必使用 mutation。排除 alert summary 可避免
  同一告警同時形成兩套狀態來源；任何額外 collector 都必須透過後續 specification
  change 新增，不能只以設定開關繞過範圍。決策證據為 2026-08-24 requester 明確
  指示；正式具名 requirements approval 仍待記錄。

#### OQ-DDAE-1-009 - Delivery artifact (resolved for v1)

- 詳細說明：Exporter 可能部署在 appliance-adjacent host、VM 或 Kubernetes。
  發布格式會影響安裝方式、升級、SBOM、checksum、runtime hardening 與支援範圍。
- 可能的選項：
  - A. 只發布 static binary。供應鏈面較小、host 部署方便，但 service management 與 filesystem/user hardening 由 operator 負責。
  - B. 只發布 OCI image。Orchestration 最方便，但對 VM/host 使用者不友善。
  - C. 同一 source revision 產生 reproducible static binary 與 minimal OCI image，共用版本、測試與 checksum/provenance。
- 最佳選項：**選項 C；v1 已選定**。
- 最佳選項的分析說明：單一 Go codebase 本來就適合產生 static binary；在此基礎上
  建立 non-root minimal image 的額外成本有限，卻能同時支援 host 與 Kubernetes。
  以同一 revision 和 release gate 產出可避免形成兩套行為或版本漂移。決策負責人為
  Project Owner/operations owner。

#### OQ-DDAE-1-010 - Exporter listener protection (resolved for v1)

- 詳細說明：`/metrics` 可能揭露 cluster/node 名稱、配置資源、容量與健康狀態。即使沒有
  credentials，也屬於 internal operational data，因此必須決定由 Exporter 本身或
  外部平台保護 listener。
- 可能的選項：
  - A. Private monitoring network 上的 plain HTTP，以 bind address、firewall 與 Prometheus network policy 控制。
  - B. Exporter 原生支援 TLS/basic auth web configuration。單一 binary 即可保護，但增加憑證與 authentication lifecycle。
  - C. 只 bind loopback/private interface，由 mTLS reverse proxy 或 service mesh 統一處理身份、憑證與 policy。
- 最佳選項：**選項 C；v1 已選定**；若部署環境沒有既有 platform proxy 或 service mesh，
  v1 才以選項 A 作為受限 fallback，並提交 deployment threat model。不要自行
  設計 authentication cryptography。
- 最佳選項的分析說明：選項 C 能重用組織既有 mTLS、rotation 與 access policy，
  避免 Exporter 重新實作安全控制。若環境沒有 proxy，選項 A 的私人 bind/firewall
  比臨時自製 auth 更可審查；未來需要原生保護時可採成熟共用元件評估選項 B。
  決策負責人為 Security/operations owner。

#### OQ-DDAE-1-011 - Go toolchain version (resolved for v1)

- 詳細說明：Go version 會影響語言語意、standard library security、dependency
  compatibility 與 reproducible build。規格目前尚未進入 implementation plan，
  因此不應提前杜撰一個可能過期的 patch version。
- 可能的選項：
  - A. 在 implementation plan 核准當日選擇最新 stable Go patch，並在 `go.mod` 的 `go`/適用時的 `toolchain` directive 鎖定。
  - B. 選擇前一個仍受支援的 Go release，以取得較長 ecosystem 驗證時間。
  - C. 不鎖定版本。開發最簡單，但 CI、開發機與 release build 可能漂移。
- 最佳選項：**選項 A；v1 已選定為 toolchain policy**。
- 最佳選項的分析說明：Monitoring exporter 長期持有 credentials 並處理 TLS/HTTP，
  應優先使用仍獲安全修正的最新 stable patch；透過 `go.mod` 與 CI image pinning
  可以同時取得安全性和可重現性。若環境要求 offline build，CI 應禁止未審核的
  automatic toolchain download，而不是放棄版本鎖定。決策負責人為 Maintainers/plan
  approver，並於 implementation-plan approval 時定案。

#### OQ-DDAE-1-012 - Kafka alert event contract (resolved for v1)

- 詳細說明：Dell 1.5.0 文件列出 serviceability alert 的 `id`、`type`、
  `acknowledged`、`count`、created/updated timestamps、component、message、reason、
  remedies、labels、links 與 nested events 等欄位，但 list response 是 weakly typed，
  detail response 才有明確欄位表，且文件沒有定義 Exporter→Kafka schema。Requester
  已決定 Kafka event 必須加入告警詳細內容；v1 因此必須固定可由既有 OpenSearch
  consumer 驗證的欄位、型別、key/header 與版本相容性契約。
- 可能的選項：
  - A. 將 DDAE alert raw JSON 原樣送 Kafka；導入最快，但 schema 漂移、敏感欄位與 payload 膨脹都不可控。
  - B. 建立版本化 typed allowlist envelope，只包含下游明確需要且經資料分類核准的欄位。
  - C. 只送 severity summary counts；資料面最小，但無法讓 OpenSearch 建立可追蹤的個別告警事件。
- 最佳選項：**選項 B；v1 已選定**，exact field allowlist、bounds、record key 與 headers
  已由本規格的 v1 Kafka contract 固定。
- 最佳選項的分析說明：選項 B 同時提供下游穩定 schema、欄位最小化與可測試的相容
  性邊界，也能以 schema version 演進。選項 A 會讓 Dell 未來新增欄位直接變成外部
  資料外洩與 consumer breakage；選項 C 失去 alert identity 與 lifecycle，無法滿足
  既有 OpenSearch 告警流程。Project Owner、Security/data owner 與既有 Kafka consumer
  owner 必須在 release gate 驗證 consumer 接受本規格固定的 fields、types、record
  key、headers 與 schema version。

#### OQ-DDAE-1-013 - Kafka delivery, replay and failure semantics (resolved for v1)

- 詳細說明：Exporter 以 polling 讀取 DDAE alert list，再對 selected IDs 讀取 detail，
  而文件沒有定義增量 cursor、pagination、update/clear 或 retention。Kafka 也可能
  暫時不可用；若沒有去重、checkpoint、buffer 與 readiness policy，可能漏告警、
  重複 detail calls/events、無限吃記憶體，或讓正常的 Prometheus 狀態資料被 Kafka
  故障連帶判定失敗。
- 可能的選項：
  - A. Best effort、無重試與無 buffer；實作最簡單，但短暫 Kafka 故障就永久遺失事件。
  - B. At-least-once delivery，使用穩定 record key、下游 idempotent upsert、bounded durable outbox/checkpoint 與明確 replay/update/clear 規則。
  - C. At-least-once delivery，但只用 bounded memory buffer；可吸收短暫故障，process restart 或長時間 outage 仍可能遺失事件。
- 最佳選項：**選項 B；v1 已選定**。
- 最佳選項的分析說明：選項 B 最能避免漏告警，並用 stable key 與 OpenSearch
  idempotent upsert 容忍 at-least-once 造成的 duplicates；durable bounded state 可跨
  restart 恢復。代價是必須定義磁碟容量、滿載處理、checkpoint 原子性、資料保留與
  recovery runbook。若部署環境禁止 local durable state，才能退而選擇 C，並明確接受
  restart loss；但該 fallback 不符合 v1。Project Owner、SRE 與 Kafka/OpenSearch
  pipeline owner 必須在 release gate 驗證 `acks=all`、idempotent upsert、outbox limits、
  full-buffer readiness 與 replay 行為符合本規格。

#### OQ-DDAE-1-014 - Service status source and semantics (resolved for v1)

- 詳細說明：Dell 1.5.0 文件明確提供 DDAE `clusterStatus`、Node `state`、appliance
  lock、power/node readiness 與 installed application name/version，但沒有個別 service
  health/state endpoint，也沒有 service identity、狀態 enum 或「服務正常」判定規則。
  因此目前不能把 lock/power 或 cluster status 無條件命名為「服務狀態」。
- 可能的選項：
  - A. 將 `system-lock` 與 `system-shutdown` 的 appliance/node signals 定義成服務狀態；不需新來源，但名稱會超出 Dell 文件語意。
  - B. 將 DDAE `clusterStatus` 同時當成 cluster 與 service 狀態；實作最少，但兩個監控項目實際是同一訊號。
  - C. v1 只輸出文件明示且名稱精確的 appliance diagnostics；individual-service health 延後到取得權威 endpoint、identity、enum 與 mapping 的後續版本。
- 最佳選項：**選項 C；v1 已選定**，且 v1 僅宣稱 appliance operability，不宣稱
  individual-service health。
- 最佳選項的分析說明：選項 C 不會把相近但不同的訊號誤包裝成服務健康，能讓後續
  dashboard 與告警規則依真正的 service identity/state 建立。選項 A 只能證明 appliance
  是否 locked、power/control-plane/node readiness，選項 B 只重複 cluster status，兩者
  都不能證明個別服務正常。因此 v1 只輸出名稱精確的 appliance
  lock/power/control-plane/node readiness metrics；未來取得權威 service source 時，
  必須以新的 specification version、metric contract 與 acceptance coverage 加入。

#### OQ-DDAE-1-015 - Alert detail fetch and refresh policy (resolved for v1)

- 詳細說明：Detail API 需要一個 alert ID，若每次 polling 對所有 ID 都呼叫 detail，
  DDAE 負載會隨 alert inventory 線性成長；若只在第一次看到 ID 時讀取，又可能漏掉
  同 ID 的 acknowledged、count、message 或 updated timestamp 變化。Dell 文件沒有
  保證 list response 提供可用的 `updatedon` change marker，因此必須定義 refresh 規則。
- 可能的選項：
  - A. 每一輪都讀取所有 alert IDs 的 detail；最不依賴 change marker，但 API 負載與重複資料量最高。
  - B. 新 ID 或可信 change marker 變更時讀 detail，另以 bounded periodic refresh 補償 marker 缺漏，並使用 durable checkpoint 與最大 requests-per-cycle。
  - C. Exporter 只送 alert list，由 Kafka/OpenSearch consumer 自行連線 DDAE 讀 detail；Exporter 負載最低，但擴散 DDAE credentials 與網路權限。
- 最佳選項：**選項 B；v1 已選定**，預設 detail refresh 10 分鐘、每輪最多 200 筆、
  concurrency 4，並使用 durable checkpoint/outbox。
- 最佳選項的分析說明：選項 B 在資料完整性與 DDAE 負載間最平衡：新／變更 alert
  可快速取得 detail，periodic refresh 可補償 list marker 不可靠，durable checkpoint
  可跨 restart 去重。選項 A 在大量歷史 alerts 時容易形成 N+1 request storm；選項 C
  把 DDAE credential 與 lifecycle 複雜度擴散到既有 consumer。DDAE administrator、
  SRE 與 Kafka/OpenSearch owner 必須在 release gate 驗證本規格的 change marker
  handling、10 分鐘 refresh、每輪 200 筆、concurrency 4、defer ordering、checkpoint
  與 detail failure/retry 行為。

### Assumptions

- Dell Data Lakehouse System Software 1.5.0 exposes the documented Management
  API v1 through the configured User Management IP/FQDN or load balancer.
- JSON fields marked optional may be absent, null or extended in later versions.
- The Prometheus server can reach the exporter, the exporter can reach DDAE,
  and the exporter can reach the existing Kafka boundary over explicitly
  authorized network paths.
- The deployment platform can inject secrets without committing them.
- The existing Kafka consumer is responsible for idempotent OpenSearch upsert
  by the v1 record key and for evaluating/sending alerts; compatibility with
  the exact v1 payload and headers is a release integration gate.
- The requester has fixed the five-area data scope, output routing, per-alert
  detail content and the v1 recommended decisions. Named governance approval
  remains separate from functional clarification.

### Open Questions

- None.

### Release Validation Gates

There are no remaining functional-design open questions in this v1 release
candidate. The following evidence is still mandatory before implementation or
release and does not count as a passing result until it actually exists:

- Named requirements and implementation-plan approvals after strict SDD and
  governance are enabled.
- A sanitized non-production DDAE 1.5.0 fixture set confirming token behavior,
  `/rest/v1/ping`, all allowlisted response shapes, status values, complete
  alert-list enumeration and list-ID/detail-ID matching.
- An authorized non-production integration run proving least-privilege GET-only
  access and proving every mutation path is absent.
- An isolated Kafka/OpenSearch integration proving the v1 key, headers, schema,
  `acks=all`, at-least-once duplicate handling, idempotent upsert, durable
  outbox/restart replay and alert notification behavior.
- A selected supported Go patch/toolchain pinned in `go.mod` and the release
  build environment; deterministic lint, test, build, integration, security and
  supply-chain Harness stages enabled and passing.
- Operator runbook evidence for secret injection, external listener protection,
  state-volume capacity/permissions, outbox-full recovery, rollback and upgrade.

## DDAE-2 Add YAML configuration, independent monitoring pipelines and guarded TLS controls

### Specification Metadata

- Specification Version: `1.1.0`
- Status: active
- Owner: `Peter Chu`
- Clarification Status: resolved
- Approval Status: Governed exclusively by the digest-bound named records in
  the DDAE-2 section of `DEVELOPMENT_PLAN.md`; this metadata line does not
  itself assert approval.
- Depends on: DDAE-1 Specification v1.0.0 and its implemented public contracts
- Target implementation language: Go

### Executive Summary

DDAE-2 adds a strict YAML configuration file as the preferred operator
interface while retaining every DDAE-1 environment variable as a compatible
higher-precedence override. Resource monitoring and alert monitoring become two
independently enabled and independently scheduled pipelines. Both remain
enabled by default, and at least one MUST be enabled.

DDAE-2 also adds an explicit, guarded ability to disable certificate-chain and
hostname verification separately for DDAE and Kafka. Verification remains on
by default. Disabling it requires both a global acknowledgement and a
target-specific flag, emits a bounded startup warning, and cannot satisfy
release integration or production-support evidence.

This section is a backward-compatible amendment to DDAE-1. Every DDAE-1
requirement, metric, Kafka event and safety rule remains normative except where
this DDAE-2 section explicitly supersedes it.

### Problem Statement

The exporter currently accepts only environment variables, always initializes
both resource and alert/Kafka pipelines, shares one interval between them and
offers no explicitly governed TLS-verification opt-out. Renaming the systemd
example as YAML without a real parser would produce an unusable deployment
contract. Operators need a backward-compatible, secret-safe YAML interface and
independent pipelines, while insecure TLS diagnostics need intentional friction
and must not silently weaken production defaults or release evidence.

### Goals

- Add one bounded, strict, versioned YAML configuration interface with explicit
  selection and deterministic environment override precedence.
- Allow resource and alert monitoring to be independently enabled and
  scheduled, with conditional workers, settings, readiness and metrics.
- Preserve DDAE-1 environment-only deployments and secure TLS defaults.
- Permit explicitly acknowledged target-specific insecure TLS only for
  authorized isolated diagnosis, with bounded warnings and release exclusion.
- Convert container, Kubernetes and systemd documentation/examples to the real
  YAML interface without storing plaintext secrets.

### Non-goals

- Removing or deprecating existing DDAE-1 environment variables in v1.
- Allowing both resource and alert monitoring to be disabled.
- Adding dynamic plugins, arbitrary collector lists, scrape-time targets,
  generic DDAE routes or new DDAE/Kafka data contracts.
- Accepting plaintext passwords, client secrets or private-key contents in
  YAML or command-line flags.
- Treating insecure TLS as production-supported, release evidence, automatic
  fallback or a replacement for installing the correct CA.
- Changing the Kafka event schema, outbox state schema or DDAE read allowlist.

### Actors and Scenarios

- Actor `Operator` needs one reviewable YAML file for non-secret settings and
  platform-mounted secret paths.
- Actor `Prometheus/SRE` needs resource-only, alert-only or dual operation with
  unambiguous readiness and metrics for disabled versus failed work.
- Actor `Kafka/OpenSearch owner` needs resource-only mode to avoid Kafka/state
  dependencies and alert-enabled mode to preserve DDAE-1 delivery semantics.
- Actor `Security operator` needs certificate verification on by default and a
  conspicuous, target-scoped, auditable opt-in when isolated diagnosis cannot
  use a valid CA.
- Actor `Maintainer` needs deterministic parsing, precedence, compatibility and
  boundary tests before implementation can be handed off.

### Compatibility and Migration

DDAE-2 explicitly supersedes only these DDAE-1 clauses:

- Configuration is no longer environment-only; a strict versioned YAML file is
  added, while the DDAE-1 environment keys remain supported.
- Resource and alert collection no longer share one mandatory runtime switch
  state or one collection interval. The routes remain compiled and allowlisted,
  but an operator MAY disable either complete pipeline.
- The absolute statement that DDAE/Kafka TLS verification cannot be disabled
  is replaced by the two-level, default-secure opt-in contract below.
- `/readyz` evaluates only enabled pipelines instead of requiring both the
  resource snapshot and alert pipeline unconditionally.
- Kafka, outbox, checkpoint and `source_instance` settings are required only
  while alert monitoring is enabled.

Existing environment-only deployments with no YAML file MUST retain DDAE-1
behavior: both pipelines enabled, the legacy collection interval applied to
both, TLS verification enabled and every existing required setting validated.
Removing environment support or changing the default pipeline/TLS state would
be an incompatible change and is not authorized by DDAE-2.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-2-001 | must | The exporter MUST support a bounded, versioned, strict YAML configuration file selected by a non-secret command-line path or environment path; unknown fields, duplicate keys, unsupported versions, malformed types, oversized files and unreadable explicitly selected files MUST fail before serving. | AC-DDAE-2-001 |
| REQ-DDAE-2-002 | must | Existing DDAE-1 environment variables MUST remain supported as higher-precedence overrides; precedence and legacy interval fallback MUST be deterministic, and an environment-only deployment MUST preserve DDAE-1 defaults and required behavior. | AC-DDAE-2-002 |
| REQ-DDAE-2-003 | must | YAML MUST accept secret file paths but MUST NOT accept plaintext DDAE passwords, client secrets, Kafka passwords or private-key contents; direct secret environment variables and `_FILE` variables MUST retain their approved conflict, size, validation and redaction rules. | AC-DDAE-2-003 |
| REQ-DDAE-2-004 | must | Resource and alert monitoring MUST each have an independent boolean enable setting and collection interval, both enabled by default; at least one pipeline MUST be enabled, disabled pipelines MUST issue no pipeline-specific DDAE/Kafka work, and settings used only by a disabled pipeline MUST not be required. | AC-DDAE-2-004 |
| REQ-DDAE-2-005 | must | `/readyz`, worker construction and Prometheus exposition MUST evaluate only enabled pipelines, MUST expose the fixed `ddae_monitoring_enabled{pipeline}` contract, and MUST distinguish an intentionally disabled pipeline from a failed enabled pipeline without reporting disabled work as successful. | AC-DDAE-2-005 |
| REQ-DDAE-2-006 | must | DDAE and Kafka certificate-chain and hostname verification MUST remain enabled by default; disabling either MUST require both `security.allow_insecure_tls=true` and that target's `insecure_skip_verify=true`, MUST retain TLS 1.2 or newer and all other authorization controls, MUST emit a bounded target-only startup warning, and MUST be rejected as release/production validation evidence. | AC-DDAE-2-006, AC-DDAE-2-007 |
| REQ-DDAE-2-007 | must | Kubernetes, VM/systemd, container and operator documentation MUST use the real YAML interface, platform secret injection and conditional pipeline settings, and MUST explain migration precedence, TLS risk, readiness, metrics and recovery without committing secret values. | AC-DDAE-2-008 |
| REQ-DDAE-2-008 | must | The YAML parser, precedence rules, pipeline combinations, conditional dependencies, readiness/metrics behavior and TLS guard MUST have deterministic regression and security tests; default-secure behavior MUST remain covered. | AC-DDAE-2-009 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-2-001 | REQ-DDAE-2-001 | Valid and invalid YAML files at, below and above the configured 1 MiB limit | Configuration is loaded through `--config` or `DDAE_EXPORTER_CONFIG_FILE` | A valid version-1 file loads; unknown or duplicate keys, malformed values, aliases that produce an invalid typed shape, unsupported versions, oversized input and an unreadable explicit path exit non-zero before any listener or remote client starts. |
| AC-DDAE-2-002 | REQ-DDAE-2-002 | Defaults, a YAML file, legacy environment keys and new environment keys containing distinct non-secret canaries | The effective configuration is resolved | Defaults are overridden by YAML, then by environment; `--config` wins over the config-path environment variable; dedicated pipeline intervals win over legacy `DDAE_COLLECTION_INTERVAL`; an environment-only DDAE-1 configuration produces both enabled pipelines with the prior effective values. |
| AC-DDAE-2-003 | REQ-DDAE-2-003 | YAML containing approved secret-file keys, attempted plaintext secret keys, direct secret environment values and `_FILE` variants | Configuration is decoded and secrets are resolved | Secret paths work, plaintext secret YAML keys are rejected as unknown, direct environment values remain runtime-only, conflicting direct/`_FILE` environment inputs fail closed, and errors/logs contain neither secret values nor secret-file contents. |
| AC-DDAE-2-004 | REQ-DDAE-2-004 | The four resource/alert enable combinations and distinct valid intervals | The application is constructed and run against recording doubles | Resource-only starts only ping/cluster/node/lock/power work and needs no Kafka/state settings; alert-only starts only list/detail/publisher work and needs no resource freshness setting or resource cycle; both starts both at their own intervals; both disabled fails before serving. |
| AC-DDAE-2-005 | REQ-DDAE-2-005 | Resource-only, alert-only and dual-pipeline states containing healthy, failed, stale and full-outbox combinations | `/metrics` and `/readyz` are queried | `ddae_monitoring_enabled{pipeline="resources|alerts"}` always reports the configured booleans; only enabled pipeline business/diagnostic families are emitted; readiness is the conjunction of enabled pipeline readiness rules; disabled work is omitted rather than represented as a successful collection. |
| AC-DDAE-2-006 | REQ-DDAE-2-006 | Trusted, untrusted and hostname-mismatched DDAE and Kafka TLS servers with default or custom-CA settings | Connections are attempted without the complete insecure opt-in | Valid verified connections succeed, invalid chains/hostnames fail closed, a target flag without the global acknowledgement is a configuration error, and the global acknowledgement alone does not disable verification. |
| AC-DDAE-2-007 | REQ-DDAE-2-006 | An authorized isolated test boundary with both global and one target-specific insecure flags enabled | The corresponding TLS connection is attempted | Certificate-chain and hostname verification are disabled only for that target, TLS 1.2 minimum and authentication remain active, the other target remains verified, one bounded startup warning names only `ddae` or `kafka`, and the run is marked ineligible for release/production evidence. |
| AC-DDAE-2-008 | REQ-DDAE-2-007 | A new operator following the container, Kubernetes and systemd documentation | Each profile is configured with one or both pipelines | Every profile passes a YAML file through the approved selector, supplies secrets through files/platform facilities, conditionally supplies Kafka/state inputs, preserves single-writer state when alerts are enabled, and documents secure defaults plus the insecure-mode limitation. |
| AC-DDAE-2-009 | REQ-DDAE-2-008 | The complete DDAE-2 deterministic configuration and security fixture matrix | Local test and security stages execute repeatedly | Results are deterministic, no secret appears in output/evidence, default TLS verification and DDAE-1 environment compatibility remain intact, and every pipeline/TLS combination follows the approved dependency and warning policy. |

### Interfaces, Data, and Failure Behavior

The public additions are the YAML file/selector, new environment overrides,
pipeline enable metric and conditional initialization/readiness behavior. No
new network endpoint, DDAE route, Kafka schema field or state format is added.
Every configuration error fails before the HTTP listener and before any DDAE,
Kafka or state client is created. Runtime collection/delivery failures retain
DDAE-1 isolation, redaction, durable state and recovery semantics.

### YAML Configuration Contract

The preferred file name is `config.yaml`; the committed systemd example MUST
be `deploy/systemd/config.example.yaml`. The file MUST be valid UTF-8 YAML, no
larger than 1 MiB, contain exactly `version: 1`, decode into typed fields and
reject unknown or duplicate keys. YAML merge/alias use MAY be parsed by the
selected reviewed library but MUST not bypass typed validation, duplicate-key
rejection, the input-size limit or secret restrictions. Environment-variable
interpolation inside YAML is not supported.

The normative shape and default values are:

```yaml
version: 1

monitoring:
  resources:
    enabled: true
    interval: 30s
    stale_after: 120s
  alerts:
    enabled: true
    interval: 30s
    list_response_max_bytes: 8388608
    detail:
      response_max_bytes: 1048576
      refresh_interval: 10m
      max_per_cycle: 200
      concurrency: 4

server:
  listen_address: 127.0.0.1:9469
  shutdown_grace_period: 15s

security:
  allow_insecure_tls: false

ddae:
  base_url: https://ddae.example.invalid
  source_instance: replace-with-stable-appliance-name
  credentials:
    username_file: /run/secrets/ddae/username
    password_file: /run/secrets/ddae/password
    client_secret_file: /run/secrets/ddae/client-secret
  tls:
    ca_file: /run/trust/ddae-ca.pem
    insecure_skip_verify: false
  request_timeout: 5s
  cycle_timeout: 20s
  response_max_bytes: 4194304
  retry_max: 2

kafka:
  brokers:
    - kafka.example.invalid:9093
  topic: ddae-serviceability-alerts
  client_id: ddae-exporter
  tls:
    ca_file: /run/trust/kafka-ca.pem
    client_cert_file: ""
    client_key_file: ""
    insecure_skip_verify: false
  sasl:
    mechanism: SCRAM-SHA-512
    username: replace-with-runtime-identity
    password_file: /run/secrets/kafka/password
  publish_timeout: 10s

state:
  dir: /var/lib/ddae-exporter
  outbox_max_bytes: 1073741824
  outbox_max_events: 100000
  checkpoint_retention: 720h
  checkpoint_max_alerts: 100000

logging:
  level: info
  format: json
```

Empty optional file paths MAY be omitted instead of written as empty strings.
All numeric, duration, URL, listener, broker, topic, SASL, file-pair and bound
validation from DDAE-1 remains in force. Values explicitly present in a
disabled section MUST still have the correct YAML type and basic scalar syntax,
but conditional remote/state dependencies are not required or initialized.

### Configuration Selection and Precedence

Configuration selection and merge order are normative:

1. Built-in defaults are established.
2. If `DDAE_EXPORTER_CONFIG_FILE` is set, that file is loaded.
3. If `--config` is supplied, its path replaces the environment-selected path.
4. The selected YAML values override built-in defaults.
5. Existing and new individual environment settings override YAML values.

An explicitly selected empty, missing, unreadable, non-regular or oversized
file MUST fail. If neither selector is supplied, no file is read and legacy
environment-only loading remains valid. `--config` accepts only a path and no
secret value; unknown command-line options fail.

All DDAE-1 environment keys retain their existing mapping and validation, with
these additions and interval rules:

| Environment setting | YAML target / behavior |
|---|---|
| `DDAE_EXPORTER_CONFIG_FILE` | Selects the YAML path unless `--config` is supplied. |
| `DDAE_RESOURCE_MONITORING_ENABLED` | Overrides `monitoring.resources.enabled`; strict `true` or `false`. |
| `DDAE_ALERT_MONITORING_ENABLED` | Overrides `monitoring.alerts.enabled`; strict `true` or `false`. |
| `DDAE_RESOURCE_COLLECTION_INTERVAL` | Overrides `monitoring.resources.interval`. |
| `DDAE_ALERT_COLLECTION_INTERVAL` | Overrides `monitoring.alerts.interval`. |
| `DDAE_COLLECTION_INTERVAL` | Legacy fallback applied to both pipeline intervals before either dedicated interval override. |
| `ALLOW_INSECURE_TLS` | Overrides `security.allow_insecure_tls`; strict `true` or `false`. |
| `DDAE_TLS_INSECURE_SKIP_VERIFY` | Overrides `ddae.tls.insecure_skip_verify`; strict `true` or `false`. |
| `KAFKA_TLS_INSECURE_SKIP_VERIFY` | Overrides `kafka.tls.insecure_skip_verify`; strict `true` or `false`. |

For secrets, YAML defines only `*_file` paths. A direct secret environment
value overrides a YAML file path. If both the direct and `_FILE` environment
forms are explicitly set, configuration fails as in DDAE-1. An environment
`_FILE` value overrides the corresponding YAML file path. The effective file
value retains the DDAE-1 64 KiB, UTF-8, NUL and single-trailing-line-ending
rules.

Conditional validation is:

- At least one of `monitoring.resources.enabled` and
  `monitoring.alerts.enabled` MUST be true.
- DDAE base URL and credentials are required whenever either pipeline is
  enabled.
- `ddae.source_instance`, all Kafka producer/authentication values and writable
  state/outbox/checkpoint settings are required only when alerts are enabled.
- `ddae.request_timeout < ddae.cycle_timeout` always holds.
- `ddae.cycle_timeout` MUST be shorter than each enabled pipeline interval.
- `monitoring.resources.stale_after` MUST exceed the resource interval when
  resources are enabled.
- Alert detail refresh MUST be at least the alert interval when alerts are
  enabled; concurrency MUST not exceed maximum details per cycle.

### Pipeline Lifecycle and HTTP Semantics

The fixed pipeline names are `resources` and `alerts`:

- `resources` owns `ping`, `clusters`, `nodes`, `lock` and `power`, the required
  resource snapshot, `ddae_up`, snapshot freshness and all cluster/node/
  appliance metric families.
- `alerts` owns `alert_list`, `alert_detail`, typed event construction, bbolt
  checkpoint/outbox state, Kafka publishing and all alert/Kafka diagnostic
  metric families.
- Both pipelines MAY share one DDAE client and coalesced token cache when both
  are enabled. Their schedules are independent, and one pipeline MUST NOT start
  a second overlapping cycle of itself. Cross-pipeline concurrency remains
  bounded by their independent fan-out and the common DDAE request limits.
- A disabled pipeline MUST have no worker, ticker, pipeline-specific API calls,
  state database or Kafka producer. Resources-only mode therefore requires no
  writable state or Kafka connectivity.

`GET /healthz` remains process liveness only. `GET /readyz` is `200` exactly
when every enabled pipeline is ready under its DDAE-1 rules. Resource-only
readiness depends only on the current resource snapshot. Alert-only readiness
depends only on authentication/list/detail/state/outbox rules. Dual mode is the
conjunction. A temporary Kafka outage remains alert-ready only while the
durable outbox is healthy and has capacity, as in DDAE-1.

### Prometheus Amendment

DDAE-2 adds exactly one metric family:

| Metric | Type | Labels | Source | Semantics |
|---|---|---|---|---|
| `ddae_monitoring_enabled` | gauge | `pipeline` | Effective configuration | Constant `1` when the fixed pipeline is enabled and `0` when disabled; exactly the two series `resources` and `alerts` are always emitted. |

`pipeline` is a fixed two-value label and MUST NOT accept configuration-derived
free text. `ddae_build_info` and `ddae_monitoring_enabled` are always emitted.
When resources are disabled, all DDAE-1 resource/target/snapshot collector and
business metric families are omitted. When alerts are disabled, all DDAE-1
alert-list/detail/Kafka metric families are omitted. Omission plus the explicit
enable metric distinguishes disabled behavior from a failed enabled pipeline;
the exporter MUST NOT report a disabled collector or pipeline as success `1`.

### Quality Attributes

- Security: YAML parsing is bounded and strict; plaintext secret fields are
  absent; default TLS remains verified; insecure mode is explicit, scoped,
  warned and excluded from release evidence.
- Reliability: disabled dependencies are not initialized, partial constructor
  failures close already-created resources, schedulers remain non-overlapping
  within each pipeline and shutdown waits only for active workers.
- Performance: independent intervals and existing fan-out/deadline limits bound
  each pipeline; YAML size is capped before decoding.
- Observability: enable state uses a fixed two-value metric label; disabled work
  is omitted instead of fabricated as success; insecure warnings contain no
  operational endpoint or secret.
- Maintainability: YAML input, environment overlay, effective validation and
  runtime construction remain separate typed boundaries with deterministic
  tests and no generic configuration map in product logic.

### Guarded TLS Verification Opt-out

The following truth table is normative for each target:

| `security.allow_insecure_tls` | target `insecure_skip_verify` | Effective behavior |
|---|---|---|
| `false` | `false` | Full certificate-chain and hostname verification. |
| `false` | `true` | Invalid configuration; fail before serving. |
| `true` | `false` | Full verification remains enabled. |
| `true` | `true` | Disable chain and hostname verification for only that target. |

For DDAE, one target setting covers both the token endpoint and every approved
GET on the same configured origin. For Kafka, it covers broker server identity
only; configured client certificates, SASL, topic authorization, producer
idempotence and `acks=all` remain active. TLS minimum version remains 1.2.

When insecure mode is effective, startup MUST emit exactly one warning per
affected target containing only a fixed target value (`ddae` or `kafka`) and a
fixed statement that certificate and hostname verification are disabled. It
MUST NOT log endpoints, brokers, topics, credentials, certificate bodies or
config dumps. A custom CA remains the recommended solution. If a CA file and
insecure mode are both selected for the same target, configuration MUST fail to
avoid falsely implying that the CA is verified.

Insecure mode is intended only for explicitly authorized isolated diagnosis or
non-production testing. Evidence gathered with it MUST be marked ineligible for
release, production support, DDAE compatibility, Kafka compatibility or E2E
closure. No automatic environment-name heuristic, hidden fallback, retry-based
downgrade or certificate-error fallback is permitted.

### Deployment and Migration

- `deploy/systemd/config.example` is replaced by
  `deploy/systemd/config.example.yaml`; the unit passes
  `--config /etc/ddae-exporter/config.yaml` and retains `LoadCredential` env
  overrides for runtime credential paths.
- Kubernetes stores the non-secret YAML as a ConfigMap key, mounts it read-only
  and passes `--config`; Secret volumes/env remain separate.
- Container examples mount YAML, trust, secrets and conditional state paths;
  no YAML or image layer contains a plaintext secret.
- Environment-only invocation remains supported for v1 compatibility. Operators
  MAY migrate one deployment at a time by first creating YAML with equivalent
  values, retaining env overrides, inspecting effective non-secret behavior,
  then removing redundant non-secret env keys.
- Rollback restores the prior binary and env-based configuration. Enabling or
  disabling alerts MUST preserve `state.db`; disabling alerts MUST not delete,
  truncate or migrate retained outbox/checkpoint data.

### Resolved Decision Register

#### OQ-DDAE-2-001 - YAML adoption and precedence

- 詳細說明：現有程式只讀 environment variables；僅重新命名範例檔會產生不可執行的
  YAML。設定介面已是 public contract，需定義 parser、selection、precedence 與
  migration。
- 可能的選項：
  - A. YAML 成為 preferred interface，既有 environment variables 保留為較高 precedence override。
  - B. 完全改為 YAML 並移除 environment variables。
  - C. 只把 `config.example` 改副檔名，不修改 loader。
- 最佳選項：**選項 A；DDAE-2 已選定。**
- 最佳選項的分析說明：A 提供真正 YAML 並維持 v1 相容與 platform secret injection；
  B 是 breaking change；C 會讓範例無法被程式讀取。決策人 Peter Chu 於
  2026-08-25 確認推薦基線。

#### OQ-DDAE-2-002 - Resource/alert pipeline controls

- 詳細說明：Resource metrics 與 alert/Kafka pipeline 有不同依賴、排程與 recovery，
  operator 需要能獨立關閉其中之一且不提供未使用的 Kafka/state 設定。
- 可能的選項：
  - A. 兩個獨立 enable/interval，預設皆開，至少一個開，readiness 只看 enabled pipeline。
  - B. 只提供一個全域 enable，兩個 pipeline 仍綁定。
  - C. 允許兩者都關閉並維持 ready。
- 最佳選項：**選項 A；DDAE-2 已選定。**
- 最佳選項的分析說明：A 符合 requested separation 且以明確 metric 區分 disabled 與
  failed；B 無法達成需求；C 產生沒有觀測功能卻 ready 的程序。決策人 Peter Chu 於
  2026-08-25 確認推薦基線。

#### OQ-DDAE-2-003 - TLS verification opt-out

- 詳細說明：DDAE-1 要求 certificate/hostname verification 不可停用；新需求允許
  不檢查 SSL，會增加 bearer credentials、alert data 與 Kafka records 遭中間人讀取
  或竄改的風險。
- 可能的選項：
  - A. 維持不可停用，只允許 custom CA。
  - B. 每個 target 提供單一 `insecure_skip_verify` 開關。
  - C. 使用 global acknowledgement 加 target-specific flag 的雙重 opt-in，default secure 且 insecure evidence 不得關閉 release gate。
- 最佳選項：**選項 C；DDAE-2 已選定。**
- 最佳選項的分析說明：C 在滿足隔離測試需求時加入 intentional friction、target
  isolation 與明確 warning；A 不符合 requester 需求；B 最容易被誤設於 production。
  決策人 Peter Chu 於 2026-08-25 確認推薦基線。

#### OQ-DDAE-2-004 - Change identity

- 詳細說明：DDAE-1 尚未通過 release gates，但 YAML public interface 與 TLS security
  boundary 是可獨立治理、實作與 rollback 的新增 outcome。
- 可能的選項：
  - A. 改寫 DDAE-1 已核准 v1.0.0 bytes 與 approval history。
  - B. 建立依賴 DDAE-1 的新 Change ID DDAE-2 與 Specification v1.1.0。
- 最佳選項：**選項 B；DDAE-2 已選定。**
- 最佳選項的分析說明：B 保留 DDAE-1 digest-bound history，讓 TLS 風險與 migration
  可獨立核准；A 會破壞已核准歷史。決策人 Peter Chu 於 2026-08-25 明確同意
  `DDAE-2`。

### Assumptions

- Deployment platforms can mount one non-secret YAML file and separate secret
  files at stable runtime paths.
- Existing DDAE-1 environment-only operators need a non-breaking migration
  period and may mix YAML with environment overrides.
- Resource monitoring means exactly DDAE-1 `ping`, `clusters`, `nodes`, `lock`
  and `power`; alert monitoring means exactly DDAE-1 list/detail, durable state
  and Kafka delivery.
- Insecure TLS is an explicit operator risk acceptance for isolated use, not a
  production-support promise or a substitute for installing the correct CA.

### Open Questions

- None.

### Release Validation Gates

In addition to every unresolved DDAE-1 gate, DDAE-2 requires:

- Digest-bound named confirmation of Specification v1.1.0 and approval of the
  DDAE-2 Plan Basis and Test Plan before implementation.
- Deterministic tests for strict YAML, file bounds, merge precedence, secret
  rejection, every pipeline combination and conditional setting dependency.
- Default-secure DDAE and Kafka TLS tests plus isolated two-level insecure mode
  tests proving target separation, warning redaction and TLS-minimum retention.
- Static deployment-contract tests and authorized Kubernetes/systemd E2E
  evidence for YAML mount/selection, secret injection, pipeline modes,
  readiness, state preservation, upgrade and rollback.
- Security review of the YAML parser dependency and the intentional TLS
  verification bypass. Evidence obtained with insecure TLS cannot close a
  release or production-support gate.

## DDAE-3 Harden delivery correctness, bounded failure and state health

### Specification Metadata

- Specification Version: `1.2.0`
- Status: active
- Owner: Peter Chu, Project Owner
- Clarification Status: resolved

Approval status is governed exclusively by the digest-bound named record in
`DEVELOPMENT_PLAN.md`. This section is a backward-compatible amendment to the
DDAE-1 and DDAE-2 contracts; requirements not explicitly amended here remain
in force.

### Problem Statement

The implemented exporter has four urgent reliability and security gaps: its
pinned Go 1.26.5 standard library contains source-reachable vulnerabilities; an
alert content transition `A -> B -> A` can suppress the final `A`; an in-flight
idempotent Kafka request can outlive `KAFKA_PUBLISH_TIMEOUT` and the shutdown
budget; and a manually dispatched workflow can run code from a selected ref on
the privileged non-production self-hosted runner.

A second group of gaps can let malformed persistent state bypass limits or
recover readiness incorrectly, serialize token refresh storms, indefinitely
starve periodic alert refreshes, and allocate excessive HTTP resources through
unbounded body/header or scrape concurrency settings. These failures can leave
downstream state stale, delay shutdown, expose a privileged runner to
unreviewed code, or misrepresent the alert pipeline as ready.

### Goals

- Deliver Batch 1 first: Go 1.26.6, correct `A -> B -> A` outbox semantics, a
  hard Kafka publish/shutdown bound, and a default-branch ref guard for the
  self-hosted integration job.
- Deliver Batch 2 only after Batch 1 is verified locally: validate and migrate
  state safely, make state failures participate independently in readiness,
  coalesce token refresh outcomes, guarantee fair alert refresh, and apply
  practical HTTP resource bounds.
- Preserve the Prometheus metric identities, Kafka schema/key/header contract,
  DDAE GET-only boundary, verified-TLS defaults, at-least-once delivery and
  existing reasonable configuration values.

### Non-goals

- Exactly-once Kafka delivery, a new event schema, a new Kafka topic, automatic
  deletion of corrupt primary state, or silent loss of outbox records.
- Changes to DDAE API routes, Prometheus business metrics, alert field
  allowlists, deployment topology, pipeline enable defaults or TLS opt-in
  semantics.
- Proving GitHub environment protection, branch protection or self-hosted
  runner isolation that is configured outside this repository; those controls
  remain separately attestable release prerequisites.
- Repairing the repository-wide governance, OCI provenance or deployment
  findings that are outside the two requested batches.

### Actors and Scenarios

- An operator needs every acknowledged alert transition to converge downstream
  to the most recently observed normalized content, including `A -> B -> A`.
- An operator delivering SIGTERM during an unavailable Kafka request needs the
  process to stop within the configured grace period while retaining an
  uncertain record for safe at-least-once replay.
- A security maintainer needs source-reachable standard-library findings fixed
  and privileged self-hosted jobs restricted to the repository default branch.
- An operator restarting with a valid DDAE-1/DDAE-2 `state.db` needs a safe,
  deterministic in-place metadata upgrade without event loss.
- An SRE needs `/readyz` to remain false after persistent-state failures until
  the responsible state path demonstrates recovery.
- DDAE and Prometheus clients need bounded concurrent token, response-header,
  response-body and `/metrics` resource use under failure or abusive traffic.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-3-001 | must | Every executable Go build source MUST use Go toolchain `1.26.6`; vulnerability policy MUST report no source-reachable Go standard-library vulnerability fixed by `1.26.6` or an earlier supported patch. | AC-DDAE-3-001 |
| REQ-DDAE-3-002 | must | Outbox deduplication MUST preserve the latest observed normalized alert state: it MUST suppress a hash already pending, MAY suppress the delivered hash only when no different hash is pending, and MUST enqueue a return to a delivered hash when a different hash is pending. | AC-DDAE-3-002 |
| REQ-DDAE-3-003 | must | Each Kafka publish attempt MUST return within the configured `KAFKA_PUBLISH_TIMEOUT` plus a bounded scheduling tolerance even after a request becomes in flight; an uncertain timeout or cancellation MUST leave the outbox record unacknowledged, MUST remain compatible with stable-key downstream idempotent upsert, and application shutdown MUST remain within `SHUTDOWN_GRACE_PERIOD`. | AC-DDAE-3-003, AC-DDAE-3-004 |
| REQ-DDAE-3-004 | must | The self-hosted non-production integration job MUST run only for an explicit `workflow_dispatch` opt-in whose workflow ref is `refs/heads/main`, MUST check out the dispatched commit without persisted Git credentials, and MUST retain the protected `ddae-nonproduction` environment boundary. | AC-DDAE-3-005 |
| REQ-DDAE-3-005 | must | Persistent state MUST carry an explicit supported schema version and MUST validate primary outbox/checkpoint records and their cross-record invariants before use; a valid unversioned DDAE-1/DDAE-2 database MUST migrate transactionally without payload loss, derived counters MUST be recomputed from validated records, and corrupt primary data or an unsupported version MUST fail closed without automatic deletion. | AC-DDAE-3-006 |
| REQ-DDAE-3-006 | must | Checkpoint reconciliation MUST never evict a checkpoint that owns a pending outbox hash; an acknowledgement whose record lacks a valid matching checkpoint MUST fail as state corruption, and capacity pressure that cannot be resolved by eligible expired checkpoints MUST remain a visible full-state failure. | AC-DDAE-3-007 |
| REQ-DDAE-3-007 | must | Persistent-state health MUST be an independent alert-readiness condition shared by the alert pipeline and Kafka publisher; any state read, write, acknowledge, reconciliation or health failure MUST make `/readyz` false and MUST remain unhealthy until the responsible path completes a successful authoritative recovery check. | AC-DDAE-3-008 |
| REQ-DDAE-3-008 | must | Concurrent token acquisition and forced renewal MUST share one in-flight result, including its error; a delayed 401 for an older token generation MUST reuse an already newer usable token rather than trigger another renewal, and transient token transport/429/5xx failures MUST use the existing bounded request/cycle retry budget while credential rejection remains non-retryable. | AC-DDAE-3-009 |
| REQ-DDAE-3-009 | must | Alert-detail selection MUST give both new/changed work and refresh-due still-listed work deterministic progress under continuous load; when both classes are backlogged, each MUST receive a non-zero fair share, including alternating service when the per-cycle limit is one. | AC-DDAE-3-010 |
| REQ-DDAE-3-010 | must | DDAE and alert response-body settings MUST reject values above 64 MiB and bounded decoding MUST avoid arithmetic overflow; the DDAE transport MUST cap response headers at 1 MiB, and `/metrics` MUST enforce at most five in-flight requests with a nine-second handler timeout while `/healthz` and `/readyz` remain independently available. | AC-DDAE-3-011, AC-DDAE-3-012 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-3-001 | REQ-DDAE-3-001 | `go.mod`, the OCI builder, local build scripts and CI plus the pinned vulnerability scanner/database | Build and security policy stages run from a clean DDAE-3 revision | All build paths report Go 1.26.6, deterministic tests/builds pass, and no source-reachable Go standard-library finding remains that is fixed by Go 1.26.6 or earlier. |
| AC-DDAE-3-002 | REQ-DDAE-3-002 | Alert `A` is delivered, `B` is queued but unacknowledged, and the source returns to normalized content `A` | The three observations are enqueued and records are acknowledged in order | The outbox contains ordered `B` then `A`, suppresses only exact pending duplicates, and finishes with delivered hash `A`, no pending hash and zero queued records across restart boundaries. |
| AC-DDAE-3-003 | REQ-DDAE-3-003 | A local Kafka-compatible blackhole accepts a connection/request but never returns a produce result | One record is published with a one-second `KAFKA_PUBLISH_TIMEOUT` | `Publish` returns a timeout within two seconds, the record is not acknowledged or removed, goroutines settle, and a later retry remains possible; duplicate broker acceptance is explicitly tolerated by the existing stable key/content-hash contract. |
| AC-DDAE-3-004 | REQ-DDAE-3-003 | The same in-flight blackhole publish and a configured shutdown grace period | SIGTERM/cancellation is delivered | Workers stop and the application returns within the grace period plus one second without acknowledging the uncertain record or hanging in producer close. |
| AC-DDAE-3-005 | REQ-DDAE-3-004 | Push, pull-request and workflow-dispatch event/ref matrices plus the committed workflow | CI policy evaluates the self-hosted job | Only an opted-in dispatch on `refs/heads/main` can schedule it; a tag, pull-request ref or non-main branch cannot; checkout uses the dispatched SHA with `persist-credentials: false`, and the job still names `ddae-nonproduction`. |
| AC-DDAE-3-006 | REQ-DDAE-3-005 | New, valid unversioned legacy, current-version, future-version, malformed-counter, corrupt-record and corrupt-checkpoint databases | The store opens and its records/health are read | New/current state opens; valid legacy state is transactionally marked current; counters are recomputed exactly; future or corrupt primary state fails before workers/listener start; no record/checkpoint is silently deleted or printed. |
| AC-DDAE-3-007 | REQ-DDAE-3-006 | Checkpoint pressure with eligible expired rows, rows owning pending hashes and an outbox record whose checkpoint is missing or mismatched | Reconciliation and acknowledgement run | Only expired non-pending checkpoints may be evicted; pending owners remain; unresolved capacity reports full; missing/mismatched acknowledgement returns bounded corruption and preserves the record/counters. |
| AC-DDAE-3-008 | REQ-DDAE-3-007 | Recording state doubles fail each Records, Acknowledge, Health, FetchState, Enqueue, MarkSeen and ReconcileListed path separately | Publisher/pipeline cycles and `/readyz` execute before and after controlled recovery | Each failure independently makes alert readiness false, another component cannot overwrite it true, metrics/logs use bounded failure classes, and only a successful check by the failing path restores that health component. |
| AC-DDAE-3-009 | REQ-DDAE-3-008 | Concurrent initial requests, concurrent 401s, a delayed old-generation 401, one shared failed renewal, transient 429/5xx and credential rejection fixtures | Token acquisition/renewal executes under a bounded parent context | Each renewal wave makes one token request and returns the same token/error to its waiters; an already newer generation is reused; only transient classes retry with bounded backoff; credentials/tokens never enter output. |
| AC-DDAE-3-010 | REQ-DDAE-3-009 | A persistent backlog of new/changed and refresh-due alerts at per-cycle limits one and greater than one | Repeated deterministic cycles run | Both work classes make progress; limit one alternates classes while both remain; larger limits reserve at least one slot and approximately one quarter for refresh-due work; within each class oldest-fetch time then alert ID is deterministic. |
| AC-DDAE-3-011 | REQ-DDAE-3-010 | Response limits at 1 byte, exact defaults, exactly 64 MiB, above 64 MiB and near signed-integer overflow plus a server returning excessive headers/bodies/trailing data | Configuration and DDAE requests execute | Values through 64 MiB use checked `limit+1` reads and exact overflow-safe bounds; larger values fail before serving; headers above 1 MiB and oversized/trailing bodies fail only the affected operation with bounded classification. |
| AC-DDAE-3-012 | REQ-DDAE-3-010 | More than five concurrent slow `/metrics` requests plus concurrent health/readiness probes | The HTTP server is exercised for longer and shorter than nine seconds | At most five metric handlers execute, excess/expired metric requests fail with bounded HTTP responses, health/readiness remain responsive, and all handler goroutines return. |

### Interfaces, Data, and Failure Behavior

#### Batch ordering and compatibility

Batch 1 contains REQ-DDAE-3-001 through REQ-DDAE-3-004. Batch 2 contains
REQ-DDAE-3-005 through REQ-DDAE-3-010 and starts only after Batch 1's mapped
local stages pass. The Kafka event schema `1.0`, record key, topic, headers and
payload allowlist do not change. Public metric identities do not change.

`KAFKA_PUBLISH_TIMEOUT` remains the single application delivery deadline. The
Kafka producer may permit cancellation of an in-flight idempotent request to
honor that deadline. Because the broker may have accepted a request whose
response was lost, the local record remains pending and may be replayed; this
is an allowed at-least-once duplicate and is reconciled by the stable key and
downstream idempotent upsert.

#### Persistent-state validation and migration

The existing `meta-v1`, `outbox-v1` and `checkpoints-v1` buckets remain. A
current schema marker is added to metadata. On open, one transaction validates
bucket presence, key encodings, sequences, record envelope/schema/size/hash/key
consistency, checkpoint key/ID/hash consistency, and every pending-hash
relationship. Event count and byte count are derived metadata and are replaced
with recomputed values only after all primary records validate.

An unversioned database that satisfies the DDAE-1/DDAE-2 invariants is marked
current in that same transaction. An unsupported newer marker or invalid
primary record aborts startup without mutation or deletion. Recovery uses an
operator-retained copy and a separately reviewed offline repair; the runtime
does not guess or discard corrupt data.

`Acknowledge` remains idempotent when the outbox sequence is already absent. If
the sequence exists, its checkpoint must exist, its bucket key must match the
stored alert ID, and its pending/delivered transition must be consistent. Any
failure preserves the record and derived counters transactionally.

#### Readiness and recovery

Alert readiness becomes the conjunction of alert collection health, durable
state health and the existing outbox-full policy. Pipeline state failures and
publisher state failures occupy separate sticky health components, so a clean
poll cannot erase an unresolved acknowledge/read failure and an idle publisher
cannot erase an unresolved pipeline mutation failure. A component clears only
its own state-health failure after its next complete authoritative state
operation/check succeeds.

Kafka broker rejection or timeout remains observable and leaves the record
buffered; it does not by itself declare durable state corrupt. Outbox full and
any durable-state operation failure make alert readiness false.

#### Token generations and transient failure

Token consumers receive a token plus its generation. A forced refresh carries
the generation that was rejected. If the cache already contains a newer usable
generation, it is returned without another token request. Otherwise exactly
one caller performs the renewal and publishes one immutable result containing
the token, refresh time, generation and error to all waiters.

Transport timeouts/failures, HTTP 429, 502, 503 and 504 from the token endpoint
use the same bounded safe-operation retry/backoff policy and remain inside the
request/cycle context. HTTP 400, 401 and 403 remain authentication failures and
are not retried. No response body or credential appears in logs/evidence.

#### Fair alert scheduling

New/changed alerts and refresh-due alerts are independently ordered by oldest
last-fetch time and then validated alert ID. With both classes non-empty and a
limit greater than one, at least one slot and approximately one quarter of the
limit (rounded down, minimum one) go to refresh-due work; remaining slots go to
new/changed work and unused quota may be borrowed. With limit one, a retained
in-memory turn alternates classes while both remain backlogged. Restart may
reset the first turn but cannot lose checkpoint age, and continued execution
restores alternation.

#### HTTP bounds

`DDAE_RESPONSE_MAX_BYTES`, `ALERT_LIST_RESPONSE_MAX_BYTES` and
`ALERT_DETAIL_RESPONSE_MAX_BYTES` keep their current defaults and accept
positive values through 67,108,864 bytes. Greater values fail effective
configuration validation for environment and YAML sources. Decoding uses
checked arithmetic and consumes through the exact JSON EOF boundary without
allowing a configured integer to wrap.

The DDAE transport uses `MaxResponseHeaderBytes=1048576`. The public server
keeps its existing method and socket timeouts; only `/metrics` receives a
maximum of five in-flight handlers and a nine-second Prometheus handler
timeout. Health and readiness handlers are outside that semaphore.

### Quality Attributes

- Security: Go 1.26.6 is uniform across build sources; self-hosted execution is
  main-ref-only; Git credentials are not retained; TLS/secret/read-only API
  controls remain unchanged.
- Correctness: outbox transitions converge to the latest source observation;
  primary persistent data is validated before derived metadata or workers are
  trusted; no state repair silently drops records.
- Availability: Kafka, token, DDAE response and metric-request work has an
  explicit upper bound; health/readiness remain cheap and independent.
- Observability: failures use existing bounded classes and counters. No stored
  payload, token, endpoint, ref-provided content or raw corruption data is
  logged or retained as Harness evidence.
- Determinism: state migration, task selection, failure recovery and tests use
  fixed ordering and controlled clocks/servers rather than timing retries.

### Compatibility and Migration

This is Specification v1.2.0 because it adds backward-compatible reliability
and validation behavior. Existing normal configuration, metric identities,
Kafka schema/key/header/topic behavior and deployment profiles remain valid.
Configurations requesting a response body above 64 MiB become invalid and
must be reduced before upgrade.

The first DDAE-3 start transactionally marks a valid unversioned database and
recomputes only derived counters. It does not rewrite event payloads or delete
records. Before rollout, stop the previous single writer and retain a protected
copy of `state.db`. A DDAE-2 binary ignores the added metadata marker, so
rollback uses the prior binary/configuration and preserved state after a clean
stop. Do not roll back by deleting or replacing a state file that contains
unacknowledged records.

The self-hosted workflow continues to require repository-external environment
protection and an authorized non-production boundary. Static CI policy proves
the committed ref guard only; release evidence must separately attest the
actual GitHub environment/runner controls.

### Assumptions

- Go 1.26.6 remains available to the declared macOS/Linux builders and has a
  reviewable pinned OCI builder digest when implementation begins.
- The existing OpenSearch consumer performs idempotent upsert by the stable
  Kafka record key as required by the DDAE-1 at-least-once contract.
- Valid deployed response limits do not intentionally exceed 64 MiB; any such
  exceptional requirement needs a renewed specification decision.
- One exporter process remains the only writer for a given `state.db`.

### Open Questions

- None.

### Release Validation Gates

DDAE-3 inherits all unresolved DDAE-1/DDAE-2 release blockers. Batch 1 requires
current `build`, `test`, `security` and `ci-policy` evidence. Batch 2 requires
current `test` and `security` evidence. Kafka blackhole tests are isolated local
component evidence, not proof of real Kafka/OpenSearch compatibility. The
authorized integration/E2E, independent review, supply-chain and external
GitHub environment/runner attestation remain required before a supported
release.
## DDAE-4 Add Serviceability Logs / Events collection and Kafka delivery

### Specification Metadata

- Specification Version: `1.3.0`
- Status: active
- Owner: Peter Chu, Project Owner
- Clarification Status: resolved
- Approval Status: active approval candidate awaiting digest-bound named
  requirements confirmation and implementation-plan approval
- Depends on: DDAE-1 Specification v1.0.0, DDAE-2 Specification v1.1.0 and
  DDAE-3 Specification v1.2.0

This section is a backward-compatible amendment selected as the active SDD
approval candidate after the DDAE-3 Batch 2 implementation and mapped local
tests completed. Selecting `DDAE-4` for approval preparation does not authorize
implementation, alter DDAE-3 evidence or modify any prior digest-bound
approval. Peter Chu resolved the six design decisions on 2026-08-27 by
directing `OK，繼續` and `請完成最終規格`; those statements select the documented
best option in each decision but do not confirm a digest that did not yet
exist. The final specification section must receive named digest confirmation,
followed by an approved DDAE-4 Plan Basis and Test Plan, before product code may
change.

### Executive Summary

DDAE-4 adds read-only collection of Dell Serviceability Logs, also named
Serviceability Events by the Management API. It adds only these routes:

| Data | DDAE source | Output |
|---|---|---|
| Serviceability Log list | `GET /rest/v1/serviceability-events` | Discovery and completeness input only |
| Serviceability Log detail | `GET /rest/v1/serviceability-events/{id}` | Typed Kafka event to a dedicated topic, then the existing OpenSearch path |

The feature is a third independent `serviceability_logs` monitoring pipeline,
disabled by default. It never sends log content to Prometheus and
does not call Support Bundle, SupportAssist, mutation or undocumented routes.
The pipeline uses a dedicated Kafka topic and durable state boundary so log
volume or malformed log data cannot consume the existing alert pipeline's
topic, outbox or checkpoints.

Dell 1.5.0 documents a list response and per-ID detail schema but does not
document list query parameter names, pagination, ordering, retention,
incremental cursors or maximum sizes. DDAE-4 therefore provides bounded,
observable polling and upsert delivery but MUST NOT claim lossless capture,
complete historical coverage or collection of every DDAE raw log. Release
support requires sanitized non-production evidence for the actual list shape,
completeness behavior and list-to-detail relationship.

### Source Basis and Evidence Limits

The Dell source, document identity, authentication caveats and general security
interpretation remain those recorded by DDAE-1. DDAE-4 additionally relies on:

- `GET /rest/v1/serviceability-events`, described as listing serviceability
  logs, on PDF pp. 43-44.
- `GET /rest/v1/serviceability-events/{id}` and its documented detail fields on
  PDF pp. 44-45.
- `DDLHAlertResponseList`, whose weakly typed `results` array plus `threshold`
  and `totalRecords` are documented on PDF pp. 88-89.
- `DDLHLog`, whose fields and descriptions are documented on PDF pp. 89-91.

The document calls `type` the log severity type and defines `count` as the
number of times the log has been created. It describes `labels` and `links` as
objects but does not define stable nested schemas. The list item schema is not
strong enough to prove that every detail field is present in a list response.
Query parameter names and semantics are not documented even though the
operation text says query parameters may be added with `curl`.

These limitations are normative:

- The exporter MUST call the list route without invented query parameters.
- A list response is complete only when sanitized integration evidence confirms
  the deployed response contract and the current response contains a valid
  non-negative `totalRecords` no greater than the number of usable unique list
  results.
- The exporter MAY process safe IDs from an incomplete list but MUST expose the
  incomplete state and MUST NOT describe the result as lossless or complete.
- Unknown fields are ignored. Undocumented `labels` and `links` are excluded
  from output rather than decoded or forwarded generically.
- Support Bundle Pod Logs and Cluster Logs remain outside scope. DDAE-4 covers
  structured Serviceability Logs / Events only.

### Problem Statement

The approved exporter sends detailed serviceability alerts to Kafka but
explicitly excludes the related Serviceability Logs / Events routes. Operators
therefore lack a governed way to collect the structured log records containing
application, component, message, reason, remedy, resource and occurrence data
for OpenSearch investigation. Directly forwarding raw DDAE JSON would expose
undocumented fields, create an unstable downstream schema and couple a
potentially higher-volume stream to alert delivery.

DDAE-4 must add useful log details while preserving the GET-only DDAE boundary,
existing alert guarantees, bounded resource use, deterministic redaction and
observable partial failure.

### Goals

- Collect Serviceability Log IDs from the documented list route and retrieve
  typed per-ID details through the documented detail route.
- Publish new or changed normalized log records to a dedicated Kafka topic for
  downstream OpenSearch indexing.
- Keep Serviceability Logs independently configurable, scheduled, buffered and
  observable so they cannot starve alert delivery.
- Preserve the existing Prometheus resource metrics and Kafka alert schema,
  topic, key, state and delivery behavior unchanged.
- Bound list/detail bodies, concurrency, per-cycle fan-out, event size,
  checkpoints and durable outbox use.
- Prevent credentials, raw responses, unknown objects and Serviceability Log
  content from entering exporter logs, Prometheus labels or retained evidence.

### Non-goals

- Support Bundle creation/download, Pod Logs, Cluster Logs, SupportAssist or
  any mutation endpoint.
- Collection of Kubernetes stdout/stderr, host OS logs, audit logs, SQL/query
  logs, hardware/BMC logs or any undocumented DDAE source.
- A claim of lossless streaming, complete historical backfill or total DDAE log
  coverage.
- Converting log severity, count or free-form content into Prometheus business
  metrics or alerting directly from the exporter.
- Defining the downstream OpenSearch index template, retention, notification
  policy or access-control administration.
- Changing the existing alert Kafka event schema `1.0`, alert topic, alert key
  or alert lifecycle behavior.

### Actors and Scenarios

- An operator enables Serviceability Logs and expects new or updated structured
  records to reach a dedicated Kafka/OpenSearch path without changing existing
  alert delivery.
- An SRE needs to see whether list enumeration is complete, how many details
  were deferred and whether state/outbox capacity is healthy without exposing
  log content as metric labels.
- A security reviewer needs a typed allowlist, size bounds and exact exclusion
  of unknown `labels`, `links`, raw bodies and credentials.
- An operator with the feature disabled expects no serviceability-event API
  calls, no log state database and no requirement for the log Kafka topic.
- A Kafka outage must retain accepted log records within a dedicated bounded
  outbox without consuming alert outbox capacity or claiming successful
  delivery.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-4-001 | must | The exporter MUST add exactly the GET routes `/rest/v1/serviceability-events` and `/rest/v1/serviceability-events/{id}` to a compiled Serviceability Logs allowlist; it MUST NOT call Support Bundle, SupportAssist, mutation or other newly allowed routes. | AC-DDAE-4-001 |
| REQ-DDAE-4-002 | must | Serviceability Logs MUST run as an independently enabled and scheduled `serviceability_logs` pipeline, disabled by default; when disabled it MUST create no worker, DDAE calls, log state or log-topic dependency. | AC-DDAE-4-002 |
| REQ-DDAE-4-003 | must | The pipeline MUST perform bounded list-plus-detail retrieval, validate each ID as one encoded path segment, require the detail `id` to match, deduplicate list IDs, detect incomplete lists and provide deterministic fair progress for new/changed and refresh-due records. | AC-DDAE-4-003, AC-DDAE-4-004 |
| REQ-DDAE-4-004 | must | Each Kafka record MUST be constructed from the typed Serviceability Log schema `1.0` and exact field allowlist below; raw response forwarding, generic maps, `labels`, `links`, unknown fields and invalid present values MUST NOT enter an event. | AC-DDAE-4-005, AC-DDAE-4-006 |
| REQ-DDAE-4-005 | must | Serviceability Log delivery MUST use a dedicated configured Kafka topic, stable kind-separated key, `acks=all`, producer idempotence, per-key ordering, at-least-once replay and a dedicated durable outbox/checkpoint namespace that cannot consume or mutate alert state. | AC-DDAE-4-007, AC-DDAE-4-008 |
| REQ-DDAE-4-006 | must | The exporter MUST publish a log upsert only for a new ID or changed canonical log content; unchanged refreshes MUST be suppressed, disappearance MUST NOT synthesize deletion/clear events, and content transitions MUST converge to the latest observed normalized value. | AC-DDAE-4-009 |
| REQ-DDAE-4-007 | must | List/detail bodies, event bytes, per-cycle requests, concurrency, checkpoints and outbox bytes/records MUST have validated hard bounds; exhaustion or corrupt state MUST fail closed without truncating a source value, deleting an unacknowledged record or advancing the affected checkpoint. | AC-DDAE-4-010, AC-DDAE-4-011 |
| REQ-DDAE-4-008 | must | Only bounded pipeline and Kafka diagnostics MAY enter Prometheus. Serviceability Log IDs, timestamps, messages, reasons, remedies, resource IDs, source labels/links, topics, endpoints and other content MUST NOT be Prometheus labels or exporter log/evidence content. | AC-DDAE-4-012 |
| REQ-DDAE-4-009 | must | Existing resource and alert configurations, metrics, routes, event schema, topic, key, outbox/checkpoints, readiness and default behavior MUST remain compatible; adding the disabled pipeline MAY add only the fixed `serviceability_logs` enable series. | AC-DDAE-4-013 |
| REQ-DDAE-4-010 | must | Supported release evidence MUST include deterministic unit/component coverage plus authorized sanitized DDAE, Kafka and OpenSearch validation of list shape/completeness, list-to-detail identity, ACL isolation, at-least-once upsert and redaction; mocks alone MUST NOT establish real compatibility. | AC-DDAE-4-014 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-4-001 | REQ-DDAE-4-001 | A recording DDAE server and every compiled operation | The logs pipeline performs discovery/detail work and an endpoint-policy test enumerates routes | Only the two documented Serviceability Event GET routes are used; Support Bundle and every mutation or unapproved route are rejected before transport. |
| AC-DDAE-4-002 | REQ-DDAE-4-002 | Existing configurations plus all resource/alert/log enable combinations | Effective configuration and application construction run | Existing configurations leave logs disabled; a disabled logs pipeline has no worker/call/state/topic dependency; any combination with at least one pipeline enabled starts only its selected dependencies. |
| AC-DDAE-4-003 | REQ-DDAE-4-003 | Complete, incomplete, malformed, duplicate and unsafe-ID list fixtures | A bounded collection cycle runs | Safe unique IDs are processed deterministically; unsafe IDs never become paths; missing/invalid totals or totals greater than usable results make list completeness false without fabricating entries. |
| AC-DDAE-4-004 | REQ-DDAE-4-003 | Continuous new/changed and refresh-due detail backlogs at limits one and greater than one | Repeated cycles and restart execute | Both classes make progress under the approved fairness rule, requests never exceed the configured cap/concurrency/deadline, returned IDs must match requested IDs and deferred work is exposed without starvation. |
| AC-DDAE-4-005 | REQ-DDAE-4-004 | Minimum, complete, unknown-field, oversized, invalid-boolean/count/time and undocumented-object detail fixtures | Typed event normalization runs | Only allowed valid fields appear; absent optional fields are omitted; unknown/`labels`/`links` fields are ignored; invalid present values or size violations fail the event rather than truncate or forward raw content. |
| AC-DDAE-4-006 | REQ-DDAE-4-004 | The same semantic detail with map/order/format noise plus one allowed-field change | Canonical normalization and hashing run | Semantically identical allowed content yields identical payload identity/hash and the allowed-field change yields a different hash; the event is valid UTF-8 and at most 256 KiB. |
| AC-DDAE-4-007 | REQ-DDAE-4-005 | Recording alert and log topics, ACLs and colliding source IDs across kinds | Alert and Serviceability Log records publish | Alerts retain their existing topic/key/schema/state; logs use only the dedicated topic and kind-separated key; neither kind can acknowledge, overwrite or consume the other's state. |
| AC-DDAE-4-008 | REQ-DDAE-4-005 | Broker rejection, timeout, restart and later recovery | A log record is persisted and published | No record is acknowledged before Kafka acknowledgement; uncertain/failed delivery stays pending; restart replays in per-key order; recovery may duplicate at least once but downstream stable-key upsert converges. |
| AC-DDAE-4-009 | REQ-DDAE-4-006 | Log content transitions `A -> A -> B -> A` and later disappearance from a complete list | Detail observations, acknowledgements, refresh and checkpoint reconciliation run | The duplicate `A` is suppressed, ordered `B` then final `A` remain deliverable across restart, disappearance creates no synthetic event, and eligible stale checkpoints follow the dedicated retention policy only when no record is pending. |
| AC-DDAE-4-010 | REQ-DDAE-4-007 | Valid boundary, over-limit, overflow-adjacent and trailing-data list/detail responses plus maximum event fields | Configuration and decoding execute | Bounds through 64 MiB are overflow-safe, values above the approved maximum fail before serving, body/trailing violations fail only the operation and no raw body appears in errors. |
| AC-DDAE-4-011 | REQ-DDAE-4-007 | Dedicated log checkpoint/outbox pressure, pending records and corrupt/future-version state | Enqueue, reconciliation, startup and readiness execute | Existing pending records are preserved; no alert capacity is consumed; unresolved capacity or primary corruption is visible and fail-closed; only fully derivable counters may be repaired under the DDAE-3 integrity model. |
| AC-DDAE-4-012 | REQ-DDAE-4-008 | Unique canaries in every sensitive/free-form field, endpoint, topic and credential | Metrics, logs, errors and retained test evidence are inspected | Canaries never appear; metrics contain only fixed bounded identities/reasons and numerical diagnostics; payload content exists only in the approved durable state and Kafka record. |
| AC-DDAE-4-013 | REQ-DDAE-4-009 | A valid pre-DDAE-4 deployment and state plus the new binary with no log settings | Upgrade, scrape, alert delivery and rollback execute | Existing resources/alerts behave byte-contract compatibly, logs remain disabled, exactly one fixed disabled enable-series is added, no log state is created and rollback requires no log migration. |
| AC-DDAE-4-014 | REQ-DDAE-4-010 | An explicitly authorized non-production DDAE, isolated log Kafka topic and OpenSearch test index with sanitized records | Integration/E2E stages execute | Actual list/detail/completeness semantics and least-privilege GET access are retained as evidence; Kafka ACL/topic/key/upsert/replay and redaction pass; no production data or Support Bundle operation is used. |

### Interfaces, Data, and Failure Behavior

The following Management API, Kafka event, retrieval, configuration and
Prometheus/readiness sections define the complete DDAE-4 interface, data and
failure contracts. A failure remains scoped to the enabled logs pipeline unless
the shared application cannot safely start or stop; no failure permits raw
content forwarding, checkpoint advancement after a failed durable write or a
synthetic success signal.

### Management API Allowlist Amendment

DDAE-4 adds only:

| Collector | Method and path | Safe list/detail fields | Purpose | Default |
|---|---|---|---|---|
| `serviceability_log_list` | `GET /rest/v1/serviceability-events` | List `results[*].id`, optional `results[*].updatedon`, `threshold`, `totalRecords` | Log discovery and completeness input | disabled |
| `serviceability_log_detail` | `GET /rest/v1/serviceability-events/{id}` | Exact Kafka allowlist below | Versioned Serviceability Log upsert | disabled |

An ID is untrusted data, not a URL. It must be 1-256 UTF-8 bytes excluding NUL,
encoded as exactly one path segment, and the returned detail `id` must equal the
requested ID. Redirects must not escape the configured verified DDAE origin.

### Serviceability Log Kafka Event Contract

The log schema is independent of the existing alert schema even though both
use version string `1.0`. An encoded record MUST be valid UTF-8 JSON no larger
than 256 KiB.

| Field | JSON type | Required | Rule/source |
|---|---|---|---|
| `schema_version` | string | yes | Constant `1.0`. |
| `event_type` | string | yes | Constant `ddae.serviceability_log.upsert`. |
| `source_system` | string | yes | Constant `dell_ddae`. |
| `source_instance` | string | yes | Existing configured stable source identity. |
| `log_id` | string | yes | Detail `id`; must match the requested list ID. |
| `content_hash_sha256` | string | yes | Lowercase SHA-256 hex of the canonical normalized `log` object. |
| `observed_at` | string | yes | Exporter observation time in UTC RFC 3339. |
| `log` | object | yes | Typed object defined below. |

The Kafka record key is lowercase SHA-256 hex over UTF-8 `source_instance`, one
NUL byte, UTF-8 constant `serviceability_log`, one NUL byte and UTF-8 `log_id`.
Required headers are `content-type=application/json`,
`ddae-schema-version=1.0` and `ddae-record-kind=serviceability_log`. No source
content, identifier, endpoint, topic or credential may appear in another
header.

The `log` object permits only:

| Field | JSON type/bounds | Dell source and normalization |
|---|---|---|
| `severity` | enum `critical`, `error`, `warning`, `info`, `normal`, `unknown` | Lowercase mapping of `type`; absent or any other value becomes `unknown`. |
| `acknowledged` | boolean | Strict case-insensitive conversion of documented string `acknowledged`; invalid present values fail. |
| `occurrence_count` | integer, >= 0 | `count`. |
| `created_at` | RFC 3339 UTC string | Parsed `createdon`; invalid present values fail. |
| `updated_at` | RFC 3339 UTC string | Parsed `updatedon`; invalid present values fail. |
| `app_name` | string, <= 256 bytes | `appname`. |
| `component` | string, <= 256 bytes | `component`. |
| `namespace` | string, <= 256 bytes | `namespace`. |
| `message` | string, <= 8192 bytes | `message`; confidential free-form operational data. |
| `reason` | string, <= 4096 bytes | `reason`; confidential free-form operational data. |
| `remedies` | array, <= 32 strings of <= 2048 bytes | `remedies`. |
| `resource_id` | string, <= 512 bytes | `resourceID`. |
| `symptom_id` | string, <= 256 bytes | `symptomid`. |
| `related` | string, <= 512 bytes | `related`; no additional lifecycle semantics are inferred. |

All strings must be valid UTF-8 and exclude NUL. Absent optional fields are
omitted. `labels`, `links` and unknown fields are excluded at every boundary.
Canonical hashing covers exactly the normalized `log` object, not observation
time or envelope fields.

### Retrieval, Completeness and Lifecycle

- Poll the list on a separate background interval, default `30s`.
  Prometheus scrapes never trigger list/detail work.
- Fetch detail immediately for a new ID or changed usable `updatedon` marker.
  Refresh every still-listed ID at least every `10m` because Dell does not
  define marker or lifecycle guarantees.
- Defaults are at most 200 details per cycle and four concurrent
  detail requests. DDAE-3 fair two-class scheduling applies independently to
  log new/changed and refresh-due work.
- Safe results from an incomplete list may be processed, but absence cannot be
  reconciled while completeness is false. The pipeline remains not ready and
  does not claim lossless capture.
- A complete list may mark an absent checkpoint for retention, but it does not
  create a deletion or clear event. A checkpoint owning pending output is never
  eligible for eviction.
- New or changed canonical content is written to the dedicated durable outbox
  before its checkpoint advances. Kafka acknowledgement advances delivered
  state. Unchanged content updates only bounded observation metadata.

### Configuration Contract Amendment

The normative YAML addition is:

```yaml
monitoring:
  serviceability_logs:
    enabled: false
    interval: 30s
    list_response_max_bytes: 8388608
    detail:
      response_max_bytes: 1048576
      refresh_interval: 10m
      max_per_cycle: 200
      concurrency: 4

kafka:
  serviceability_logs_topic: ddae-serviceability-logs

state:
  serviceability_logs_outbox_max_bytes: 1073741824
  serviceability_logs_outbox_max_events: 100000
  serviceability_logs_checkpoint_retention: 720h
  serviceability_logs_checkpoint_max_records: 100000
```

The corresponding environment overrides are:

| Environment setting | YAML target |
|---|---|
| `DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED` | `monitoring.serviceability_logs.enabled` |
| `DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL` | `monitoring.serviceability_logs.interval` |
| `SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES` | `monitoring.serviceability_logs.list_response_max_bytes` |
| `SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES` | `monitoring.serviceability_logs.detail.response_max_bytes` |
| `SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL` | `monitoring.serviceability_logs.detail.refresh_interval` |
| `SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE` | `monitoring.serviceability_logs.detail.max_per_cycle` |
| `SERVICEABILITY_LOG_DETAIL_CONCURRENCY` | `monitoring.serviceability_logs.detail.concurrency` |
| `KAFKA_SERVICEABILITY_LOG_TOPIC` | `kafka.serviceability_logs_topic` |
| `SERVICEABILITY_LOG_OUTBOX_MAX_BYTES` | `state.serviceability_logs_outbox_max_bytes` |
| `SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS` | `state.serviceability_logs_outbox_max_events` |
| `SERVICEABILITY_LOG_CHECKPOINT_RETENTION` | `state.serviceability_logs_checkpoint_retention` |
| `SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS` | `state.serviceability_logs_checkpoint_max_records` |

Settings are required only when the logs pipeline is enabled. Body settings
accept positive integers through 67,108,864 bytes and use overflow-safe
`limit+1` decoding. Detail refresh must be at least the collection interval;
concurrency cannot exceed the per-cycle maximum; the DDAE cycle timeout must be
shorter than the enabled interval. YAML remains strict and environment
precedence remains DDAE-2 compatible.

### Prometheus and Readiness Amendment

`ddae_monitoring_enabled` adds the fixed third series
`pipeline="serviceability_logs"`. No configuration-derived pipeline value is
allowed. The new diagnostic families are:

| Metric | Type | Labels | Semantics |
|---|---|---|---|
| `ddae_serviceability_log_list_complete` | gauge | none | `1` only when the latest list passes structural and total-record completeness checks. |
| `ddae_serviceability_log_detail_deferred` | gauge | none | Number of listed details deferred by the last bounded cycle. |
| `ddae_serviceability_log_pipeline_ready` | gauge | none | `1` only when enabled list/detail, dedicated state and capacity conditions are healthy. |
| `ddae_serviceability_log_kafka_publish_success` | gauge | none | Last required log-topic publish batch was acknowledged. |
| `ddae_serviceability_log_kafka_publish_duration_seconds` | gauge | none | Duration of the last required log-topic publish batch. |
| `ddae_serviceability_log_records_published_total` | counter | none | Log records acknowledged by Kafka, excluding producer retry attempts. |
| `ddae_serviceability_log_records_failed_total` | counter | `reason` | Records whose bounded delivery policy ended in a fixed error class. |
| `ddae_serviceability_log_buffered_records` | gauge | none | Records currently retained in the dedicated log outbox. |

The generic collector label set adds only fixed values
`serviceability_log_list` and `serviceability_log_detail`. Log content never
becomes a label. When the pipeline is disabled, only its fixed enable series is
emitted; its workers and other metric families are absent.

Overall `/readyz` remains the conjunction of enabled pipeline readiness. A
Kafka outage may remain log-ready only while the dedicated durable state is
healthy and has capacity. Incomplete list/detail collection, corrupt state or
full log outbox makes only the enabled logs pipeline not ready and cannot be
overwritten by resource, alert or Kafka success from another pipeline.

### Quality Attributes

- Security and privacy: read-only exact-route access, typed output allowlists,
  runtime secrets, verified TLS by default and content-free diagnostics.
- Reliability: durable at-least-once replay, acknowledgement before state
  advance, ordered transitions and fail-closed integrity checks.
- Performance and capacity: bounded responses, event size, cycle fan-out,
  concurrency, checkpoint count and dedicated outbox bytes/records.
- Operability: independent enablement, fixed low-cardinality metrics, honest
  readiness, deterministic recovery and no default external dependency.
- Testability: controlled clocks, recording transports/producers, temporary
  isolated state and explicit real-environment release gates.

### Compatibility and Migration

The compatibility, migration and recovery contract below is additive and keeps
the existing resource and alert public interfaces unchanged. Operators opt in
only after provisioning the dedicated topic and state capacity; disabling the
pipeline is the non-destructive rollback path.

### Compatibility, Migration and Recovery

Specification v1.3.0 is a backward-compatible minor amendment because existing
configuration leaves the new pipeline disabled. The four resource areas and
alert pipeline keep their current routes, schedules, metrics, Kafka schema,
topic, record key and state. The sole default-visible addition is the fixed
disabled `ddae_monitoring_enabled{pipeline="serviceability_logs"}` series.

Enabling logs requires a dedicated topic ACL and writable dedicated state. The
implementation must not reuse alert checkpoint identities or capacity. A
failed first enable can be rolled back by disabling only the logs pipeline and
retaining its state for later replay; alert/resource work continues. Removing
log state with pending records is destructive and remains an explicit operator
recovery action, never an automatic rollback step.

DDAE-4 implementation must start only after DDAE-3 Batch 2 is complete or after
a separately approved rebase demonstrates that the same state-integrity,
readiness, token, fairness and HTTP-bound contracts apply to the new pipeline.

### Resolved Decision Register

#### OQ-DDAE-4-001 - Kafka output isolation (resolved for v1.3.0)

- 詳細說明：Serviceability Logs 可能比告警量大，且包含不同 schema 與 OpenSearch
  retention；若共用告警 topic，既有 consumer、ACL、容量與告警延遲都會被改變。
- 可能的選項：
  - A. 使用獨立 Kafka topic、獨立 record kind 與既有 OpenSearch 後段的獨立 index mapping。
  - B. 與告警共用 topic，以 `event_type` 讓 consumer 分流。
  - C. 不送 Kafka，改成 Prometheus metrics。
- 最佳選項：**選項 A；已由 requester 選定。**
- 最佳選項的分析說明：A 隔離 schema、ACL、容量與 retention，且不改變既有告警
  consumer 契約；B 會讓現有 topic 成為混合 schema 並可能延遲告警；C 會把高基數
  自由文字錯誤地放入 Prometheus。Peter Chu 於 2026-08-27 以 `OK，繼續` 及
  `請完成最終規格` 確認此選項。

#### OQ-DDAE-4-002 - Pipeline activation and default (resolved for v1.3.0)

- 詳細說明：新增 collection 會增加 DDAE request、Kafka、磁碟與 OpenSearch 負載；若
  預設啟用，既有部署升級後會立即開始新流量並要求新的 topic ACL。
- 可能的選項：
  - A. 建立獨立 `serviceability_logs` pipeline，預設關閉並由 operator 明確啟用。
  - B. 併入既有 alerts pipeline，alerts 啟用時自動收集 logs。
  - C. 建立獨立 pipeline，但升級後預設啟用。
- 最佳選項：**選項 A；已由 requester 選定。**
- 最佳選項的分析說明：A 維持升級相容並允許獨立容量、排程與故障隔離；B 無法
  獨立停用高量日誌；C 會在未配置 topic、ACL 與 retention 時產生非預期負載。
  Peter Chu 於 2026-08-27 以 `OK，繼續` 及 `請完成最終規格` 確認此選項。

#### OQ-DDAE-4-003 - List and detail retrieval policy (resolved for v1.3.0)

- 詳細說明：Dell 文件的 list `results` 是弱型別且未定義 pagination/query 參數，只有
  per-ID detail 具明確 `DDLHLog` 欄位；只讀 list 不能保證完整內容或完整列舉。
- 可能的選項：
  - A. list 用於發現 ID，再以 bounded per-ID detail 建立 event，並以 `totalRecords` 與整合證據標示 completeness。
  - B. 直接把 list item 當完整 log 發送，不呼叫 detail。
  - C. 猜測未記載的 pagination/query 參數以追求完整列舉。
- 最佳選項：**選項 A；已由 requester 選定。**
- 最佳選項的分析說明：A 僅使用文件化 GET 並依強型別 detail 建立穩定契約；B 依賴
  弱型別 list 且可能漏欄位；C 會把未驗證假設變成 production API 行為。即使選 A，
  沒有真實整合證據前仍不得宣稱 lossless 或完整歷史收集。Peter Chu 於 2026-08-27
  以 `OK，繼續` 及 `請完成最終規格` 確認此選項。

#### OQ-DDAE-4-004 - Field allowlist and sensitive objects (resolved for v1.3.0)

- 詳細說明：`message`、`reason`、`remedies`、`resourceID` 等欄位對診斷有用但屬機密
  operational data；`labels` 與 `links` 是未定義 object，可能含 endpoint、識別資訊或
  無界 key/value。
- 可能的選項：
  - A. 使用本規格 typed allowlist，保留有界診斷欄位並排除 `labels`、`links` 與 unknown fields。
  - B. 完整轉送 DDAE detail JSON，讓 OpenSearch 自動建立欄位。
  - C. 只保留 ID、severity 與時間，排除所有自由文字診斷欄位。
- 最佳選項：**選項 A；已由 requester 選定，仍須獨立 Security/Data review。**
- 最佳選項的分析說明：A 在診斷價值與資料最小化間提供可測試界線；B 會造成 schema
  drift、敏感資料外洩與 mapping explosion；C 最安全但失去使用者要求加入 Logs 的
  主要故障分析價值。Peter Chu 於 2026-08-27 以 `OK，繼續` 及 `請完成最終規格`
  確認此產品選項；release gate 仍要求獨立 Security/Data review。

#### OQ-DDAE-4-005 - Durable state and alert protection (resolved for v1.3.0)

- 詳細說明：Logs 與 alerts 若共用 outbox/checkpoint 容量，高量 logs 可能使 alert
  pipeline full、延遲或無法寫入；若完全不落地，Kafka 故障時會遺失已取得的 logs。
- 可能的選項：
  - A. 使用獨立 log outbox/checkpoint namespace、limits 與 readiness component，保留 at-least-once replay。
  - B. 與 alerts 共用同一容量與 checkpoint namespace。
  - C. Logs 採 memory-only best effort，Kafka 失敗即丟棄。
- 最佳選項：**選項 A；已由 requester 選定。**
- 最佳選項的分析說明：A 保留現有 durable delivery 模型且阻止 logs 消耗 alert
  容量；B 破壞告警隔離與既有容量假設；C 無法在 Kafka 中斷或程序重啟後提供可靠
  的 OpenSearch 收集。Peter Chu 於 2026-08-27 以 `OK，繼續` 及 `請完成最終規格`
  確認此選項。

#### OQ-DDAE-4-006 - Change ordering (resolved for v1.3.0)

- 詳細說明：DDAE-3 Batch 2 正在修改 state integrity、readiness、token coalescing、
  fairness 與 HTTP bounds；DDAE-4 會重用相同概念，若平行實作容易形成兩套不一致
  的安全與恢復行為。
- 可能的選項：
  - A. DDAE-4 保持 queued final candidate，等 DDAE-3 Batch 2 完成後再核准與實作。
  - B. 立即切換 active change 到 DDAE-4，暫停 DDAE-3。
  - C. 同時實作兩個 change，共用尚未完成的 state/client 程式。
- 最佳選項：**選項 A；已由 requester 選定。**
- 最佳選項的分析說明：A 保留現有核准與可驗證依賴順序，並讓 Logs 直接採用完成的
  DDAE-3 邊界；B 會讓已核准但未完成的安全工作停在中間；C 增加規格、實作與證據
  互相覆蓋的風險。Peter Chu 於 2026-08-27 以 `OK，繼續` 及 `請完成最終規格`
  確認此選項。

### Assumptions

- The user's 2026-08-27 direction confirms the six documented product choices
  and requests a final specification; it does not confirm the subsequently
  calculated specification digest or authorize implementation.
- The existing downstream environment can provision a dedicated Kafka topic
  and OpenSearch mapping before the feature is enabled.
- Serviceability Log IDs are stable enough for upsert only after authorized
  non-production validation; a mismatch requires renewed specification rather
  than a fallback identity.
- The Dell 1.5.0 list and detail routes are available to a least-privilege
  read-only identity without granting Support Bundle or mutation permission.

### Open Questions

- None.

### Release Validation Gates

- Named confirmation of the final DDAE-4 Specification v1.3.0 digest.
- An approved marker-bounded DDAE-4 Plan Basis and Test Plan with stable task
  and test IDs before implementation.
- Completion of DDAE-3 Batch 2 or a separately approved rebase of every reused
  state/readiness/token/fairness/HTTP-bound requirement.
- Deterministic list/detail, schema, redaction, fairness, transition, state,
  capacity, configuration, readiness and backward-compatibility tests.
- Authorized sanitized non-production DDAE evidence for the two GET routes,
  least-privilege authorization, list shape/completeness and list/detail ID
  relationship.
- Isolated Kafka/OpenSearch evidence for dedicated ACL/topic routing, stable
  key, schema mapping, at-least-once replay, idempotent upsert, capacity and
  data-retention controls.
- Independent security/data review of free-form operational fields, topic/index
  access, retention, redaction and evidence handling.
- All inherited integration/E2E, supply-chain, deployment, independent-review
  and release gates remain unresolved until current evidence closes them.

## DDAE-5 Add independently configurable DDAE API path prefixes

### Specification Metadata

- Specification Version: `2.0.0`
- Status: active
- Owner: Peter Chu, Project Owner
- Clarification Status: resolved
- Approval Status: final approval candidate awaiting digest-bound named
  requirements confirmation and implementation-plan approval
- Depends on: DDAE-1 Specification v1.0.0, DDAE-2 Specification v1.1.0,
  DDAE-3 Specification v1.2.0 and DDAE-4 Specification v1.3.0

This section is an incompatible default-route amendment selected as a new
change because the implemented DDAE-4 baseline and v1.0.0-rc2 artifacts use
`/rest/v1` for every Management API GET. Field validation supplied by the user
shows the deployed `dv-admin-api` recognizes `/ping` and `/v1/*` instead. On
2026-08-31 Peter Chu selected `/ping` and `/v1/*` as the new defaults and
selected independent bounded prefixes so operators can explicitly restore the
old or PDF-shaped routes. That design decision does not confirm digests that
do not yet exist and does not authorize product-code changes.

### Executive Summary

DDAE-5 separates the path prefix used by Ping from the path prefix used by all
other allowlisted DDAE Management API GET operations:

| Route family | Configured prefix | Compiled suffix | New default path |
|---|---|---|---|
| Ping | `ddae.paths.ping_prefix`, default empty | `/ping` | `/ping` |
| Management API | `ddae.paths.api_prefix`, default `/v1` | one fixed allowlisted suffix | `/v1/*` |
| OAuth token | not configurable by DDAE-5 | `/auth/realms/ddae/protocol/openid-connect/token` | unchanged |

The exporter composes one exact route for each operation. It does not probe,
auto-detect, retry an alternate path or accept arbitrary per-operation URLs.
The existing HTTP methods, response DTOs, body limits, metrics, Kafka events,
state and pipeline behavior remain unchanged.

### Source Basis and Evidence Limits

The Dell source remains `Dell_Data_Lakehouse_System_Software_1.5.0.pdf`,
document SHA-256
`d96dd1bbcee34b7853391a4be3913cff8244ae7f29aab10e46cbcc0ed11812d7`.
Its Management API introduction describes `/rest/{API-VERSION}/{RESOURCE}` and
most operation examples use `/rest/v1/*`. Its Ping material is internally
inconsistent: the tutorial uses `/rest/v1/ping` while the operation entry uses
`/rest/ping`. The document does not describe independently configurable
gateway prefixes.

The user supplied the following field-test observations:

- Public-FQDN requests to `/rest/v1/ping` and
  `/rest/v1/serviceability-issues` returned HTTP 404.
- Direct service requests to `/ping` and `/v1/ddae-clusters` returned HTTP
  401, and `dv-admin-api` logs showed requests to `/ping`,
  `/v1/ddae-clusters`, `/v1/dr-sites`, `/v1/ddpe-configuration` and
  `/v1/resource-allocation`.
- DNS, TLS, ingress, Kubernetes Service and pod-health checks were reported as
  healthy.

HTTP 401 is evidence that a route is reachable and protected; it is not
evidence that authenticated response bodies match the exporter DTOs. The
field report also does not prove whether an ingress rewrite is absent or
misconfigured. Peter Chu nevertheless selected the deployed service-shaped
paths as the product defaults. Sanitized authenticated response and schema
evidence remains an external integration gate.

### Problem Statement

The current binary compiles `/rest/v1` into every DDAE GET route. In the tested
environment those public paths return 404, while the directly reached
`dv-admin-api` exposes `/ping` and `/v1/*`. The exporter therefore cannot
collect even though DNS, TLS and the service are reachable.

A single shared prefix cannot express both the new default `/ping` and
`/v1/*` families. Arbitrary per-operation paths or automatic fallback would
broaden the security allowlist and make traffic non-deterministic. DDAE-5 must
correct the defaults while retaining a narrow, reviewable configuration
boundary that can express both the RC2 and Dell PDF path shapes.

### Goals

- Use `/ping` as the default Ping route and `/v1` as the default prefix for all
  other allowlisted Management API GET routes.
- Let operators configure the Ping and Management API prefixes independently
  through strict YAML and environment overrides.
- Keep every operation suffix, HTTP method, origin and token route compiled and
  non-configurable.
- Reject unsafe or ambiguous prefixes before the exporter starts serving.
- Preserve all non-route product contracts and document exact migration,
  rollback and configuration examples.
- Retain real authenticated DDAE route/schema validation as an explicit
  release gate rather than treating 401 responses or local mocks as success.

### Non-goals

- Runtime endpoint discovery, probing, fallback, dual requests or version
  negotiation.
- Full URL, origin, query, fragment or per-operation path configuration.
- Changing the OAuth token path, password-grant flow, credentials or TLS
  policy.
- Adding the field-observed `/v1/dr-sites`, `/v1/ddpe-configuration` or
  `/v1/resource-allocation` operations.
- Changing response schemas, Prometheus metrics, Kafka event contracts,
  durable state, scheduling, timeouts, retries or readiness ownership.
- Claiming authenticated DDAE compatibility from unauthenticated 401/404
  observations.

### Actors and Scenarios

- An operator using the tested AIDP deployment accepts the defaults and expects
  Ping at `/ping` and all resource, alert and Serviceability Log requests under
  `/v1`.
- An RC2 operator configures both prefixes as `/rest/v1` and expects the
  previous route set without changing any suffix or data contract.
- An operator following the two Ping examples in the Dell PDF can select either
  `/rest/v1` or `/rest` for Ping independently from the API prefix.
- A security reviewer expects prefixes to alter only bounded path namespaces,
  never the scheme, authority, token route, operation suffix or HTTP method.
- An SRE expects a route error to identify only the bounded operation and error
  class, with no alternate request, raw endpoint label or credential exposure.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-5-001 | must | The exporter MUST default Ping to `GET /ping` and every other allowlisted DDAE Management API operation to its current fixed suffix under `/v1`; the OAuth token MUST remain `POST /auth/realms/ddae/protocol/openid-connect/token`. | AC-DDAE-5-001 |
| REQ-DDAE-5-002 | must | Strict YAML MUST expose independent `ddae.paths.ping_prefix` and `ddae.paths.api_prefix` settings, environment variables `DDAE_PING_PATH_PREFIX` and `DDAE_API_PATH_PREFIX` MUST override them, and the precedence MUST be environment over YAML over the empty and `/v1` defaults respectively. | AC-DDAE-5-002, AC-DDAE-5-003 |
| REQ-DDAE-5-003 | must | Each prefix MUST satisfy the exact bounded grammar below and MUST be joined without URL resolution or normalization to a compiled suffix; invalid values, origin changes, arbitrary endpoints, alternate-path retries and method changes MUST be rejected before transport. | AC-DDAE-5-004 |
| REQ-DDAE-5-004 | must | Explicit `/rest/v1` prefixes MUST reproduce the RC2 Management API GET routes, while DTO decoding, body bounds, retry/auth behavior, metrics, Kafka schemas/topics/keys, durable state and pipeline selection MUST remain compatible except for the approved default-route change. | AC-DDAE-5-005 |
| REQ-DDAE-5-005 | must | Route failures MUST remain scoped to the owning operation and pipeline, MUST use bounded operation/error identities, MUST affect readiness under the existing rules, and MUST NOT disclose configured prefixes, full endpoints, credentials or raw response bodies in metrics, logs or retained evidence. | AC-DDAE-5-006 |
| REQ-DDAE-5-006 | must | The YAML examples, Kubernetes and systemd profiles, README and runbook MUST document both prefix settings, defaults, validation, environment overrides, resulting route matrix, RC2/PDF compatibility values and rollback procedure. | AC-DDAE-5-007 |
| REQ-DDAE-5-007 | must | Supported-release evidence MUST include deterministic local route/configuration/security coverage plus an authorized sanitized authenticated DDAE run that proves the selected prefixes reach every enabled allowlisted operation and that actual responses satisfy the existing typed contracts; 401/404 observations and mocks alone MUST NOT establish compatibility. | AC-DDAE-5-008 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-5-001 | REQ-DDAE-5-001 | No prefix settings and a recording TLS server | Ping, resources, alerts, Serviceability Logs and authentication are exercised | The exact GET paths are `/ping`, `/v1/ddae-clusters`, `/v1/infrastructure-nodes`, `/v1/system-lock`, `/v1/system-shutdown`, `/v1/serviceability-issues`, `/v1/serviceability-issues/{id}`, `/v1/serviceability-events` and `/v1/serviceability-events/{id}`; the token request remains the fixed POST path. |
| AC-DDAE-5-002 | REQ-DDAE-5-002 | Default, RC2-compatible and PDF-compatible prefix configurations | The compiled operation registry and recording transport are inspected | Defaults produce `/ping` plus `/v1/*`; `/rest/v1` plus `/rest/v1` reproduces RC2; `/rest` plus `/rest/v1` produces the PDF Ping-operation and API examples without altering suffixes. |
| AC-DDAE-5-003 | REQ-DDAE-5-002 | YAML values, empty YAML values, environment overrides and strict unknown fields | Effective configuration loads | Environment values override YAML exactly, YAML overrides defaults, explicitly empty prefixes are preserved, omitted values receive defaults and unknown/mistyped fields fail before serving. |
| AC-DDAE-5-004 | REQ-DDAE-5-003 | Every valid boundary and invalid prefix class plus safe and unsafe detail IDs | Configuration and all allowlisted operations execute against a recording transport | Valid prefixes compose exactly once; invalid prefixes fail with no request; detail IDs remain one escaped segment; no redirect, normalization, path fallback, origin change, arbitrary suffix or non-GET Management API request occurs. |
| AC-DDAE-5-005 | REQ-DDAE-5-004 | RC2 configuration/state and golden resource, alert and log responses | The new binary runs with both prefixes set to `/rest/v1` | Recorded GET routes match RC2 and all decoded metrics, event payloads/keys/headers/topics, checkpoints, outboxes and enabled-pipeline behavior remain contract-compatible. |
| AC-DDAE-5-006 | REQ-DDAE-5-005 | Unique canaries in prefixes, origins, credentials and response bodies plus 401, 404, timeout and malformed-body failures | Metrics, logs, errors, readiness and retained local evidence are inspected | Only bounded operation/error identities are observable, canaries are absent, no alternate request occurs and readiness/failure ownership follows the existing pipeline contracts. |
| AC-DDAE-5-007 | REQ-DDAE-5-006 | Every committed configuration/deployment profile and operator document | Static deployment and documentation contract tests run | Each surface shows the two settings and exact defaults, valid/invalid forms, environment mapping, new route matrix, RC2/PDF examples and a configuration-first rollback without claiming unverified response compatibility. |
| AC-DDAE-5-008 | REQ-DDAE-5-007 | An explicitly authorized non-production DDAE endpoint, least-privilege credentials and sanitized evidence handling | Integration executes with the deployment's selected prefixes and every enabled operation | Authentication succeeds, every request uses only the expected origin/method/path, actual typed decoding passes, no fallback route is requested and sanitized evidence records the selected prefix identities without secrets or raw operational content. |

### Interfaces, Data, and Failure Behavior

#### Configuration contract

The strict YAML v1 schema adds:

```yaml
ddae:
  base_url: https://ddae.example.invalid
  paths:
    ping_prefix: ""
    api_prefix: /v1
```

| YAML key | Environment override | Type/default | Meaning |
|---|---|---|---|
| `ddae.paths.ping_prefix` | `DDAE_PING_PATH_PREFIX` | string, empty | Namespace inserted before the fixed `/ping` suffix. |
| `ddae.paths.api_prefix` | `DDAE_API_PATH_PREFIX` | string, `/v1` | Namespace inserted before every non-Ping Management API suffix. |

Environment values override YAML even when explicitly empty. YAML values
override the defaults even when explicitly empty. The YAML `paths` mapping and
both fields participate in existing strict unknown-field rejection. The base
URL remains one HTTPS origin with no path, query, fragment or user information.

Each prefix is valid only when it is either empty or a canonical ASCII absolute
path namespace satisfying all of these rules:

- total length is at most 128 bytes;
- a non-empty value begins with `/` and does not end with `/`;
- every non-empty segment matches `[A-Za-z0-9._~-]+` but a complete segment
  cannot be `.` or `..`;
- repeated slash, percent encoding, backslash, whitespace, control characters,
  query, fragment, scheme, authority and user information are forbidden;
- no trimming, case conversion, path cleaning, decoding or normalization is
  performed.

An invalid value is a configuration error before the HTTP server, collectors
or background workers start. Errors name the configuration field and bounded
reason only; they do not echo the supplied value.

#### Fixed suffix and route composition contract

| Operation | Method | Prefix family | Fixed suffix | Default result |
|---|---|---|---|---|
| `ping` | GET | Ping | `/ping` | `/ping` |
| `clusters` | GET | API | `/ddae-clusters` | `/v1/ddae-clusters` |
| `nodes` | GET | API | `/infrastructure-nodes` | `/v1/infrastructure-nodes` |
| `lock` | GET | API | `/system-lock` | `/v1/system-lock` |
| `power` | GET | API | `/system-shutdown` | `/v1/system-shutdown` |
| `alert_list` | GET | API | `/serviceability-issues` | `/v1/serviceability-issues` |
| `alert_detail` | GET | API | `/serviceability-issues/{id}` | `/v1/serviceability-issues/{id}` |
| `serviceability_log_list` | GET | API | `/serviceability-events` | `/v1/serviceability-events` |
| `serviceability_log_detail` | GET | API | `/serviceability-events/{id}` | `/v1/serviceability-events/{id}` |

The effective route is the selected prefix concatenated byte-for-byte with the
compiled suffix. Detail IDs continue through their approved validation and
single-segment escaping after the route is composed. The token manager does not
use either prefix. Redirects remain disabled and the transport cannot change
the configured verified DDAE origin.

The following complete configurations are supported:

| Compatibility target | `ping_prefix` | `api_prefix` | Ping result | API example |
|---|---|---|---|---|
| New default / field-tested service shape | empty | `/v1` | `/ping` | `/v1/ddae-clusters` |
| v1.0.0-rc2 | `/rest/v1` | `/rest/v1` | `/rest/v1/ping` | `/rest/v1/ddae-clusters` |
| Dell PDF Ping-operation plus API examples | `/rest` | `/rest/v1` | `/rest/ping` | `/rest/v1/ddae-clusters` |

#### Failure, observability and data behavior

Configuration errors prevent partial startup. An HTTP 401, 404, timeout,
bounded-body error or typed-decoding error follows the existing retry,
classification, snapshot and readiness rules for its operation. It never
causes a second route family to be tried. No new Prometheus family or label is
introduced for prefix values or full paths.

The operation registry may expose only the fixed bounded operation identity and
the effective path to internal policy tests. Runtime logs, errors, metrics and
retained test reports must not contain configured origin/prefix canaries,
credentials, tokens or raw bodies. Product data and persistent formats are
unchanged.

### Quality Attributes

- Determinism: one configuration produces exactly one route per operation and
  no runtime detection.
- Security: strict prefix grammar preserves the HTTPS origin, compiled suffix
  allowlist, GET-only boundary and single-segment ID handling.
- Compatibility: explicit configuration reproduces prior paths without data,
  state, metric or event migration.
- Operability: documentation states the exact resulting paths and provides a
  configuration-first rollback.
- Testability: recording TLS fixtures can assert exact method/path/token
  behavior without external infrastructure; real response compatibility stays
  separately observable.

### Compatibility and Migration

This is Specification v2.0.0 because changing omitted-setting defaults from
`/rest/v1/ping` and `/rest/v1/*` to `/ping` and `/v1/*` is incompatible for
operators whose gateway still requires `/rest/v1`.

Before upgrading, an RC2 operator can preserve behavior by adding:

```yaml
ddae:
  paths:
    ping_prefix: /rest/v1
    api_prefix: /rest/v1
```

No database, Kafka, OpenSearch, Prometheus, secret-file or credential migration
is required. Rollback is configuration-first: restore both prefixes to the
prior route namespace, restart, confirm bounded operation health, and retain
all alert/log state. Binary rollback uses the prior configuration syntax after
a clean stop and never deletes state or unacknowledged records.

### Resolved Design Decisions

#### OQ-DDAE-5-001 - Route-family structure

- Detailed issue: Ping and all other Management API operations need different
  default namespaces, so one prefix cannot express the selected deployment.
- Options:
  - A. Two bounded prefixes: one for Ping and one for all other allowlisted API operations.
  - B. One deployment-profile enum with compiled route sets.
  - C. Arbitrary per-operation URLs or paths.
- Recommended and selected option: **A, selected by Peter Chu.**
- Analysis: A expresses the two observed families and known compatibility forms
  while retaining fixed suffixes. B is narrow but cannot represent an
  unanticipated gateway prefix without a code release. C is flexible but
  expands the trusted configuration and weakens allowlist review.

#### OQ-DDAE-5-002 - Default routes

- Detailed issue: Dell documentation primarily uses `/rest/v1/*` while the
  tested service recognizes `/ping` and `/v1/*`.
- Options:
  - A. Default to `/ping` and `/v1/*`, retaining `/rest/v1` through explicit configuration.
  - B. Keep RC2 defaults and require configuration for the tested environment.
  - C. Probe both route sets at runtime.
- Recommended and selected option: **A, selected by Peter Chu.**
- Analysis: A directly satisfies the tested deployment and keeps deterministic
  compatibility. B leaves the known deployment broken by default. C creates
  extra requests, ambiguous failure semantics and a wider observable attack
  surface.

#### OQ-DDAE-5-003 - Token path and runtime fallback

- Detailed issue: Only Management API GET paths were implicated; the token
  route and response schemas have no contrary evidence.
- Options:
  - A. Keep the token route fixed and forbid route probing/fallback.
  - B. Apply the API prefix to the token route.
  - C. Add automatic token and API path discovery.
- Recommended and selected option: **A, selected by Peter Chu.**
- Analysis: A keeps the authentication boundary and traffic deterministic.
  B would change a working documented contract without evidence. C could send
  credentials or requests to unintended paths and obscure configuration
  errors.

### Assumptions

- The field report accurately represents an AIDP environment relevant to the
  intended deployment, but it is not a sanitized authenticated integration
  result.
- The existing typed response contracts remain applicable after only the
  namespace changes; this assumption must be proven by the integration gate.
- The configured HTTPS origin is still the correct public or service origin;
  DDAE-5 does not repair ingress routing or choose between those network
  boundaries.
- Peter Chu's 2026-08-31 `OK` confirms the documented route-prefix design
  direction, not the subsequently calculated digests or implementation.

### Open Questions

- None.

### Release Validation Gates

- Named confirmation of the final DDAE-5 Specification v2.0.0 digest.
- Named approval of the marker-bounded DDAE-5 Plan Basis and exact Test Plan
  digest before implementation.
- Deterministic configuration, route-matrix, token invariance, prefix grammar,
  path-segment, no-fallback, redaction, compatibility and documentation tests.
- Authorized sanitized authenticated DDAE validation for every enabled
  allowlisted operation and existing typed response contract.
- Inherited DDAE/Kafka/OpenSearch E2E, deployment, independent review,
  supply-chain and supported-release gates remain unresolved until current
  evidence closes them.

## DDAE-6 Align DDAE 1.5.0 response decoding with documented and observed payloads

### Specification Metadata

- Specification Version: `2.1.0`
- Status: active
- Owner: Peter Chu, Project Owner
- Clarification Status: resolved
- Approval Status: final approval candidate awaiting digest-bound named
  requirements confirmation and implementation-plan approval
- Depends on: DDAE-1 Specification v1.0.0 through DDAE-5 Specification v2.0.0

This backward-compatible amendment follows an authenticated field-payload
review requested by Peter Chu on 2026-09-02. The supplied payloads are evidence
of response structure, not permission to retain operational values in the
repository and not complete integration evidence. Product code remains
unchanged until the exact DDAE-6 Specification, Plan Basis and Test Plan
digests receive the required named confirmation and approval.

### Source Basis and Evidence Limits

The Dell source remains `Dell_Data_Lakehouse_System_Software_1.5.0.pdf`,
document SHA-256
`d96dd1bbcee34b7853391a4be3913cff8244ae7f29aab10e46cbcc0ed11812d7`.
The relevant document evidence is:

- Page 38 defines `GET /rest/v1/infrastructure-nodes` with an object response
  containing a `results` array.
- Pages 64-66 define `ClusterNodeConditions` as an object containing string
  fields `diskPressure` and `memoryPressure`; they define node CPU as an integer
  and storage as the camelCase string field `ephemeralStorage`.
- Pages 43-44 define `GET /rest/v1/serviceability-events` with
  `results`, `threshold` and `totalRecords`, plus
  `GET /rest/v1/serviceability-events/{id}` for a detail record.
- Pages 45-46 define the equivalent list and detail operations for
  `serviceability-issues`.
- Pages 88-91 define the shared list envelope and log fields. The document
  describes severity `type` as a string and describes the summary field
  `info` as the count of Informational alerts, but does not enumerate every
  value returned by list or detail records.

Three user-supplied JSON payloads were reviewed without committing their raw
operational content:

| Operation | Evidence identity | Structural observation |
|---|---|---|
| `infrastructure-nodes` | SHA-256 `aceaa7fbf6993c31deb570fe698800eb3cbb8ca332bf444b1634978e87106ceb` | object with 9 `results`; numeric CPU; camelCase storage; object conditions |
| `serviceability-events` | SHA-256 `8b55ab15b7119a46479ab3fb2eacceceb425835cb5207e677fb42c9abba91802` | 500 results, threshold 500 and totalRecords 1167; records carry detail-shaped fields |
| `serviceability-issues` | SHA-256 `b953bf2541e2c2e097d3ef4b176523e833967163b8fecc01a223c1fef317b2a7` | 14 results, threshold 500 and totalRecords 14; records carry detail-shaped fields |

The payloads do not include responses captured from either `/{id}` operation.
They also do not identify supported pagination parameter names, cursor rules,
ordering guarantees or snapshot consistency. The exporter MUST NOT infer those
contracts from `threshold` and `totalRecords` alone.

### Problem Statement

The implemented node client decodes `infrastructure-nodes` as a bare JSON array
whose CPU values are strings, storage field is `ephemeral-storage`, and
conditions are an array of `{type,status}` objects. The Dell 1.5.0 document and
the supplied field payload instead use a `results` envelope, integer CPU,
`ephemeralStorage`, and a conditions object. Direct decoding therefore fails
before any node metrics can be normalized.

The implemented alert and Serviceability Log list decoders accept the supplied
list envelopes. Their pipelines intentionally use list IDs to retrieve the
documented `/{id}` detail operations. The supplied payloads do not prove those
detail responses, and the events payload is incomplete because totalRecords is
larger than the returned result count. In addition, both supplied
serviceability payloads contain the severity string `Informational`, which the
current normalizers classify as `unknown` instead of the established `info`
category.

### Goals

- Decode the documented `infrastructure-nodes` envelope and field types while
  preserving compatibility with the implemented legacy bare-array fixtures.
- Normalize documented and legacy node representations to the existing node
  snapshot and Prometheus metric contracts without adding labels or metrics.
- Treat `Informational` as the existing bounded `info` severity in both alert
  and Serviceability Log events.
- Preserve the documented serviceability list-to-detail retrieval model,
  existing response bounds, detail limits, state, Kafka schemas and readiness
  ownership.
- Preserve honest incomplete-list behavior when `totalRecords` exceeds the
  unique valid IDs returned by the list.
- Derive only minimal synthetic fixtures from observed structure; do not copy
  raw field payloads, private endpoints or operational identifiers into Git or
  retained test output.

### Non-goals

- Changing `/ping`, `/v1/*`, configured prefixes, compiled suffixes, OAuth or
  any DDAE HTTP method.
- Inventing query parameters, pagination, cursors, ordering, retention or
  snapshot guarantees that the Dell document and supplied evidence do not
  define.
- Treating list records as authoritative detail records or removing the
  documented `serviceability-events/{id}` and `serviceability-issues/{id}`
  requests.
- Adding metrics for extra node fields such as addresses, GPUs, workload
  assignment or acceleration disks.
- Changing Prometheus descriptors, Kafka topics, event schema versions,
  outbox/checkpoint schemas, collection intervals or configuration keys.
- Claiming complete DDAE compatibility from local fixtures or from list
  payloads that omit authenticated detail responses.

### Actors and Scenarios

- An operator enables resource monitoring against DDAE 1.5.0 and expects the
  documented node envelope to produce the existing CPU, memory, ephemeral
  storage, state and pressure metrics.
- An existing operator uses a legacy-compatible response fixture and expects a
  DDAE-6 binary to preserve the same normalized metric values.
- An alert or log consumer receives a source severity of `Informational` and
  expects the existing event severity to be `info`, with no schema change.
- An SRE observes an events list with 500 returned records and 1167 total
  records and expects incomplete-list diagnostics/readiness rather than a
  false complete snapshot or inferred disappearance.
- A security reviewer expects only structural synthetic fixtures to be
  committed and expects unknown/free-form source fields to remain outside
  metrics, labels, logs, evidence and Kafka payloads unless already allowlisted.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-6-001 | must | The node client MUST accept the Dell 1.5.0 object response with a non-null `results` array and MUST continue to accept the implemented legacy bare array; any other top-level shape, missing/null documented `results`, malformed item or trailing JSON MUST fail through the existing bounded node operation. | AC-DDAE-6-001 |
| REQ-DDAE-6-002 | must | Node decoding MUST accept CPU as either a non-negative JSON integer or the existing bounded quantity string, MUST accept `ephemeralStorage` and the legacy `ephemeral-storage` alias, and MUST accept documented condition objects and legacy condition arrays while rejecting conflicting aliases or invalid represented values. | AC-DDAE-6-002, AC-DDAE-6-003 |
| REQ-DDAE-6-003 | must | Both accepted node shapes MUST normalize to the existing snapshot and Prometheus contracts with identical values for equivalent inputs; unknown source fields MUST remain ignored and MUST NOT create new metrics, labels, logs or retained content. | AC-DDAE-6-004 |
| REQ-DDAE-6-004 | must | Alert and Serviceability Log severity normalization MUST map case-insensitive, whitespace-trimmed `Informational` to the existing `info` value while preserving all existing accepted values, unknown-value handling and event schemas. | AC-DDAE-6-005 |
| REQ-DDAE-6-005 | must | Serviceability alert and log pipelines MUST retain the documented list-to-detail flow, bounded detail selection and existing state behavior; a malformed list or `totalRecords` greater than the unique valid returned IDs MUST remain incomplete, MUST prevent absence inference and collection readiness, and MUST NOT trigger invented pagination or list-as-detail fallback. | AC-DDAE-6-006, AC-DDAE-6-007 |
| REQ-DDAE-6-006 | must | Committed tests and documentation MUST use minimal sanitized synthetic structures derived from the PDF and reviewed payloads, MUST record provenance by document/payload digest without raw operational values, and MUST preserve existing response-size, redaction and typed-allowlist protections. | AC-DDAE-6-008 |
| REQ-DDAE-6-007 | must | Supported-release evidence MUST include an authorized authenticated DDAE run proving node list decoding and both serviceability list/detail relationships; pagination MAY be added only by a later approved change after authoritative parameter and consistency evidence exists. | AC-DDAE-6-009 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-6-001 | REQ-DDAE-6-001 | Synthetic documented-envelope, legacy-array, missing/null/wrong-shape, malformed-item and trailing-data node responses | `Client.Nodes` decodes under the existing response limit | Both supported shapes return typed nodes; every unsupported or ambiguous shape returns the bounded node decode/validation failure and no alternate request occurs. |
| AC-DDAE-6-002 | REQ-DDAE-6-002 | CPU values as zero/positive integers and existing valid strings, plus negative, fractional, exponent, boolean, null and oversized values | Node resources decode and normalize | Supported values produce the same existing CPU core units; invalid present values do not become zero or a successful complete family. |
| AC-DDAE-6-003 | REQ-DDAE-6-002 | CamelCase and legacy storage aliases, equal/conflicting dual aliases, documented condition objects and legacy arrays | Nodes decode | Either single alias and both condition shapes normalize correctly; equal dual aliases are deterministic; conflicting aliases and malformed pressure values fail closed without fabricated values. |
| AC-DDAE-6-004 | REQ-DDAE-6-003 | Semantically equivalent documented and legacy node fixtures containing unknown and operational canaries | The resource collector and Prometheus gather run | Existing node state, capacity, allocatable and pressure series are byte-equivalent; no canary or new descriptor/label appears. |
| AC-DDAE-6-005 | REQ-DDAE-6-004 | Alert and log details containing `Informational`, case/whitespace variants, existing severities and an unsupported value | Typed events are built | Informational variants yield `info`; existing values remain unchanged; unsupported input yields `unknown`; schemas, keys, headers and hashes follow the existing canonical contracts. |
| AC-DDAE-6-006 | REQ-DDAE-6-005 | Alert and log lists whose records contain detail-shaped extra fields plus recording detail endpoints | One poll runs | Only list identity/marker fields drive scheduling, selected IDs use the compiled escaped `/{id}` GET routes, and list-only extra fields never bypass detail validation or enter events. |
| AC-DDAE-6-007 | REQ-DDAE-6-005 | Complete, malformed, duplicate-ID and truncated lists including 500 results with totalRecords 1167 | One poll and readiness evaluation run | Valid returned IDs may be refreshed within existing bounds, but incomplete input remains diagnosed/not ready, causes no absence event, and produces no pagination or fallback request. |
| AC-DDAE-6-008 | REQ-DDAE-6-006 | The repository, generated fixtures, test output, logs and metrics seeded with unique private/operational canaries | Test and security stages inspect retained content | Only synthetic values and source digests remain; raw payload values, endpoints and excluded fields are absent; existing body bounds and typed allowlists still pass. |
| AC-DDAE-6-009 | REQ-DDAE-6-007 | An authorized non-production DDAE 1.5.0 endpoint, least-privilege credentials and sanitized evidence capture | Nodes and both serviceability list/detail families run with configured paths | Authentication succeeds, the documented/selected shapes decode, every detail ID relationship is proven, completeness is reported honestly, no mutation or invented query occurs, and evidence contains no secret or raw operational content. |

### Interfaces, Data, and Failure Behavior

#### Node list and field compatibility

`Client.Nodes` keeps its public return type. A private response decoder first
classifies the top level as either the documented object envelope or the legacy
array. It does not probe, retry, select by configuration or silently treat a
missing documented `results` field as an empty inventory.

The internal node DTO continues to expose quantities to the existing collector
as strings. A documented integer CPU is converted losslessly to canonical
base-10 text before existing CPU normalization. The integer form is limited to
non-negative decimal JSON integers within the same response and quantity
bounds; fractions, signs below zero, exponent notation, booleans, arrays and
objects are invalid. Existing valid quantity strings remain supported.

`ephemeralStorage` is the documented name and `ephemeral-storage` is a legacy
decode alias. If both are present with unequal values, the node response is
ambiguous and fails. An object condition becomes the equivalent internal
`DiskPressure` and `MemoryPressure` condition entries. Existing array entries
continue to normalize through the same condition-name and boolean-value rules.
Unknown top-level and node fields remain ignored by the typed decoder.

#### Serviceability retrieval and completeness

The list DTOs continue to retain only identity and update-marker fields needed
for scheduling. Extra detail-shaped fields observed inside list results do not
enter Kafka and do not replace the documented detail request. Existing ID
validation, path escaping, detail concurrency, request caps, fairness,
deadlines, response limits, returned-ID validation and durable state remain
unchanged.

`threshold` is retained only as observed metadata. It is not a documented page
size command. `totalRecords > unique valid returned IDs`, a missing/malformed
required total, a malformed item or an unsafe identity keeps the list
incomplete under existing rules. Returned valid IDs may still receive bounded
detail refresh, but disappearance is not inferred and collection readiness is
false. No query string, second list request or list-as-detail fallback is sent.

#### Severity and output compatibility

Both event builders normalize after trimming surrounding whitespace and
comparing case-insensitively. `Informational` joins `info` as input for the
existing output value `info`. The output enum, JSON field name, Kafka schema
version, topic, key, headers and content-hash rules do not change.

All new decode failures use the existing bounded operation/error classes. Raw
responses, field values, IDs, endpoints and payload excerpts do not enter
errors, logs, metric labels or retained evidence.

### Quality Attributes

- **Compatibility:** documented DDAE 1.5.0 nodes and legacy fixtures are both
  supported without configuration or public-contract changes.
- **Correctness:** structural ambiguity and invalid present values fail rather
  than becoming empty inventory, zero capacity or false pressure.
- **Boundedness:** existing response byte limits, collection deadlines,
  concurrency, detail caps and state limits remain authoritative.
- **Security and privacy:** no raw supplied payload is copied into the
  repository; tests use synthetic IDs, values and canaries only.
- **Observability:** incomplete lists and parser failures retain their current
  owning operation/pipeline and never masquerade as complete success.
- **Maintainability:** compatibility rules live at the DDAE DTO boundary so
  collector, metric and Kafka code continue to consume one canonical model.

### Compatibility and Migration

DDAE-6 is Specification v2.1.0 because it adds backward-compatible accepted
input representations and one severity alias. No setting, path, metric,
event-schema, topic, state or deployment migration is required.

Rollback uses the pre-DDAE-6 binary after a clean stop. No persistent data is
migrated by this change. Events already emitted with severity `info` remain
valid under both versions; rolling back causes newly observed
`Informational` values to return to `unknown`. Operators must not use rollback
to claim node compatibility with the documented envelope.

### Resolved Design Decisions

#### OQ-DDAE-6-001 - Node response compatibility strategy

- Detailed issue: the documented/observed node shape conflicts with the
  implemented fixture shape.
- Options:
  - A. Accept both shapes at one typed boundary and normalize to the current model.
  - B. Replace the old shape with the PDF shape.
  - C. Add a configuration switch selecting the node schema.
- Recommended and selected option: **A, accepted by Peter Chu's direction to
  continue with the recommended next step on 2026-09-02.**
- Analysis: A repairs the field failure without breaking existing deployments
  or adding operator burden. B is a needless compatibility break. C exposes an
  implementation detail as configuration and can be set incorrectly.

#### OQ-DDAE-6-002 - Serviceability list records versus detail calls

- Detailed issue: supplied list records contain many detail-shaped fields, but
  the PDF separately defines `/{id}` detail operations.
- Options:
  - A. Keep list-as-index and fetch documented details.
  - B. Publish directly from list records.
  - C. Try detail and fall back to list content.
- Recommended and selected option: **A, accepted by Peter Chu's direction to
  continue with the recommended next step on 2026-09-02.**
- Analysis: A follows the documented interface and existing validation/state
  contract. B and C would create unproven schema and completeness behavior and
  could publish partially represented records.

#### OQ-DDAE-6-003 - Incomplete events list and pagination

- Detailed issue: one payload returns 500 of 1167 records, while no source
  identifies a supported pagination request.
- Options:
  - A. Preserve incomplete-list safety and defer pagination until authoritative evidence exists.
  - B. Invent common page/offset parameters.
  - C. Treat the first 500 records as complete.
- Recommended and selected option: **A, accepted by Peter Chu's direction to
  continue with the recommended next step on 2026-09-02.**
- Analysis: A is honest and sends no undocumented traffic. B can request the
  wrong records or overload the service. C can emit false disappearance and
  readiness signals.

#### OQ-DDAE-6-004 - Informational severity

- Detailed issue: field payloads use `Informational`; the current bounded event
  enum already contains `info`, and the PDF describes `info` as Informational.
- Options:
  - A. Normalize `Informational` to `info`.
  - B. Keep it as `unknown`.
  - C. Add a new `informational` output enum.
- Recommended and selected option: **A, accepted by Peter Chu's direction to
  continue with the recommended next step on 2026-09-02.**
- Analysis: A preserves the established output contract while representing the
  documented meaning. B discards known severity information. C changes the
  consumer schema and canonical hashes without a need.

### Assumptions

- The supplied payloads came from the intended DDAE/AIDP environment and were
  not transformed in a way that changes JSON structure.
- A documented integer CPU represents CPU cores and can be losslessly passed
  through the existing CPU normalization after canonical decimal conversion.
- The documented detail routes remain available under the DDAE-5 configured
  API prefix; this remains unverified until authenticated detail evidence is
  captured.
- The exact pagination contract is unavailable. DDAE-6 deliberately keeps
  incomplete-list behavior instead of assuming one.

### Open Questions

- None.

### Release Validation Gates

- Named confirmation of the final DDAE-6 Specification v2.1.0 digest.
- Named approval of the marker-bounded DDAE-6 Plan Basis and exact Test Plan
  digest before product-code or test-fixture implementation.
- Deterministic node-envelope, field-alias, quantity, condition, metric,
  severity, list/detail, incomplete-list, redaction and regression tests.
- Authorized sanitized authenticated DDAE validation for nodes and both
  serviceability list/detail families.
- Pagination remains outside DDAE-6 and requires a later approved specification
  supported by authoritative request/consistency evidence.
- Inherited DDAE/Kafka/OpenSearch E2E, deployment, independent review,
  supply-chain and supported-release gates remain unresolved until current
  evidence closes them.
## DDAE-7 Windows 11 portable diagnostics and protected HTTP capture

### Specification Metadata

- Specification Version: `2.2.0`
- Status: active
- Owner: Peter Chu, Project Owner
- Clarification Status: resolved
- Approval Status: candidate; exact requirements, plan and test digests await named approval
- Depends on: DDAE-1 through DDAE-6; current locally implemented DDAE-6 working tree

Peter Chu requested a self-contained test package under repository-root
`Portable/`, selected Windows 11, and directed work to begin. These are scope
decisions, not confirmation of the previously unseen digest-bound candidate.
The security exception below is a proposed decision for his explicit approval.
No product implementation is authorized by this document alone.

### Problem Statement

The maintainer has no DDAE appliance. Field operators need a package that runs
without installing Go, Docker, Python, Git, WSL or third-party diagnostic tools,
records actual API exchanges, and returns useful evidence for parser debugging.
The existing executable supports normal exporter operation; its bounded logs
omit endpoints, response bodies and detailed decode errors. Existing integration
scripts require the source tree, Go and external Kafka prerequisites.

The approved baseline forbids retaining raw DDAE bodies. Turning up log verbosity
does not authorize raw capture. A separate, explicitly enabled, encrypted
diagnostic artifact boundary is required; normal exporter logs must stay safe.

### Goals

- Deliver all Windows field runtime assets within `Portable/`; copying this
  folder is sufficient once operators supply environment-specific inputs.
- Support native Windows 11 x64 and ARM64 without administrative installation.
- Record bounded business API exchanges before parsing, including failing and
  non-JSON responses, and reuse the actual exporter parsers and normalizers.
- Separate transport, decode, semantic validation and list completeness results.
- Provide readable Chinese summaries, protected original payloads and offline
  self-test, decryption and parser replay facilities.
- Complete local deterministic implementation and tests in one approved pass;
  retain native Windows and authenticated field evidence as separate gates.

### Non-goals

- Saving token-endpoint bodies, credentials, bearer tokens or authentication
  cookies as deliberate capture fields; dumping all process/environment data.
- Changing normal exporter configuration, metrics, Kafka schemas or state.
- Fixing the previously audited cluster envelope or alert completeness defects
  implicitly; the package reports baseline behavior without masking defects.
- DDAE mutation, arbitrary endpoints, pagination guesses, route discovery or
  fallback, SQL/query telemetry, Kafka publication or OpenSearch ingestion tests.
- Public Internet connectivity on the field host, automatic upload, installer,
  service registration, registry/security-policy changes, or a supported release
  claim based only on cross-compilation and mocks.
- Packet capture: this is application-level HTTP evidence, not a TLS/TCP dump.

### Actors and Scenarios

- Peter Chu prepares a portable folder and the capture recipient public key,
  keeping the corresponding private key on a trusted analysis host.
- A Windows operator extracts the folder to a writable local NTFS directory,
  supplies authorized non-production DDAE credentials and CA information, runs
  the self-test, then explicitly starts field diagnostics.
- A maintainer reads the safe summary, decrypts selected evidence locally, and
  replays the captured response through the same parsing code without DDAE.
- A reviewer verifies that the normal exporter cannot enable raw capture and
  that raw artifacts are not confused with sanitized release/Harness evidence.

### Requirement Index

| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-7-001 | must | The package MUST contain native Windows amd64 and arm64 exporter and diagnostic executables, architecture-selecting CMD launchers, configuration examples, embedded synthetic self-test assets, Chinese instructions, dependency notices and SHA-256/build manifests under `Portable/`, with no field-time dependency download or installation. | AC-DDAE-7-001 |
| REQ-DDAE-7-002 | must | Diagnostic configuration MUST be strict versioned YAML, resolve relative paths against its own directory, reject unsupported/unsafe inputs before network access, ignore exporter environment overrides, use file-based credentials and guard insecure TLS with two explicit opt-ins. | AC-DDAE-7-002 |
| REQ-DDAE-7-003 | must | Field diagnostics MUST use the fixed OAuth token flow and existing GET allowlist with the two approved prefixes, bounded retries and no redirect/fallback/query/mutation; disabled checks MUST make no request for that check and no Kafka or OpenSearch client may start. | AC-DDAE-7-003 |
| REQ-DDAE-7-004 | must | Diagnostics MUST reuse the production client, DTO decoders and applicable normalizers, record independent HTTP/decode/validation/completeness results, validate selected list/detail IDs, and expose rather than repair baseline parser/contract disagreements. | AC-DDAE-7-004 |
| REQ-DDAE-7-005 | must | Explicit raw capture MUST pair each actual business HTTP attempt with its request and response, retain parser-input body bytes before decoding, include non-2xx/malformed/non-JSON bodies, and label every limit, read error or absent response without silently claiming complete capture. | AC-DDAE-7-005 |
| REQ-DDAE-7-006 | must | Capture MUST exclude token-route bodies and authentication header/cookie values before persistence, keep business originals exclusively in the encrypted channel, and keep URLs, payload values and identities out of ordinary logs, filenames, console and safe reports. | AC-DDAE-7-006 |
| REQ-DDAE-7-007 | must | Raw archives MUST use authenticated encryption with a configured recipient public key, no plaintext spool or fallback, independent authenticated records and an authenticated final index; key generation and offline decryption MUST be provided and private keys MUST never enter the field distribution or result archive. | AC-DDAE-7-007 |
| REQ-DDAE-7-008 | must | Runs MUST bound time, response/header bytes, detail fan-out, request count and disk output, use non-overlapping cycles, cancel on Ctrl+C, retain completed evidence where storage permits, and report interrupted or limit-exhausted work as incomplete. | AC-DDAE-7-008 |
| REQ-DDAE-7-009 | must | Each run MUST create a unique protected directory containing the safe summary/report/step log and manifest, distinguish PASS/FAIL/SKIP/INCOMPLETE, use documented exit codes, and export only an explicit report-and-encrypted-artifact allowlist. | AC-DDAE-7-009 |
| REQ-DDAE-7-010 | must | The diagnostic executable MUST supply offline synthetic self-test and offline captured-body replay through the production decode/validation path, with outbound transport disabled, no field credentials required and no claim of real DDAE compatibility from these results. | AC-DDAE-7-010 |
| REQ-DDAE-7-011 | must | Packaging and documentation MUST explain every included command, required local input, secret-file creation, TLS/prefix settings, encryption/key custody, limits, transfer and cleanup; packaging MUST use an explicit clean allowlist that excludes actual settings, secrets, private keys, results and state. | AC-DDAE-7-011 |
| REQ-DDAE-7-012 | must | Existing normal exporter behavior, defaults, error redaction, metric/Kafka contracts and state MUST remain unchanged; new capture access MUST be confined to the diagnostic entry point and existing regression assertions MUST not be weakened. | AC-DDAE-7-012 |
| REQ-DDAE-7-013 | must | A Windows 11 runtime claim MUST have actual native Windows evidence for the claimed architecture, launchers, ACLs, self-test, encrypted round trip, cancellation and paths containing spaces/non-ASCII characters; cross-build evidence MUST remain separately identified. | AC-DDAE-7-013 |
| REQ-DDAE-7-014 | must | A field-compatibility claim MUST have an explicitly authorized non-production DDAE run proving authentication, selected paths, actual payload parsing and list/detail relationships, with safe retained acceptance evidence and no substitution by mocks or raw archive existence. | AC-DDAE-7-014 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-7-001 | REQ-DDAE-7-001 | Pinned Go 1.26.6, cached locked dependencies and a source-content identity | The portable build runs | Both architectures contain valid matching PE executables; all runtime assets and notices are inside the folder; hashes verify; no source/Go/WSL or online download is required on the target. |
| AC-DDAE-7-002 | REQ-DDAE-7-002 | Valid and malformed YAML, Windows paths, missing secrets/CA, environment canaries and TLS flag combinations | Prepare/config validation runs | Valid inputs resolve deterministically; secrets are never echoed; invalid/unacknowledged settings fail before any network call; environment variables cannot enable publishing or alter the selected endpoint. |
| AC-DDAE-7-003 | REQ-DDAE-7-003 | Recording synthetic TLS servers, both prefix families, disabled checks, 401/403/404/429/5xx and unsafe IDs | Field-mode components execute | Only selected existing GETs and the fixed token POST occur, actual retries are counted, no fallback/query/mutation occurs and no Kafka/OpenSearch connection starts. |
| AC-DDAE-7-004 | REQ-DDAE-7-004 | Documented/legacy structures, missing/null results, 500-of-1167 lists, detail-ID mismatch and invalid optional values | Diagnostic and normal clients consume identical bodies | Their production parser results agree; contract observations separately identify missing/null results and incomplete coverage; HTTP 200 never alone means parser or collection success. |
| AC-DDAE-7-005 | REQ-DDAE-7-005 | Synthetic valid, invalid UTF-8, malformed JSON, HTML error, chunked/compressed and oversized/short bodies | Explicit capture executes and is decrypted | Captured bytes equal the body bytes presented to the parser within bounds, attempts pair correctly, read/limit/absent-response states are explicit, and no extra request is made to obtain a cleaner body. |
| AC-DDAE-7-006 | REQ-DDAE-7-006 | Secret/header/token/URL/body canaries in success and failure paths | Every output and decrypted capture metadata is examined | Token bodies and known authentication header values are absent; business body canaries appear only inside authorized encrypted content before decryption; ordinary exporter logs and safe summaries contain none. |
| AC-DDAE-7-007 | REQ-DDAE-7-007 | Recipient keys, wrong keys, bit flips, reordered/removed/duplicated records and storage faults | Key generation, encryption and decryption run | Correct keys recover exact captured bytes, integrity faults are rejected or explicitly marked incomplete, no plaintext temporary archive exists, and private keys never enter packaging or reporting. |
| AC-DDAE-7-008 | REQ-DDAE-7-008 | Boundary settings, slow servers, retry storms, disk-full, cancellation and forced termination | A bounded run executes | Calls and retained bytes stay within their ceilings, workers stop within the grace period, prior complete records remain readable where possible and interrupted/partial work never becomes PASS. |
| AC-DDAE-7-009 | REQ-DDAE-7-009 | Concurrent runs, pre-existing outputs, empty/disabled/incomplete checks and unsafe destinations | Reports and transfer archive are finalized | Unique protected paths prevent overwrites; exit code and run/step statuses agree; a missing final marker remains incomplete; archive members contain only allowlisted safe files and encrypted captures. |
| AC-DDAE-7-010 | REQ-DDAE-7-010 | Embedded synthetic cases or a valid encrypted capture and matching private key | Self-test/replay runs without credentials with network access forbidden | Deterministic expected outcomes are verified, captured bytes use production decoders, replay cannot execute captured URLs or publish data, and output is labelled synthetic/replay rather than field evidence. |
| AC-DDAE-7-011 | REQ-DDAE-7-011 | A dirty workspace seeded with fake secrets, private keys, old results and unrelated files | Bundle generation and instruction checks run | No seeded sensitive/unrelated file enters the distribution, config/script paths agree, source/build identity is honest, every required user action is documented and generated/runtime paths are Git-ignored. |
| AC-DDAE-7-012 | REQ-DDAE-7-012 | The existing DDAE-1 through DDAE-6 regression suite and normal CLI/YAML examples | Local test/security/build stages run | Existing descriptors, events, state, config and failure behavior remain; normal exporter cannot activate capture; no audited defect is silently patched or hidden by diagnostic assertions. |
| AC-DDAE-7-013 | REQ-DDAE-7-013 | An authorized native Windows 11 host for each claimed architecture, with no extra runtime installed | CMD preparation, self-test, capture round trip, replay and interrupt tests run using synthetic local data | Actual OS/architecture, exit codes and sanitized results are recorded; NTFS DACLs and Unicode/space paths work; absent architectures remain explicitly unverified. |
| AC-DDAE-7-014 | REQ-DDAE-7-014 | An authorized non-production DDAE endpoint, read-only credentials, CA and selected API paths | Windows field diagnostics run | Real authenticated API and parser outcomes, detail coverage and completeness are retained honestly; endpoint/body originals stay protected and sanitized evidence records the authorization/version and results. |

### Interfaces, Data, and Failure Behavior

#### Distribution and commands

All names in this amendment are proposed additions, not existing commands.

```text
Portable/
  README.zh-TW.md
  Prepare.cmd
  Run-SelfTest.cmd
  Run-Diagnostics.cmd
  Create-Analysis-Key.cmd
  Review-Results.cmd
  Run-Exporter.cmd
  config.example.yaml
  exporter.example.yaml
  SHA256SUMS
  build-manifest.json
  THIRD-PARTY-NOTICES.txt
  bin/windows-amd64/{ddae-diagnose.exe,ddae-exporter.exe}
  bin/windows-arm64/{ddae-diagnose.exe,ddae-exporter.exe}
  tools/darwin-arm64/ddae-diagnose
  secrets/                 # operator-provided, never distributed with values
  trust/                   # operator-provided CA, when needed
  keys/                    # recipient PUBLIC key only in a field copy
  results/                 # created per run, never included in a fresh bundle
```

The macOS analysis helper supports the maintainer's current host for key
generation, decryption and replay; it is not a macOS field-support claim.
Synthetic fixtures are embedded into executables. CMD wrappers select native
ARM64 or AMD64 (including a 32-bit shell on a 64-bit OS), anchor paths to their
own directory, preserve exit status and keep interactive failure messages
visible. They do not change PowerShell execution policy or security software.

`Prepare.cmd` validates platform, permissions and bundled file hashes, copies
examples only when runtime files are absent, and explains missing inputs.
`Run-SelfTest.cmd` is network-free except for process-owned loopback fixtures.
`Run-Diagnostics.cmd` runs the bounded DDAE probe, not the full Kafka pipelines.
`Run-Exporter.cmd` is a separate foreground resources-only normal-exporter
example with loopback `127.0.0.1:9469`; it does not capture raw HTTP. Its config
and ordinary stdout/stderr are separate from the diagnostic reports.
`Review-Results.cmd` decrypts/replays on a trusted analysis host using an explicit
private-key path. That key is not needed on the field host.

Runtime inputs cannot be prepackaged: DDAE address, credentials, optional
organization CA, authorization to access non-production data, and the recipient
public key when capture is enabled. Extract to writable local NTFS; reject
reparse-point destinations and filesystems unable to enforce the required ACL.
No administrator rights, background service or registry entry are required by
the design. Organization application-control restrictions remain authoritative.

#### Diagnostic YAML and bounds

Diagnostic YAML version `1` is independent of normal exporter YAML version `1`.
It accepts only the fields below; unknown keys, duplicates and multiple YAML
documents fail. No general environment override or inline credential is allowed.
The existing DDAE URL/prefix, TLS, secret-file, timeout and body-limit validators
are reused without broadening their accepted network boundary. Examples use
`https://ddae.example.invalid`, never real infrastructure values.

| Setting | Default | Bounds / meaning |
|---|---|---|
| `version` | required `1` | Diagnostic configuration schema |
| `ddae.base_url` | required | Existing HTTPS origin validation |
| `ddae.paths.ping_prefix`, `ddae.paths.api_prefix` | empty, `/v1` | Existing two-prefix grammar |
| `ddae.credentials.username_file`, `password_file`, `client_secret_file` | required | Relative to config directory; actual contents never shipped |
| `ddae.tls.ca_file` | empty | Windows trust store plus optional PEM CA |
| `security.allow_insecure_tls`, `ddae.tls.insecure_skip_verify` | false, false | Both required for opt-out; mark evidence insecure, not release-valid |
| `ddae.request_timeout`, `ddae.retry_max` | `5s`, `2` | Existing validator bounds; all attempts count toward run cap |
| `checks.ping`, `checks.resources`, `checks.alerts`, `checks.serviceability_logs` | true | Independently selected probes; at least one field check enabled |
| `run.duration`, `run.interval` | `5m`, `30s` | 1 second to 1 hour; interval 1 to 300 seconds; no overlap |
| `run.max_requests` | `1000` | 1 to 10000 actual HTTP attempts, including authentication |
| `run.max_details_per_family_per_cycle` | `10` | 0 to 100; details come only from validated list IDs |
| `run.shutdown_grace_period` | `15s` | 1 to 30 seconds; completion remains best-effort on storage/OS failure |
| `capture.enabled` | false | Explicit opt-in to the protected raw-data exception |
| `capture.recipient_public_key_file` | required when enabled | RSA public key of at least 3072 bits |
| `capture.max_body_bytes` | `8388608` | 1 to 67108864; effective cap also respects the production operation limit |
| `capture.max_total_bytes` | `536870912` | 1 MiB to 2 GiB, ciphertext and framing included |
| `output.directory`, `output.max_report_bytes` | `results`, `16777216` | Reports 1 to 64 MiB; unique child directory and reserved final-summary space |

Resource bodies retain the production default 4 MiB, serviceability lists
8 MiB and details 1 MiB; the capture cap can lower, never raise, those effective
limits. HTTP headers retain the production 1 MiB ceiling. Oversized or partially
read bodies are explicitly incomplete, not silently truncated JSON.

Details are deduplicated, safely escaped and selected deterministically with
run-local rotation across cycles. Report available/selected/successful detail
counts: a list with complete results does not imply all details were checked.
A detail cap of zero yields SKIP for detail checks. List completeness is
separate from detail coverage. No pagination is inferred from `threshold`.

#### Capture and privacy exception

Subject to Peter Chu approving this candidate, DDAE-7 adds a narrow exception
to REQ-DDAE-1-009, REQ-DDAE-5-005 and the corresponding DDAE-6 raw-content
exclusions: explicitly selected business API exchanges may be retained ONLY
as encrypted diagnostic artifacts. Those inherited requirements remain fully
effective for normal exporter logs, metrics, Kafka, repository fixtures and
sanitized Harness/release evidence. Impacted exception paths start Planned
under REQ-DDAE-7-005 through REQ-DDAE-7-007 and REQ-DDAE-7-012; old historical
trace rows are not rewritten as though they approved raw capture.

Capture request metadata includes method, URL/path, HTTP version and headers;
GET body is explicitly empty. Authentication, proxy-authorization, cookie and
set-cookie headers are removed case-insensitively before serialization. Known
runtime authentication values are also excluded from header metadata. Token
endpoint request/response bodies and header values never reach the capture
sink, including error responses. Its safe status/timing may be recorded.

Business body bytes are captured before any JSON reformatting, normalization
or typed filtering, after the same HTTP content decoding presented to the
production parser. Original transport headers and an explicit decoded-content
flag prevent confusion about compression/content length. This is not a
byte-for-byte wire exchange. Unknown/free-form business bodies can themselves
contain sensitive data, including server-echoed secrets; raw preservation does
not claim to redact those bytes. Encryption and restricted access, rather than
regex sanitization, protect this explicitly approved confidential channel.

Safe reports use fixed operation IDs, numeric run-local request IDs, status
codes, timings, counts, bounded classifications and allowlisted expected/actual
types. Unknown field names and raw error strings are not safe-report data.
Configured endpoints, source IDs, usernames, original headers and body hashes
belong only in the encrypted channel. Pair every retry separately. No new
network request is sent merely to recapture a failed exchange.

Missing response, exceeded body cap, exceeded total cap, read failure and
cancellation have explicit `capture_complete: false` and a bounded reason.
Global resource exhaustion stops new network work and marks the run incomplete.
If capture is enabled but encryption/key/ACL initialization fails, no field
request is made and no plaintext fallback is attempted.

#### Encryption, files and replay

Use Go standard-library RSA-OAEP-SHA256 to wrap a fresh random 256-bit run key,
and AES-256-GCM for individually authenticated capture records. Each run has a
fresh cryptographic run ID/key and a unique monotonic nonce counter. Version,
run ID, header digest and sequence number are authenticated with each record.
An encrypted final index authenticates count/order/completion; loss of this
index is incomplete even when earlier independent records can be recovered.
Record framing and ciphertext sizes are bounded before allocation/decryption.

Key generation never overwrites an existing destination and writes the private
PKCS#8 key only to the caller-selected trusted analysis location with restricted
permissions. Only the public PKIX PEM key is copied into a field package.
There is no private key or universal decryption password embedded in binaries.
Windows uses explicit protected DACLs for the current user and SYSTEM; POSIX
analysis files use private directory/file modes. Fail closed when protection
cannot be established. Encryption does not protect a compromised host or memory.

Each run produces `summary.txt`, `report.json`, `steps.jsonl`, `diagnostic.log`,
`schema-observations.json`, `manifest.json`, and optional `http-capture.ddaecap`.
A transfer ZIP is not itself the encryption boundary: its raw member is already
encrypted and the other members are safe summaries. It never includes runtime
YAML, credential files, CA/key directories, normal exporter state or unrelated
files. It is created from an explicit member allowlist, never recursive capture
of `Portable/`. Do not auto-delete evidence; document manual retention/cleanup
under the operator's data policy.

Offline replay selects fixed operations recorded inside authenticated content,
uses the same production decode/normalization functions, and cannot execute
stored URLs, load plugins or publish data. Decryption requires an explicit
protected output directory and never overwrites or follows unsafe archive paths.
Interrupted archives may expose authenticated completed records with an
INCOMPLETE warning; never describe them as an intact full run.

Statuses are PASS, FAIL, SKIP and INCOMPLETE. HTTP success, decode success,
semantic validation, source-list completeness and detail coverage are separate
fields. A missing prerequisite has a reason rather than an invented result.
Exit codes: `0` all enabled/available selected checks pass and run is complete;
`1` completed with a failed check; `2` usage/configuration/platform/input error;
`3` runtime limit/storage failure prevents completion; `130` user interruption.
Priority is interruption, runtime incompleteness, input error, failed check,
success. Disabled checks are SKIP; enabled unavailable detail work prevents an
unqualified full-coverage claim. Startup failures before a protected output
directory can be created may have only a safe console message and nonzero exit.

### Quality Attributes

- Repeatability: build manifest identifies exact source content, dirty state,
  toolchain, architecture, spec candidate and test-evidence scope.
- Independence: all field dependencies are executable or embedded assets;
  credentials, CA and recipient key are environment inputs, not downloads.
- Confidentiality/integrity: raw data is opt-in, encrypted before disk and
  isolated from ordinary logs; transport/auth protections remain unchanged.
- Diagnosability: original parser-input bytes and separate status layers expose
  both format failures and permissive-parser/contract disagreements.
- Bounded failure: no unbounded file, retry, response, detail traversal or run.

### Compatibility and Migration

This is an additive Specification v2.2.0 diagnostic boundary, not a new exporter
release version. The DDAE-6 local working tree is the implementation baseline,
not a claim that all of its release gates or audited defects are resolved.
Normal exporter schema/version/CLI and supported deployment profiles remain
unchanged. Packaging records the actual dirty source-content fingerprint rather
than attributing uncommitted changes to the last release or commit alone.

Rollback consists of stopping diagnostic processes and using the previous
exporter; no DDAE mutation or existing outbox/checkpoint migration occurs.
Retain private keys separately for the archives they decrypt. Preserve valuable
old tests and add narrowly scoped exception tests, never disable raw-data checks
globally. Generated binaries/runtime captures are ignored, not committed.

### Design Decisions for Approval

| Issue / evidence | Options, one per line within this register | Recommendation and consequence | Decision authority / actual status |
|---|---|---|---|
| Windows 11 CPU architecture was not specified | Ship x64 only | Smaller package but does not establish native ARM64 support | Peter Chu; not selected |
| Same architecture issue | Ship native x64 and ARM64 with automatic selection | Recommended: removes a field setup dependency; each native runtime still needs its own evidence | Peter Chu; proposed for digest approval |
| Existing raw retention ban conflicts with requested complete content | Retain structural summaries only | Lower exposure but cannot faithfully reproduce unknown-body parsing failures | Peter Chu; superseded recommendation, not the requested capture outcome |
| Same raw retention issue | Retain encrypted business originals, omit authentication material | Recommended: preserves parser input; requires confidential-data custody and a separate private key | Peter Chu; proposed exception, not yet approved |
| Same raw retention issue | Retain unredacted authentication and plaintext bodies | Not recommended: unnecessary credential exposure and conflicts with existing secret policy | Peter Chu; not authorized |
| Only macOS local execution is available | Claim Windows support from cross-build | Rejected: PE creation is not Windows runtime validation | Existing evidence rule |
| Same runtime evidence issue | Deliver locally verified candidate and retain native Windows/field gates | Recommended: useful portable candidate with honest verification limits | Peter Chu; proposed for local-pass approval |

### Assumptions

- The field operator can extract the folder to local writable NTFS, run approved
  executables and reach an authorized non-production DDAE over its internal
  network; application-control exceptions are handled by the organization.
- A trusted analysis host retains the private key. Its public key, CA and
  credentials can be transferred through the operator's approved process.
- Windows x64 and ARM64 are package targets; no exact Windows build number or
  live DDAE availability is assumed to have been tested.
- Full raw business bodies are confidential and can contain unknown sensitive
  values. No automatic upload or raw production fixture is authorized.

### Open Questions

- None.

### Release Validation Gates

- Named approval of this specification, marker-bounded Plan Basis and Test Plan
  before diagnostic source, launchers or deliverable binaries are produced.
- Deterministic local tests, race/coverage/lint/security and target builds.
- Actual Windows 11 execution per claimed architecture and separately
  authorized DDAE field evidence. Neither is inferred from local results.
- Existing independent security review, supply-chain and external release
  gates remain; no commit/tag/release/publication is authorized by this change.

## DDAE-8 Cluster response compatibility

### Specification Metadata

- Specification Version: 2.4.0
- Status: active
- Owner: Repository Maintainers
- Clarification Status: resolved

### Problem Statement

The authorized demo returned an object containing a results array. The live
cluster client expects a bare array and fails to decode it. Other resource
collectors succeeded. This independent amendment supersedes DDAE-7's excluded
cluster fix only; historical DDAE-7 requirements and evidence remain preserved.

### Goals

Accept both cluster list shapes through the same production/replay decoder.

### Non-goals

CA trust, API defaults, Windows STATE_DIR, alerts, Kafka, packaging and releases.

### Actors and Scenarios

Operators collect resources from an authorized DDAE target; maintainers replay
synthetic or authorized captured responses using the same typed parser.

### Requirement Index

| Requirement ID | Priority | Requirement | Acceptance IDs |
|---|---|---|---|
| REQ-DDAE-8-001 | must | The cluster decoder MUST accept bare arrays and objects containing a non-null array-valued `results`. Both forms MUST yield the same typed clusters and existing normalization/metrics for equivalent content. Unknown envelope fields MUST NOT enter metrics or logs. | AC-DDAE-8-001 |
| REQ-DDAE-8-002 | must | Missing/null/non-array `results`, unsupported top levels, null/non-object items, malformed JSON and trailing JSON MUST fail. Existing response-size, nesting, request, retry, identity, optional-resource validation and stale/partial-failure limits MUST remain effective. | AC-DDAE-8-002 |
| REQ-DDAE-8-003 | must | Live collection, recorded-body decoding and portable diagnostics/replay MUST use the same cluster decoder. A valid envelope MUST no longer fail solely because its top level is an object. | AC-DDAE-8-003 |

| REQ-DDAE-8-004 | must | The cluster decoder MUST accept legacy string clusterStatus and an object with a required non-null string status; it MUST preserve absent legacy status behavior, use the unchanged status normalizer, ignore message/reason metadata and reject malformed object status. | AC-DDAE-8-004 |
| REQ-DDAE-8-005 | must | Coordinator and worker resources MUST accept legacy direct CPU/memory and nested resources objects; CPU quantity strings and non-negative base-10 JSON integers MUST map to the existing quantity-string DTO, retaining existing optional-value handling, quantity validation and metric units; other numeric forms/types and malformed/null resources objects MUST fail. | AC-DDAE-8-005 |
| REQ-DDAE-8-006 | must | Simultaneous direct and nested resource representations for the same role MUST fail; existing routes, TLS, security, metric identity and freshness behavior MUST remain unchanged. | AC-DDAE-8-006 |

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-8-001 | REQ-DDAE-8-001 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | synthetic nonempty and empty arrays/envelopes produce equivalent typed data and cluster metrics. |
| AC-DDAE-8-002 | REQ-DDAE-8-002 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | invalid-shape, malformed-item, trailing/oversized-body, duplicate/missing-ID and missing/invalid optional-resource cases remain failures at their appropriate decode/validation layer; failures do not become successful empty inventory. |
| AC-DDAE-8-003 | REQ-DDAE-8-003 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | client/recorded/portable results agree for equivalent valid and invalid fixtures; synthetic self-test expectations match the new contract. |

| AC-DDAE-8-004 | REQ-DDAE-8-004 | Synthetic legacy and observed field layouts | Live and recorded cluster decoding and metric normalization run | Equivalent legacy/object status yields the same normalized state; missing/null/non-string object status fails; metadata does not enter output. |
| AC-DDAE-8-005 | REQ-DDAE-8-005 | Synthetic legacy and observed field layouts | Live and recorded cluster decoding and metric normalization run | Both layouts produce identical configured CPU/memory metrics and recorded/live values; invalid numeric forms and resource shapes fail; absent optional quantities are omitted. |
| AC-DDAE-8-006 | REQ-DDAE-8-006 | Synthetic legacy and observed field layouts | Live and recorded cluster decoding and metric normalization run | Mixed layouts within one role fail even if values agree, while coordinator and worker may independently use either layout; no output contains ignored metadata. |

### Interfaces, Data, and Failure Behavior

Preserve existing cluster DTO fields, public return types, metrics, API
allowlist, authentication, deadlines/retries, body/nesting limits and freshness.
Accept [] and {"results":[]} as empty lists; reject absent/null inventory.
Preserve identity and optional-resource normalization. The approved field-mapping
amendment adds only clusterStatus.status, role.resources and integer CPU input;
normalized DTO fields and metric meanings remain unchanged.
Unknown envelope metadata is not exposed. Diagnostic decode, semantic validity
and contract results remain separate.

### Quality Attributes

Bounded typed decoding, synthetic regression fixtures and honest partial health.
Live diagnostic TLS bypass is functional evidence only, not trusted-TLS evidence.

### Compatibility and Migration

Preserve valid legacy arrays; expand accepted results envelopes. Deliberately
reject top-level null and null items. No state migration or API version claim.
Rollback uses the previous binary without discarding unrelated local changes.

### Assumptions

The observed structure identifies the decoder mismatch, not a new supported
product version. Existing cluster field semantics and validation remain valid.

### Open Questions

- None.

## Verification Record Location

Current execution results and limits are retained in plans/DDAE-8.md.

## DDAE-9 Query monitoring

### Specification Metadata

- Specification Version: 2.5.0
- Status: active
- Owner: Current repository user in this Codex conversation
- Clarification Status: resolved

### Problem Statement

Management API collection does not include query queue, duration or submitter.
The approved candidate demonstrates version-specific Insights read access.

### Goals

Add disabled-by-default query metrics and optional isolated Kafka details.

### Non-goals

No SQL capture/submission/cancellation, permission changes, Portable restoration,
complete audit guarantee, automatic endpoint guessing or release certification.

### Actors and Scenarios

Operators configure a read-only Insights account with all-query visibility and
consume aggregate metrics or sanitized per-query details.

### Requirement Index

| Requirement | Priority | Requirement text | Acceptance |
|---|---|---|---|
| REQ-DDAE-9-001 | must | Query pipeline MUST 預設關閉；啟用後獨立設定 Insights HTTPS origin、明確 auth HTTPS origin/realm、角色與獨立密碼檔。MUST NOT 將 Management API client secret 自動送到 Insights。關閉時不驗證、不建立或使用此 pipeline 的認證、Kafka 或 state 資源。 | AC-DDAE-9-001 |
| REQ-DDAE-9-002 | must | MUST 使用觀察到的 OIDC 網頁登入與記憶體 cookie jar；登入 redirect 最多 8 次，只准設定的兩個 origin 及限定 auth/callback 路徑。密碼 POST 只能送到指定 auth origin 的 realm login-actions/authenticate；拒絕其他 host、HTTP、未知表單與 MFA/額外流程，不自動降級。一般業務請求只能 GET allowlist。401 最多重新登入一次；403 不以其他角色繞過。全程 TLS 預設驗證，沿用明確雙重 diagnostic opt-in，且與 Kafka/Management transport 隔離。 | AC-DDAE-9-002 |
| REQ-DDAE-9-003 | must | 每次 collection MUST 以設定角色確認 cluster/info.allQueries=true，否則不可將局部資料標示為叢集統計。角色只在 request header 選擇，不修改授權。受支援 schema 以 479-e.4 驗證；缺欄位、錯誤型別、HTML、截斷 JSON、重複 identity 或不合法時間 MUST 回報可辨識失敗。 | AC-DDAE-9-003 |
| REQ-DDAE-9-004 | must | running/queued MUST 取 overview 最新有效 upstream time 樣本，使用 gauge；樣本過期或沒有樣本不可輸出零或延長其新鮮度。MUST NOT 把 failedQueries 樣本或歷史清單筆數當作累積計數。背景輪詢不重疊；scrape 不觸發 API。 | AC-DDAE-9-004 |
| REQ-DDAE-9-005 | must | MUST 用 bounded history list + bounded detail GET 蒐集已觀察到的逐筆查詢；以 source_instance/query_id 識別。API 的 list 完整性與保留期限未經證明時，history coverage MUST 標為 unknown，不承諾完整稽核，也不依缺席推論查詢已完成或刪除。detail 未取得前不可捏造 user 或 duration。 | AC-DDAE-9-005 |
| REQ-DDAE-9-006 | must | Kafka query schema v1 MUST 只包含 source_instance、query_id、user、source、固定狀態、提交/完成/觀察時間及 elapsed/queued/execution/cpu 秒數；缺少 optional 欄位省略。MUST NOT 包含 SQL、prepared SQL、IP、principal、session properties、catalog/table metadata、任意錯誤文字、憑證或 cookie。user 與 source 是 detail data，不能作為 Prometheus labels。 | AC-DDAE-9-006 |
| REQ-DDAE-9-007 | must | Query detail Kafka MUST 使用獨立 topic、query-events.db、publisher 與 bounded durable outbox；預設 event 輸出關閉。topic 不可與 alerts/logs 重複。穩定 key、at-least-once、下游 idempotent upsert；重複 poll/重啟不得重複計入已觀察完成數/耗時。入列、dedup checkpoint 與 aggregate 更新需同一 transaction。滿額、寫入/送出失敗不可靜默丟失，MUST 顯示 unhealthy/backpressure。 | AC-DDAE-9-007 |
| REQ-DDAE-9-008 | must | MUST 提供有界 metrics、freshness、scope、history coverage、detail failure、outbox 與 publish 狀態。持久化已觀察 terminal query 的計數與 histogram aggregate，明確命名 observed，不能宣称代表所有查詢。未知 state 映射 unknown，不當成功或失敗終態。整體 health/readiness 需反映已啟用 Query pipeline，保留其他 pipeline 的 partial failure 語意。 | AC-DDAE-9-008 |
| REQ-DDAE-9-009 | must | MUST 更新 README、runbook、YAML/env precedence、deployment examples、NetworkPolicy 對 Insights/IdP 的設定需求與 SDD traceability；正常建置不含 Portable。測試 fixture 僅用合成資料。實測結果須區分功能、TLS、非零負載、Kafka 與完整 Harness gate，不聲稱未執行的項目通過。 | AC-DDAE-9-009 |

### Acceptance Criteria

| Acceptance | Requirement | Criterion |
|---|---|---|
| AC-DDAE-9-001 | REQ-DDAE-9-001 | TEST-DDAE-9-001 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-002 | REQ-DDAE-9-002 | TEST-DDAE-9-002 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-003 | REQ-DDAE-9-003 | TEST-DDAE-9-003 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-004 | REQ-DDAE-9-004 | TEST-DDAE-9-004 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-005 | REQ-DDAE-9-005 | TEST-DDAE-9-005 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-006 | REQ-DDAE-9-006 | TEST-DDAE-9-006 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-007 | REQ-DDAE-9-007 | TEST-DDAE-9-007 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-008 | REQ-DDAE-9-008 | TEST-DDAE-9-008 verifies the approved requirement with synthetic success, boundary and failure cases. |
| AC-DDAE-9-009 | REQ-DDAE-9-009 | TEST-DDAE-9-009 verifies the approved requirement with synthetic success, boundary and failure cases. |

### Interfaces, Data, and Failure Behavior


新增 internal/queryclient（schema、固定 GET routes、OIDC session）與
internal/queries（scheduler、normalization）、internal/querystate（獨立 bbolt
狀態/outbox）、query publisher；沿用現有 Kafka transport/TLS 實作，避免改變
alert/log event schema。整合 app lifecycle、snapshot、metrics、server health、
config/YAML/env，query-only 模式不得要求 Management API 認證。

固定業務 GET routes：/ui/api/insights/cluster/info、
/ui/api/insights/overview/queries、/ui/api/insights/history/queries、
/ui/api/insights/history/queries/{validated-id}。不跟隨回應中的 self URL。
近期 /ui/api/query 已探索，但本次不作為完整並行數來源，也不新增第二套 duration
schema：並行數使用 overview、逐筆 detail 使用 Insights history。

ID 僅接受 1–256 bytes 的 ASCII 字母、數字、底線、連字號；URL escape path
segment；任何不相符 ID 回報 invalid，不帶入任意 URL。JSON duration 嚴格接受
非負整數 milliseconds，轉為 seconds；缺值與 0 分開。UTC RFC3339 timestamp
嚴格驗證；拒絕不合理未來樣本（超過 5 秒），elapsed 不能以 CPU time 取代。
同一 history list 內重複 ID 視為完整性錯誤。

預設 interval=30s、request_timeout=5s、cycle_timeout=20s、stale_after=90s、
response_max_bytes=16MiB（上限64MiB）、detail.max_per_cycle=100、concurrency=4
（1–32）、max_history_records=1000（1–10000）、retry_max=2，重试仅限 GET
的暫時性錯誤且受 cycle deadline 約束。overview 與 details 狀態獨立，detail
失敗不得抹去有效的 overview；但 enabled pipeline readiness 必須反映失敗。

Checkpoint 預設上限100000、retention=720h；過期 checkpoint 再出現的終態 record
不得重新計入 aggregate：以持久化 terminal watermark/retention floor 限制
可接納時間，晚到且超界的 record 回報 gap，不假裝已完整處理。達到列表上限、
超過本地預算或 history continuity 未證明時，coverage 保持 unknown/limited。
不使用未驗證的 pagination 參數來宣稱掃描完整。

Outbox 預設256MiB/10000事件，单事件上限64KiB；user/source 各上限1024 UTF-8
bytes，不截斷身份造成碰撞。Kafka event export 關閉時仍需 query state 以保留
observed counters；不建立 Kafka producer。安全例外只涵蓋選定 query detail
欄位，禁止原始 payload logging/capture。

### Metric mapping

所有 metric HELP 必須說明 upstream sample 或 observed coverage；下列皆無 user、
query_id、source、SQL 等 unbounded labels。固定 state label 不超過
finished/failed/canceled/unknown；Histogram state 僅前三個 terminal 狀態。

| Prometheus name | Type / unit | Source and behavior |
|---|---|---|
| ddae_queries_running | gauge / queries | latest overview.metric.runningQueries；非負整數；stale 時省略 |
| ddae_queries_queued | gauge / queries | latest overview.metric.queuedQueries；非負整數；stale 時省略 |
| ddae_query_sample_timestamp_seconds | gauge / Unix seconds | overview.time；不使用 HTTP fetch time 取代 |
| ddae_query_collection_success | gauge / boolean | 本次 overview/schema/scope 檢查成功 |
| ddae_query_detail_collection_success | gauge / boolean | 本次 bounded detail collection 無失敗 |
| ddae_query_scope_all | gauge / boolean | allQueries=true 且本次 scope 檢查有效；失敗為0 |
| ddae_query_history_complete | gauge / boolean | 本次 list 的完整性有證據才為1；本版 UI API 未證明時為0 |
| ddae_queries_observed_completed_total{state} | persistent counter / queries | deduplicated terminal records，非叢集完整總數 |
| ddae_query_observed_elapsed_seconds{state} | persistent histogram / seconds | 終態 elapsedTime/1000，只記一次；buckets .01,.05,.1,.5,1,5,10,30,60,300,+Inf |
| ddae_query_observed_execution_seconds{state} | persistent histogram / seconds | 終態 executionTime/1000，缺值省略，不填0 |
| ddae_query_observed_queued_seconds{state} | persistent histogram / seconds | 終態 queuedTime/1000，缺值省略，不填0 |
| ddae_query_events_pending | gauge / events | durable outbox pending count |
| ddae_query_event_publish_success | gauge / boolean | query publisher 狀態；未啟用 event export 時省略 |

新增 query mode 的 series budget 固定 <200；不得把 sampled gauges 單純積分成
準確 throughput，也不得用 truncated history 推算平均延遲/P95 的完整母體。


### Quality Attributes

Bounded requests, durable deduplication, safe secret handling and fixed labels.

### Compatibility and Migration

Existing pipelines remain unchanged; separate query-events.db and topic;
disable query monitoring to roll back without deleting data.

### Assumptions

Insights UI API compatibility was observed on 479-e.4. History coverage is
unknown, TLS bypass evidence is diagnostic only, and MFA is unsupported.

### Open Questions

- None.

## DDAE-10 Bounded history backfill

### Specification Metadata

- Specification Version: 2.6.0
- Status: active
- Owner: Current repository user in this Codex conversation
- Clarification Status: resolved

### Problem Statement

Single lists truncate serviceability logs at 500 and query history at 1000.

### Goals

Approved bounded time-window backfill, durable progress, safe replay and isolated polling.

### Non-goals

No full permanent audit guarantee, new service-health metrics, SQL, Portable or deployment.

### Actors and Scenarios

Operators opt in to bounded recovery of histories beyond one response.

### Requirement Index

| Requirement | Priority | Requirement text | Acceptance |
|---|---|---|---|
| REQ-DDAE-10-001 | must | MUST 提供獨立、預設關閉的 logs/query backfill 設定；關閉時不建立回補 worker/狀態。啟用須有對應主 pipeline。YAML/env precedence 沿用現有規則。 | AC-DDAE-10-001 |
| REQ-DDAE-10-002 | must | MUST 僅由 typed UTC bounds 產生固定清單路徑及必要參數；禁止任意 filter、URL、offset、user/SQL 篩選；沿用各自認證與 TLS 約束。 | AC-DDAE-10-002 |
| REQ-DDAE-10-003 | must | MUST 固定每輪掃描的上下界；logs 用 updatetime，queries 用 startDate/endDate。達上限即切分，邊界保守重疊再去重；最小 1 秒仍達上限時停止推進並標 incomplete。 | AC-DDAE-10-003 |
| REQ-DDAE-10-004 | must | MUST 持久化未完成區間、待處理 ID/marker、已確認進度，綁定來源與設定指紋。只有資料已交給既有 durable store 後才可確認進度；崩潰可重做、不能跳過。 | AC-DDAE-10-004 |
| REQ-DDAE-10-005 | must | MUST 沿用既有事件白名單、獨立 topic、at-least-once 及 observed aggregates；跨前景/回補/重啟重複不能重計終態，不能以舊 marker 覆寫較新狀態。filtered list MUST NOT 用於 absence reconciliation。 | AC-DDAE-10-005 |
| REQ-DDAE-10-006 | must | MUST 限制頁數、detail 數、併發、response bytes、工作佇列與每輪期限；獨立背景回補不占用即時 overview 的執行鎖／請求配額；scrape 不呼叫 DDAE。 | AC-DDAE-10-006 |
| REQ-DDAE-10-007 | must | MUST 回報有界進度、失敗、過期與容量狀態；完成僅表示指定窗口走訪結束。query_history_complete 維持 0，不能把有限窗口當全域完整清單。 | AC-DDAE-10-007 |
| REQ-DDAE-10-008 | must | MUST 定義回滾與 retention gap；不因持久化進度損毀而自動丟棄資料，不因 interval 改變重設 checkpoint。停用回補保留既有事件及進度。 | AC-DDAE-10-008 |
| REQ-DDAE-10-009 | must | MUST 更新完整設定範例、README、runbook、traceability，並區分合成測試、Demo 功能、trusted TLS、Kafka 及完整 Harness 結果。 | AC-DDAE-10-009 |

### Acceptance Criteria

| Acceptance | Requirement | Criterion |
|---|---|---|
| AC-DDAE-10-001 | REQ-DDAE-10-001 | TEST-DDAE-10-001 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-002 | REQ-DDAE-10-002 | TEST-DDAE-10-002 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-003 | REQ-DDAE-10-003 | TEST-DDAE-10-003 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-004 | REQ-DDAE-10-004 | TEST-DDAE-10-004 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-005 | REQ-DDAE-10-005 | TEST-DDAE-10-005 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-006 | REQ-DDAE-10-006 | TEST-DDAE-10-006 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-007 | REQ-DDAE-10-007 | TEST-DDAE-10-007 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-008 | REQ-DDAE-10-008 | TEST-DDAE-10-008 exercises approved success, failure and boundary matrix. |
| AC-DDAE-10-009 | REQ-DDAE-10-009 | TEST-DDAE-10-009 exercises approved success, failure and boundary matrix. |

### Interfaces, Data, and Failure Behavior



以下兩處具有相同 leaf keys：
monitoring.serviceability_logs.backfill、monitoring.queries.backfill。
環境前綴分別為 SERVICEABILITY_LOG_BACKFILL_ 與 QUERY_BACKFILL_。
主 pipeline 預設開關不改動；本功能不會自動啟用 queries 或 logs。

| YAML leaf / ENV suffix | 預設 | 限制與意義 |
|---|---|---|
| enabled / ENABLED | false | 對應主 pipeline 關閉而本設定為 true 時報錯 |
| lookback / LOOKBACK | 24h | 1h–720h，且不得大於對應 checkpoint retention |
| overlap / OVERLAP | 2m | 1s–1h、不得大於 lookback；增量掃描向前重疊 |
| interval / INTERVAL | 30s | 5s–1h；相同 worker 不重疊執行 |
| cycle_timeout / CYCLE_TIMEOUT | 20s | 主 client request_timeout < 此值 < interval |
| rescan_interval / RESCAN_INTERVAL | 1h | >= interval、<= lookback；重新走訪滾動 lookback 以捕捉晚到資料 |
| max_pages_per_cycle / MAX_PAGES_PER_CYCLE | 4 | 1–32，邏輯 list 呼叫數，retry 另受既有 retry_max 與 cycle deadline 限制 |
| detail_max_per_cycle / DETAIL_MAX_PER_CYCLE | 25 | 1–1000，與前景配額分開計算，文件列明合併上界 |
| detail_concurrency / DETAIL_CONCURRENCY | 2 | 1–8，<= detail_max_per_cycle |
| max_pending_records / max_pending_records 的大寫形式 | 10000 | 1000–100000，超限 backpressure，不丟棄 ID |

固定限制：最多 4096 待掃區間、32 MiB 進度邏輯資料、單次進度交易有界。
回補頁面沿用相應 list response byte limit。Query 頁面判斷使用觀察到的
1000 截斷界限；QUERY_MAX_HISTORY_RECORDS 必須 >=1000 才能啟用回補，
它不是可控制伺服器回傳上限的參數。Logs 採有效 threshold 及 totalRecords。

初次回補下界為 now-lookback，上界為啟動時固定的 now（秒精度向外取整）。
後續增量由 last completed end-overlap 起至新固定 end；每 rescan_interval
重掃當時的 rolling lookback，捕捉較晚才出現且仍在窗口內的紀錄。
不自動回補窗口之外的歷史；改 lookback 明確建立新的 bounded sweep，
已入列事件與 dedup checkpoint 不清除。

### API, State and Failure Contract

Logs：GET <configured api prefix>/serviceability-events，唯一新增參數 filter，
由程式產生 `(updatetime ge "<UTC>") and (updatetime le "<UTC>")`。
Queries：GET /ui/api/insights/history/queries，唯一新增參數為
sortBy=createDate、sortOrder=desc、filter={startDate,endDate}。
Scope 檢查仍在 query worker 使用既定 cluster/info 執行；報表 endpoint
僅屬探索驗證，不納入正式 runtime API。

驗證所有頁面 identity、時間型別與範圍；未符合邊界（容許 outward rounding
重疊）不可當作完成窗口。Logs totalRecords 與有效 unique count 不同時
切分；缺 threshold/total、malformed、duplicate ID 或無法切分均 fail closed。
Query 頁面 count>=1000 必須切分；小於上限只能作該窗口可走訪的操作性證據，
不是完整永久歷史。結果超出本地 response/max record 限制仍屬失敗。

使用新的獨立 STATE_DIR/history-backfill.db（schema v1）存兩種 pipeline 的
游標／待處理 IDs；不儲存 SQL、user、來源地址或原始回應。維持既有
query-events.db / serviceability-logs.db schema 與事件格式。
跨 DB 採保守順序：先 durable 記錄事件／既有去重結果，再 ack 進度。
任何中斷只能重播；禁止先 ack 造成遺漏。資料庫來源 origin/source identity
與窗口設定指紋不相符須報出可處理錯誤，不能自動套用到另一目標。

前景與回補可能讀到同 ID：共享既有 store 的交易序列化與事件版本檢查，
source updated marker 更舊時不覆寫较新 checkpoint／pending event；詳細資料
回應需重新驗證 ID。缺失 source marker 的事件仍按既有 content hash 去重，
不得虛構來源時間保證。Query terminal 計數僅使用既有原子 Record 入口一次。

超過既有 retention floor 的 query 不回補計數；記錄 gap、保持 incomplete。
進度未完成時不得因 pruning 悄悄移動下界；明確回報 expired 狀態，等待縮小
窗口／重新開始的操作指示。404、403、解析失敗、容量不足均保留待處理項目，
不由 disappearance 推論業務完成或刪除。取消必須在既有 shutdown grace 內結束。

### Observability and Readiness

新增固定 pipeline label：serviceability_logs、queries（僅啟用者輸出）。

| Metric | Type/unit | 行為 |
|---|---|---|
| ddae_history_backfill_enabled | gauge/bool | 已啟用回補 |
| ddae_history_backfill_success | gauge/bool | 最近 worker cycle 無 source/state/detail 錯誤；啟動前 0 |
| ddae_history_backfill_pending_windows | gauge/windows | durable 待走訪區間數 |
| ddae_history_backfill_pending_records | gauge/records | 尚未交付 store 的 IDs 數 |
| ddae_history_backfill_last_completed_timestamp_seconds | gauge/Unix seconds | 最近完成固定窗口的上界；從未完成時省略 |
| ddae_history_backfill_incomplete | gauge/bool | 當前窗口仍待掃／受阻／gap，無未完成項目時 0 |
| ddae_history_backfill_blocked | gauge/bool | 同秒超限、過期、容量或持久化錯誤阻止前進 |

每名稱 HELP 說明有限窗口，不代表所有歷史；最多 14 額外 series。
沒有 time/ID/user/source/error message labels；時間只能作 metric value。

回補關閉：既有 /readyz 行為保持。回補開啟：
- pending 本身不等於故障；最近成功、未過期且沒有 blocked 的 worker 可 ready。
- 連續來源／detail／state 失敗、同秒超限、retention gap 或 stale (>3*interval)
  使該回補分支不 ready；不移除仍有效的其他 metrics。
- 對 logs，在回補模式下，已驗證合法但被截斷的前景清單可繼續收集，
  readiness 由 healthy 前景及回補共同決定；不再單憑截斷就永久不 ready。
  原全域 list_complete 仍保持 0，時間窗口完成不得將它改為 1。
- 對 query，running/queued 的 freshness/scope 判斷不變；回補 worker 失敗
  影響 readiness 但不得覆寫即時 overview 狀態。healthz 仍是 process liveness。


### Quality Attributes

Bounded requests, durable progress, privacy, freshness and stable outputs.

### Compatibility and Migration

Default off; preserve existing state schemas and outputs; separate progress database.

### Assumptions

Demo 479-e.4 observed time filtering; source retention and transactional snapshot semantics unknown.

External release gates remain incomplete; source retention, timestamp saturation and Kafka/E2E remain known limitations as in the approved candidate.

### Open Questions

- None.
## DDAE-11 Query-only Kafka SASL credential loading

### Specification Metadata

- Specification Version: 2.7.0
- Status: active
- Owner: Current repository user in this Codex conversation
- Clarification Status: resolved

### Problem Statement

With alerts and Serviceability Logs disabled, query events omit configured
Kafka SASL credentials. Resource monitoring does not affect this condition.
The common producer consumes the resulting empty credential fields.

### Goals

Load and validate existing Kafka SASL credentials for query events through
the common configuration loader, including query-only operation.

### Non-goals

No new authentication methods, configuration keys, dependencies, TLS changes,
Kafka schema/topic/state changes, deployment, commit or release.
Inherited verification gates and historical approvals remain unchanged.

### Actors and Scenarios

An operator enables query events and uses PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512.
Valid credentials reach Config; missing/invalid credentials fail at startup.

### Requirement Index

| Requirement | Priority | Requirement text | Acceptance |
|---|---|---|---|
| REQ-DDAE-11-001 | must | 任一 Kafka 輸出（alerts、serviceability logs、query events）啟用且選用 SASL 時，MUST 透過既有 loader 載入 username 與 password。適用 PLAIN、SCRAM-SHA-256、SCRAM-SHA-512；MUST 保留 YAML/env precedence 與 direct/file secret 規則。 | AC-DDAE-11-001 |
| REQ-DDAE-11-002 | must | Query-only events + SASL MUST 在設定載入時拒絕缺少或不合法的必要帳密、不可讀取／空值／過大／內容不合法的密碼檔與 direct/file 衝突；錯誤不得包含帳密、檔案內容或底層錯誤中的敏感值。 | AC-DDAE-11-002 |
| REQ-DDAE-11-003 | must | MUST 保留 query metrics-only 與 resources-only 的 Kafka secret 隔離，保留 alerts/logs/mixed profiles 的既有認證行為。未選用 SASL 時不新增 SASL 帳密要求；既有 TLS/mTLS 規則、broker/topic 驗證與 query pipeline 關閉行為不變。 | AC-DDAE-11-003 |

### Acceptance Criteria

| Acceptance | Requirement | Criterion |
|---|---|---|
| AC-DDAE-11-001 | REQ-DDAE-11-001 | Three mechanisms, direct/file password, YAML and env overrides yield the exact configured credential values. |
| AC-DDAE-11-002 | REQ-DDAE-11-002 | Missing/invalid/conflicting credentials and invalid secret files fail loading without exposing canaries. |
| AC-DDAE-11-003 | REQ-DDAE-11-003 | All publisher combinations preserve credential loading/isolation; disabled query and no-SASL behavior remain unchanged. |

AC-DDAE-11-001：使用三種 SASL mechanism，測試 query-only events 的 direct
password、password file、YAML 設定，以及 env 對 YAML username/mechanism/
password file 的覆寫。逐一驗證實際 Config secret 值，不能只檢查無錯誤。

AC-DDAE-11-002：測試缺少 username/password、空值、不合法值、密碼檔讀取
失敗、大小界限與 direct/file 衝突；驗證錯誤名稱及合成 canary 不外洩。
密碼沿用 loadSecret 的既有定義，不另加 trimming 或新的字元限制。

AC-DDAE-11-003：涵蓋三種 Kafka 輸出的所有開關組合；均關閉時另以
query metrics-only 與 resources-only 驗證不讀取 Kafka 密碼檔。
另測 query 關閉但 events 原始設定為 true 的既有隔離行為，以及 events
啟用但 SASL 為空時不要求帳密。既有格式、衝突與 TLS 檢查繼續生效。

### Interfaces, Data, and Failure Behavior

Existing KAFKA_SASL_MECHANISM, KAFKA_SASL_USERNAME, KAFKA_SASL_PASSWORD /
KAFKA_SASL_PASSWORD_FILE and corresponding YAML settings retain their meaning.
The only implementation change is including cfg.Query.Events in the existing
SASL credential loading predicate. Existing requiredText, loadSecret and
layeredLookup define validation, newline handling, precedence and conflicts.

### Quality Attributes

Reuse the existing secret boundary, preserve redacted errors and TLS/mTLS
behavior, and avoid reading unused Kafka credential files.
Unit tests do not certify broker authentication or real integration.

### Compatibility and Migration

Existing valid query-only SASL configuration works after rebuilding/replacing
the executable and restarting. Missing/invalid credentials fail earlier at
configuration loading. No state migration or new setting is required.
Rollback preserves state but restores this defect. Do not delete outboxes or
disable TLS/SASL as a workaround. Independent review and full gates remain required.

### Assumptions

- The user approved the exact linked candidate by replying "Ok" on 2026-09-11.
- Candidate SHA256: 7f213471d37ef90860b890f7ea76cb0dcd313554aab427122ffe2330cde884f8.
- Canonical headings/tables transcribe that candidate; they do not add behavior.
- DDAE-11 is a new change following the published RC4 baseline.

### Open Questions

- None.
## DDAE-12 Audit bug fixes and unused-code cleanup

### Specification Metadata

- Specification Version: 2.8.0
- Status: active
- Owner: Current repository user in this Codex conversation
- Clarification Status: resolved

### Problem Statement

- B1：`internal/outbox/store.go` 以 hash 判斷是否清除 pending
- B2：`internal/alerts/pipeline.go`、`internal/serviceability/pipeline.go` 依最後成功時間排序
- B3：`internal/snapshot/store.go` 只用整輪完成時間判斷健康
- B4：`internal/ddae/types.go` 省略物件格式的 represented null pressure
- B5 / S1：`internal/querystate/store.go` 缺少既有檔案與持久化內容完整性檢查
- B6：`internal/queries/pipeline.go`、`internal/historyscan/adapters.go`、`internal/queryclient/types.go` 共用請求前時間做未來時間檢查
- B7：`internal/outbox/store.go` 未將 checkpoint 滿額納入 full
- B8：`internal/config/config.go`、`internal/config/query.go` Kafka topic 名稱驗證不完整

### Goals

Fix the eight confirmed audit bugs, protect retained data and remove the six identified unused-code groups with regression coverage.

### Non-goals

此次範圍包括 B5 同根因的 S1：拒絕被污染的 query outbox payload。
S2「security-policy.sh 的兩個 TLS target 規則過時」是另一項安全流程變更，
本提案不修改該規則，也不關閉或弱化安全 gate。完整驗證仍須如實列出此阻擋。
其他不確定問題，例如重複 legacy node condition 的衝突值處理，另行釐清，
不混入已確認 Bug 的修復。

不新增 API、設定名稱、metric、label、Kafka schema、topic、認證機制或相依套件。
保留既有 read-only DDAE 存取、API/版本邊界、secret/TLS 規則、HTTP routes、
逾時／併行／回應大小上限、Kafka at-least-once delivery、shutdown 與部署契約。
不清空 state、不自動修復損毀 DB、不做 migration；不包含 commit、push、tag、
release、部署或重新建立 Docker 測試環境。


### Actors and Scenarios

Operators collecting resources and publishing alert/log/query events encounter repeated states,
failed detail requests, staggered or slow responses, full checkpoints and invalid topic names.
Valid existing state must reopen; provably corrupt or polluted state fails closed without reset.

### Requirement Index

| Requirement | Priority | Requirement text | Acceptance |
|---|---|---|---|
| REQ-DDAE-12-001 | must | 同一 alert 的 A → B → A 三筆同時待送時，MUST 可逐筆 acknowledge 並於任一階段重啟；只有確認最新 pending sequence 才清除其 pending 標記，保留其他事件與順序。 | AC-DDAE-12-001 |
| REQ-DDAE-12-002 | must | 持續失敗的 detail MUST 不阻止同類別其他持續合格 ID 輪到執行；兩條 pipeline 均保留既有類別配額、總量與併行限制，不把失敗寫成成功 checkpoint。 | AC-DDAE-12-002 |
| REQ-DDAE-12-003 | must | resources 所需任一 family 缺少、過期或其必要採集失敗時，MUST 正確反映整體 readiness 與 `ddae_up`；保留單項原始採集時間與其他 pipeline 的隔離規則。 | AC-DDAE-12-003 |
| REQ-DDAE-12-004 | must | 已出現但為 null 的 pressure 值 MUST 與 legacy array 的無效值一致，令該 collector 不完整；真正省略的 optional 欄位保持原契約，其他可用欄位仍可呈現。 | AC-DDAE-12-004 |
| REQ-DDAE-12-005 | must | 既有零長度、格式損毀、schema/source 不符或可核對的不一致 MUST 明確失敗並保留資料，不初始化成新 DB；污染事件 MUST 在 replay 前被拒絕，合法 retention、重啟與累計值保持相容。 | AC-DDAE-12-005 |
| REQ-DDAE-12-006 | must | 即時採集與 backfill MUST 以回應取得時的時間驗證未來時間上限；請求開始的 observation ordering MUST 保留，慢但未逾時的合法回應可接收，舊觀察不能覆蓋新狀態。 | AC-DDAE-12-006 |
| REQ-DDAE-12-007 | must | checkpoint 數量恰好等於上限時 MUST 回報容量已滿，並反映既有 pipeline health/readiness；只按既有規則清理可過期、非 pending 的 checkpoint。 | AC-DDAE-12-007 |
| REQ-DDAE-12-008 | must | alerts、logs、queries 的 topic MUST 共用合法名稱檢查：1–249 個 ASCII 英數字、`.`、`_`、`-`，且排除 `.` 與 `..`；保留各模式既有必填／關閉／topic 隔離語意與設定優先順序。 | AC-DDAE-12-008 |
| REQ-DDAE-12-009 | must | MUST 移除已確認無效的部分，保留真正使用中的 refresh 時間、handler closure、測試／契約 helper 與 build/provenance 介面，不藉清理新增 runtime 行為。 | AC-DDAE-12-009 |
| REQ-DDAE-12-010 | must | MUST 補足前述回歸測試、保留有效歷史測試及 DDAE-11 修正，更新受影響的雙語操作文件，分別記錄實際通過、失敗、blocked 與未執行的驗證，不將合成測試稱為真實外部整合。 | AC-DDAE-12-010 |

### Acceptance Criteria

| Acceptance | Requirement | Criterion |
|---|---|---|
| AC-DDAE-12-001 | REQ-DDAE-12-001 | 同一 alert 的 A → B → A 三筆同時待送時，MUST 可逐筆 acknowledge 並於任一階段重啟；只有確認最新 pending sequence 才清除其 pending 標記，保留其他事件與順序。 |
| AC-DDAE-12-002 | REQ-DDAE-12-002 | 持續失敗的 detail MUST 不阻止同類別其他持續合格 ID 輪到執行；兩條 pipeline 均保留既有類別配額、總量與併行限制，不把失敗寫成成功 checkpoint。 |
| AC-DDAE-12-003 | REQ-DDAE-12-003 | resources 所需任一 family 缺少、過期或其必要採集失敗時，MUST 正確反映整體 readiness 與 `ddae_up`；保留單項原始採集時間與其他 pipeline 的隔離規則。 |
| AC-DDAE-12-004 | REQ-DDAE-12-004 | 已出現但為 null 的 pressure 值 MUST 與 legacy array 的無效值一致，令該 collector 不完整；真正省略的 optional 欄位保持原契約，其他可用欄位仍可呈現。 |
| AC-DDAE-12-005 | REQ-DDAE-12-005 | 既有零長度、格式損毀、schema/source 不符或可核對的不一致 MUST 明確失敗並保留資料，不初始化成新 DB；污染事件 MUST 在 replay 前被拒絕，合法 retention、重啟與累計值保持相容。 |
| AC-DDAE-12-006 | REQ-DDAE-12-006 | 即時採集與 backfill MUST 以回應取得時的時間驗證未來時間上限；請求開始的 observation ordering MUST 保留，慢但未逾時的合法回應可接收，舊觀察不能覆蓋新狀態。 |
| AC-DDAE-12-007 | REQ-DDAE-12-007 | checkpoint 數量恰好等於上限時 MUST 回報容量已滿，並反映既有 pipeline health/readiness；只按既有規則清理可過期、非 pending 的 checkpoint。 |
| AC-DDAE-12-008 | REQ-DDAE-12-008 | alerts、logs、queries 的 topic MUST 共用合法名稱檢查：1–249 個 ASCII 英數字、`.`、`_`、`-`，且排除 `.` 與 `..`；保留各模式既有必填／關閉／topic 隔離語意與設定優先順序。 |
| AC-DDAE-12-009 | REQ-DDAE-12-009 | MUST 移除已確認無效的部分，保留真正使用中的 refresh 時間、handler closure、測試／契約 helper 與 build/provenance 介面，不藉清理新增 runtime 行為。 |
| AC-DDAE-12-010 | REQ-DDAE-12-010 | MUST 補足前述回歸測試、保留有效歷史測試及 DDAE-11 修正，更新受影響的雙語操作文件，分別記錄實際通過、失敗、blocked 與未執行的驗證，不將合成測試稱為真實外部整合。 |

The approved failure/boundary matrix is transcribed into TEST_PLAN.md and plans/DDAE-12.md.

### Interfaces, Data, and Failure Behavior

**B1：acknowledge 依 sequence 清除 pending。**

沿用 `internal/logstate/store.go` 已有模式，在同一個 bbolt transaction
掃描時保留最新 pending hash 與 sequence。hash 一致性檢查仍存在；
以被確認的 sequence 是否為最新 pending 決定清除，而非只比較內容 hash。
保留已不存在 record 的 acknowledge 冪等性、per-alert 狀態及 transaction 原子性。
不更動資料格式；已損毀的 DB 繼續明確報錯，不自行刪掉阻塞事件。
新規格只修訂 DDAE-3 對 pending 清除條件的 hash-only 描述，歷史 basis 不改字。

**B2：為 detail 採集加入有界輪轉次序。**

兩條 pipeline 各自維護記憶體中的 eligible-ID 排程次序。首次建立次序時
採既有最後成功時間與 ID 決定順序；仍在等待的 ID 保持次序，新加入的 ID
排在已等待者之後。每次被選入本輪配額，即消耗一個排程輪次並移到隊尾，
不論該次請求最後成功、失敗或取消；採集與成功 checkpoint 則照實際結果處理。
每輪移除已不合格／已不在清單的排程記錄，記憶體量受目前 bounded list 限制。
重啟後從有效 checkpoint 重新建立排序，不新增持久化格式。

保留 new/changed 與 refresh 兩類配額：quota=1 時兩類交替，較大 quota
維持 refresh 至少 1、通常為 floor(limit/4)，並保留不足時的配額借用。
同類別內不再讓最後「成功」時間永久決定失敗者優先權；持續合格者前方
只有有限等待項目，新加入者不能不斷插隊。此為 DDAE-3／DDAE-4 排序說明的
明確修訂，保留其不得無限期飢餓與所有資源上限要求。

**B3、B4、B7：健康狀態與資料狀態一致。**

- B3：集中使用一致的 freshness 判斷，確認 Ping、Clusters、Nodes、Lock、
  Power 必要 snapshot 的 Present 與各自 CollectedAt；同時保留整輪與
  collector success 條件。沿用既有 stale 邊界，不把早取得資料的時間改晚。
- B4：區分 object pressure key 不存在與值為 null；後者保留為可辨識的
  無效 condition，走既有 normalization／partial failure 路徑。與 legacy
  array 做對照，不丟棄其餘合法 node 欄位，也不擴充新的 pressure metric。
- B7：checkpoint 計數納入 alert `Stats.Full`／Health 的 `>=` 邊界，
  reconciliation 採既有安全 pruning 後再次判斷 full；參照 logstate 行為，
  保留 pending checkpoint，不以淘汰事件換取健康狀態。

**B5 / S1：在可證明的資料邊界拒絕損毀與污染。**

1. 開啟前區分新檔與既有檔。只有確定不存在的新檔可初始化 schema 1；
   既有零長度／非 regular／symlink／無法確認狀態的路徑明確失敗。
   既有檔缺失 meta、checkpoints、events 或版本／source 不合時，不建立替代 bucket。
2. 啟動時先驗證既有 logical state，再執行既有 retention pruning 或啟動
   publisher。檢查 bucket、sequence key／bucket sequence、checkpoint key/value、
   aggregate 結構，以及 outbox 的 pending count／payload byte sum 是否一致。
   檢查不新增每次 Prometheus scrape 全表掃描；持久化讀寫入口共用必要驗證。
3. 對持久化事件使用明確欄位 allowlist 與型別：拒絕未知／重複欄位、trailing
   JSON、無效 UTF-8、缺少必要值、source mismatch、無效 ID／state／timestamp
   結構、超過 64 KiB payload 及超出既有界限的 duration。沿用目前 Event 的
   欄位與正規化 state，保留 optional 欄位語意。輸入 Record 亦須符合這些
   可持久化條件，Records 在回傳可送往 Kafka 的 bytes 前再驗證該 record。
   不靠刪除未知欄位後繼續送出來掩蓋污染，錯誤不包含 payload 或 SQL canary。
4. 只使用資料模型確實保證的跨筆不變量。尚未早於「已儲存 retention Floor」
   的 pending 終態事件，必須有對應且一致的 final checkpoint；驗證發生在
   推進 Floor 之前，不能用本次 prune 掩蓋遺失。已早於 Floor 的 pending
   事件可能合法失去 checkpoint，仍保留並 replay，不因缺 checkpoint 判錯。
   非終態可以有多筆歷史變更，events 關閉期間也可更新 checkpoint；不把所有
   pending hash 強制等同目前 checkpoint。對非終態使用能確立的結構／來源檢查，
   不臆測不存在的交易歷史。
5. 累計 Completed／histograms 不能由已裁剪的 checkpoint 或待送事件重算。
   只驗證合法 state/key、histogram 維度、有限且非負的 sum、bucket/count
   一致性等可證明條件，保留真實累計值。不假裝能偵測所有一致性造假的離線
   修改，亦不以新增 metadata、schema migration 或重置計數器擴張本提案。
6. 合法 schema 1 DB 保持相容；負向測試在 reopen 失敗前後比較原始內容，
   確認沒有清空、刪除、重建或先 prune。保留 0700/0600 權限、單一 writer
   與有界讀取／既有容量限制。若實作發現必須改 schema 或修復既有資料才能
   達成要求，停止並提出修訂，不自行增加 migration。

持久化事件的 observed_at 仍是請求開始的排序時間；合法 completed_at 可以
晚於 observed_at。既有 DB 驗證不使用錯誤的「completed <= observed」條件，
也不以 reopen 當下的時鐘重新判定歷史事件的 freshness。

**B6：分開 observation ordering 與 response validation 時間。**

保留請求前的 observation time，用來建立 Event.Observed 及防止較舊請求覆蓋
新 checkpoint。取得 HTTP response 後再讀時間，用於 submission/completion
是否超前現在 5 秒的檢查。透過小型內部 helper 明確傳入兩個時間，保留現有
DecodeDetail 呼叫的相容入口；即時 query 與 backfill 均使用修正路徑。
不放寬 5 秒上限，不改變 HTTP deadline，也不把較舊請求因較晚完成而當作新觀察。

**B8：共用 Kafka topic validator。**

以一個小型 helper 檢查 Kafka 已有名稱語法，三種輸出共用；沿用各 caller
現有何時驗證、是否必填、topic 是否需互異的規則。合法 topic 無須修改。
非法名稱由啟動後重試失敗改為載入設定時明確失敗；錯誤只指出設定問題，
不帶 credential。規則依 Apache Kafka 4.3.1 的
[Topic validator](https://raw.githubusercontent.com/apache/kafka/4.3.1/clients/src/main/java/org/apache/kafka/common/internals/Topic.java)。

**C1–C6：清理範圍與保留項目。**

| ID | 修改 | 明確保留 |
|---|---|---|
| C1 | 移除 `internal/alerts/event.go` 的 `var _ = fmt.Sprintf` 及唯一對應 import | 既有 event 編碼、欄位與 redaction |
| C2 | 移除 `tokenRefreshResult.refreshAt` 欄位及其寫入 | 真正控制更新時機的 `tokenManager.refreshAt` |
| C3 | 移除 `Server.state`／`staleAfter` 欄位與重複初始化 | handler closure 使用的 state／staleAfter 參數與其他被讀取欄位 |
| C4 | 移除 `app.BuildInfo.Revision`／`BuildDate` 及 main → app 的無效傳遞 | `main.revision`／`main.buildDate` linker symbols、VERSION／REVISION／BUILD_DATE build inputs、ldflags、Docker／reproducible build／provenance 產物契約 |
| C5 | 移除 historyscan 測試的未用 `testSource.fail` | 既有測試與本次真正會觸發失敗的回歸測試 |
| C6 | 刪除 `ALLOW_INSECURE_TLS` 第二次相同載入／驗證區塊 | 第一次載入及所有三種 TLS target 的雙重 opt-in 檢查 |

C4 的 linker symbols 是刻意保留的打包介面，不新增假引用或 runtime
輸出來消除工具警告。若 unused 檢查列出它們，應逐項註明原因，而不是
關閉整體檢查。保留 `RequiredCollectors`、`config.Load`、`routeSet.operations`、
`ApprovedOperations`、`ApprovedOperationsForPrefixes` 等有測試／契約用途
的 helpers；不單憑 production entry 的 call graph 就刪除。


### Quality Attributes

Preserve bounded work, privacy, read-only DDAE access, honest freshness/capacity health,
Kafka at-least-once semantics, existing metrics/schema/labels and fail-closed state validation.
Synthetic tests are local evidence, not real DDAE integration or independent review.

### Compatibility and Migration

合法設定與 schema 1 state 原地相容，不新增 migration。非法 topic 在啟動即
被拒絕；過期資料與 checkpoint 滿額會更早且正確反映健康狀態；已損毀的
query state 會停止啟動並保留資料，操作者需檢查損毀／還原來源，不能刪 DB
當成正常安裝步驟。其他 metric identity、Kafka payload/key/header 與
at-least-once 語意維持不變，不保證消除 Kafka 已送出但 ack 前中斷的合法重送。

本次只操作測試建立的隔離資料，不接觸使用者 runtime state。未來 rollout
前停止 exporter，保留完整 state 備份後替換 binary；rollback 使用原 binary
與保留的有效 state，會重新帶回舊缺陷。已損毀 state 不因 rollback 自動恢復。
任何資料修復、清空、遷移、部署與發布需另取得具體授權。


### Assumptions

- User approval: "同意", 2026-09-11, to the exact linked DDAE-12 candidate.
- Candidate SHA256: 5d76c6a3034e1d57deed691d7c84206cb5b041efab316b619baeb21b84e63b2e.
- DDAE-12 explicitly amends hash-only pending clearing and success-only within-class scheduling descriptions in DDAE-3/4; historical approval bytes remain unchanged.
- No automatic database repair, schema migration, S2 rule change or release is authorized.

### Open Questions

- None.
