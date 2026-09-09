# DDAE-1 Deliver the v1 DDAE exporter and its two supported deployment profiles

Use this plan only for the same unfinished observable outcome. If delivery work
reveals a behavior, design or acceptance-test change, record the discovery,
pause implementation, renew the governed source, digests and approvals, and
then continue. Isolated exploration is not deliverable implementation or
acceptance evidence.

This document is currently a planning draft. It does not authorize product
implementation and it contains no claim that a validation command or external
integration has run.

## Purpose and Observable Outcome

Deliver one Go process per Dell DDAE target that uses only the approved
read-only Management API v1 GET routes for Dell Data Lakehouse System Software
1.5.0. Background collectors publish a bounded, atomic Prometheus snapshot for
cluster status, node status/capacity, appliance operability and configured
coordinator/worker CPU and memory. A separate bounded alert pipeline reads the
serviceability alert list and approved per-ID details, creates typed schema-1.0
events, persists them in a bounded durable outbox and publishes them to Kafka
with at-least-once semantics for the existing OpenSearch/alerting consumer.

The release produces a reproducible static binary and minimal non-root OCI
image from one revision. Kubernetes and VM/systemd are both fully documented
and tested v1 deployment profiles. Operators can observe process liveness,
snapshot readiness, partial DDAE failures and Kafka/outbox health without
exposing credentials, endpoints, raw responses or alert payloads.

## Context and Constraints

- `SPECIFICATION.md` v1.0.0 is the complete functional release candidate and
  contains 24 requirements and 29 acceptance criteria. It is not authoritative
  until strict SDD is enabled and its digest receives named confirmation.
- Product implementation does not exist. Only bootstrap documentation and the
  portable Harness are present.
- `HARNESS/config.env` still has SDD, governance and product stages disabled.
  It must be changed to `required` with real commands before product code starts.
- The repository has no configured Git remote and most bootstrap files are not
  tracked. The selected repository/module identity is
  `github.com/crispkid/dell-ddae-metrics-exporter`, inferred from Peter Chu's
  configured GitHub account and subject to correction before digest approval.
- The selected toolchain is Go 1.26.5. Selected direct dependency versions are
  `github.com/prometheus/client_golang` v1.24.1,
  `github.com/twmb/franz-go` v1.21.6, `go.etcd.io/bbolt` v1.5.0 and
  `k8s.io/apimachinery` v0.36.4; other runtime code prefers the Go standard
  library. The versions were resolved from the Go module proxy on 2026-08-24;
  license and vulnerability stages remain required release evidence.
- The DDAE, Kafka and OpenSearch non-production boundaries are unavailable for
  current validation. No real integration claim may be made until Peter Chu
  authorizes the environments and retained evidence is sanitized.
- Peter Chu is currently the named project, approval, review and operational
  actor. For boundaries requiring an independent reviewer, Peter Chu satisfies
  independence only if he did not implement the reviewed change. If Peter Chu
  implements any such change, a different qualified reviewer must be named
  before that change can pass review.
- Runtime secrets are supplied only by Kubernetes Secret/CSI-mounted files,
  systemd credentials or equivalent runtime secret injection. They never enter
  Git, ordinary command arguments, logs, metrics, events or Harness evidence.
- DDAE alert payloads and inventory are confidential operational data. Only the
  versioned Kafka event allowlist may contain approved alert content.
- The environment-dependent validation deferral permits planning only. It does
  not waive integration, security, supply-chain, deployment or release gates.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

The implementation covers every requirement `REQ-DDAE-1-001` through
`REQ-DDAE-1-024` and every acceptance criterion `AC-DDAE-1-001` through
`AC-DDAE-1-029`. `TEST_PLAN.md` defines executable coverage, and
`TRACEABILITY.md` provides the exact requirement-to-task-to-test-to-stage map.

The product boundary is fixed:

- one configured DDAE 1.5.0 Management API v1 target per process;
- compiled GET-only methods for ping, clusters, infrastructure nodes, system
  lock, system shutdown, serviceability alert list and one encoded per-ID alert
  detail path;
- no generic proxy, scrape-time target, DDAE mutation method or out-of-scope
  collector;
- Prometheus metrics exactly as defined by the v1 metric contract, using fixed
  label sets, bounded enum normalization, Kubernetes quantity conversion and
  omission rather than fabricated zero for absent optional values;
- Kafka event schema `1.0`, the specified content hash, record key and headers,
  typed field allowlist, maximum event size and at-least-once lifecycle;
- no synthesized clear event when an alert disappears, because Dell 1.5.0 does
  not provide an authoritative clear lifecycle;
- background collection and alert polling; Prometheus scrapes never trigger
  DDAE fan-out;
- both Kubernetes and VM/systemd as supported deployment profiles.

### Technical Design

#### Module and component boundaries

- `cmd/ddae-exporter` contains only process composition, build metadata and
  exit-code handling. The module path is
  `github.com/crispkid/dell-ddae-metrics-exporter`.
- `internal/config` parses environment and `_FILE` secret variants into typed
  configuration. Secret wrappers do not implement string formatting, direct and
  file variants conflict, and complete validation happens before listeners or
  workers start.
- `internal/ddae` owns a shared `http.Transport`, origin validation, custom CA
  roots, fixed route methods, bounded response decoding and the coalesced token
  cache. Redirects cannot cross origin, proxy-from-environment is disabled and
  callers cannot provide arbitrary methods or URLs.
- `internal/collector` schedules non-overlapping cycles under one aggregate
  deadline. Fixed collectors run with bounded concurrency and publish typed
  results without retaining raw bodies.
- `internal/snapshot` publishes immutable snapshots atomically. Each family
  retains its real observation time and collector result; target families are
  withheld when stale while diagnostic metrics remain available.
- `internal/metrics` owns one private Prometheus registry and the fixed metric
  descriptors. It rejects duplicate identities and exports no Go process data
  capable of exposing command lines, environment values or request URLs.
- `internal/alerts` validates and deduplicates list IDs, derives exactly one
  encoded detail segment, applies new/change/periodic-refresh scheduling and
  builds a new typed schema-1.0 event. It never forwards a generic decoded map
  or raw DDAE response.
- `internal/outbox` uses one `bbolt` database beneath `STATE_DIR`, opened with
  restrictive permissions and a single process lock. Versioned buckets contain
  immutable event records, per-alert pending/delivered hashes, list markers,
  last-detail times and absence-retention metadata.
- `internal/kafka` uses `franz-go` with broker identity verification, bounded
  timeouts and buffers, producer idempotence, `acks=all` and ordering per record
  key. It receives only committed outbox records and never logs record values.
- `internal/server` serves only `/metrics`, `/healthz` and `/readyz`, applies
  explicit HTTP timeouts and performs graceful shutdown within the configured
  budget. Profiling and debug endpoints are absent from the public listener.
- `internal/observability` defines bounded error classes and structured logging
  with redaction at trust boundaries.

#### Collection, authentication and failure isolation

One shared token state serves all collectors. A positive documented lifetime
schedules renewal at the earlier of 80 percent of lifetime or 60 seconds before
expiry. The first 401 triggers one coalesced renewal and one retry of the same
safe request; a second 401 ends the attempt. No credential, request body,
Authorization header or raw response is retained or logged.

The default 30-second scheduler starts independent Prometheus and alert work
under the configured 20-second cycle deadline and 5-second request deadline.
Overlapping cycles are skipped or coalesced. Retries apply only to explicitly
classified transient safe operations and use bounded exponential backoff with
jitter. Every response is limited before decoding.

The required Prometheus snapshot is current only when ping, clusters, nodes,
lock and power succeeded within `DDAE_STALE_AFTER`. Partial failure preserves
unrelated current families, exposes the failed collector and real snapshot age,
and withholds stale target families. Kafka failure cannot falsify those metric
families, but persistent-state, alert-list/detail and outbox conditions feed the
separate alert-pipeline readiness and overall `/readyz` contract.

#### Metrics and cardinality

The committed metric table in `SPECIFICATION.md` is implemented verbatim. A
single descriptor definition is the source for name, help, type, unit, labels
and missing-value behavior. `cluster` and `node` accept only validated non-empty
Dell IDs and duplicate IDs fail the affected collector. State, condition,
collector and reason values come only from fixed sets; unknown enums normalize
to `unknown`. Timestamps, messages, request/event IDs, endpoints, broker/topic
names and other unbounded values never become labels.

#### Alert scheduling and typed event construction

The alert list runs every collection interval. IDs are length/character checked,
deduplicated and encoded as one data segment; the response ID must match. New or
changed usable `updatedon` markers are selected first, followed by oldest
periodic refreshes, with defaults of 200 details per cycle, concurrency four
and a ten-minute refresh ceiling. Ordering is deterministic and still-listed
IDs cannot starve. A list with `totalRecords` greater than usable results is
incomplete and degrades readiness while safe returned IDs may still be handled.

Event construction uses dedicated structs, validates every optional present
field, enforces the 256 KiB encoded bound and calculates canonical alert-object
SHA-256 before persistence. Dell labels, links, unknown fields and unapproved
nested data are never copied. Invalid detail data produces no partial event.

#### Durable outbox and Kafka delivery

In one durable transaction, a new normalized event is inserted before its
pending content hash and refresh checkpoint advance. The outbox record includes
its stable key, schema headers, payload, hash and ordering metadata. Kafka
acknowledgement permits a second transaction to mark the hash delivered and
remove the acknowledged outbox record. A crash before acknowledgement replays
the record; a crash after broker acknowledgement but before local commit may
produce a duplicate, which is accepted by the at-least-once contract and
deduplicated by downstream idempotent upsert.

Byte and event counts are checked transactionally before insert. Reaching
either hard limit preserves existing records, rejects the new write, stops the
affected checkpoint, sets readiness false and requires operator recovery. No
record expires automatically. Absent-alert checkpoints age out after retention;
currently listed alert checkpoints are never evicted for count pressure.

#### Build, CI and deployment

GitHub Actions is the CI control plane. GitHub-hosted runners execute formatting,
lint, unit/race tests, coverage, build, security, supply-chain and policy checks
that need no private network. An explicitly authorized self-hosted runner will
execute DDAE/Kafka/OpenSearch integration and deployment smoke stages when the
environment becomes available. Environment secrets are protected GitHub
environment inputs and never appear in reports.

The binary is built with a pinned Go toolchain, `CGO_ENABLED=0` where supported,
`-trimpath` and injected version/revision metadata. The OCI image uses a pinned
minimal non-root runtime with CA roots, a read-only root filesystem and a
dedicated writable state mount. Binary and image originate from the same source
revision and publish checksums, SBOM and provenance according to the release
workflow.

The Kubernetes profile provides manifests or an approved packaging format for
Secret/CSI injection, a ReadWriteOnce persistent filesystem volume with fsync
and file-lock semantics, probes, resource/security contexts, network policy and
external mTLS. The VM profile provides a systemd unit, systemd credentials,
dedicated non-login user, restrictive `STATE_DIR`, reverse-proxy mTLS boundary,
hardening settings and atomic binary/config rollback. Both profiles document
capacity, upgrade, outbox-full recovery and shutdown behavior.

#### Verification policy

Committed Harness commands will schedule `lint`, `test`, `coverage`, `build`,
`integration`, `e2e`, `security`, `supply-chain` and `ci-policy`. Unit/component
tests use deterministic sanitized fixtures and local HTTP/Kafka doubles.
Integration and E2E claims require an authorized non-production DDAE 1.5.0
system, isolated Kafka topic/broker and test OpenSearch consumer. Coverage
remains at least 80 percent, but numeric coverage never substitutes for mapped
acceptance evidence.

Environment-dependent validation is currently deferred. A deferred, skipped or
blocked required stage is not a pass and cannot close a trace row or release.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-1-001 | REQ-DDAE-1-001, REQ-DDAE-1-014, REQ-DDAE-1-015 | approved governance; final repository/module identity | `go.mod`, `cmd/ddae-exporter`, `internal/config`, `internal/observability` | TEST-DDAE-1-001, TEST-DDAE-1-017, TEST-DDAE-1-018 |
| TASK-DDAE-1-002 | REQ-DDAE-1-002, REQ-DDAE-1-012, REQ-DDAE-1-016 | TASK-DDAE-1-001 | `internal/server`, `internal/app`, `internal/snapshot` | TEST-DDAE-1-002, TEST-DDAE-1-014, TEST-DDAE-1-015, TEST-DDAE-1-019 |
| TASK-DDAE-1-003 | REQ-DDAE-1-003, REQ-DDAE-1-004, REQ-DDAE-1-005, REQ-DDAE-1-009, REQ-DDAE-1-011, REQ-DDAE-1-013 | TASK-DDAE-1-001 | `internal/ddae` | TEST-DDAE-1-003 through TEST-DDAE-1-006, TEST-DDAE-1-010, TEST-DDAE-1-011, TEST-DDAE-1-013, TEST-DDAE-1-016, TEST-DDAE-1-030 |
| TASK-DDAE-1-004 | REQ-DDAE-1-006, REQ-DDAE-1-007, REQ-DDAE-1-008, REQ-DDAE-1-010, REQ-DDAE-1-012 | TASK-DDAE-1-002, TASK-DDAE-1-003 | `internal/collector`, `internal/metrics`, `internal/snapshot` | TEST-DDAE-1-007 through TEST-DDAE-1-009, TEST-DDAE-1-012, TEST-DDAE-1-014, TEST-DDAE-1-015, TEST-DDAE-1-031 |
| TASK-DDAE-1-005 | REQ-DDAE-1-021, REQ-DDAE-1-024 | TASK-DDAE-1-003 | `internal/alerts` list/detail scheduler | TEST-DDAE-1-024, TEST-DDAE-1-028, TEST-DDAE-1-029 |
| TASK-DDAE-1-006 | REQ-DDAE-1-009, REQ-DDAE-1-010, REQ-DDAE-1-017, REQ-DDAE-1-023 | TASK-DDAE-1-005 | `internal/alerts` typed schema and sanitized fixtures | TEST-DDAE-1-020, TEST-DDAE-1-026, TEST-DDAE-1-027 |
| TASK-DDAE-1-007 | REQ-DDAE-1-014, REQ-DDAE-1-015, REQ-DDAE-1-022, REQ-DDAE-1-023 | TASK-DDAE-1-006 | `internal/outbox`, `internal/kafka` | TEST-DDAE-1-025, TEST-DDAE-1-027, TEST-DDAE-1-029 |
| TASK-DDAE-1-008 | REQ-DDAE-1-017 | TASK-DDAE-1-003 through TASK-DDAE-1-007 | deterministic fixtures and component regression suites | TEST-DDAE-1-020 |
| TASK-DDAE-1-009 | REQ-DDAE-1-018 | authorized non-production DDAE/Kafka/OpenSearch and self-hosted runner | `integration` | TEST-DDAE-1-021 |
| TASK-DDAE-1-010 | REQ-DDAE-1-019 | TASK-DDAE-1-001 through TASK-DDAE-1-008 | release scripts, Dockerfile, SBOM/provenance | TEST-DDAE-1-022, TEST-DDAE-1-032 |
| TASK-DDAE-1-011 | REQ-DDAE-1-020 | TASK-DDAE-1-002 through TASK-DDAE-1-010 | `docs/runbook`, `deploy/kubernetes`, `deploy/systemd` | TEST-DDAE-1-023 |
| TASK-DDAE-1-012 | REQ-DDAE-1-001, REQ-DDAE-1-002, REQ-DDAE-1-003, REQ-DDAE-1-004, REQ-DDAE-1-005, REQ-DDAE-1-006, REQ-DDAE-1-007, REQ-DDAE-1-008, REQ-DDAE-1-009, REQ-DDAE-1-010, REQ-DDAE-1-011, REQ-DDAE-1-012, REQ-DDAE-1-013, REQ-DDAE-1-014, REQ-DDAE-1-015, REQ-DDAE-1-016, REQ-DDAE-1-017, REQ-DDAE-1-018, REQ-DDAE-1-019, REQ-DDAE-1-020, REQ-DDAE-1-021, REQ-DDAE-1-022, REQ-DDAE-1-023, REQ-DDAE-1-024 | all prior tasks; named independent review | Harness policy, CI workflow, trace and current-run evidence | all tests and all scheduled stages |

### Verification Basis

- `lint`: formatting, `go vet` and a pinned Go linter configuration; advisory
  style does not replace behavior tests.
- `test`: deterministic unit/component tests, race detection where supported,
  Prometheus gather/contract checks, HTTP doubles, Kafka doubles, crash/restart
  state tests and boundary/failure cases.
- `coverage`: repository-wide unit/component coverage at or above 80 percent
  without excluding changed behavior merely to reach the number.
- `build`: clean pinned-toolchain build of the exporter and compile-time build
  metadata checks.
- `integration`: real authorized non-production token/GET paths plus isolated
  Kafka publish and OpenSearch idempotent upsert, with sanitized evidence only.
- `e2e`: operator-runbook execution for both Kubernetes and VM/systemd profiles,
  including health, shutdown, state persistence, replay and rollback.
- `security`: default-deny DDAE route/method analysis, secret/redaction tests,
  vulnerability scanning and deployment hardening review.
- `supply-chain`: locked dependencies, license review, SBOM, checksum,
  provenance and reproducibility checks for binary and OCI artifacts.
- `ci-policy`: confirms every required stage is scheduled with protected secret
  handling and that private integration runs only on the authorized runner.

No stage has run for this draft. External environment unavailability is retained
as a truthful blocker, not converted to a pass or waiver.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-08-24: v1 functional choices were consolidated in
  `SPECIFICATION.md` 1.0.0 before this plan.
- [x] 2026-08-24: Peter Chu selected the formal-repository, continued DDAE-1,
  dual Kubernetes/VM, GitHub Actions, early DDAE discovery, early Kafka consumer
  validation, bbolt, recommended Go dependency baseline and named-review model.
- [x] 2026-08-24: Peter Chu was recorded as the named owner/approver/reviewer
  actor for the draft; this was not recorded as digest approval.
- [x] 2026-08-24: Environment-dependent validation was explicitly deferred and
  no validation command was run for this planning update.
- [x] 2026-08-24: selected Go 1.26.5, module identity
  `github.com/crispkid/dell-ddae-metrics-exporter` and the four exact direct
  dependency versions recorded in the approved plan basis.
- [x] 2026-08-24: Peter Chu confirmed the module identity and approved the
  Specification, Plan Basis and Test Plan digests recorded in
  `DEVELOPMENT_PLAN.md`.
- [x] 2026-08-24: strict SDD/governance and real Harness stages were enabled;
  `governance:approved` passed before product implementation.
- [ ] Implement TASK-DDAE-1-001 through TASK-DDAE-1-012. Core source, local
  contract tests and both deployment profiles were added on 2026-08-25;
  independent review and environment-dependent evidence remain outstanding.
- [x] 2026-08-25: local lint, race test, 80% coverage, build,
  reproducible-binary and CI-policy checks passed. Coverage was 80.9%.
- [ ] 2026-08-25: the security stage's static and redaction tests passed, then
  the stage correctly blocked because `govulncheck` is not installed. The
  supply-chain checks verified modules and binary reproducibility during
  development; the current stage correctly blocks on the uncommitted working
  tree and will next require `cyclonedx-gomod` v1.10.0. No pass is claimed for
  either required stage.
- [ ] Authorize non-production environments and complete every required stage.

## Surprises and Discoveries

- The working tree contains a mature functional specification but no product
  implementation, while most bootstrap files are still untracked and no remote
  is configured. Formal repository identity must precede `go.mod` and approval.
- Supporting both Kubernetes and VM/systemd as full v1 deployment profiles adds
  a second secret, listener, state, upgrade and recovery boundary; estimated
  delivery increases from roughly 6-9 to 7-11 developer-weeks, excluding access
  and approval wait time.
- One person may hold several decision roles, but independent review is defined
  relative to implementation. Peter Chu cannot independently review a sensitive
  change that Peter Chu personally implemented.
- The current environment cannot supply the external validation required by the
  release contract. Unit implementation can only begin after governance approval,
  and the change cannot be handed off or released while required environment
  stages remain blocked.

## Decision Log

- 2026-08-24 — Repository identity: use
  `github.com/crispkid/dell-ddae-metrics-exporter`, inferred from Peter Chu's
  configured GitHub account. No remote was created. The inference must be
  corrected before digest approval if it is not the intended identity.
- 2026-08-24 — Change governance: continue `DDAE-1` and use named confirmation
  and plan approval. Decision owner and planned approver: Peter Chu. Actual
  digest approval: not yet performed.
- 2026-08-24 — Deployment: support Kubernetes and VM/systemd as equal v1
  profiles. Consequence: both require smoke, recovery and runbook evidence.
  Decision owner and operations owner: Peter Chu.
- 2026-08-24 — CI: use GitHub Actions; use an authorized self-hosted runner for
  private DDAE/Kafka/OpenSearch stages when available. Decision owner: Peter Chu.
- 2026-08-24 — DDAE evidence: prefer early non-production GET-only discovery and
  sanitized structural fixtures. Environment availability currently defers the
  action; it remains a release blocker. DDAE/security owner: Peter Chu.
- 2026-08-24 — Kafka/OpenSearch: validate the isolated broker/topic and existing
  idempotent consumer before release. Environment availability currently defers
  the action. Kafka/OpenSearch owner: Peter Chu.
- 2026-08-24 — State: use `bbolt` on a protected persistent filesystem/PVC for
  checkpoints and durable outbox. Recovery owner: Peter Chu.
- 2026-08-24 — Runtime baseline: Go 1.26.5 with `client_golang` v1.24.1,
  `franz-go` v1.21.6, `bbolt` v1.5.0 and `k8s.io/apimachinery` v0.36.4. Exact
  versions were resolved from the Go module proxy; license and vulnerability
  results are not yet claimed. Owner: Peter Chu.
- 2026-08-24 — Review: Peter Chu is the named reviewer for security/DDAE,
  Prometheus/SRE and Kafka/OpenSearch boundaries, provided he is independent of
  the reviewed implementation. Otherwise another qualified reviewer is required.
- 2026-08-24 — Validation deferral: because the necessary environment is not
  available, perform no validation work now. Consequence: no product, integration
  or release claim and no `Verified` trace status. Decision owner: Peter Chu.

## Recovery and Idempotence

Planning changes are ordinary Git files and can be reviewed before activation.
Do not enable strict SDD or begin product implementation until the repository
identity, exact technical pins, stage commands and approvals are complete. Once
approved, changes inside the approved plan-basis markers or `TEST_PLAN.md`
require new digests and renewed plan approval; specification changes also require
renewed requirements confirmation.

Implementation tasks must be restart-safe. Build and generation commands write
only reproducible artifacts under ignored output paths. Fixture acquisition uses
an allowlisted read-only discovery tool and a sanitization boundary; raw private
responses never become repository artifacts. Database migrations are versioned,
transactional and backed up before upgrade. Outbox replay is idempotent by stable
record key and downstream upsert. Deployment recovery returns to the last
verified binary/image plus versioned non-secret configuration while retaining
the state volume unless the recovery runbook explicitly diagnoses corruption.

## Outcomes and Retrospective

The governed plan is approved and implementation is in progress. Core Go
components, deterministic local tests, an OCI build definition and both
deployment-profile documents now exist. No external integration, E2E or
independent-review evidence is claimed. The release milestone remains blocked
until non-production DDAE, Kafka/OpenSearch and both deployment-profile stages
produce current sanitized evidence.
