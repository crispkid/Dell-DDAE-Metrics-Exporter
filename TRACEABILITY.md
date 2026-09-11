# Traceability

## DDAE-1 Requirement Trace

Every row is `Implemented`: all mapped implementation and executable test paths
exist. Local lint, race test, 80% coverage, build and reproducibility checks
have run successfully, but no row is `Verified` because required security,
supply-chain, integration, E2E and independent-review evidence is incomplete.
Environment-dependent verification remains deferred and is not acceptance
evidence.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-1-001 | 1.0.0 | AC-DDAE-1-001 | pinned Go module/toolchain and official Prometheus client | TASK-DDAE-1-001, TASK-DDAE-1-012 | `path:go.mod` | TEST-DDAE-1-001 | `build` | `stage:build` | Implemented |
| REQ-DDAE-1-002 | 1.0.0 | AC-DDAE-1-002 | three fixed HTTP endpoints over atomic state | TASK-DDAE-1-002, TASK-DDAE-1-012 | `path:internal/server/server.go` | TEST-DDAE-1-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-003 | 1.0.0 | AC-DDAE-1-003 | compiled typed GET allowlist and no generic mutation client | TASK-DDAE-1-003, TASK-DDAE-1-012 | `path:internal/ddae/allowlist.go` | TEST-DDAE-1-003, TEST-DDAE-1-030 | `test,security` | `stage:test,stage:security` | Implemented |
| REQ-DDAE-1-004 | 1.0.0 | AC-DDAE-1-004, AC-DDAE-1-005 | runtime-only secrets and coalesced in-memory token renewal | TASK-DDAE-1-003, TASK-DDAE-1-012 | `path:internal/ddae/auth.go` | TEST-DDAE-1-004, TEST-DDAE-1-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-005 | 1.0.0 | AC-DDAE-1-006 | verified TLS by default with optional added CA roots | TASK-DDAE-1-003, TASK-DDAE-1-012 | `path:internal/ddae/client.go` | TEST-DDAE-1-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-006 | 1.0.0 | AC-DDAE-1-007 | exact four-area metric surface plus bounded diagnostics | TASK-DDAE-1-004, TASK-DDAE-1-012 | `path:internal/metrics/contract.go` | TEST-DDAE-1-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-007 | 1.0.0 | AC-DDAE-1-008 | compiled scope and fixed Prometheus/Kafka routing | TASK-DDAE-1-004, TASK-DDAE-1-012 | `path:internal/collector/registry.go` | TEST-DDAE-1-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-008 | 1.0.0 | AC-DDAE-1-009 | one committed descriptor source for metric identity | TASK-DDAE-1-004, TASK-DDAE-1-012 | `path:internal/metrics/contract.go` | TEST-DDAE-1-009 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-009 | 1.0.0 | AC-DDAE-1-010, AC-DDAE-1-011 | typed allowlist decoding with pre-decode size bounds | TASK-DDAE-1-003, TASK-DDAE-1-006, TASK-DDAE-1-012 | `path:internal/ddae/types.go` | TEST-DDAE-1-010, TEST-DDAE-1-011 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-010 | 1.0.0 | AC-DDAE-1-012 | boundary-specific allowlists and fixed label/error classes | TASK-DDAE-1-004, TASK-DDAE-1-006, TASK-DDAE-1-012 | `path:internal/observability/redaction.go` | TEST-DDAE-1-012, TEST-DDAE-1-031 | `test,security` | `stage:test,stage:security` | Implemented |
| REQ-DDAE-1-011 | 1.0.0 | AC-DDAE-1-013 | non-overlap scheduler with bounded deadlines, retries and workers | TASK-DDAE-1-003, TASK-DDAE-1-012 | `path:internal/collector/scheduler.go` | TEST-DDAE-1-013 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-012 | 1.0.0 | AC-DDAE-1-014, AC-DDAE-1-015 | immutable family snapshots with honest partial/stale state | TASK-DDAE-1-002, TASK-DDAE-1-004, TASK-DDAE-1-012 | `path:internal/snapshot/store.go` | TEST-DDAE-1-014, TEST-DDAE-1-015 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-013 | 1.0.0 | AC-DDAE-1-016 | per-family validation isolation and bounded unknown enum | TASK-DDAE-1-003, TASK-DDAE-1-012 | `path:internal/collector/result.go` | TEST-DDAE-1-016 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-014 | 1.0.0 | AC-DDAE-1-017 | typed fail-before-serve config with `_FILE` secret inputs | TASK-DDAE-1-001, TASK-DDAE-1-007, TASK-DDAE-1-012 | `path:internal/config/config.go` | TEST-DDAE-1-017 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-015 | 1.0.0 | AC-DDAE-1-018 | structured bounded logs with trust-boundary redaction | TASK-DDAE-1-001, TASK-DDAE-1-007, TASK-DDAE-1-012 | `path:internal/observability/log.go` | TEST-DDAE-1-018 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-016 | 1.0.0 | AC-DDAE-1-019 | context-driven stop order and bounded graceful shutdown | TASK-DDAE-1-002, TASK-DDAE-1-012 | `path:internal/app/app.go` | TEST-DDAE-1-019 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-017 | 1.0.0 | AC-DDAE-1-020 | sanitized deterministic DDAE and Kafka contract fixtures | TASK-DDAE-1-006, TASK-DDAE-1-008, TASK-DDAE-1-012 | `path:testdata/ddae-1.5.0/` | TEST-DDAE-1-020 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-018 | 1.0.0 | AC-DDAE-1-021 | authorized non-production boundary and sanitized evidence | TASK-DDAE-1-009, TASK-DDAE-1-012 | `path:integration/ddae_kafka_test.go` | TEST-DDAE-1-021 | `integration` | `stage:integration` | Implemented |
| REQ-DDAE-1-019 | 1.0.0 | AC-DDAE-1-022 | one revision produces reproducible binary and OCI artifacts | TASK-DDAE-1-010, TASK-DDAE-1-012 | `path:Dockerfile` | TEST-DDAE-1-022, TEST-DDAE-1-032 | `supply-chain` | `stage:supply-chain` | Implemented |
| REQ-DDAE-1-020 | 1.0.0 | AC-DDAE-1-023 | operator-verified Kubernetes and VM/systemd runbooks | TASK-DDAE-1-011, TASK-DDAE-1-012 | `path:docs/runbook.md` | TEST-DDAE-1-023 | `e2e` | `stage:e2e` | Implemented |
| REQ-DDAE-1-021 | 1.0.0 | AC-DDAE-1-024 | bounded list/detail pipeline to typed Kafka events only | TASK-DDAE-1-005, TASK-DDAE-1-012 | `path:internal/alerts/pipeline.go` | TEST-DDAE-1-024 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-022 | 1.0.0 | AC-DDAE-1-025 | bounded idempotent Kafka producer backed by durable outbox | TASK-DDAE-1-007, TASK-DDAE-1-012 | `path:internal/kafka/producer.go` | TEST-DDAE-1-025 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-023 | 1.0.0 | AC-DDAE-1-026, AC-DDAE-1-027 | exact schema-1.0 envelope, key, hash and lifecycle | TASK-DDAE-1-006, TASK-DDAE-1-007, TASK-DDAE-1-012 | `path:internal/alerts/event.go` | TEST-DDAE-1-026, TEST-DDAE-1-027 | `test` | `stage:test` | Implemented |
| REQ-DDAE-1-024 | 1.0.0 | AC-DDAE-1-028, AC-DDAE-1-029 | one-segment validated ID plus fair bounded refresh/checkpoint | TASK-DDAE-1-005, TASK-DDAE-1-012 | `path:internal/alerts/detail.go` | TEST-DDAE-1-028, TEST-DDAE-1-029 | `test` | `stage:test` | Implemented |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`. A handoff
requires `Verified`; strict SDD has no row-level waiver. Return an impacted row
to `Planned` whenever its requirement, acceptance criteria, design, task,
implementation reference, test, stage or evidence mapping changes. Restore
`Verified` only after current approvals, completed references and a same-run
`verify` close the updated row.

## DDAE-2 Requirement Trace

Every DDAE-2 row is `Implemented`: the mapped product and executable test paths
now implement the approved YAML, pipeline-selection and guarded TLS behavior.
Local deterministic tests pass, but no row is `Verified` because the required
external E2E/integration, complete security toolchain, supply-chain and
independent-review evidence is incomplete. No DDAE-1 evidence is carried
forward as DDAE-2 acceptance evidence, and insecure runs remain ineligible.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-2-001 | 1.1.0 | AC-DDAE-2-001 | bounded versioned strict YAML selected before client/server construction | TASK-DDAE-2-001, TASK-DDAE-2-007 | `path:internal/config/yaml.go` | TEST-DDAE-2-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-002 | 1.1.0 | AC-DDAE-2-002 | defaults then YAML then compatible environment overrides with dedicated interval precedence | TASK-DDAE-2-001, TASK-DDAE-2-007 | `path:internal/config/config.go` | TEST-DDAE-2-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-003 | 1.1.0 | AC-DDAE-2-003 | YAML file-path-only secrets and preserved direct/FILE environment boundary | TASK-DDAE-2-001, TASK-DDAE-2-007 | `path:internal/config/yaml.go` | TEST-DDAE-2-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-004 | 1.1.0 | AC-DDAE-2-004 | conditional resource/alert workers, dependencies and independent schedules | TASK-DDAE-2-002, TASK-DDAE-2-007 | `path:internal/app/app.go` | TEST-DDAE-2-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-005 | 1.1.0 | AC-DDAE-2-005 | enabled-pipeline readiness plus fixed enable metric and disabled-family omission | TASK-DDAE-2-002, TASK-DDAE-2-003, TASK-DDAE-2-007 | `path:internal/metrics/collector.go` | TEST-DDAE-2-005, TEST-DDAE-2-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-006 | 1.1.0 | AC-DDAE-2-006, AC-DDAE-2-007 | default verified TLS with global and target-specific guarded opt-out | TASK-DDAE-2-004, TASK-DDAE-2-007 | `path:internal/config/config.go` | TEST-DDAE-2-007, TEST-DDAE-2-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-2-007 | 1.1.0 | AC-DDAE-2-008 | one mounted YAML interface with separate platform secrets across deployment profiles | TASK-DDAE-2-005, TASK-DDAE-2-007 | `path:deploy/systemd/config.example.yaml` | TEST-DDAE-2-009, TEST-DDAE-2-010 | `test,e2e` | `stage:test,stage:e2e` | Implemented |
| REQ-DDAE-2-008 | 1.1.0 | AC-DDAE-2-009 | deterministic regression/security matrix retaining default-secure DDAE-1 behavior | TASK-DDAE-2-006, TASK-DDAE-2-007 | `path:scripts/security-policy.sh` | TEST-DDAE-2-011 | `security` | `stage:security` | Implemented |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`. DDAE-2 is
implemented but cannot become `Verified` until same-run mapped-stage evidence
and the required independent review close every row. Insecure-TLS evidence is
never eligible to close a release/E2E row.

## DDAE-3 Requirement Trace

Batch 1 rows are `Verified` after named approval, implementation and successful
mapped local stages on 2026-08-26. Batch 2 is `Implemented` after its approved
tasks and local `test`/`security` evidence completed on 2026-08-27. The rows do
not become `Verified` until the same-run full `verify` requirement and inherited
release gates complete.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-3-001 | 1.2.0 | AC-DDAE-3-001 | uniform Go 1.26.6 build sources and source-reachable vulnerability gate | TASK-DDAE-3-001 | `path:go.mod` | TEST-DDAE-3-001, TEST-DDAE-3-002 | `build,security` | `stage:build,stage:security` | Verified |
| REQ-DDAE-3-002 | 1.2.0 | AC-DDAE-3-002 | dedupe pending identity while preserving ordered return to a delivered hash | TASK-DDAE-3-002 | `path:internal/outbox/store.go` | TEST-DDAE-3-003 | `test` | `stage:test` | Verified |
| REQ-DDAE-3-003 | 1.2.0 | AC-DDAE-3-003, AC-DDAE-3-004 | cancel uncertain in-flight idempotent produce, retain durable record and bounded shutdown | TASK-DDAE-3-003 | `path:internal/kafka/producer.go` | TEST-DDAE-3-004, TEST-DDAE-3-005 | `test` | `stage:test` | Verified |
| REQ-DDAE-3-004 | 1.2.0 | AC-DDAE-3-005 | explicit dispatch opt-in plus main-ref guard and credentialless exact-SHA checkout | TASK-DDAE-3-004 | `path:.github/workflows/ci.yml` | TEST-DDAE-3-006 | `ci-policy` | `stage:ci-policy` | Verified |
| REQ-DDAE-3-005 | 1.2.0 | AC-DDAE-3-006 | transactional schema validation, legacy marking and derived-only counter repair | TASK-DDAE-3-005 | `path:internal/outbox/store.go` | TEST-DDAE-3-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-3-006 | 1.2.0 | AC-DDAE-3-007 | pending checkpoints are ineligible for eviction and existing-record acknowledgement requires a matching checkpoint | TASK-DDAE-3-005 | `path:internal/outbox/store.go` | TEST-DDAE-3-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-3-007 | 1.2.0 | AC-DDAE-3-008 | pipeline and publisher own independent sticky durable-state health components | TASK-DDAE-3-006 | `path:internal/snapshot/store.go` | TEST-DDAE-3-009 | `test` | `stage:test` | Implemented |
| REQ-DDAE-3-008 | 1.2.0 | AC-DDAE-3-009 | generation-aware shared refresh result and bounded transient token retry | TASK-DDAE-3-007 | `path:internal/ddae/auth.go` | TEST-DDAE-3-010 | `test` | `stage:test` | Implemented |
| REQ-DDAE-3-009 | 1.2.0 | AC-DDAE-3-010 | two-class weighted fairness with limit-one alternation and deterministic ordering | TASK-DDAE-3-008 | `path:internal/alerts/pipeline.go` | TEST-DDAE-3-011 | `test` | `stage:test` | Implemented |
| REQ-DDAE-3-010 | 1.2.0 | AC-DDAE-3-011, AC-DDAE-3-012 | 64 MiB checked bodies, 1 MiB headers and isolated five-request/nine-second metrics bound | TASK-DDAE-3-009 | `path:internal/ddae/client.go` | TEST-DDAE-3-012, TEST-DDAE-3-013, TEST-DDAE-3-014 | `test,security` | `stage:test,stage:security` | Implemented |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`. Move a
row only after its approved task and references actually exist. Final handoff
requires the exact mapped stages to pass in the same `verify` run; DDAE-3 does
not waive any inherited release gate.

## DDAE-4 Requirement Trace

REQ-DDAE-4-001 through REQ-DDAE-4-009 are `Implemented` after Peter Chu's
digest-bound approval, completion of TASK-DDAE-4-001 through TASK-DDAE-4-008,
and successful deterministic local race-enabled tests on 2026-08-28.
REQ-DDAE-4-010 remains `Planned` because its authorized DDAE, Kafka and
OpenSearch evidence is outside this development pass. External integration,
E2E and inherited release gates are still required before any DDAE-4 row can
be `Verified`.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-4-001 | 1.3.0 | AC-DDAE-4-001 | exactly two compiled GET operations with one-segment ID validation and origin-safe transport | TASK-DDAE-4-001 | `path:internal/ddae/allowlist.go` | TEST-DDAE-4-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-002 | 1.3.0 | AC-DDAE-4-002 | independently selected disabled-by-default workers and conditional dependencies | TASK-DDAE-4-002 | `path:internal/config/config.go` | TEST-DDAE-4-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-003 | 1.3.0 | AC-DDAE-4-003, AC-DDAE-4-004 | bounded typed list/detail retrieval with completeness and independent DDAE-3 fairness | TASK-DDAE-4-003 | `path:internal/serviceability/pipeline.go` | TEST-DDAE-4-003, TEST-DDAE-4-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-004 | 1.3.0 | AC-DDAE-4-005, AC-DDAE-4-006 | exact typed schema with canonical allowed-content hash and no raw/generic forwarding | TASK-DDAE-4-004 | `path:internal/serviceability/event.go` | TEST-DDAE-4-005, TEST-DDAE-4-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-005 | 1.3.0 | AC-DDAE-4-007, AC-DDAE-4-008 | dedicated topic, kind key, producer and durable namespace with acknowledged replay | TASK-DDAE-4-005 | `path:internal/logstate/store.go` | TEST-DDAE-4-007, TEST-DDAE-4-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-006 | 1.3.0 | AC-DDAE-4-009 | ordered new-or-changed upserts with no disappearance event and pending-safe retention | TASK-DDAE-4-005 | `path:internal/logstate/store.go` | TEST-DDAE-4-009 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-007 | 1.3.0 | AC-DDAE-4-010, AC-DDAE-4-011 | checked body/event/request/state bounds plus primary-integrity fail-closed behavior | TASK-DDAE-4-006 | `path:internal/logstate/store.go` | TEST-DDAE-4-010, TEST-DDAE-4-011 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-008 | 1.3.0 | AC-DDAE-4-012 | fixed bounded diagnostics and canary-proven content exclusion | TASK-DDAE-4-007 | `path:internal/metrics/collector.go` | TEST-DDAE-4-012 | `test` | `stage:test` | Implemented |
| REQ-DDAE-4-009 | 1.3.0 | AC-DDAE-4-013 | disabled-default additive compatibility with old state/config and alert byte contracts | TASK-DDAE-4-008 | `path:internal/contract/serviceability_compatibility_test.go` | TEST-DDAE-4-013, TEST-DDAE-4-014 | `test,e2e` | `stage:test,stage:e2e` | Implemented |
| REQ-DDAE-4-010 | 1.3.0 | AC-DDAE-4-014 | retain authorized real-system evidence as a release gate separate from mocks | TASK-DDAE-4-009 | `path:scripts/stage-integration.sh` | TEST-DDAE-4-015, TEST-DDAE-4-016 | `integration,e2e` | `stage:integration,stage:e2e` | Planned |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`. Local
development may complete without TEST-DDAE-4-014 through TEST-DDAE-4-016, but
it must report those stages as unavailable and cannot claim supported release
or complete DDAE-4 verification.

## DDAE-5 Requirement Trace

Peter Chu confirmed the exact Specification digest and approved the exact Plan
Basis and Test Plan digests on 2026-08-31. REQ-DDAE-5-001 through
REQ-DDAE-5-006 are `Implemented` after completion of TASK-DDAE-5-001 through
TASK-DDAE-5-005 and successful local race, coverage, lint, build, security and
CI-policy stages on 2026-09-01. REQ-DDAE-5-007 remains `Planned` because an
authorized authenticated DDAE integration run is outside this development
pass.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-5-001 | 2.0.0 | AC-DDAE-5-001 | empty Ping prefix, `/v1` API prefix and unchanged fixed token path | TASK-DDAE-5-002 | `path:internal/ddae/allowlist.go` | TEST-DDAE-5-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-5-002 | 2.0.0 | AC-DDAE-5-002, AC-DDAE-5-003 | two strict YAML/environment prefix fields with environment-over-YAML-over-default precedence | TASK-DDAE-5-001, TASK-DDAE-5-002 | `path:internal/config/yaml.go` | TEST-DDAE-5-002, TEST-DDAE-5-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-5-003 | 2.0.0 | AC-DDAE-5-004 | bounded canonical prefix grammar plus compiled suffixes, fixed origin/method and no fallback | TASK-DDAE-5-001, TASK-DDAE-5-002, TASK-DDAE-5-003 | `path:internal/ddae/client.go` | TEST-DDAE-5-004, TEST-DDAE-5-005 | `test,security` | `stage:test,stage:security` | Implemented |
| REQ-DDAE-5-004 | 2.0.0 | AC-DDAE-5-005 | explicit RC2 prefixes preserve every non-route data, metric, event and state contract | TASK-DDAE-5-004 | `path:internal/contract/path_prefix_compatibility_test.go` | TEST-DDAE-5-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-5-005 | 2.0.0 | AC-DDAE-5-006 | existing bounded failure/readiness ownership with no prefix, endpoint, credential or body disclosure | TASK-DDAE-5-003, TASK-DDAE-5-004 | `path:internal/ddae/client.go` | TEST-DDAE-5-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-5-006 | 2.0.0 | AC-DDAE-5-007 | exact configuration, route, migration and rollback contracts on every operator surface | TASK-DDAE-5-005 | `path:README.md` | TEST-DDAE-5-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-5-007 | 2.0.0 | AC-DDAE-5-008 | authenticated sanitized DDAE validation stays separate from route reachability and mocks | TASK-DDAE-5-006 | `path:integration/ddae_kafka_test.go` | TEST-DDAE-5-009 | `integration` | `stage:integration` | Planned |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`.
REQ-DDAE-5-001 through REQ-DDAE-5-006 are locally implemented but not
`Verified` because the same-run full Harness closure includes excluded external
stages. REQ-DDAE-5-007 remains `Planned` until an authorized authenticated
integration run exists; inherited release gates remain separate.

## DDAE-6 Requirement Trace

Every DDAE-6 row is `Planned`. The PDF/payload audit identifies the required
behavior and pre-implementation failures but is not implementation or
acceptance evidence. Raw supplied payloads are excluded from repository
fixtures and retained evidence.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-6-001 | 2.1.0 | AC-DDAE-6-001 | one bounded decoder accepts documented envelope and legacy array | TASK-DDAE-6-002 | `path:internal/ddae/client.go` | TEST-DDAE-6-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-6-002 | 2.1.0 | AC-DDAE-6-002, AC-DDAE-6-003 | typed quantity, storage-alias and condition-shape compatibility | TASK-DDAE-6-002 | `path:internal/ddae/types.go` | TEST-DDAE-6-002, TEST-DDAE-6-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-6-003 | 2.1.0 | AC-DDAE-6-004 | canonical node model preserves existing metric contract and excludes extras | TASK-DDAE-6-002, TASK-DDAE-6-005 | `path:internal/collector/collector.go` | TEST-DDAE-6-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-6-004 | 2.1.0 | AC-DDAE-6-005 | Informational is an input alias for existing output info | TASK-DDAE-6-003 | `path:internal/alerts/event.go` | TEST-DDAE-6-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-6-005 | 2.1.0 | AC-DDAE-6-006, AC-DDAE-6-007 | documented detail calls plus incomplete-list safety and no invented pagination | TASK-DDAE-6-004 | `path:internal/serviceability/pipeline.go` | TEST-DDAE-6-006, TEST-DDAE-6-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-6-006 | 2.1.0 | AC-DDAE-6-008 | synthetic structural fixtures with digest provenance and canary scanning | TASK-DDAE-6-001, TASK-DDAE-6-005 | `path:testdata/ddae-1.5.0/` | TEST-DDAE-6-008 | `security` | `stage:security` | Implemented |
| REQ-DDAE-6-007 | 2.1.0 | AC-DDAE-6-009 | authorized authenticated nodes and list/detail evidence; pagination deferred | TASK-DDAE-6-006 | `path:integration/ddae_kafka_test.go` | TEST-DDAE-6-009 | `integration` | `stage:integration` | Planned |

Allowed active statuses are `Planned`, `Implemented`, and `Verified`.
REQ-DDAE-6-001 through REQ-DDAE-6-006 are `Implemented` after the mapped local
race, coverage, lint, build, security and CI-policy stages passed on
2026-09-02. REQ-DDAE-6-007 remains `Planned`; local tests cannot close it or
the inherited external release gates.

## DDAE-7 Requirement Trace

All rows are Planned. Referenced new implementation/test paths are intended
targets, not claims that files or passing evidence already exist. The earlier
DDAE-6 baseline and its unresolved external gates retain their history.

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-7-001 | 2.2.0 | AC-DDAE-7-001 | native dual Windows binaries and complete runtime assets | TASK-DDAE-7-001 | `path:scripts/build-portable.sh` | TEST-DDAE-7-001 | `build` | `stage:build` | Implemented |
| REQ-DDAE-7-002 | 2.2.0 | AC-DDAE-7-002 | isolated strict diagnostic YAML and safe local preparation | TASK-DDAE-7-002 | `path:internal/portable/config.go` | TEST-DDAE-7-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-003 | 2.2.0 | AC-DDAE-7-003 | fixed production network boundary without publishing | TASK-DDAE-7-003 | `path:internal/portable/runner.go` | TEST-DDAE-7-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-004 | 2.2.0 | AC-DDAE-7-004 | same production parser with separate contract observations | TASK-DDAE-7-004 | `path:internal/portable/parser.go` | TEST-DDAE-7-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-005 | 2.2.0 | AC-DDAE-7-005 | bounded exact parser-input capture per actual attempt | TASK-DDAE-7-005 | `path:internal/ddae/diagnostic.go` | TEST-DDAE-7-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-006 | 2.2.0 | AC-DDAE-7-006 | separate safe/encrypted sinks and token exclusion | TASK-DDAE-7-006 | `path:internal/ddae/diagnostic.go` | TEST-DDAE-7-006 | `security` | `stage:security` | Implemented |
| REQ-DDAE-7-007 | 2.2.0 | AC-DDAE-7-007 | authenticated encrypted records and separately held private key | TASK-DDAE-7-007 | `path:internal/portable/crypto.go` | TEST-DDAE-7-007 | `security` | `stage:security` | Implemented |
| REQ-DDAE-7-008 | 2.2.0 | AC-DDAE-7-008 | bounded run, shutdown and partial-evidence semantics | TASK-DDAE-7-008 | `path:internal/portable/runner.go` | TEST-DDAE-7-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-009 | 2.2.0 | AC-DDAE-7-009 | typed safe report, honest statuses and explicit archive members | TASK-DDAE-7-009 | `path:internal/portable/report.go` | TEST-DDAE-7-009 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-010 | 2.2.0 | AC-DDAE-7-010 | embedded synthetic self-test and network-disabled parser replay | TASK-DDAE-7-010 | `path:internal/portable/offline.go` | TEST-DDAE-7-010 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-011 | 2.2.0 | AC-DDAE-7-011 | clean distribution and honest source/docs/runtime hygiene | TASK-DDAE-7-011 | `path:Portable/README.zh-TW.md` | TEST-DDAE-7-011 | `build` | `stage:build` | Implemented |
| REQ-DDAE-7-012 | 2.2.0 | AC-DDAE-7-012 | normal exporter cannot activate capture; existing behavior preserved | TASK-DDAE-7-012 | `path:internal/ddae/client.go` | TEST-DDAE-7-012 | `test` | `stage:test` | Implemented |
| REQ-DDAE-7-013 | 2.2.0 | AC-DDAE-7-013 | actual native Windows evidence separate from cross-build | TASK-DDAE-7-013 | `path:integration/portable_windows_test.go` | TEST-DDAE-7-013 | `e2e` | `stage:e2e` | Planned |
| REQ-DDAE-7-014 | 2.2.0 | AC-DDAE-7-014 | authorized authenticated field proof separate from mocks | TASK-DDAE-7-014 | `path:integration/portable_ddae_test.go` | TEST-DDAE-7-014 | `integration` | `stage:integration` | Planned |

The raw-retention exception affects only new diagnostic paths mapped to
REQ-DDAE-7-005 through REQ-DDAE-7-007 and normal-path isolation under
REQ-DDAE-7-012. It does not retroactively approve raw retention under DDAE-1,
DDAE-5 or DDAE-6. Native Windows and DDAE rows remain environment-dependent.

## DDAE-8 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-8-001 | 2.4.0 | AC-DDAE-8-001 | shared cluster list decoder | TASK-DDAE-8-001, TASK-DDAE-8-002, TASK-DDAE-8-003, TASK-DDAE-8-005 | `path:internal/ddae/types.go` | TEST-DDAE-8-001, TEST-DDAE-8-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-002 | 2.4.0 | AC-DDAE-8-002 | strict bounded shape and existing semantics | TASK-DDAE-8-001, TASK-DDAE-8-002, TASK-DDAE-8-003, TASK-DDAE-8-005 | `path:internal/ddae/types.go` | TEST-DDAE-8-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-003 | 2.4.0 | AC-DDAE-8-003 | recorded parser reuses live client | TASK-DDAE-8-001, TASK-DDAE-8-004, TASK-DDAE-8-005 | `path:internal/portable/parser.go` | TEST-DDAE-8-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-004 | 2.4.0 | AC-DDAE-8-004 | typed field mapping with unchanged output | TASK-DDAE-8-006 | `path:internal/ddae/types.go` | TEST-DDAE-8-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-005 | 2.4.0 | AC-DDAE-8-005 | typed field mapping with unchanged output | TASK-DDAE-8-007 | `path:internal/ddae/types.go` | TEST-DDAE-8-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-006 | 2.4.0 | AC-DDAE-8-006 | typed field mapping with unchanged output | TASK-DDAE-8-008 | `path:internal/ddae/types.go` | TEST-DDAE-8-007 | `test` | `stage:test` | Implemented |

## DDAE-9 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-9-001 | 2.5.0 | AC-DDAE-9-001 | isolated bounded query pipeline | TASK-DDAE-9-001 | `path:internal/config/query.go` | TEST-DDAE-9-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-002 | 2.5.0 | AC-DDAE-9-002 | isolated bounded query pipeline | TASK-DDAE-9-002 | `path:internal/queryclient/client.go` | TEST-DDAE-9-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-003 | 2.5.0 | AC-DDAE-9-003 | isolated bounded query pipeline | TASK-DDAE-9-003 | `path:internal/queryclient/client.go` | TEST-DDAE-9-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-004 | 2.5.0 | AC-DDAE-9-004 | isolated bounded query pipeline | TASK-DDAE-9-004 | `path:internal/queries/pipeline.go` | TEST-DDAE-9-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-005 | 2.5.0 | AC-DDAE-9-005 | isolated bounded query pipeline | TASK-DDAE-9-005 | `path:internal/queries/pipeline.go` | TEST-DDAE-9-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-006 | 2.5.0 | AC-DDAE-9-006 | isolated bounded query pipeline | TASK-DDAE-9-006 | `path:internal/queryclient/types.go` | TEST-DDAE-9-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-007 | 2.5.0 | AC-DDAE-9-007 | isolated bounded query pipeline | TASK-DDAE-9-007 | `path:internal/querystate/store.go` | TEST-DDAE-9-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-008 | 2.5.0 | AC-DDAE-9-008 | isolated bounded query pipeline | TASK-DDAE-9-008 | `path:internal/queries/pipeline.go` | TEST-DDAE-9-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-9-009 | 2.5.0 | AC-DDAE-9-009 | isolated bounded query pipeline | TASK-DDAE-9-009 | `path:docs/query-monitoring.md` | TEST-DDAE-9-009 | `test` | `stage:test` | Implemented |

## DDAE-10 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-10-001 | 2.6.0 | AC-DDAE-10-001 | bounded durable time windows | TASK-DDAE-10-001 | `path:internal/config/backfill.go` | TEST-DDAE-10-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-002 | 2.6.0 | AC-DDAE-10-002 | bounded durable time windows | TASK-DDAE-10-002 | `path:internal/historyscan/adapters.go` | TEST-DDAE-10-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-003 | 2.6.0 | AC-DDAE-10-003 | bounded durable time windows | TASK-DDAE-10-003 | `path:internal/historyscan/scan.go` | TEST-DDAE-10-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-004 | 2.6.0 | AC-DDAE-10-004 | bounded durable time windows | TASK-DDAE-10-004 | `path:internal/historystate/store.go` | TEST-DDAE-10-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-005 | 2.6.0 | AC-DDAE-10-005 | bounded durable time windows | TASK-DDAE-10-005 | `path:internal/historyscan/adapters.go` | TEST-DDAE-10-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-006 | 2.6.0 | AC-DDAE-10-006 | bounded durable time windows | TASK-DDAE-10-006 | `path:internal/historyscan/scan.go` | TEST-DDAE-10-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-007 | 2.6.0 | AC-DDAE-10-007 | bounded durable time windows | TASK-DDAE-10-007 | `path:internal/historyscan/scan.go` | TEST-DDAE-10-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-008 | 2.6.0 | AC-DDAE-10-008 | bounded durable time windows | TASK-DDAE-10-004, TASK-DDAE-10-008 | `path:internal/historyscan/scan.go` | TEST-DDAE-10-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-10-009 | 2.6.0 | AC-DDAE-10-009 | bounded durable time windows | TASK-DDAE-10-009 | `path:internal/config/yaml.go` | TEST-DDAE-10-009 | `test` | `stage:test` | Implemented |

## DDAE-11 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-11-001 | 2.7.0 | AC-DDAE-11-001 | Include query events in the existing SASL loader predicate | TASK-DDAE-11-001, TASK-DDAE-11-004 | `path:internal/config/config.go` | TEST-DDAE-11-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-11-002 | 2.7.0 | AC-DDAE-11-002 | Include query events in the existing SASL loader predicate | TASK-DDAE-11-002, TASK-DDAE-11-004 | `path:internal/config/config.go` | TEST-DDAE-11-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-11-003 | 2.7.0 | AC-DDAE-11-003 | Include query events in the existing SASL loader predicate | TASK-DDAE-11-003, TASK-DDAE-11-004 | `path:internal/config/config.go` | TEST-DDAE-11-003 | `test` | `stage:test` | Implemented |

Inherited trace rows remain unchanged. Local test success alone does not close
full handoff or independent-review gates.


## DDAE-12 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-12-001 | 2.8.0 | AC-DDAE-12-001 | Candidate-scoped data-preserving correction | TASK-DDAE-12-001 | `path:internal/outbox/store.go` | TEST-DDAE-12-001 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-002 | 2.8.0 | AC-DDAE-12-002 | Candidate-scoped data-preserving correction | TASK-DDAE-12-002 | `path:internal/alerts/pipeline.go` | TEST-DDAE-12-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-003 | 2.8.0 | AC-DDAE-12-003 | Candidate-scoped data-preserving correction | TASK-DDAE-12-003 | `path:internal/snapshot/store.go` | TEST-DDAE-12-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-004 | 2.8.0 | AC-DDAE-12-004 | Candidate-scoped data-preserving correction | TASK-DDAE-12-004 | `path:internal/ddae/types.go` | TEST-DDAE-12-004 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-005 | 2.8.0 | AC-DDAE-12-005 | Candidate-scoped data-preserving correction | TASK-DDAE-12-005 | `path:internal/querystate/store.go` | TEST-DDAE-12-005 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-006 | 2.8.0 | AC-DDAE-12-006 | Candidate-scoped data-preserving correction | TASK-DDAE-12-006 | `path:internal/queryclient/types.go` | TEST-DDAE-12-006 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-007 | 2.8.0 | AC-DDAE-12-007 | Candidate-scoped data-preserving correction | TASK-DDAE-12-007 | `path:internal/outbox/store.go` | TEST-DDAE-12-007 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-008 | 2.8.0 | AC-DDAE-12-008 | Candidate-scoped data-preserving correction | TASK-DDAE-12-008 | `path:internal/config/config.go` | TEST-DDAE-12-008 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-009 | 2.8.0 | AC-DDAE-12-009 | Candidate-scoped data-preserving correction | TASK-DDAE-12-009 | `path:internal/app/app.go` | TEST-DDAE-12-009 | `test` | `stage:test` | Implemented |
| REQ-DDAE-12-010 | 2.8.0 | AC-DDAE-12-010 | Candidate-scoped data-preserving correction | TASK-DDAE-12-010 | `path:docs/runbook.md` | TEST-DDAE-12-010 | `test` | `stage:test` | Implemented |

These rows govern the affected behavior and are Implemented with local regression evidence
recorded in plans/DDAE-12.md. Full handoff and independent review remain open; inherited
records retain historical status and are not claimed as newly Verified. Superseded descriptions
are explicit in DDAE-12; all valuable earlier regression tests remain required.
