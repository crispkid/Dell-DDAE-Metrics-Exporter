# DDAE-3 Harden delivery correctness, bounded failure and state health

Use this plan only for the same unfinished observable outcome. If delivery work
reveals a behavior, design or acceptance-test change, record the discovery,
pause affected implementation, renew the governed source, digests and
approvals, and then continue. Isolated exploration is not deliverable
implementation or acceptance evidence.

## Purpose and Observable Outcome

Deliver one backward-compatible reliability and security amendment in two
ordered batches. Batch 1 moves every build to Go 1.26.6, makes outbox
deduplication converge across `A -> B -> A`, forces Kafka publish and shutdown
to honor hard deadlines while retaining uncertain records, and prevents a
selected non-main ref from running on the privileged self-hosted integration
runner.

After Batch 1 passes its mapped local gates, Batch 2 validates and versions
persistent state, preserves pending checkpoints, separates durable-state
health in readiness, coalesces token refresh outcomes by generation, guarantees
fair alert refresh, and bounds DDAE response headers/bodies plus `/metrics`
concurrency. Operators retain the same metric and Kafka contracts and receive
explicit startup/runtime failures instead of silent corruption, stale
downstream state or unbounded work.

## Context and Constraints

- DDAE-1 v1.0.0 and DDAE-2 v1.1.0 are implemented locally but still lack
  external integration/E2E, complete supply-chain evidence and independent
  review. DDAE-3 does not waive those release blockers.
- The 2026-08-25 review reproduced five source-reachable Go 1.26.5 standard
  library findings fixed in 1.26.6, the outbox `A -> B -> A` suppression path,
  franz-go's documented refusal to cancel uncertain in-flight idempotent
  records by default, missing state validation/readiness isolation, token and
  scheduler concurrency gaps, and HTTP resource-limit gaps.
- The existing event key/content hash and downstream idempotent-upsert contract
  already permits at-least-once duplicates. A Kafka hard deadline therefore
  prefers bounded shutdown over client-side duplicate avoidance for an
  uncertain in-flight request; the durable record is never acknowledged on an
  uncertain result.
- State currently uses `meta-v1`, `outbox-v1` and `checkpoints-v1` without a
  schema marker. Valid legacy state must upgrade in place. Primary corruption
  fails closed; only event-count and byte-count metadata are automatically
  recomputed because they are completely derivable from validated records.
- GitHub environment branch/reviewer rules and runner lifecycle live outside
  the repository. Static policy can prove the workflow guard and environment
  name, while an authorized operator must separately attest external controls.
- The user directed one SDD change with two batches. This preserves one
  approval/recovery boundary while preventing Batch 2 from obscuring the four
  urgent Batch 1 outcomes.
- Governed artifacts are currently ignored by repository `.gitignore`. This
  plan can be validated locally, but durable version-controlled handoff remains
  blocked until the separately identified governance-hygiene issue is resolved.
- Peter Chu confirmed the generated specification digest and approved the
  plan-basis and test-plan digests on 2026-08-26, authorizing Batch 1 only.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

- Batch 1 covers REQ-DDAE-3-001 through REQ-DDAE-3-004 and
  AC-DDAE-3-001 through AC-DDAE-3-005.
- Batch 2 covers REQ-DDAE-3-005 through REQ-DDAE-3-010 and
  AC-DDAE-3-006 through AC-DDAE-3-012.
- Existing DDAE-1/DDAE-2 requirements remain regression contracts. No DDAE-3
  requirement changes metric identity, DDAE route scope, Kafka event schema,
  TLS defaults or pipeline enable defaults.
- `TEST_PLAN.md` defines the executable pre-implementation failures and
  boundary fixtures. `TRACEABILITY.md` provides exact requirement/task/test/
  stage mappings, all initially `Planned`.

### Technical Design

#### Batch 1: toolchain and urgent delivery boundaries

Update the `toolchain` directive, pinned OCI builder tag/digest, build-policy
expectations and operator documentation from Go 1.26.5 to 1.26.6. Resolve the
official immutable builder digest during implementation, verify the module
graph, and run the pinned `govulncheck` policy. Do not change direct dependency
versions unless renewed review shows it is necessary to build on Go 1.26.6.

Change outbox deduplication to this state table:

| Pending hash | Delivered hash equals candidate | Action |
|---|---|---|
| candidate | either | suppress exact pending duplicate and update observation metadata |
| empty | yes | suppress already converged delivered content and update observation metadata |
| different non-empty hash | yes or no | enqueue candidate after existing ordered records and make it the current pending hash |
| empty or different | no | enqueue candidate and make it pending |

This deliberately allows more than one queued transition for one alert.
Acknowledging an older record updates delivered hash but clears pending hash
only when it equals that acknowledged record; the newest pending hash remains.
The `A -> B -> A` regression covers acknowledgement and restart boundaries.

Configure franz-go with in-flight idempotent produce cancellation enabled and
explicit request/delivery/dial bounds no greater than
`KAFKA_PUBLISH_TIMEOUT`. Keep `acks=all`, idempotent internal retries, stable
record key, one application record at a time and the durable outbox. The outer
publish context is authoritative. Any timeout/cancellation returns the bounded
Kafka timeout class and does not call `Acknowledge`. A local TCP/Kafka
blackhole test must prove return timing and goroutine cleanup after the request
is in flight, not merely before dialing.

Keep the application shutdown order: cancel workers, wait within
`SHUTDOWN_GRACE_PERIOD`, then close producer/client/state. The producer close
path must not reintroduce an unbounded flush. A component test retains the
record and proves application return within the grace budget plus the stated
test tolerance.

Strengthen the self-hosted job's `if` condition with
`github.ref == 'refs/heads/main'`, check out `${{ github.sha }}`, set
`persist-credentials: false`, and retain `workflow_dispatch`, explicit
`run_integration`, the `ddae-nonproduction` environment and self-hosted labels.
Extend CI policy to parse/assert the event/ref matrix rather than rely only on
incidental strings. External GitHub environment protection remains an
attestation prerequisite, not a repository-generated claim.

#### Batch 2: state integrity and readiness

Add a schema-version value to `meta-v1`. `Open` performs one validation/update
transaction before returning the store. New databases receive current buckets,
zero derived counters and the marker. Unversioned databases are treated as
legacy candidates: validate every primary record/checkpoint and cross-record
invariant, recompute count/bytes, then add the marker atomically. Current
databases undergo the same validation/recomputation. Unknown versions and
primary corruption abort without writes.

Validate outbox bucket keys as non-zero big-endian sequences equal to the JSON
record sequence; validate alert IDs, timestamps, 64-character lowercase hex
record/content hashes, schema-1.0 payload identity/hash/size and the stable
source-instance/alert-ID record key. Validate checkpoint bucket key equals
`AlertID`, timestamp/hash shapes are valid, and every non-empty pending hash has
a queued record for the same alert. Multiple queued transitions are allowed;
the checkpoint pending hash represents the newest queued state.

Treat stored count and byte values as derived caches. Missing, malformed or
mismatched derived values are overwritten from the fully validated outbox in
the same transaction. Never automatically alter/delete a primary record or
checkpoint to make validation pass.

Reconciliation may delete only a checkpoint that is absent from a complete
list, older than retention and has an empty pending hash. Capacity pressure
never broadens that eligibility. If eligible deletions do not reduce the count,
return `FullError`. `Acknowledge` is a no-op only when its sequence is absent;
an existing record with a missing/mismatched checkpoint returns a bounded state
corruption error and leaves all bytes unchanged.

Split alert readiness into collection readiness, pipeline-state health and
publisher-state health. The final alert readiness is their conjunction plus
not-full state. Each component sets its own sticky state-health bit false for
any store operation it owns and may clear only after its next full authoritative
state sequence succeeds. Publisher success cannot clear pipeline state health
and a pipeline cycle cannot clear publisher state health. Kafka broker errors
remain delivery diagnostics, not state-corruption signals.

#### Batch 2: authentication, fairness and HTTP bounds

Replace the token wait channel with a generation-bearing immutable refresh
result shared by all waiters. A caller holds the generation used for its DDAE
request. On 401 it asks for a refresh after that rejected generation; a newer
usable cache entry wins immediately. Otherwise one request runs and every
waiter observes the same token or the same bounded error. Classify token 429,
502, 503, 504 and transport/timeouts as transient and apply existing bounded
safe-request backoff under the parent context; do not retry credential
400/401/403 responses.

Partition detail tasks into new/changed and periodic-refresh queues. Sort both
by oldest fetch then alert ID. When the per-cycle limit is greater than one and
both queues have work, allocate `max(1, floor(limit/4))` slots to refresh and
the rest to new/changed, borrowing unused quota. With limit one, keep an
in-memory next-class turn and alternate while both classes remain non-empty.
Existing per-cycle/concurrency/deadline bounds remain unchanged.

Replace unbounded positive response-size parsing with an inclusive 1 through
67,108,864-byte validator for all three public body-limit settings. Use checked
read-limit arithmetic and exact EOF/trailing-content enforcement. Set the DDAE
transport response-header limit to 1,048,576 bytes.

Wrap only the `/metrics` Prometheus handler with five maximum in-flight
requests and a nine-second handler timeout. Keep `/healthz` and `/readyz`
outside the metric semaphore so overload cannot mask liveness/readiness. Retain
the server's existing method, read/write and idle timeouts.

#### Compatibility, documentation and recovery

Update README/runbook/config examples for Go 1.26.6, hard Kafka timeout and
duplicate implication, state validation/startup failure/recovery, the 64 MiB
configuration ceiling, metric overload responses and self-hosted main-ref
precondition. No new secret or event field is introduced.

Roll out with one stopped writer and a protected state copy. The added metadata
marker and derived-counter correction are backward-readable by DDAE-2, which
ignores the extra meta key. Rollback stops DDAE-3 cleanly and restores the prior
binary/config while preserving the current database and outbox records.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-3-001 | REQ-DDAE-3-001 | approved DDAE-3 basis | `go.mod`, `Dockerfile`, build/security scripts, README/runbook | TEST-DDAE-3-001, TEST-DDAE-3-002 |
| TASK-DDAE-3-002 | REQ-DDAE-3-002 | TASK-DDAE-3-001 | `internal/outbox`, state regression fixtures | TEST-DDAE-3-003 |
| TASK-DDAE-3-003 | REQ-DDAE-3-003 | TASK-DDAE-3-001, TASK-DDAE-3-002 | `internal/kafka`, `internal/app`, blackhole fixture, runbook | TEST-DDAE-3-004, TEST-DDAE-3-005 |
| TASK-DDAE-3-004 | REQ-DDAE-3-004 | TASK-DDAE-3-001 | `.github/workflows/ci.yml`, CI policy script | TEST-DDAE-3-006 |
| TASK-DDAE-3-005 | REQ-DDAE-3-005, REQ-DDAE-3-006 | Batch 1 mapped local stages pass | `internal/outbox`, state schema/integrity/checkpoint tests, runbook | TEST-DDAE-3-007, TEST-DDAE-3-008 |
| TASK-DDAE-3-006 | REQ-DDAE-3-007 | TASK-DDAE-3-005 | `internal/snapshot`, `internal/alerts`, `internal/kafka`, `internal/server` | TEST-DDAE-3-009 |
| TASK-DDAE-3-007 | REQ-DDAE-3-008 | Batch 1 mapped local stages pass | `internal/ddae` token manager/client and failure fixtures | TEST-DDAE-3-010 |
| TASK-DDAE-3-008 | REQ-DDAE-3-009 | TASK-DDAE-3-005 | `internal/alerts` scheduler and controlled-clock fixtures | TEST-DDAE-3-011 |
| TASK-DDAE-3-009 | REQ-DDAE-3-010 | Batch 1 mapped local stages pass | `internal/config`, `internal/ddae`, `internal/server`, security policy and documentation | TEST-DDAE-3-012, TEST-DDAE-3-013, TEST-DDAE-3-014 |

### Verification Basis

- `build`: clean Go 1.26.6 build with exact toolchain/build-metadata checks.
- `security`: pinned `govulncheck`, source/policy scans and response-bound
  checks; no network-derived result is claimed if the approved database/tool is
  unavailable.
- `test`: race-enabled deterministic package/component tests for transition
  ordering, in-flight blackholes and shutdown, state migration/corruption,
  readiness ownership, token waves, fairness and HTTP exhaustion.
- `ci-policy`: structural and event/ref matrix validation for the self-hosted
  job. It cannot attest external GitHub environment settings.
- Existing `coverage`, `integration`, `e2e` and `supply-chain` stages remain
  full-repository release gates. DDAE-3 local acceptance does not convert their
  unavailable or incomplete results into a pass.

No stage result obtained before implementation closes a DDAE-3 row. Batch 2
starts only after Batch 1's mapped local tests and policy stages pass.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-08-26: Peter Chu requested one SDD change with two ordered batches
  covering the ten named security/reliability outcomes.
- [x] 2026-08-26: the prior review evidence was mapped to stable requirements,
  acceptance tests, task dependencies, compatibility and recovery behavior.
- [x] 2026-08-26: Harness structural validation generated stable Specification,
  Plan Basis and Test Plan digests; `instructions:doctor` passed and the
  negative `sdd:check` correctly stopped at missing named confirmation.
- [x] 2026-08-26: Peter Chu explicitly confirmed the Specification digest,
  approved the Plan Basis and Test Plan digests, and authorized Batch 1; the
  named evidence is recorded in `DEVELOPMENT_PLAN.md`.
- [x] 2026-08-26: implemented Batch 1 and passed its mapped `build`, `test`,
  `security` and `ci-policy` stages; Batch 2 may now begin under the approved
  ordering when separately requested.
- [x] 2026-08-27: Peter Chu requested one-pass Batch 2 development and local
  unit/component testing while external environment validation remains
  excluded. Implemented TASK-DDAE-3-005 through TASK-DDAE-3-009.
- [x] 2026-08-27: local race-enabled `test`, `coverage`, `lint`, `build`,
  `security` and `ci-policy` stages passed; coverage was 81.0% and the pinned
  `govulncheck` v1.7.0 reported no vulnerabilities.
- [ ] Complete the same-run full `verify`, independent review and inherited
  external/release gates before changing Batch 2 trace rows to `Verified`.

## Surprises and Discoveries

- franz-go v1.21.6 explicitly offers in-flight idempotent produce cancellation
  for deployments that prefer a hard time bound and accept possible
  application-level duplicates; this matches the existing durable outbox and
  downstream idempotent-upsert contract.
- Outbox pending state represents only the newest content hash while the queue
  may legitimately contain multiple ordered transitions for one alert. State
  validation must preserve that model rather than require one record per alert.
- Derived outbox counters can be repaired without guessing, while payload,
  checkpoint or relationship corruption cannot. The plan therefore separates
  safe metadata recomputation from fail-closed primary data handling.
- The SDD/governance files are ignored by Git. Harness can validate their local
  bytes, but a clean Git status does not prove those approvals are retained.

## Decision Log

- 2026-08-26 — Change identity: one new `DDAE-3` with two ordered batches,
  rather than rewriting DDAE-1/DDAE-2 or using two independently approved
  changes. Evidence is the user's explicit singular-change/two-batch request;
  requirements decision owner is Peter Chu. This is scope direction, not
  digest approval.
- 2026-08-26 — Kafka deadline: use franz-go in-flight cancellation, retain
  idempotent internal retries/outbox, and accept possible stable-key duplicate
  replay. Waiting indefinitely preserves producer sequence guarantees but
  violates the requested hard timeout; disabling idempotence weakens normal
  retries. The selected option best fits bounded shutdown plus the existing
  at-least-once consumer contract; approval owner is Peter Chu.
- 2026-08-26 — State recovery: automatically recompute only fully derivable
  counters; fail closed on primary/cross-record corruption and require an
  operator-controlled offline repair. Silently dropping primary data risks
  alert loss; refusing any counter mismatch makes safe recovery unnecessarily
  manual. The selected split minimizes data-loss risk; approval owner is Peter
  Chu.
- 2026-08-26 — Fairness: reserve periodic-refresh capacity and alternate at
  limit one. Strict new/change priority causes starvation; pure global ordering
  can delay new critical alerts. Weighted two-class service gives a calculable
  non-zero share to both; approval owner is Peter Chu.
- 2026-08-26 — HTTP bounds: retain current defaults, cap configurable bodies at
  64 MiB, headers at 1 MiB, and metrics at five requests/nine seconds. Leaving
  them unbounded permits memory/goroutine exhaustion; much smaller values risk
  breaking current normal payloads/scrapes. These limits provide substantial
  headroom and fixed worst cases; approval owner is Peter Chu.

## Recovery and Idempotence

Apply tasks in dependency order and keep each batch independently revertible.
All state mutations use bbolt transactions. Re-running schema validation or
derived-counter recomputation produces the same marker/count/bytes. Re-running
an uncertain Kafka record may duplicate broker data but preserves the stable
key and content hash for downstream upsert.

Before any state-writing DDAE-3 run, stop the only writer and retain a protected
copy. Failed validation leaves primary data unchanged. Rollback stops workers,
uses the prior binary/configuration and keeps the database; do not delete an
uncertain record or replace current state with an older copy after new events
have arrived.

## Outcomes and Retrospective

Batch 1 completed locally on 2026-08-26. Go build sources now select 1.26.6 and
the OCI builder pins `golang:1.26.6-alpine3.23` at digest
`sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406`.
The A-to-B-to-A regression survives restart and ordered acknowledgements. A
protocol-aware TLS Kafka blackhole proves an in-flight produce returns within
the one-second deadline tolerance, remains unacknowledged, and permits bounded
producer/application shutdown. The privileged CI job is dispatch-only on main
and checks out the exact SHA without persisted credentials.

The mapped Batch 1 Harness `test`, `build`, `security` and `ci-policy` stages
passed; the race suite passed all packages and `govulncheck` reported no
source-reachable vulnerabilities. Lint, module tidy-diff and the reproducible
binary build also passed.

Batch 2 completed implementation and local tests on 2026-08-27. Startup now
validates/migrates versioned state and repairs only derived counters;
reconciliation protects pending checkpoints and acknowledgement fails closed
on checkpoint mismatch. Pipeline and publisher state health are independent
and sticky. Token renewal shares generation-aware results, alert scheduling
gives deterministic progress to both classes, and DDAE body/header plus
`/metrics` limits are enforced. Race-enabled tests, 81.0% aggregate coverage,
lint, build, CI policy and security including `govulncheck` all passed.

The same-run full `verify`, integration/E2E, supply-chain, independent review
and external GitHub-control attestations remain open, so Batch 2 trace rows are
`Implemented`, not `Verified`, and no supported release is claimed.
