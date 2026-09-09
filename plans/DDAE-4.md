# Execution Plan: DDAE-4 Serviceability Logs / Events

This living plan governs the backward-compatible addition of an independent,
disabled-by-default Serviceability Logs pipeline. Keep progress and observed
results outside the digest-bound Plan Basis block.

## Purpose

Implement DDAE-4 in one development pass with deterministic local unit and
component tests. The pass must add typed list/detail collection, dedicated
Kafka delivery, isolated durable state, bounded diagnostics, configuration,
deployment contracts and operator documentation without changing existing
resource or alert behavior.

## Context and Constraints

- DDAE-3 Batch 2 implementation and its mapped local `test` and `security`
  stages completed before this approval package was prepared. DDAE-4 reuses
  those implemented state-integrity, readiness, token, fairness and HTTP-bound
  contracts; their external and full-release evidence remains unresolved.
- Peter Chu requested a one-pass implementation plus local unit/component
  testing because no surrounding DDAE, Kafka or OpenSearch test environment is
  currently available.
- External integration/E2E evidence remains required for a supported release
  but is outside the current development pass. No mock result may be described
  as real DDAE, Kafka or OpenSearch compatibility.
- Product implementation begins only after Peter Chu confirms the exact
  Specification digest and approves the exact Plan Basis and Test Plan digests.
- The existing uncommitted DDAE-3 implementation is part of the working
  baseline and must be preserved.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

- TASK-DDAE-4-001 through TASK-DDAE-4-008 cover the locally implementable
  product, compatibility, documentation and deterministic-test work for
  REQ-DDAE-4-001 through REQ-DDAE-4-009.
- TASK-DDAE-4-009 retains the environment-dependent release evidence required
  by REQ-DDAE-4-010. It is intentionally outside the current local development
  pass and cannot be closed with test doubles.
- Existing DDAE-1 through DDAE-3 requirements remain regression contracts.
  DDAE-4 does not change their routes, metric identities, alert event schema,
  alert topic/key, alert state, TLS defaults or pipeline defaults.
- `TEST_PLAN.md` defines exact local and environment-dependent evidence paths.
  `TRACEABILITY.md` maps every requirement to tasks, tests and Harness stages;
  every DDAE-4 row starts as `Planned`.

### Technical Design

#### Activation and configuration

Extend the strict YAML and environment configuration with the exact
`monitoring.serviceability_logs`, `kafka.serviceability_logs_topic` and
`state.serviceability_logs_*` settings defined by Specification v1.3.0. The
effective default is disabled. At least one of resources, alerts or
Serviceability Logs must be enabled.

Require DDAE source identity and Kafka connection settings when alerts or logs
need them. Require the dedicated log topic and log-state limits only when logs
are enabled. Keep alert topic validation conditional on alerts. Reuse Kafka
transport credentials, TLS and SASL settings, but construct a separate log
producer instance bound to the dedicated topic. Strict YAML unknown-field
rejection and DDAE-2 environment precedence remain unchanged.

Application construction creates the log pipeline, log publisher, log producer
and log state only when the pipeline is enabled. A disabled configuration
creates no log database, worker, DDAE request or log-topic dependency and emits
only `ddae_monitoring_enabled{pipeline="serviceability_logs"} 0`.

#### DDAE list/detail boundary

Add exactly two compiled GET operations: the literal Serviceability Event list
route and one detail route whose untrusted ID is validated, escaped and
represented as one path segment. Preserve the existing origin and redirect
checks. Support Bundle, SupportAssist, mutations and any other route remain
rejected before transport.

Decode list and detail responses through bounded typed structures. The list
uses only safe unique IDs, optional update markers and the documented total.
Missing, invalid or excessive totals make completeness false without inventing
records. An incomplete list may supply safe work but cannot reconcile absence
or report readiness. Detail responses must return the exact requested ID.

Run list/detail work on its own interval and cycle budget. Partition detail
work into new/changed and refresh-due queues. Reuse DDAE-3 deterministic
ordering, weighted allocation, quota borrowing and limit-one alternation in an
independent scheduler so continuous new work cannot starve refreshes. Bound
requests per cycle, concurrency, request/cycle deadlines and deferred counts.

#### Typed event and canonical identity

Create a dedicated Serviceability Log domain package rather than reusing the
alert event type. Normalize only the v1.3.0 allowlisted fields into schema
`1.0`; ignore `labels`, `links` and unknown source fields. Invalid present
booleans, counts, timestamps, UTF-8, NUL-containing or oversized values fail
the event without truncation.

Canonical JSON for the normalized `log` object determines
`content_hash_sha256`. Observation time is outside that hash. The envelope,
required constants, maximum 256 KiB payload, record key and three fixed headers
must exactly match the Specification. Validate the same invariants when
reopening stored events so corrupt or future data fails closed.

#### Dedicated durable delivery and lifecycle

Use a separate protected `serviceability-logs.db` file under `STATE_DIR` with
dedicated meta, outbox and checkpoint buckets, schema marker, derived counters
and configured byte/record/checkpoint limits. Do not reuse the alert database,
buckets, sequence, checkpoint identity or capacity. Opening or repairing the
log store must never write the alert store.

Persist a valid log record before advancing its pending checkpoint. Keep
ordered transitions so `A -> A -> B -> A` suppresses only the duplicate second
`A` and retains deliverable `B` then final `A`. Acknowledge only after Kafka
acknowledgement. Uncertain delivery remains queued and restart replays in
per-key order, accepting duplicate delivery under the at-least-once contract.

Complete-list disappearance produces no deletion event. Retention can remove
only an old absent checkpoint with no pending record. Capacity pressure cannot
evict pending work, mutate an alert checkpoint or advance state after a failed
enqueue. Validate all primary records and cross-record invariants before any
derived-only counter correction.

#### Metrics, readiness and redaction

Add only the fixed diagnostic metric families and fixed collector identities
listed in Specification v1.3.0. Do not create labels from IDs, timestamps,
messages, reasons, remedies, resources, topics, endpoints or source objects.
Use bounded failure classes already approved by the observability contract.

Track log collection, log pipeline-state health and log publisher-state health
independently so success in another pipeline cannot clear a log-state failure.
An enabled pipeline is not ready for incomplete list/detail work, corrupt state
or exhausted capacity. A broker failure alone may remain ready while durable
state is healthy and has capacity. Overall readiness remains the conjunction
of every enabled pipeline.

Errors and structured logs contain operation identity and bounded class only.
Local tests insert unique canaries in every free-form value, endpoint, topic
and credential position and scan metrics, logs, errors and retained fixtures to
prove content does not cross the diagnostic boundary.

#### Compatibility, documentation and recovery

Preserve existing resource and alert behavior byte-contract compatibly. Add
local regression coverage for old YAML, all enable combinations, existing
metrics, alert event/key/topic, alert state and default startup. The only
default-visible addition is the fixed disabled enable series.

Update the YAML example, Kubernetes and systemd profiles, README and runbook
with every Serviceability Log setting, unit/range/default, topic/state
prerequisite, metric, readiness rule, payload contract and safe recovery step.
No new secret file type is introduced.

Deployment starts with the log pipeline disabled. Before first enable, provision
the dedicated topic/ACL and protected state capacity. Rollback disables only
the log pipeline or restores the prior binary/config while retaining
`serviceability-logs.db` for later replay. Deleting log state with queued data
is an explicit destructive operator action and is never automated.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-4-001 | REQ-DDAE-4-001 | approved DDAE-4 basis | `internal/ddae`, Serviceability route policy and path construction | TEST-DDAE-4-001 |
| TASK-DDAE-4-002 | REQ-DDAE-4-002 | TASK-DDAE-4-001 | `internal/config`, `internal/app`, deployment examples | TEST-DDAE-4-002 |
| TASK-DDAE-4-003 | REQ-DDAE-4-003 | TASK-DDAE-4-001, TASK-DDAE-4-002 | `internal/serviceability`, bounded list/detail scheduler | TEST-DDAE-4-003, TEST-DDAE-4-004 |
| TASK-DDAE-4-004 | REQ-DDAE-4-004 | TASK-DDAE-4-003 | typed log normalization, event encoding and stored-event validation | TEST-DDAE-4-005, TEST-DDAE-4-006 |
| TASK-DDAE-4-005 | REQ-DDAE-4-005, REQ-DDAE-4-006 | TASK-DDAE-4-004 | `internal/logstate`, dedicated Kafka producer/publisher and lifecycle | TEST-DDAE-4-007, TEST-DDAE-4-008, TEST-DDAE-4-009 |
| TASK-DDAE-4-006 | REQ-DDAE-4-007 | TASK-DDAE-4-003, TASK-DDAE-4-005 | response/event/state bounds, integrity and fail-closed readiness | TEST-DDAE-4-010, TEST-DDAE-4-011 |
| TASK-DDAE-4-007 | REQ-DDAE-4-008 | TASK-DDAE-4-003, TASK-DDAE-4-005, TASK-DDAE-4-006 | `internal/metrics`, `internal/snapshot`, `internal/observability`, security policy | TEST-DDAE-4-012 |
| TASK-DDAE-4-008 | REQ-DDAE-4-009 | TASK-DDAE-4-002 through TASK-DDAE-4-007 | compatibility contracts, YAML/deployment examples, README and runbook | TEST-DDAE-4-013, TEST-DDAE-4-014 |
| TASK-DDAE-4-009 | REQ-DDAE-4-010 | TASK-DDAE-4-001 through TASK-DDAE-4-008 plus authorized external prerequisites | isolated DDAE, Kafka and OpenSearch release evidence | TEST-DDAE-4-015, TEST-DDAE-4-016 |

### Verification Basis

- `test`: race-enabled deterministic unit/component tests using local TLS HTTP
  fixtures, temporary bbolt databases, fake/recording producers, controlled
  clocks, isolated registries and redaction canaries. These tests cover the
  current development pass without external infrastructure.
- `security`: source/policy checks preserve exact route, body/event/state bounds,
  fixed metric identity, redaction and dedicated state/topic construction.
  Existing repository security gates remain applicable to the final diff.
- `coverage`, `lint`, `build` and `ci-policy` remain whole-repository local
  quality gates even where no DDAE-4 trace row duplicates them.
- `integration`: authorized sanitized non-production DDAE route, permission,
  list/completeness and list/detail identity evidence. This stage is deferred
  until the environment is supplied.
- `e2e`: pre-DDAE-4 upgrade/rollback plus isolated Kafka ACL/topic/key/replay
  and OpenSearch upsert/redaction evidence. This stage is deferred until the
  environment is supplied.
- `supply-chain`, deployment attestation and independent security/data review
  remain inherited release gates. Local success does not waive or simulate
  them.

No pre-implementation result closes a DDAE-4 row. The current one-pass scope is
TASK-DDAE-4-001 through TASK-DDAE-4-008 and local deterministic tests
TEST-DDAE-4-001 through TEST-DDAE-4-013. TEST-DDAE-4-014 through
TEST-DDAE-4-016 remain release evidence and are not claimed without their
authorized environments.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-08-27: Peter Chu selected the six recommended DDAE-4 product design
  choices and requested the final specification.
- [x] 2026-08-27: Specification v1.3.0 was completed as a clarification-resolved
  final candidate without implementation authority.
- [x] 2026-08-27: DDAE-3 Batch 2 product changes and mapped local tests were
  completed; inherited external release evidence remains unresolved.
- [x] 2026-08-28: DDAE-4 was selected as the active approval candidate and its
  Plan Basis, Test Plan and traceability were drafted.
- [x] 2026-08-28: Harness structural validation accepted the complete
  requirement/task/test/stage mapping and generated stable approval digests.
- [x] 2026-08-28: Peter Chu confirmed the exact Specification digest, approved
  the exact Plan Basis and Test Plan digests, confirmed the module path and
  authorized the one-pass local implementation/testing scope.
- [x] 2026-08-28: Implemented TASK-DDAE-4-001 through TASK-DDAE-4-008 in one
  pass.
- [x] 2026-08-28: Ran deterministic race-enabled tests, coverage, lint, build,
  security and CI policy. The approved `govulncheck` completed with no
  source-reachable vulnerabilities found.
- [x] Retained TEST-DDAE-4-014 through TEST-DDAE-4-016 for a later authorized
  release-validation run.

## Discoveries

- Dell 1.5.0 documents the list and detail routes but does not define pagination
  or stable list completeness semantics; the implementation therefore cannot
  invent query parameters or claim lossless capture.
- Existing alert packages and state structures are alert-specific. A dedicated
  log domain and state database provide the clearest isolation and minimize
  regression risk to the approved alert contract.
- Test doubles can fully exercise bounded local control flow, state integrity,
  schema and redaction, but cannot establish real DDAE permission/list behavior
  or Kafka/OpenSearch compatibility.
- Weakly typed list decoding must retain safe typed IDs from mixed malformed
  results while marking the list incomplete; rejecting the whole response would
  prevent useful bounded progress required by AC-DDAE-4-003.
- Dedicated checkpoint capacity is exhausted when the configured record count
  is reached, so readiness reports full before a new ID could exceed the hard
  bound.
- The first expanded concurrency test exposed a race in its in-memory test
  double; protecting that test state made the race suite deterministic. The
  production bbolt state did not exhibit the race.
- The security policy must allow exactly the fixed
  `ddae_serviceability_log_records_failed_total{reason}` failure-class label
  while rejecting content-derived reason labels elsewhere.

## Decisions

- Use the implemented DDAE-3 Batch 2 behavior as the local implementation
  baseline while preserving its `Implemented` trace status and unresolved
  release gates.
- Use a separate `serviceability-logs.db`, producer instance, topic, domain
  schema and readiness components rather than sharing alert capacity or
  identities.
- Keep environment-dependent validation outside the current development pass
  while retaining it in the governed Test Plan and release gates.

## Recovery and Idempotence

All product edits are additive and remain behind the disabled default. If an
implementation step fails, keep existing DDAE-3 changes, disable or remove only
the incomplete DDAE-4 construction path, and rerun the narrow local tests. Do
not delete or rewrite an existing alert database. Repeated local tests use
temporary directories and isolated registries so reruns do not depend on prior
state.

## Outcomes and Retrospective

- Added the independently enabled Serviceability Logs list/detail worker,
  strict configuration, typed schema-1.0 event, dedicated Kafka producer/topic,
  protected `serviceability-logs.db`, durable replay/lifecycle, bounded metrics
  and readiness isolation.
- Updated Kubernetes/systemd YAML, README and runbook with configuration,
  installation, event, metric, capacity and recovery contracts.
- `test` passed with the race detector across all Go packages; `coverage`
  passed at 81.2% against the 80% gate; `lint`, `build` and `ci-policy` passed.
- The security stage passed source-policy checks, `go vet`, targeted Go tests
  and the approved `govulncheck`; no source-reachable vulnerabilities were
  found.
- Integration, E2E, supply-chain, deployment attestation and independent review
  were not run and remain explicit release gates. DDAE-4 is implemented
  locally, not verified or released.
