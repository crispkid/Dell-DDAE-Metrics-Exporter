# Execution Plan: DDAE-6 documented and observed payload compatibility

This living plan governs the backward-compatible DDAE 1.5.0 response-decoding
correction. Keep progress and observed results outside the digest-bound Plan
Basis block.

## Purpose

Make node collection accept the Dell-documented and field-observed response
shape, preserve the existing legacy representation, normalize Informational
severity correctly, and retain honest serviceability list/detail and
incomplete-list behavior without inventing API contracts.

## Context and Constraints

- The current node client expects a bare array and cannot decode the supplied
  documented `results` envelope or its numeric CPU values.
- The supplied serviceability lists decode, but no `/{id}` detail responses or
  supported pagination parameters were supplied.
- Raw field payloads contain confidential operational data and must not be
  copied into the repository or retained evidence.
- DDAE-5 path-prefix behavior, all public metrics, Kafka contracts, durable
  state and deployment settings remain unchanged.
- Product code and test fixtures must not change until Peter Chu confirms the
  exact Specification digest and approves the exact Plan Basis and Test Plan
  digests.
- Local deterministic implementation/testing is proposed. Authenticated DDAE,
  Kafka/OpenSearch, deployment and inherited release gates remain separate.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

- TASK-DDAE-6-001 through TASK-DDAE-6-005 cover the locally implementable
  parser, canonical-model, severity, serviceability-regression, fixture,
  security and documentation work for REQ-DDAE-6-001 through REQ-DDAE-6-006.
- TASK-DDAE-6-006 retains the authorized authenticated evidence required by
  REQ-DDAE-6-007. Local fixtures and the supplied list payloads cannot close it.
- DDAE-1 through DDAE-5 remain regression contracts. DDAE-6 changes only
  accepted input shapes and the `Informational` input alias; it does not change
  paths, settings, metrics, event schemas or state.
- `TEST_PLAN.md` defines the exact deterministic and external cases.
  `TRACEABILITY.md` maps each requirement to tasks, tests and scheduled Harness
  stages; every DDAE-6 row begins `Planned`.

### Technical Design

#### Synthetic evidence fixtures first

Create minimal deterministic fixtures under `testdata/ddae-1.5.0/` that model
only the reviewed structures. Use synthetic IDs, quantities, timestamps,
messages and addresses. Record the Dell document and supplied-payload SHA-256
identities in adjacent test comments or documentation, but never copy raw
payload bodies, private addresses, operational names or free-form text.

Retain the current legacy node fixture. Add a documented-envelope fixture with
`results`, numeric CPU, `ephemeralStorage` and object conditions. Add small
serviceability list fixtures that prove detail-shaped list fields are ignored,
`Informational` is normalized, and `totalRecords > len(results)` remains
incomplete. Seed unique canaries in unknown/excluded fields for leakage tests.

#### Node response boundary

Add a private node-list response decoder at the DDAE client/DTO boundary.
Inspect the first non-whitespace token and accept exactly:

- an object containing a non-null `results` array; or
- the implemented legacy bare array.

Reject missing/null `results` in an object, scalar top levels, malformed items
and trailing JSON. Do not add configuration, probing, fallback or a second
request. Return the existing `[]InfrastructureNode` public client result.

Implement bounded custom decoding for node resources. CPU accepts the existing
JSON string or a non-negative decimal JSON integer. Convert integers to
canonical base-10 text for the existing collector. Reject negative, fractional,
exponent, boolean, object, array and out-of-bound forms. Memory remains a
string quantity.

Decode documented `ephemeralStorage` and legacy `ephemeral-storage` into one
field. When both are present, accept only equal values; reject a conflict.
Decode documented `{diskPressure,memoryPressure}` conditions into the existing
internal condition representation. Continue to accept the legacy condition
array. Preserve existing condition-name/status normalization and optional-field
failure behavior. Ignore unknown fields through the existing typed allowlist.

#### Canonical metrics and failure ownership

Keep `collector.normalizeNodes`, snapshot types and metric descriptors as the
canonical output path. Equivalent documented and legacy fixtures must yield
identical snapshots and Prometheus exposition. No extra supplied node field is
added to a snapshot, metric, label or log.

Structural decode errors fail the bounded `nodes` operation. Invalid quantity
or condition content retains the current family validation/partial-success
rules after decoding where applicable. No error includes a raw field value,
response fragment, endpoint or operational identifier.

#### Severity compatibility

Update both alert and Serviceability Log severity normalizers to trim
surrounding whitespace, compare case-insensitively, and map `informational` to
the existing output `info`. Preserve `critical`, `error`, `warning`, `info`,
`normal`, `unknown` and unsupported-to-unknown behavior as currently applicable.
Do not alter event structs, JSON schema version, canonical field order, topic,
key, headers or hash algorithm.

#### Serviceability list/detail and completeness regression

Do not promote extra list fields into detail events. Preserve list identity and
marker extraction, selected `/{id}` GETs, returned-ID validation, detail caps,
fairness, deadlines, response limits, state and publisher behavior.

Add regression coverage for complete, malformed, duplicate and truncated
alert/log lists. When `totalRecords` exceeds unique valid returned IDs, valid
IDs may receive bounded detail refresh, but the list remains incomplete,
readiness remains false and no absence transition is inferred. Recording
servers must assert that no query parameters, pagination, alternate list
requests or list-as-detail fallback occur.

#### Documentation, security and external evidence

Update the README and runbook only with concise response-compatibility facts:
the accepted node shapes, fixed list/detail model, incomplete-list behavior and
the fact that pagination is not implemented without a documented contract.
Do not include raw payload examples.

Extend security policy/tests to detect committed field-payload names/values and
canary leakage where practical, while preserving response bounds and typed
allowlist assertions.

The external integration task uses an explicitly authorized non-production
DDAE 1.5.0 endpoint and least-privilege read-only credentials. It captures only
sanitized structural evidence for nodes, event list/detail and issue
list/detail. It records completeness honestly and sends no undocumented query
or mutation. It cannot be replaced by mocks or the supplied list files.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-6-001 | REQ-DDAE-6-006 | approved DDAE-6 basis | minimal synthetic fixtures under `testdata/ddae-1.5.0/` and provenance notes | TEST-DDAE-6-008 |
| TASK-DDAE-6-002 | REQ-DDAE-6-001, REQ-DDAE-6-002, REQ-DDAE-6-003 | TASK-DDAE-6-001 | `internal/ddae/types.go`, `internal/ddae/client.go` and node collector regression | TEST-DDAE-6-001, TEST-DDAE-6-002, TEST-DDAE-6-003, TEST-DDAE-6-004 |
| TASK-DDAE-6-003 | REQ-DDAE-6-004 | TASK-DDAE-6-001 | `internal/alerts/event.go` and `internal/serviceability/event.go` | TEST-DDAE-6-005 |
| TASK-DDAE-6-004 | REQ-DDAE-6-005 | TASK-DDAE-6-001 | alert/serviceability pipeline regression and recording transports | TEST-DDAE-6-006, TEST-DDAE-6-007 |
| TASK-DDAE-6-005 | REQ-DDAE-6-003, REQ-DDAE-6-006 | TASK-DDAE-6-002 through TASK-DDAE-6-004 | contract/security tests, README and runbook compatibility notes | TEST-DDAE-6-004, TEST-DDAE-6-008 |
| TASK-DDAE-6-006 | REQ-DDAE-6-007 | TASK-DDAE-6-001 through TASK-DDAE-6-005 plus authorized external prerequisites | sanitized authenticated DDAE nodes and list/detail evidence | TEST-DDAE-6-009 |

### Verification Basis

- `test`: deterministic unit/component and contract tests use minimal synthetic
  envelope/array, quantity, alias, condition, severity and list/detail
  fixtures. Recording TLS servers prove exact requests, no pagination/fallback
  and unchanged metrics/events/state behavior.
- `security`: policy and canary tests prove raw operational content and excluded
  fields do not enter committed fixtures, errors, logs, metrics, events or
  retained local evidence; existing body bounds and typed allowlists remain.
- `integration`: an explicitly authorized authenticated run proves node list
  decoding and both serviceability list/detail relationships with sanitized
  structural evidence. This stage remains environment-dependent.
- `lint`, `coverage`, `build` and `ci-policy` remain whole-repository local
  gates. Inherited `e2e`, `supply-chain`, deployment attestation and independent
  review remain supported-release gates.

No supplied payload or pre-implementation audit closes a DDAE-6 trace row.
The local implementation pass may cover TASK-DDAE-6-001 through
TASK-DDAE-6-005 and TEST-DDAE-6-001 through TEST-DDAE-6-008. The external
integration row remains separate.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-09-02: Compared the supplied payloads with the Dell 1.5.0 PDF,
  current specifications, DTOs, collectors and serviceability pipelines.
- [x] 2026-09-02: Reproduced the node decode failure and confirmed the alert/log
  list decoders, incomplete-list behavior and Informational normalization gap.
- [x] 2026-09-02: Peter Chu directed continuation with the recommended formal
  next step; prepared the DDAE-6 approval candidate without product-code edits.
- [x] 2026-09-02: Peter Chu confirmed Specification v2.1.0 digest
  `sha256:1385fb34bc0edb906d36f23600dcf528fa5eb7e2166fe57c80ea40a407a88f4b`
  and approved the exact Plan Basis and Test Plan digests, confirmed the module
  path and authorized the local implementation pass.
- [x] 2026-09-02: Implemented TASK-DDAE-6-001 through TASK-DDAE-6-005.
- [x] 2026-09-02: Race-enabled tests, 81.8% coverage, lint, build, security,
  CI policy and govulncheck passed; the supplied node payload also decoded
  through a non-retained local `Client.Nodes` component test.
- [ ] Run TASK-DDAE-6-006 only in an explicitly authorized non-production
  environment.

## Discoveries

- The PDF and field payload agree on every node field that conflicts with the
  current parser: envelope, CPU type, storage spelling and condition shape.
- The serviceability list payloads contain enough fields to decode as details,
  but the PDF explicitly defines separate detail routes, so structural
  similarity does not authorize bypassing them.
- The events payload is a concrete incomplete-list case. The PDF exposes totals
  but does not identify a safe pagination request contract.
- The PDF summary calls `info` the count of Informational alerts, providing a
  semantic basis for the bounded severity alias without changing output schema.
- DDAE-1 traceability referenced a deleted fixture README; DDAE-6 planning
  corrects the living trace to the existing fixture directory.

## Decisions

- Use new Change ID DDAE-6 because response-schema compatibility is materially
  different from DDAE-5 route-prefix behavior.
- Use Specification v2.1.0 because accepted inputs expand without breaking
  output, configuration or state contracts.
- Support both node shapes at one typed boundary; do not add a schema setting.
- Keep documented detail calls and incomplete-list safety; do not invent
  pagination or list-as-detail fallback.
- Map source `Informational` to existing output `info`.
- Commit only minimal synthetic fixtures, never raw field payloads.

## Recovery and Idempotence

No persistent migration is planned. Parser and severity edits are reversible by
a clean binary rollback. Repeated tests use temporary servers, state and
synthetic fixtures. If implementation reveals a new API behavior, stop, update
the governed specification/plan/test basis, regenerate digests and obtain
renewed approval before continuing.

## Outcomes and Retrospective

- Added one bounded node response decoder supporting the documented `results`
  envelope and legacy array, documented/legacy resource aliases and condition
  shapes, with deterministic rejection of ambiguous or malformed input.
- Preserved existing node snapshots and Prometheus descriptors; equivalent
  documented and legacy structures produce identical node metrics.
- Mapped `Informational` to the existing `info` severity for alert and log
  events without changing their schemas, topics, keys or state.
- Preserved list-to-detail retrieval and incomplete-list safety; duplicate log
  IDs now explicitly make the list incomplete.
- Added synthetic fixtures, English/Traditional Chinese operator guidance and
  security checks that reject copied raw field payload files.
- Authenticated DDAE integration, Kafka/OpenSearch E2E, deployment,
  supply-chain and independent-review gates were not run and remain unresolved.
