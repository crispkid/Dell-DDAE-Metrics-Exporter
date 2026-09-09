# Test Plan

## DDAE-1 Test Design

No test listed here has been executed for this planning draft. The common
pre-implementation state is `not executed: product implementation does not
exist and environment-dependent validation is deferred by Peter Chu on
2026-08-24`. A listed path is the planned executable evidence location; it may
remain absent only while the corresponding trace row is `Planned`.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-1-001 | AC-DDAE-1-001 | REQ-DDAE-1-001 | `path:scripts/build.sh` | build | clean pinned Go toolchain and empty module/build caches | not executed; no `go.mod` or product source exists | `build` |
| TEST-DDAE-1-002 | AC-DDAE-1-002 | REQ-DDAE-1-002 | `path:internal/server/server_test.go` | component | valid, invalid, ready, stale and degraded application states | not executed; HTTP server does not exist | `test` |
| TEST-DDAE-1-003 | AC-DDAE-1-003 | REQ-DDAE-1-003 | `path:internal/ddae/allowlist_test.go` | component | recording HTTP server and all compiled DDAE operations | not executed; DDAE client does not exist | `test` |
| TEST-DDAE-1-004 | AC-DDAE-1-004 | REQ-DDAE-1-004 | `path:internal/ddae/auth_test.go` | component | sanitized token server, runtime secret files and protected target | not executed; token client does not exist | `test` |
| TEST-DDAE-1-005 | AC-DDAE-1-005 | REQ-DDAE-1-004 | `path:internal/ddae/auth_failure_test.go` | component | expired/rejected tokens, concurrent 401s and redaction canaries | not executed; token renewal does not exist | `test` |
| TEST-DDAE-1-006 | AC-DDAE-1-006 | REQ-DDAE-1-005 | `path:internal/ddae/tls_test.go` | component | trusted, untrusted and hostname-mismatched TLS servers plus test CA | not executed; DDAE TLS client does not exist | `test` |
| TEST-DDAE-1-007 | AC-DDAE-1-007 | REQ-DDAE-1-006 | `path:internal/metrics/selected_areas_test.go` | component | sanitized cluster, node, lock and power/readiness fixtures | not executed; collectors and metrics do not exist | `test` |
| TEST-DDAE-1-008 | AC-DDAE-1-008 | REQ-DDAE-1-007 | `path:internal/collector/scope_test.go` | contract | production route registry, configs, fixtures, gathered metrics and events | not executed; collector registry does not exist | `test` |
| TEST-DDAE-1-009 | AC-DDAE-1-009 | REQ-DDAE-1-008 | `path:internal/metrics/contract_test.go` | contract | committed metric descriptors and expected exposition | not executed; metric descriptors do not exist | `test` |
| TEST-DDAE-1-010 | AC-DDAE-1-010 | REQ-DDAE-1-009 | `path:internal/ddae/typed_decode_test.go` | component | responses with approved, unknown and sensitive fields | not executed; typed decoders do not exist | `test` |
| TEST-DDAE-1-011 | AC-DDAE-1-011 | REQ-DDAE-1-009 | `path:internal/ddae/response_limit_test.go` | component | oversized, deeply nested, truncated and malformed JSON | not executed; bounded decoder does not exist | `test` |
| TEST-DDAE-1-012 | AC-DDAE-1-012 | REQ-DDAE-1-010 | `path:internal/observability/redaction_test.go` | security component | unique canaries for secrets, IPs, messages, IDs, timestamps and contacts | not executed; output boundaries do not exist | `test` |
| TEST-DDAE-1-013 | AC-DDAE-1-013 | REQ-DDAE-1-011 | `path:internal/collector/scheduler_test.go` | component | slow/failing servers, retryable classes and concurrent scrapes | not executed; scheduler does not exist | `test` |
| TEST-DDAE-1-014 | AC-DDAE-1-014 | REQ-DDAE-1-012 | `path:internal/snapshot/partial_failure_test.go` | component | one failed collector with other current successful collectors | not executed; snapshot store does not exist | `test` |
| TEST-DDAE-1-015 | AC-DDAE-1-015 | REQ-DDAE-1-012 | `path:internal/snapshot/staleness_test.go` | component | fake clock, prior success, repeated failure and stale threshold | not executed; freshness policy does not exist | `test` |
| TEST-DDAE-1-016 | AC-DDAE-1-016 | REQ-DDAE-1-013 | `path:internal/collector/compatibility_test.go` | component | added fields, missing optionals, unknown enums and malformed family data | not executed; typed collectors do not exist | `test` |
| TEST-DDAE-1-017 | AC-DDAE-1-017 | REQ-DDAE-1-014 | `path:internal/config/config_test.go` | unit | missing, conflicting, invalid and secret-file settings | not executed; configuration package does not exist | `test` |
| TEST-DDAE-1-018 | AC-DDAE-1-018 | REQ-DDAE-1-015 | `path:internal/observability/log_test.go` | component | success and every bounded DDAE/Kafka failure class with canaries | not executed; structured logging boundary does not exist | `test` |
| TEST-DDAE-1-019 | AC-DDAE-1-019 | REQ-DDAE-1-016 | `path:internal/app/shutdown_test.go` | component | in-flight collection/publish, fake signals and grace deadline | not executed; application lifecycle does not exist | `test` |
| TEST-DDAE-1-020 | AC-DDAE-1-020 | REQ-DDAE-1-017 | `path:internal/contract/regression_test.go` | regression | complete sanitized DDAE 1.5.0 fixture suite and Kafka golden events | not executed; fixtures and product contracts do not exist | `test` |
| TEST-DDAE-1-021 | AC-DDAE-1-021 | REQ-DDAE-1-018 | `path:integration/ddae_kafka_test.go` | integration | authorized non-production DDAE 1.5.0, isolated Kafka topic and sanitized evidence sink | blocked; authorized environments are not currently available | `integration` |
| TEST-DDAE-1-022 | AC-DDAE-1-022 | REQ-DDAE-1-019 | `path:scripts/reproducible-build.sh` | supply chain | two approved clean builders, one source revision and pinned OCI base | not executed; release build does not exist | `supply-chain` |
| TEST-DDAE-1-023 | AC-DDAE-1-023 | REQ-DDAE-1-020 | `path:integration/deployment_runbook_test.go` | end-to-end | new-operator run through Kubernetes and VM/systemd profiles | blocked; deployment environments are not currently available | `e2e` |
| TEST-DDAE-1-024 | AC-DDAE-1-024 | REQ-DDAE-1-021 | `path:internal/alerts/pipeline_test.go` | component | alert list/detail fixtures and recording Kafka producer | not executed; alert pipeline does not exist | `test` |
| TEST-DDAE-1-025 | AC-DDAE-1-025 | REQ-DDAE-1-022 | `path:internal/kafka/failure_test.go` | component | unavailable, slow, TLS-invalid, auth-rejecting producer and full outbox | not executed; Kafka producer/outbox does not exist | `test` |
| TEST-DDAE-1-026 | AC-DDAE-1-026 | REQ-DDAE-1-023 | `path:internal/alerts/event_schema_test.go` | contract | all documented, unknown, nested, free-form and sensitive alert fields | not executed; typed event schema does not exist | `test` |
| TEST-DDAE-1-027 | AC-DDAE-1-027 | REQ-DDAE-1-023 | `path:internal/alerts/lifecycle_test.go` | component | duplicate, updated, disappeared and restart/replay alerts | not executed; event lifecycle and durable state do not exist | `test` |
| TEST-DDAE-1-028 | AC-DDAE-1-028 | REQ-DDAE-1-024 | `path:internal/alerts/detail_path_test.go` | security component | valid, duplicate, oversized, slash/dot/escape injection and mismatched IDs | not executed; detail path construction does not exist | `test` |
| TEST-DDAE-1-029 | AC-DDAE-1-029 | REQ-DDAE-1-024 | `path:internal/alerts/refresh_checkpoint_test.go` | component | new, unchanged, updated, deferred, failing and post-restart alerts | not executed; refresh/checkpoint behavior does not exist | `test` |
| TEST-DDAE-1-030 | AC-DDAE-1-003 | REQ-DDAE-1-003 | `path:scripts/security-policy.sh` | security static analysis | production Go syntax/packages and compiled route manifest | not executed; security policy script and product code do not exist | `security` |
| TEST-DDAE-1-031 | AC-DDAE-1-012 | REQ-DDAE-1-010 | `path:scripts/security-policy.sh` | security scanning | repository, build artifacts, logs and generated evidence with seeded canaries | not executed; security policy script and product artifacts do not exist | `security` |
| TEST-DDAE-1-032 | AC-DDAE-1-022 | REQ-DDAE-1-019 | `path:scripts/supply-chain.sh` | supply chain | locked module graph, licenses, SBOM, checksums and provenance inputs | not executed; dependencies and release artifacts do not exist | `supply-chain` |

### Failure and Boundary Coverage

- Authentication: absent/conflicting secret sources, malformed token response,
  missing lifetime, concurrent renewal, first/second 401, invalid credentials and
  secret-bearing server errors.
- Network/TLS: DNS/transport failure, timeout and cancellation, untrusted CA,
  hostname mismatch, cross-origin redirect, proxy environment injection,
  non-HTTPS base URL and bounded retry exhaustion.
- DDAE authorization: only seven compiled GET operations, no mutation-capable
  request interface and authorized non-production proof that the monitoring
  identity can read every allowlisted path without mutation privilege.
- Decoding: empty/truncated/malformed/oversized/deep JSON, unknown fields and
  enums, missing optional and required identities, duplicate IDs, invalid
  quantities, overflow and invalid timestamps.
- Prometheus: descriptor consistency, duplicate/conflicting series, concurrent
  gathers, partial failure, stale withholding, bounded label values, omission
  rather than false zero and absence of alert content.
- Alert detail: malicious or duplicate IDs, one-segment escaping, returned-ID
  mismatch, incomplete list, request-cap ordering, non-starvation, aggregate
  deadline, per-response limit and no incomplete successful event.
- Kafka event: exact typed allowlist and schema/header/key, canonical hashing,
  nested-event depth, string/array/event size bounds and total exclusion of raw
  bodies, labels, links, credentials and unapproved fields.
- Outbox: transactional insert/checkpoint order, crash at every persistence and
  acknowledgement boundary, replay order, duplicate tolerance, count/byte hard
  limits, no eviction, checkpoint retention and filesystem corruption/startup
  failure.
- Runtime: overlapping-cycle prevention, bounded goroutines/memory, slow Kafka
  isolation, concurrent scrapes, readiness aggregation and shutdown at every
  in-flight boundary.
- Deployment: missing secret/CA/state mount, wrong permissions, read-only root,
  non-root identity, unavailable reverse proxy/service mesh, restart with state,
  rolling upgrade, rollback and outbox-full recovery for Kubernetes and VM.
- Evidence privacy: deterministic sanitized fixtures only; no production data,
  private endpoint, credentials, payload logs or broker/topic labels in retained
  reports.

Preserve useful regression tests when requirements change. Add or update tests
linked to amended acceptance IDs; remove a test only after the requirement or
compatibility promise it protects is formally removed or replaced. Every listed
stage must be scheduled by committed `HARNESS_VERIFY_STAGES` before approval.
Executable commands and numeric gates remain authoritative only in committed
`HARNESS/config.env`.

## DDAE-2 Test Design

DDAE-2 behavior is not implemented at this planning gate. Existing DDAE-1
tests are regression evidence only and do not prove YAML loading, conditional
pipelines or guarded insecure TLS. Environment-dependent E2E remains blocked;
no blocked or insecure run is acceptance evidence.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-2-001 | AC-DDAE-2-001 | REQ-DDAE-2-001 | `path:internal/config/yaml_test.go` | unit | valid version-1 YAML plus unknown, duplicate, multi-document, malformed, unsupported, unreadable and over-1-MiB files | unsupported; loader reads environment only | `test` |
| TEST-DDAE-2-002 | AC-DDAE-2-002 | REQ-DDAE-2-002 | `path:internal/config/precedence_test.go` | unit | distinct default, YAML, legacy env, dedicated env and config-path canaries | unsupported; no merge or config selector exists | `test` |
| TEST-DDAE-2-003 | AC-DDAE-2-003 | REQ-DDAE-2-003 | `path:internal/config/yaml_secret_test.go` | security unit | YAML file paths, attempted plaintext secret keys, direct/`_FILE` conflicts, size/UTF-8/NUL and redaction canaries | unsupported; YAML secret surface does not exist | `test` |
| TEST-DDAE-2-004 | AC-DDAE-2-004 | REQ-DDAE-2-004 | `path:internal/app/pipeline_selection_test.go` | component | all four enable combinations, recording DDAE/Kafka/state doubles and fake clocks with distinct intervals | unsupported; app always initializes both pipelines | `test` |
| TEST-DDAE-2-005 | AC-DDAE-2-005 | REQ-DDAE-2-005 | `path:internal/server/pipeline_readiness_test.go` | component | resource-only, alert-only and dual healthy/stale/failing/full-outbox snapshots | unsupported; readiness unconditionally requires both pipelines | `test` |
| TEST-DDAE-2-006 | AC-DDAE-2-005 | REQ-DDAE-2-005 | `path:internal/metrics/pipeline_contract_test.go` | contract | both enable booleans and gathered descriptor/series sets for every mode | unsupported; enable metric and conditional families do not exist | `test` |
| TEST-DDAE-2-007 | AC-DDAE-2-006 | REQ-DDAE-2-006 | `path:internal/contract/tls_policy_test.go` | security component | trusted, untrusted and hostname-mismatched DDAE/Kafka TLS servers, custom CAs and incomplete opt-ins | default verification exists; global/target policy does not | `test` |
| TEST-DDAE-2-008 | AC-DDAE-2-007 | REQ-DDAE-2-006 | `path:internal/contract/insecure_tls_test.go` | security component | isolated TLS 1.1/1.2 servers, DDAE and Kafka target flags, auth doubles and log canaries | unsupported; verification cannot currently be disabled | `test` |
| TEST-DDAE-2-009 | AC-DDAE-2-008 | REQ-DDAE-2-007 | `path:internal/contract/deployment_test.go` | static contract | YAML example, systemd unit, Kubernetes ConfigMap/volume/args, Secret references and conditional state contract | unsupported; deployment profiles inject non-secret settings through environment | `test` |
| TEST-DDAE-2-010 | AC-DDAE-2-008 | REQ-DDAE-2-007 | `path:integration/deployment_runbook_test.go` | end-to-end | authorized container, Kubernetes and VM/systemd profiles in resource-only, alert-only and dual modes | blocked; deployment environments are unavailable | `e2e` |
| TEST-DDAE-2-011 | AC-DDAE-2-009 | REQ-DDAE-2-008 | `path:scripts/security-policy.sh` | security regression | source/dependency scan, forbidden plaintext YAML keys, TLS downgrade patterns, warning/output canaries and DDAE-1 env fixtures | unsupported; policy does not cover DDAE-2 | `security` |

### Failure and Boundary Coverage

- YAML: empty and missing explicit paths, directories, unreadable files,
  symlink-backed Kubernetes projections, zero/exact/over-limit files, invalid
  UTF-8, comments, anchors, aliases, duplicate keys, merge keys, multiple
  documents, unknown fields, unsupported versions and invalid scalar types.
- Precedence: no file, environment-selected file, CLI-selected file, YAML-only,
  env-only and combined inputs; legacy interval fallback and dedicated interval
  overrides; false booleans must not be confused with omission.
- Secrets: approved YAML file paths, forbidden plaintext names, direct env,
  `_FILE` env, YAML-to-env override, explicit env conflict, unreadable content,
  64 KiB limit, NUL/UTF-8 and redaction canaries.
- Pipelines: resource-only, alert-only, dual and both-disabled; independent fake
  clocks, no same-pipeline overlap, no calls/clients/state for disabled work,
  conditional source/Kafka/state requirements and shutdown cleanup after
  partial construction failure.
- Readiness/metrics: every enabled pipeline healthy/failing/stale/full
  combination, fixed two-series enable metric, always-present build info,
  disabled family omission and no disabled-success signal.
- TLS: verified default, custom CA, hostname mismatch, global-only,
  target-only, DDAE-only, Kafka-only and dual insecure flags, CA/insecure
  conflict, TLS 1.1 rejection, TLS 1.2 success, auth retention, no fallback and
  fixed warning redaction.
- Deployment: actual YAML selection in every profile, separate Secret mounts,
  conditional Kafka/state setup, one alert-state writer, upgrade/rollback and
  preservation of a disabled alert pipeline's database.

### Stage and Evidence Policy

- `test` proves deterministic parser, merge, app, readiness, metric, TLS-client
  and static deployment behavior with local fixtures.
- `security` proves forbidden secret/TLS downgrade patterns, warning redaction
  and dependency policy. It does not make insecure transport acceptable.
- `e2e` proves operator-facing profile behavior only in authorized environments
  with verified TLS for release evidence.
- DDAE-1 `integration`, `coverage`, `build`, `supply-chain` and `ci-policy`
  remain required by the full repository handoff even when no DDAE-2 test row
  duplicates them.

Any insecure-TLS run is explicitly diagnostic and cannot close AC-DDAE-2-006,
AC-DDAE-2-008, a release integration gate or production-support evidence.

## DDAE-3 Test Design

DDAE-3 begins from the reproduced pre-implementation failures recorded by the
2026-08-25 whole-repository review. Tests use local deterministic fixtures and
controlled clocks/connections; they do not claim authorized DDAE, Kafka,
OpenSearch, deployment or GitHub environment evidence.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-3-001 | AC-DDAE-3-001 | REQ-DDAE-3-001 | `path:scripts/build.sh` | build contract | `go.mod`, Dockerfile and clean local build metadata | failing: executable sources pin Go 1.26.5 rather than 1.26.6 | `build` |
| TEST-DDAE-3-002 | AC-DDAE-3-001 | REQ-DDAE-3-001 | `path:scripts/security-policy.sh` | security scan | pinned `govulncheck` v1.7.0 and current vulnerability database | failing: five source-reachable standard-library findings are fixed by Go 1.26.6 | `security` |
| TEST-DDAE-3-003 | AC-DDAE-3-002 | REQ-DDAE-3-002 | `path:internal/outbox/latest_state_test.go` | component/regression | A delivered, B pending, A returned, duplicates, acknowledgement and restart crash points | failing: final A is suppressed when DeliveredHash is A while PendingHash is B | `test` |
| TEST-DDAE-3-004 | AC-DDAE-3-003 | REQ-DDAE-3-003 | `path:internal/kafka/hard_timeout_test.go` | component | local protocol-aware blackhole after in-flight produce, one-second timeout, goroutine counters | failing: default idempotent franz-go may refuse context cancellation after an uncertain in-flight request | `test` |
| TEST-DDAE-3-005 | AC-DDAE-3-004 | REQ-DDAE-3-003 | `path:internal/app/shutdown_test.go` | component | in-flight blackhole publisher, fake signal/context, retained outbox record and bounded producer closer | failing: shutdown waits for a publisher whose in-flight idempotent request may outlive the grace budget | `test` |
| TEST-DDAE-3-006 | AC-DDAE-3-005 | REQ-DDAE-3-004 | `path:scripts/stage-ci-policy.sh` | CI security policy | push/PR/dispatch, main/branch/tag ref matrix and checkout/environment assertions | failing: self-hosted job has no `github.ref == 'refs/heads/main'` guard and checkout persists credentials by default | `ci-policy` |
| TEST-DDAE-3-007 | AC-DDAE-3-006 | REQ-DDAE-3-005 | `path:internal/outbox/integrity_test.go` | state component | new, valid legacy/current/future, malformed derived counters and corrupt primary/cross-record databases | failing: state has no schema marker/full scan and malformed counters decode as zero | `test` |
| TEST-DDAE-3-008 | AC-DDAE-3-007 | REQ-DDAE-3-006 | `path:internal/outbox/checkpoint_retention_test.go` | state component | pending/non-pending expired checkpoints, capacity pressure, missing/mismatched acknowledgement checkpoint | failing: capacity reconciliation may evict pending checkpoints and acknowledgement may create an empty-ID checkpoint | `test` |
| TEST-DDAE-3-009 | AC-DDAE-3-008 | REQ-DDAE-3-007 | `path:internal/server/state_readiness_test.go` | component | per-operation pipeline/publisher state failures, recovery sequences and concurrent `/readyz` | failing: publisher state errors only update Kafka diagnostics and a later pipeline poll can report alert readiness true | `test` |
| TEST-DDAE-3-010 | AC-DDAE-3-009 | REQ-DDAE-3-008 | `path:internal/ddae/token_coalescing_test.go` | concurrency component | 32 waiters, failed shared renewal, concurrent/delayed 401 generations, transient and credential statuses | failing: failed refresh waiters can serialize new token requests and delayed old-token 401 can refresh an already newer generation | `test` |
| TEST-DDAE-3-011 | AC-DDAE-3-010 | REQ-DDAE-3-009 | `path:internal/alerts/starvation_test.go` | deterministic scheduler component | continuous new/changed and refresh-due backlogs, limits one/two/four, restart and controlled last-fetch times | failing: strict priority ordering can indefinitely defer refresh-due alerts | `test` |
| TEST-DDAE-3-012 | AC-DDAE-3-011 | REQ-DDAE-3-010 | `path:internal/ddae/response_limit_test.go` | boundary component | environment/YAML values 1, defaults, 64 MiB, over-limit and MaxInt64 plus excessive headers/body/trailing JSON | failing: positive body limits have no maximum, `limit+1` may overflow and response headers use Go's larger default | `test` |
| TEST-DDAE-3-013 | AC-DDAE-3-012 | REQ-DDAE-3-010 | `path:internal/server/http_bounds_test.go` | HTTP component | six-plus blocked metric gathers, nine-second fake timeout and concurrent health/readiness probes | failing: `/metrics` has no in-flight or handler timeout bound | `test` |
| TEST-DDAE-3-014 | AC-DDAE-3-011 | REQ-DDAE-3-010 | `path:scripts/security-policy.sh` | security static analysis | effective config validators, DDAE transport and Prometheus HandlerOpts source contracts | failing: policy has no assertion for the new body/header/scrape ceilings | `security` |

### Failure and Boundary Coverage

- Toolchain: local module directive, OCI builder tag/digest, clean cache,
  vulnerability database age, source-reachable standard-library findings and
  absence of an unreviewed dependency change.
- Outbox convergence: delivered-only duplicate, exact pending duplicate,
  `A -> B -> A`, `A -> B -> C`, multiple alerts, acknowledgement order, crash
  before/after each transaction and reopen.
- Kafka: no route, refused connection, TLS/auth rejection, broker that accepts
  TCP only, broker that reads a complete produce request and never replies,
  parent cancellation, deadline, retry and producer close. Timing assertions
  include bounded tolerance and goroutine cleanup.
- CI: every supported trigger, boolean opt-in, main/branch/tag/pull-request ref,
  dispatched SHA, checkout credential persistence, environment name and
  self-hosted label. External environment controls remain separately attested.
- State: empty/new, valid legacy/current, future marker, missing buckets,
  malformed keys/JSON/hashes/timestamps, record/payload/key/checkpoint mismatch,
  missing/malformed/mismatched derived counters, multiple queued transitions,
  full limits and transaction rollback.
- Readiness: each state API failure from pipeline and publisher, simultaneous
  failures, one-side recovery, full outbox, broker-only failure, idle outbox,
  concurrent probes and bounded failure-class/redaction canaries.
- Token: initial and forced singleflight success/failure, waiter cancellation,
  old/new generations, concurrent 401s, delayed 401, timeout/transport/429/5xx,
  400/401/403 and secret/token canaries.
- Fairness: deterministic ordering inside each class, quota borrowing, limit
  one alternation, persistent mixed backlog, changing markers, failures,
  incomplete lists, deadlines and no work beyond max/concurrency.
- HTTP: exact configured body boundaries through both YAML/environment,
  overflow-safe read, large/trailing whitespace/JSON, excessive response
  headers, five/excess/expired metric requests, health/readiness isolation,
  cancellation and goroutine return.

Preserve all useful DDAE-1/DDAE-2 regressions. Every DDAE-3 test remains mapped
to an existing scheduled Harness stage; local component success cannot close
inherited external integration, E2E, supply-chain or independent-review gates.

## DDAE-4 Test Design

DDAE-4 local tests use only deterministic in-process fixtures, temporary files
and controlled clocks. The listed pre-implementation state is not an acceptance
result: DDAE-4 product code is not yet authorized and the planned packages do
not yet exist. Environment-dependent rows remain explicit so local mocks cannot
be mistaken for real compatibility evidence.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-4-001 | AC-DDAE-4-001 | REQ-DDAE-4-001 | `path:internal/serviceability/allowlist_test.go` | endpoint-policy component | recording TLS server, every compiled operation, safe/unsafe IDs and redirect cases | not executed; Serviceability routes are not implemented | `test` |
| TEST-DDAE-4-002 | AC-DDAE-4-002 | REQ-DDAE-4-002 | `path:internal/app/serviceability_selection_test.go` | configuration/application component | old YAML plus every resource/alert/log enable combination and filesystem/request spies | not executed; the third pipeline is not implemented | `test` |
| TEST-DDAE-4-003 | AC-DDAE-4-003 | REQ-DDAE-4-003 | `path:internal/serviceability/list_test.go` | bounded decode component | complete, incomplete, duplicate, malformed, excessive-total and unsafe-ID list fixtures | not executed; typed Serviceability list decoding is not implemented | `test` |
| TEST-DDAE-4-004 | AC-DDAE-4-004 | REQ-DDAE-4-003 | `path:internal/serviceability/fairness_test.go` | deterministic scheduler component | controlled clock, mixed persistent backlogs, limits one/two/four, failures and restart | not executed; log detail scheduling is not implemented | `test` |
| TEST-DDAE-4-005 | AC-DDAE-4-005 | REQ-DDAE-4-004 | `path:internal/serviceability/event_schema_test.go` | schema component | minimum/complete/unknown/invalid/oversized detail fixtures including labels and links | not executed; the typed log event does not exist | `test` |
| TEST-DDAE-4-006 | AC-DDAE-4-006 | REQ-DDAE-4-004 | `path:internal/serviceability/event_canonical_test.go` | canonicalization component | semantic equivalents, source-order noise, one-field changes and 256 KiB boundaries | not executed; canonical log hashing does not exist | `test` |
| TEST-DDAE-4-007 | AC-DDAE-4-007 | REQ-DDAE-4-005 | `path:internal/app/serviceability_isolation_test.go` | cross-pipeline component | colliding IDs, recording alert/log producers, distinct topics/keys/stores and capacity pressure | not executed; log topic/state construction does not exist | `test` |
| TEST-DDAE-4-008 | AC-DDAE-4-008 | REQ-DDAE-4-005 | `path:internal/logstate/replay_test.go` | durable delivery component | broker rejection/timeout, crash points, restart, later acknowledgement and ordered records | not executed; dedicated log durable delivery does not exist | `test` |
| TEST-DDAE-4-009 | AC-DDAE-4-009 | REQ-DDAE-4-006 | `path:internal/logstate/lifecycle_test.go` | state/lifecycle component | `A -> A -> B -> A`, complete/incomplete disappearance, retention and pending records | not executed; log checkpoints and lifecycle do not exist | `test` |
| TEST-DDAE-4-010 | AC-DDAE-4-010 | REQ-DDAE-4-007 | `path:internal/serviceability/response_limit_test.go` | boundary component | byte limits 1/default/64 MiB/over-limit/overflow-adjacent, trailing JSON and maximum fields | not executed; log body/event bound handling does not exist | `test` |
| TEST-DDAE-4-011 | AC-DDAE-4-011 | REQ-DDAE-4-007 | `path:internal/logstate/integrity_test.go` | state integrity component | new/current/future/corrupt databases, derived-counter repair, full limits and pending records | not executed; dedicated log store does not exist | `test` |
| TEST-DDAE-4-012 | AC-DDAE-4-012 | REQ-DDAE-4-008 | `path:internal/serviceability/redaction_test.go` | observability/security component | unique canaries in every content, endpoint, topic and credential position plus gathered metrics/logs/errors | not executed; log diagnostics and metrics do not exist | `test` |
| TEST-DDAE-4-013 | AC-DDAE-4-013 | REQ-DDAE-4-009 | `path:internal/contract/serviceability_compatibility_test.go` | compatibility contract | pre-DDAE-4 YAML/state, disabled defaults, alert golden event/key/topic and metric descriptors | not executed; compatibility amendment is not implemented | `test` |
| TEST-DDAE-4-014 | AC-DDAE-4-013 | REQ-DDAE-4-009 | `path:scripts/stage-e2e.sh` | deployment E2E | valid pre-DDAE-4 deployment/state, new binary with no log settings and rollback profile | unavailable; no authorized deployment environment is recorded | `e2e` |
| TEST-DDAE-4-015 | AC-DDAE-4-014 | REQ-DDAE-4-010 | `path:scripts/stage-integration.sh` | DDAE integration | authorized non-production DDAE, read-only identity and sanitized list/detail evidence | unavailable; no authorized DDAE environment is recorded | `integration` |
| TEST-DDAE-4-016 | AC-DDAE-4-014 | REQ-DDAE-4-010 | `path:scripts/stage-e2e.sh` | Kafka/OpenSearch E2E | isolated topic/ACL/index, sanitized records, outage/replay and stable-key upsert | unavailable; no authorized Kafka/OpenSearch environment is recorded | `e2e` |

### Failure and Boundary Coverage

- Endpoint policy: literal list route, safely encoded single detail segment,
  origin-preserving redirects, every mutation and adjacent Support route.
- Activation: old YAML, strict unknown fields, all enable combinations,
  conditional Kafka/topic/state requirements and proof of no disabled worker,
  call, state file or metric family beyond the fixed enable series.
- Retrieval: missing/negative/non-integer/excessive totals, duplicate or unsafe
  IDs, returned-ID mismatch, incomplete lists, deadlines, concurrency and
  deterministic two-class fairness including limit-one alternation.
- Schema: every allowed optional field, invalid boolean/count/time/UTF-8/NUL,
  exact byte boundaries, remedies limits, ignored unknown/labels/links, stable
  canonical hash/key/headers and stored-event revalidation.
- Delivery/state: topic and database isolation, at-least-once uncertainty,
  acknowledgement ordering, restart, multiple transitions, full capacity,
  complete/incomplete disappearance, retention, corruption, future schema and
  derived-only repair.
- Observability/readiness: each collection/state/publisher failure owner,
  one-side recovery, broker-only failure, full state, incomplete lists,
  concurrent probes, fixed metric descriptors and canary scans.
- Compatibility: existing resource/alert configuration, routes, metrics,
  golden payload/key/topic/state, deployment manifests and disabled-default
  rollback behavior.
- External release: actual least-privilege GET authorization, list shape and
  completeness, dedicated Kafka ACL/topic/replay and OpenSearch mapping/upsert.
  These cases stay unexecuted until explicitly authorized environments exist.

TEST-DDAE-4-001 through TEST-DDAE-4-013 are the deterministic local evidence
planned for the one-pass implementation. TEST-DDAE-4-014 through
TEST-DDAE-4-016 remain environment-dependent release evidence and cannot be
closed by local test doubles.

## DDAE-5 Test Design

DDAE-5 begins with all rows unexecuted. Local recording servers can prove exact
route construction, configuration, compatibility and failure behavior, but
they cannot prove that a deployed DDAE accepts credentials or returns the
existing typed schemas. The authenticated integration row remains separate.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-5-001 | AC-DDAE-5-001 | REQ-DDAE-5-001 | `path:internal/ddae/path_prefix_test.go` | route/token component | recording TLS server, omitted prefix settings and every compiled operation | current code sends every GET under `/rest/v1`; the new default matrix is unsupported | `test` |
| TEST-DDAE-5-002 | AC-DDAE-5-002 | REQ-DDAE-5-002 | `path:internal/ddae/path_prefix_test.go` | route compatibility component | default, RC2-compatible and PDF-compatible prefix pairs | no independent prefixes exist | `test` |
| TEST-DDAE-5-003 | AC-DDAE-5-003 | REQ-DDAE-5-002 | `path:internal/config/path_prefix_test.go` | strict configuration unit | omitted, empty and non-empty YAML values, environment overrides and unknown fields | YAML/environment prefix fields are unknown or unused | `test` |
| TEST-DDAE-5-004 | AC-DDAE-5-004 | REQ-DDAE-5-003 | `path:internal/config/path_prefix_test.go` | validation and path-security component | empty, 128-byte, segment-boundary and every invalid grammar class plus safe/unsafe detail IDs | bounded prefix validation and composition do not exist | `test` |
| TEST-DDAE-5-005 | AC-DDAE-5-004 | REQ-DDAE-5-003 | `path:scripts/security-policy.sh` | static security policy | operation registry, client route construction, token path, redirect and fallback source assertions | policy requires full `/rest/v1` literals and does not check prefix families | `security` |
| TEST-DDAE-5-006 | AC-DDAE-5-005 | REQ-DDAE-5-004 | `path:internal/contract/path_prefix_compatibility_test.go` | regression contract | RC2 prefix pair, golden responses/events/metrics/state and all pipeline combinations | current RC2 routes work but cannot be selected through the new settings | `test` |
| TEST-DDAE-5-007 | AC-DDAE-5-006 | REQ-DDAE-5-005 | `path:internal/ddae/path_prefix_error_test.go` | failure/redaction component | prefix/origin/body/credential canaries plus 401, 404, timeout and malformed response fixtures | existing errors are bounded, but configurable-prefix leakage/no-fallback is untested | `test` |
| TEST-DDAE-5-008 | AC-DDAE-5-007 | REQ-DDAE-5-006 | `path:internal/contract/deployment_test.go` | deployment/documentation contract | root/systemd/Kubernetes YAML, README and runbook route/configuration assertions | committed surfaces document only `/rest/v1` routes and no prefix settings | `test` |
| TEST-DDAE-5-009 | AC-DDAE-5-008 | REQ-DDAE-5-007 | `path:integration/ddae_kafka_test.go` | authenticated DDAE integration | authorized non-production endpoint, least-privilege credentials, selected prefixes and sanitized typed responses | unavailable; supplied observations are unauthenticated and retain no response-contract evidence | `integration` |

### Failure and Boundary Coverage

- Defaults and composition: omitted values, explicitly empty values, every
  operation, fixed token POST, fixed GET suffixes and exact no-double-slash
  composition.
- Precedence: environment over YAML over defaults, empty-value preservation,
  strict unknown/mistyped YAML and environment-only compatibility.
- Prefix grammar: zero and 128 bytes; 129 bytes; missing leading slash;
  trailing/repeated slash; empty segment; complete dot/dot-dot segments;
  percent encoding; backslash; whitespace/control/NUL; query/fragment;
  scheme/authority/user information; Unicode; and valid unreserved segments.
- Detail safety: minimum/maximum IDs, slash, percent, question mark, fragment,
  Unicode and NUL cases with exactly one escaped segment and no prefix escape.
- Transport: redirects disabled, no URL resolution or normalization, no
  fallback after 401/404/timeout/decoding failure and no method/origin change.
- Compatibility: both `/rest/v1` prefixes reproduce RC2 paths and preserve
  typed resource output, alert/log Kafka contracts, metrics, state and pipeline
  selection.
- Observability: unique canaries in prefix, origin, credentials and bodies do
  not enter errors, logs, metrics or retained local evidence; readiness remains
  owned by the failed operation/pipeline.
- Documentation: two YAML keys, two environment names, defaults, grammar,
  route matrix, new/RC2/PDF examples and state-preserving rollback appear on
  every applicable committed surface.
- External integration: successful authentication, exact route capture and
  typed response validation for every enabled operation. HTTP 401/404 alone
  never passes this case.

TEST-DDAE-5-001 through TEST-DDAE-5-008 are planned deterministic local
evidence. TEST-DDAE-5-009 remains an authorized environment-dependent release
gate and cannot be closed with mocks or the supplied field report.

## DDAE-6 Test Design

DDAE-6 begins with all rows unexecuted. The three supplied payloads were used
only for a read-only structural audit. They are not committed fixtures and do
not prove authenticated detail endpoints, pagination, Kafka/OpenSearch delivery
or a supported release. Local tests must use minimal synthetic data.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-6-001 | AC-DDAE-6-001 | REQ-DDAE-6-001 | `path:internal/ddae/node_response_test.go` | decoder component | synthetic documented envelope, legacy array, missing/null/wrong top level, malformed item and trailing JSON | failing: `Client.Nodes` decodes only a bare array | `test` |
| TEST-DDAE-6-002 | AC-DDAE-6-002 | REQ-DDAE-6-002 | `path:internal/ddae/node_quantity_test.go` | decoder boundary | numeric zero/positive CPU, legacy strings, negative/fraction/exponent/type/size boundaries | failing: numeric CPU cannot unmarshal into `*string` | `test` |
| TEST-DDAE-6-003 | AC-DDAE-6-003 | REQ-DDAE-6-002 | `path:internal/ddae/node_alias_condition_test.go` | decoder compatibility | camel/hyphen storage aliases, equal/conflicting dual values, object/array conditions and malformed statuses | failing: camelCase storage is ignored and object conditions cannot unmarshal into a slice | `test` |
| TEST-DDAE-6-004 | AC-DDAE-6-004 | REQ-DDAE-6-003 | `path:internal/contract/node_payload_compatibility_test.go` | metric regression contract | equivalent documented/legacy synthetic nodes plus unknown and operational canaries | failing: documented envelope produces no node snapshot or metrics | `test` |
| TEST-DDAE-6-005 | AC-DDAE-6-005 | REQ-DDAE-6-004 | `path:internal/contract/informational_severity_test.go` | event regression contract | alert/log variants of Informational, existing severities, unsupported values and canonical events | failing: Informational normalizes to `unknown` | `test` |
| TEST-DDAE-6-006 | AC-DDAE-6-006 | REQ-DDAE-6-005 | `path:internal/contract/serviceability_list_detail_test.go` | pipeline component | detail-shaped list entries, selected IDs, escaped routes, returned-ID mismatch and recording transport | existing code calls details; no retained fixture proves supplied list-shape compatibility and no bypass | `test` |
| TEST-DDAE-6-007 | AC-DDAE-6-007 | REQ-DDAE-6-005 | `path:internal/contract/serviceability_incomplete_list_test.go` | readiness/state component | complete/malformed/duplicate/truncated lists including 500 returned versus 1167 total | core incomplete behavior exists; exact field-observed regression and no-query assertion are absent | `test` |
| TEST-DDAE-6-008 | AC-DDAE-6-008 | REQ-DDAE-6-006 | `path:scripts/security-policy.sh` | fixture/evidence security | repository plus synthetic fixtures/output seeded with raw-payload and operational canaries | current policy does not cover DDAE-6 fixture provenance or raw-payload exclusion | `security` |
| TEST-DDAE-6-009 | AC-DDAE-6-009 | REQ-DDAE-6-007 | `path:integration/ddae_kafka_test.go` | authenticated DDAE integration | authorized non-production DDAE, least-privilege credentials and sanitized nodes/event/issue list-detail evidence | unavailable: supplied files contain lists only and are not an authorized integration run | `integration` |

### Failure and Boundary Coverage

- Node top level: empty/truncated/malformed JSON, object without `results`, null
  or non-array results, scalar, bare array, documented envelope, duplicate IDs,
  malformed item, unknown fields and trailing JSON.
- CPU: zero, positive integer, maximum supported decimal, existing quantity
  strings, empty/oversized string, negative, fraction, exponent notation,
  boolean, null, array and object without fabricated zero values.
- Resource aliases: either storage spelling, equal dual values, conflicting
  dual values, missing optionals, invalid quantities and canary-bearing unknown
  fields.
- Conditions: documented object with either/both pressure fields, legacy array,
  missing fields, true/false case variants, unsupported status, wrong scalar,
  unknown condition name and no false pressure default.
- Metric compatibility: identical snapshots and exposition for equivalent
  shapes, unchanged descriptor set/units/labels, partial-family validation and
  complete exclusion of extra node fields.
- Severity: case/whitespace variants of Informational, all existing values,
  nil/empty/unsupported input, alert/log parity and unchanged canonical
  schema/key/header/hash behavior.
- Serviceability: list fields cannot bypass detail GET or detail validation;
  valid/unsafe/duplicate IDs, returned-ID mismatch, caps/fairness/deadlines,
  complete/incomplete disappearance and restart state remain covered.
- No invented API: a 500-of-1167 response sends no pagination query, offset,
  cursor, alternate list request or list-as-detail fallback and remains not
  ready without an absence transition.
- Privacy: raw supplied files, private endpoints, operational IDs/messages and
  seeded excluded-field canaries never enter Git, logs, metrics, events or
  retained evidence; source digests are allowed.
- External validation: successful authentication, configured paths, actual
  node shape, both list/detail ID relationships, honest completeness and
  read-only requests. This remains unexecuted until explicitly authorized.

TEST-DDAE-6-001 through TEST-DDAE-6-008 are planned deterministic local
evidence. TEST-DDAE-6-009 remains an authorized environment-dependent release
gate and cannot be closed with mocks, the PDF or the supplied list payloads.
## DDAE-7 Test Design

All rows are unexecuted candidate tests. The unchanged exporter cross-build
is isolated feasibility evidence, not acceptance of diagnostic behavior.
Only synthetic data may enter committed fixtures or retained local test output.

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-7-001 | AC-DDAE-7-001 | REQ-DDAE-7-001 | `path:scripts/build-portable.sh` | cross-build/package | pinned cached toolchain and declared source manifest | normal Windows exporter cross-build succeeds; diagnostic assets/packaging do not exist | `build` |
| TEST-DDAE-7-002 | AC-DDAE-7-002 | REQ-DDAE-7-002 | `path:internal/portable/config_test.go` | unit/platform adapter | strict YAML, BOM/newline secrets, Unicode/space paths, env canaries and TLS flags | independent diagnostic configuration/prepare contract does not exist | `test` |
| TEST-DDAE-7-003 | AC-DDAE-7-003 | REQ-DDAE-7-003 | `path:internal/portable/runner_test.go` | TLS component | fixed recording token/GET routes, disabled checks, unsafe IDs and retry responses | normal client allowlist exists; diagnostic budgeted runner does not | `test` |
| TEST-DDAE-7-004 | AC-DDAE-7-004 | REQ-DDAE-7-004 | `path:internal/portable/parser_test.go` | parser/contract component | documented/legacy bodies, missing/null results, incomplete lists and detail mismatch | production parsers exist; separate diagnostic contract status does not | `test` |
| TEST-DDAE-7-005 | AC-DDAE-7-005 | REQ-DDAE-7-005 | `path:internal/portable/capture_test.go` | HTTP/capture component | exact valid/invalid/HTML/compressed/chunked bodies, limits and read faults | production client discards raw body/decode detail; capture is absent | `test` |
| TEST-DDAE-7-006 | AC-DDAE-7-006 | REQ-DDAE-7-006 | `path:internal/portable/privacy_test.go` | privacy security | token/header/cookie/URL/body canaries and every output channel | normal redaction exists; protected-capture exception and token exclusion tests are absent | `security` |
| TEST-DDAE-7-007 | AC-DDAE-7-007 | REQ-DDAE-7-007 | `path:internal/portable/crypto_test.go` | crypto/storage security | synthetic key pairs, tampered/reordered/truncated ciphertext and unsafe outputs | key tools, authenticated capture and protection adapters are absent | `security` |
| TEST-DDAE-7-008 | AC-DDAE-7-008 | REQ-DDAE-7-008 | `path:internal/portable/limits_test.go` | failure component | fake clock, request/detail/body/header/disk caps, slow peers and cancellation | diagnostic run/output budgeting and partial recovery are absent | `test` |
| TEST-DDAE-7-009 | AC-DDAE-7-009 | REQ-DDAE-7-009 | `path:internal/portable/report_test.go` | report/exit contract | parallel runs, disabled/unavailable checks, failure priorities and seeded ZIP extras | safe standalone reports, exit schema and export allowlist are absent | `test` |
| TEST-DDAE-7-010 | AC-DDAE-7-010 | REQ-DDAE-7-010 | `path:internal/portable/offline_test.go` | offline component | embedded synthetic fixtures, encrypted captured cases and denied outbound transport | self-test/replay commands do not exist | `test` |
| TEST-DDAE-7-011 | AC-DDAE-7-011 | REQ-DDAE-7-011 | `path:scripts/build-portable.sh` | package/hygiene contract | dirty source with fake sensitive settings, private keys, results and unrelated files | no clean Portable distribution allowlist or operator instructions exist | `build` |
| TEST-DDAE-7-012 | AC-DDAE-7-012 | REQ-DDAE-7-012 | `path:internal/contract/portable_isolation_test.go` | normal-exporter regression | existing DDAE-1 through DDAE-6 contracts plus capture activation attempts | normal contracts exist; isolation against new capture entry point is not implemented | `test` |
| TEST-DDAE-7-013 | AC-DDAE-7-013 | REQ-DDAE-7-013 | `path:integration/portable_windows_test.go` | native Windows E2E | authorized Windows 11 x64/ARM64 host, NTFS, synthetic local API, Unicode/space paths | unavailable: current host is macOS; PE cross-build is not native evidence | `e2e` |
| TEST-DDAE-7-014 | AC-DDAE-7-014 | REQ-DDAE-7-014 | `path:integration/portable_ddae_test.go` | authenticated DDAE integration | authorized non-production DDAE, read-only identity, CA, public key and safe evidence | unavailable: no authorized live target or credential has been supplied | `integration` |

### Failure and Boundary Coverage

- Platform: AMD64/ARM64, 32-bit shell architecture selection, unsupported OS,
  missing/corrupt executable, absent optional CA, local NTFS versus unsupported
  filesystem, DACL refusal, reparse points, long/space/Unicode paths and an
  existing configuration that prepare must not overwrite.
- Configuration: duplicate/unknown keys, multiple documents, malformed values,
  relative-path base, empty/missing/BOM-sensitive credential files, environment
  override canaries, invalid prefixes/origins and every TLS opt-in combination.
- Transport: exact compiled token/GET routes, no business POST, redirects,
  401 refresh, 403/404, 429/5xx, token failures, all actual attempts counted,
  deadlines, disabled checks and no Kafka/OpenSearch/outbox access.
- Parser parity: documented and legacy nodes, cluster results envelope, null or
  missing results, malformed/trailing JSON, partial optional fields, unsafe
  IDs, list/detail mismatch, duplicate lists and 500-of-1167 incompleteness.
  Production outcome and contract observation remain separate; no hidden fix.
- Capture fidelity: every retry pair, no-response status, non-JSON/invalid UTF-8,
  non-2xx bytes, compression/chunking, empty body, exact-cap/cap-plus-one,
  truncated stream, delayed body, parser failure and no recapture request.
- Secret boundary: token responses and credential-bearing request bodies on
  both success/error paths, mixed-case auth/cookie headers and secret header
  values; inspect all safe outputs and decrypted metadata with synthetic
  canaries. Unknown sensitive business-body bytes are permitted only in the
  encrypted channel under the candidate exception, never safe logs/fixtures.
- Encryption: independent round-trip byte equality, fresh key/nonce behavior,
  wrong recipient, modified header/body/index, duplicated/reordered/deleted
  records, missing final index, malicious framing lengths, path traversal,
  output collision, key ACL failure and storage failure before/after a record.
  Test-only cryptographic controls use synthetic keys, never field key material.
- Runtime: all configuration minima/maxima, total ciphertext/framing cap,
  reserved summary space, disk-full/read-only directory, concurrent runs,
  Ctrl+C versus forced termination, request-budget exhaustion and no overlap.
- Reporting: no completed marker after partial finalization, correct exit-code
  priority, explicit SKIP reason and incomplete coverage, no raw URLs/IDs/
  messages/header/body hashes in safe output, and fixed allowlisted ZIP members.
- Offline: no external transport/credential/publication required; synthetic
  expected-failure cases are explicitly asserted; replay never follows an
  archive-provided URL or executes payload text.
- Packaging/regression: hashes/PE architecture/notices, actual dirty product
  identity, no unrelated document or seeded secret/private key/config/state/
  old result in the clean ZIP, unchanged metrics/events/normal config/readiness,
  and unchanged original redaction tests.
- External: actual Windows 11 launcher/ACL behavior per claimed architecture,
  then separately authorized authenticated DDAE API and parser evidence.
  Native execution requires real host results, not only imported self-reports.

Project build/security scripts must explicitly invoke the mapped new checks
while retaining all current checks. The existing test stage remains
race-enabled. TEST-DDAE-7-013 and TEST-DDAE-7-014 must have real executable
entry points wired into E2E/integration stages, but are not executed or closed
by this local pass. Missing external prerequisites remain blocked gates.

## DDAE-8 Test Design

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-8-001 | AC-DDAE-8-001 | REQ-DDAE-8-001 | `path:internal/ddae/cluster_response_test.go` | component | synthetic TLS arrays/envelopes, empty/populated | envelope fails existing decoder | `test` |
| TEST-DDAE-8-002 | AC-DDAE-8-002 | REQ-DDAE-8-002 | `path:internal/ddae/cluster_response_test.go` | boundary | missing/null/wrong types, invalid items, malformed/trailing/oversized JSON | null can be accepted; envelope unsupported | `test` |
| TEST-DDAE-8-003 | AC-DDAE-8-001 | REQ-DDAE-8-001 | `path:internal/collector/cluster_response_test.go` | component | equivalent values and invalid identity/optional resources | envelope cannot reach normalization | `test` |
| TEST-DDAE-8-004 | AC-DDAE-8-003 | REQ-DDAE-8-003 | `path:internal/portable/parser_test.go` | component | production recorded-body parity and synthetic self-test expectations | portable contract expects envelope failure | `test` |

| TEST-DDAE-8-005 | AC-DDAE-8-004 | REQ-DDAE-8-004 | `path:internal/ddae/cluster_response_test.go` | component | synthetic legacy/object/nested and malformed/mixed cases | object status unsupported and nested quantities omitted | `test` |
| TEST-DDAE-8-006 | AC-DDAE-8-005 | REQ-DDAE-8-005 | `path:internal/collector/cluster_response_test.go` | component | synthetic legacy/object/nested and malformed/mixed cases | object status unsupported and nested quantities omitted | `test` |
| TEST-DDAE-8-007 | AC-DDAE-8-006 | REQ-DDAE-8-006 | `path:internal/portable/parser_test.go` | component | synthetic legacy/object/nested and malformed/mixed cases | object status unsupported and nested quantities omitted | `test` |

### Failure and Boundary Coverage

Use the approved matrix in plans/DDAE-8.md: populated/empty arrays and envelopes,
whitespace/unknown metadata, invalid top levels/results/items, malformed and
trailing JSON, response bounds, identities, and optional resource errors.
Compare exact normalized cluster and metric values, and retain honest failure.
Run targeted tests, repository race tests, lint, coverage, build and full verify.
Retain failed/unavailable stages explicitly; no mock proves real integration.
Two authorized live resource cycles are supplemental diagnostic evidence only.

Approved 2.4.0 additions cover required object status, nested resources,
non-negative integer and quantity-string CPU, wrong types/numeric forms, mixed
layout rejection, missing optional values, ignored metadata and exact metric
parity. The previous numeric CPU rejection is intentionally superseded.

## Verification Record Location

Current execution results and limits are retained in plans/DDAE-8.md.

## DDAE-9 Test Plan

| Test | Acceptance | Requirement | Implementation | Kind | Cases | Prior state | Stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-9-001 | AC-DDAE-9-001 | REQ-DDAE-9-001 | `path:internal/config/query_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-002 | AC-DDAE-9-002 | REQ-DDAE-9-002 | `path:internal/queryclient/client_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-003 | AC-DDAE-9-003 | REQ-DDAE-9-003 | `path:internal/queryclient/client_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-004 | AC-DDAE-9-004 | REQ-DDAE-9-004 | `path:internal/queries/pipeline_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-005 | AC-DDAE-9-005 | REQ-DDAE-9-005 | `path:internal/queries/pipeline_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-006 | AC-DDAE-9-006 | REQ-DDAE-9-006 | `path:internal/queryclient/types_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-007 | AC-DDAE-9-007 | REQ-DDAE-9-007 | `path:internal/querystate/store_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-008 | AC-DDAE-9-008 | REQ-DDAE-9-008 | `path:internal/queries/pipeline_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |
| TEST-DDAE-9-009 | AC-DDAE-9-009 | REQ-DDAE-9-009 | `path:internal/contract/query_test.go` | component | approved candidate failure/success boundaries | query pipeline absent | `test` |

### Failure and Boundary Coverage

Full matrix in plans/DDAE-9.md and immutable candidate. Include source-bounded redirect authentication, stale/invalid/partial data, privacy, persistent dedup and backpressure; real integration remains separate.

## DDAE-10 Test Plan

| Test | Acceptance | Requirement | Implementation | Kind | Cases | Prior state | Stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-10-001 | AC-DDAE-10-001 | REQ-DDAE-10-001 | `path:internal/config/backfill_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-002 | AC-DDAE-10-002 | REQ-DDAE-10-002 | `path:internal/historyscan/adapters_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-003 | AC-DDAE-10-003 | REQ-DDAE-10-003 | `path:internal/historyscan/scan_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-004 | AC-DDAE-10-004 | REQ-DDAE-10-004 | `path:internal/historystate/store_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-005 | AC-DDAE-10-005 | REQ-DDAE-10-005 | `path:internal/historyscan/replay_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-006 | AC-DDAE-10-006 | REQ-DDAE-10-006 | `path:internal/historyscan/worker_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-007 | AC-DDAE-10-007 | REQ-DDAE-10-007 | `path:internal/historyscan/metrics_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-008 | AC-DDAE-10-008 | REQ-DDAE-10-008 | `path:internal/historyscan/recovery_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |
| TEST-DDAE-10-009 | AC-DDAE-10-009 | REQ-DDAE-10-009 | `path:internal/config/examples_test.go` | component | immutable approved candidate matrix | no time-filter/backfill progress support | `test` |

### Failure and Boundary Coverage

Use the immutable approved DDAE-10 candidate and plan matrix: caps, timestamp ties, overlap, replay, crashes, unknown schema, scope/TLS, concurrency, stale/readiness, privacy and recovery. Real integration evidence remains separate.
