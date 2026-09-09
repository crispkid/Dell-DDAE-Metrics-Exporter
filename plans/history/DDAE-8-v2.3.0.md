# DDAE-8 v2.3.0 historical basis

Superseded by the approved field-mapping amendment. No historical acceptance is claimed.


## Archived SPECIFICATION.md

## DDAE-8 Cluster response compatibility

### Specification Metadata

- Specification Version: 2.3.0
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

### Acceptance Criteria

| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-DDAE-8-001 | REQ-DDAE-8-001 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | synthetic nonempty and empty arrays/envelopes produce equivalent typed data and cluster metrics. |
| AC-DDAE-8-002 | REQ-DDAE-8-002 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | invalid-shape, malformed-item, trailing/oversized-body, duplicate/missing-ID and missing/invalid optional-resource cases remain failures at their appropriate decode/validation layer; failures do not become successful empty inventory. |
| AC-DDAE-8-003 | REQ-DDAE-8-003 | Synthetic valid and invalid cluster responses | Production client, collection and recorded parsing run | client/recorded/portable results agree for equivalent valid and invalid fixtures; synthetic self-test expectations match the new contract. |

### Interfaces, Data, and Failure Behavior

Preserve existing cluster DTO fields, public return types, metrics, API
allowlist, authentication, deadlines/retries, body/nesting limits and freshness.
Accept [] and {"results":[]} as empty lists; reject absent/null inventory.
Preserve identity and optional-resource normalization; add no aliases or types.
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


## Archived TEST_PLAN.md

## DDAE-8 Test Design

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-DDAE-8-001 | AC-DDAE-8-001 | REQ-DDAE-8-001 | `path:internal/ddae/cluster_response_test.go` | component | synthetic TLS arrays/envelopes, empty/populated | envelope fails existing decoder | `test` |
| TEST-DDAE-8-002 | AC-DDAE-8-002 | REQ-DDAE-8-002 | `path:internal/ddae/cluster_response_test.go` | boundary | missing/null/wrong types, invalid items, malformed/trailing/oversized JSON | null can be accepted; envelope unsupported | `test` |
| TEST-DDAE-8-003 | AC-DDAE-8-001 | REQ-DDAE-8-001 | `path:internal/collector/cluster_response_test.go` | component | equivalent values and invalid identity/optional resources | envelope cannot reach normalization | `test` |
| TEST-DDAE-8-004 | AC-DDAE-8-003 | REQ-DDAE-8-003 | `path:internal/portable/parser_test.go` | component | production recorded-body parity and synthetic self-test expectations | portable contract expects envelope failure | `test` |

### Failure and Boundary Coverage

Use the approved matrix in plans/DDAE-8.md: populated/empty arrays and envelopes,
whitespace/unknown metadata, invalid top levels/results/items, malformed and
trailing JSON, response bounds, identities, and optional resource errors.
Compare exact normalized cluster and metric values, and retain honest failure.
Run targeted tests, repository race tests, lint, coverage, build and full verify.
Retain failed/unavailable stages explicitly; no mock proves real integration.
Two authorized live resource cycles are supplemental diagnostic evidence only.


## Archived TRACEABILITY.md

## DDAE-8 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-DDAE-8-001 | 2.3.0 | AC-DDAE-8-001 | shared cluster list decoder | TASK-DDAE-8-001, TASK-DDAE-8-002, TASK-DDAE-8-003, TASK-DDAE-8-005 | `path:internal/ddae/types.go` | TEST-DDAE-8-001, TEST-DDAE-8-003 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-002 | 2.3.0 | AC-DDAE-8-002 | strict bounded shape and existing semantics | TASK-DDAE-8-001, TASK-DDAE-8-002, TASK-DDAE-8-003, TASK-DDAE-8-005 | `path:internal/ddae/types.go` | TEST-DDAE-8-002 | `test` | `stage:test` | Implemented |
| REQ-DDAE-8-003 | 2.3.0 | AC-DDAE-8-003 | recorded parser reuses live client | TASK-DDAE-8-001, TASK-DDAE-8-004, TASK-DDAE-8-005 | `path:internal/portable/parser.go` | TEST-DDAE-8-004 | `test` | `stage:test` | Implemented |

## Archived DEVELOPMENT_PLAN.md

## DDAE-8 Cluster response compatibility

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Current repository user in this Codex conversation
- Requirements Confirmer Role: Requesting user / Requirements Approver
- Requirements Confirmed On: 2026-09-09
- Requirements Confirmation Evidence: The user replied "OK" to the explicit requirements, plan and acceptance-test approval request for plans/DDAE-8.md, reviewed full-file SHA-256 d7ecdc502a366efd98b449ffa7d499a4f90014a7b2f7365d3964041e29a327e6. No personal name is inferred.
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Current repository user in this Codex conversation
- Implementation Plan Approver Role: Requesting user / Implementation Plan Approver
- Implementation Plan Approved On: 2026-09-09
- Implementation Plan Approval Evidence: The same explicit "OK" approves the linked plan and test scope. Canonical tables/headings are an administrative transcription of that reviewed candidate; no new product behavior was added. Canonical digests below are computed identities, not a claim that the user typed those digests.
- Approved Specification Version: 2.3.0
- Approved Specification Digest: sha256:a771310ed12558b1ad6de6774b24a49f2ba4e07d0ac715456a511b5422a2fa36
- Approved Plan Basis Digest: sha256:cfabb71cda71dd143909bba3dcf2b1339d72e5086d75de1f24cce639aee7d422
- Approved Test Plan Digest: sha256:1c384314d9aa563833a6f36759e45955224b783ca32693b2d6b5c99d8ac0ca5a

### Execution Plan Reference

- Execution Plan: plans/DDAE-8.md

## Archived plans/DDAE-8.md

# Execution Plan: DDAE-8 cluster response compatibility

## Purpose and Observable Outcome

Fix the cluster response parser so the exporter can consume a non-null
`results` array in an object as well as the existing bare array. The live
resource collector and portable diagnostic/replay parser must agree.

Status: candidate for requirements and implementation-plan approval. No
implementation or approval is claimed. Proposed specification version: 2.3.0.
The user requested "fix parser first" on 2026-09-09 after reviewing the live
failure. This records the requested outcome, not approval of this new basis.

## Context and Constraints

- The authorized demo test returned HTTP 200 with a `results` object for
  clusters. Two collection cycles reported cluster decode failures while
  ping, nodes, lock and power succeeded. No payload or endpoint is retained here.
- `Client.Clusters` currently decodes directly into `[]Cluster`.
- DDAE-7 explicitly excluded this fix. Its approved basis and historical
  evidence remain unchanged; DDAE-8 is a separately governed outcome.
- The portable parser explicitly requires an array, and its parity test and
  synthetic self-test currently expect the envelope to fail.
- Keep changes limited to cluster decoding, corresponding diagnostics,
  regression tests, adjacent documentation and required governance records.
- CA trust, routes/defaults, Windows STATE_DIR, alert completeness, Kafka,
  portable packaging and release work are excluded.
- No new dependency, real payload fixture, secret, commit or deployment.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

This marker-bounded basis is a candidate until explicitly approved.

### Requirement Coverage

| Requirement | Normative behavior | Acceptance criteria |
|---|---|---|
| REQ-DDAE-8-001 | The cluster decoder MUST accept bare arrays and objects containing a non-null array-valued `results`. Both forms MUST yield the same typed clusters and existing normalization/metrics for equivalent content. Unknown envelope fields MUST NOT enter metrics or logs. | AC-DDAE-8-001: synthetic nonempty and empty arrays/envelopes produce equivalent typed data and cluster metrics. |
| REQ-DDAE-8-002 | Missing/null/non-array `results`, unsupported top levels, null/non-object items, malformed JSON and trailing JSON MUST fail. Existing response-size, nesting, request, retry, identity, optional-resource validation and stale/partial-failure limits MUST remain effective. | AC-DDAE-8-002: invalid-shape, malformed-item, trailing/oversized-body, duplicate/missing-ID and missing/invalid optional-resource cases remain failures at their appropriate decode/validation layer; failures do not become successful empty inventory. |
| REQ-DDAE-8-003 | Live collection, recorded-body decoding and portable diagnostics/replay MUST use the same cluster decoder. A valid envelope MUST no longer fail solely because its top level is an object. | AC-DDAE-8-003: client/recorded/portable results agree for equivalent valid and invalid fixtures; synthetic self-test expectations match the new contract. |

The existing cluster field names/types, metric names/help/types/units/labels,
cardinality, API allowlist, authentication, TLS defaults, collection deadlines,
retry policy and freshness rules are inherited without amendment. No additional
API/product version is certified by this structural compatibility fix.

Rejecting top-level JSON null explicitly avoids treating absent data as a
successful empty inventory. `[]` and `{"results":[]}` remain valid empty lists.
Missing/duplicate IDs and invalid optional quantities retain the existing
normalizer behavior rather than gaining new field aliases or conversions.

### Technical Design

Add a small internal cluster-list JSON decoder near the existing node-list
decoder in `internal/ddae/types.go`. Select the bare array or `results` array,
decode into the existing `Cluster` DTO, reject non-object items, and assign the
result only after successful decoding. Reuse established bounded decoding;
do not introduce generic network fallback, pagination or a second permissive
diagnostic parser.

Use this type in `Client.Clusters` and the cluster branch of `DecodeRecorded`,
returning `[]Cluster` to preserve callers and existing normalizer dispatch.
Remove/update the portable array-only contract check after production decode
has accepted the envelope. Keep decode, validation and contract outcomes
distinct. Update only the cluster case in synthetic self-test data.

### Task Breakdown

| Task | Requirements | Work and planned evidence |
|---|---|---|
| TASK-DDAE-8-001 | REQ-DDAE-8-001, REQ-DDAE-8-002, REQ-DDAE-8-003 | Publish this exact candidate specification/basis/test matrix into the canonical SDD artifacts, append the specification changelog, add Planned trace rows and record actual approvals using the Harness digest workflow before implementation. Preserve DDAE-7 history. |
| TASK-DDAE-8-002 | REQ-DDAE-8-001, REQ-DDAE-8-002 | Add synthetic client response tests in `internal/ddae/cluster_response_test.go`; demonstrate envelope failure before implementation. TEST-DDAE-8-001/002 cover AC-001/002 under Harness `test`. |
| TASK-DDAE-8-003 | REQ-DDAE-8-001, REQ-DDAE-8-002 | Implement the shared decoder and retain public return types and existing limits. Add collector/metric regression coverage in `internal/collector/cluster_response_test.go` for equivalent values and honest failure behavior. TEST-DDAE-8-003 covers AC-001/002 under `test`. |
| TASK-DDAE-8-004 | REQ-DDAE-8-003 | Update recorded decoding, portable contract/parity tests and synthetic self-test expectations. TEST-DDAE-8-004 covers AC-003 under `test`, including both accepted shapes and negative cases. |
| TASK-DDAE-8-005 | REQ-DDAE-8-001, REQ-DDAE-8-002, REQ-DDAE-8-003 | Update the portable README's obsolete cluster-envelope limitation, review the final diff, execute checks and record actual outcomes/limitations. |

### Verification Basis

Tests use synthetic fixtures only. The test matrix includes populated and empty
lists, whitespace/unknown metadata, missing/null/wrong-type results, scalar/null
top levels and items, malformed/trailing JSON, bounded-body failure, identity
validation and optional CPU/memory errors. Matching valid formats must produce
identical cluster IDs/status and resource quantities, not merely HTTP success.

Run the narrow Go package tests first, then repository `test` (race), lint,
coverage and build stages and the declared `./HARNESS/harness.sh verify` gate.
If Bash, a race compiler or required external stages are unavailable, report
the exact failed/unavailable checks; direct Go checks do not substitute for a
successful Harness handoff. No trace row becomes Verified without its required
current-run evidence and approved digests.

Repeat the previously authorized bounded, resource-only live exporter test
after rebuilding. Keep credentials runtime-only, disable Kafka, stop the test
process afterwards, and retain only sanitized counts/status. Check cluster
collector success and actual configured-resource metrics across two cycles.
Do not force overall readiness if a different field/collector still fails.
The current demo requires explicit diagnostic TLS bypass; such a run is
functional evidence only, not trusted-TLS or software-version certification.
CA remediation remains separate and no real integration version is invented.

### Compatibility, Recovery and Idempotence

Preserve valid legacy arrays, DTO/public metric contracts and all unrelated
collectors. This deliberately expands the accepted cluster envelope and
rejects null as invalid data. There is no state migration. Recovery is to
restore the prior binary; never reset unrelated working-tree changes.
Build outputs and safe evidence stay in ignored directories. Do not rebuild
or overwrite the portable distribution as part of this parser-only change.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- 2026-09-09: inspected production client, node decoder, portable contract,
  existing parity/self-test expectations, project policy and Harness stages.
- Candidate prepared; canonical SDD activation/digests and implementation
  remain pending approval. No product files or tests changed in this step.

## Surprises and Discoveries

Updating only the live client would leave recorded-body replay inconsistent
and the portable contract rejecting successful envelope decoding. Both paths
and the historical synthetic expectation must change together.

## Decision Log

- Recommended to the Requirements Approver: use a new DDAE-8 amendment because
  DDAE-7 explicitly excludes this independent outcome. Reusing DDAE-7 would
  alter its previously approved portable scope/history.
- Recommended: share the established typed decoder pattern instead of adding
  an intermediary proxy or modifying the DDAE response. This fixes the observed
  incompatibility in the exporter while preserving the configured API routes.
- Approval of this candidate is not yet recorded.

## Outcomes and Retrospective

Pending implementation, verification and review. Current live observations
establish the existing failure only; they are not evidence of a completed fix.

## Approval and canonical transcription

2026-09-09: the current repository user replied "OK" to the explicit request
to approve this candidate's requirements, implementation plan and acceptance
tests. The reviewed full-file SHA-256 was
`d7ecdc502a366efd98b449ffa7d499a4f90014a7b2f7365d3964041e29a327e6`.
This records the actual conversation actor; no personal name is inferred.
The canonical specification/test tables and Harness headings/full requirement
IDs transcribe that approved content without adding product behavior. Earlier
candidate/pending statements describe the document as submitted for review.
DecodeRecorded already dispatches through Client.Clusters, so updating the
shared client updates replay automatically; no separate decoder is needed.

## Implementation and verification update (2026-09-09)

- The approved envelope decoder, live/replay parity, synthetic self-test
  expectation and adjacent portable documentation are implemented. The public
  Clusters return type remains []Cluster. No external payloads are fixtures.
- Pre-implementation tests demonstrated envelope/metadata/empty-envelope failure
  and erroneous acceptance of null inventory/items. Evidence:
  output/ddae-live-test/ddae8-before.txt.
- Full internal/ddae, internal/collector and internal/metrics packages pass.
  Portable production/cluster parser tests pass. go vet ./... and the rebuilt
  Windows exporter pass. Changed Go files are gofmt-clean.
- Full Go tests fail in Windows path defaults, permission assumptions, portable
  configuration and deployment newline expectations. The race run cannot start
  because the host lacks cgo configuration/compiler. These failures are not
  waived, and no full-suite/required handoff success is claimed.
- The first full-test attempt accidentally included the downloaded Go toolchain
  below output. output/go.mod now isolates generated artifacts from package
  discovery; the clean rerun still reports the product/environment failures
  above. Evidence: output/ddae-live-test/ddae8-all-tests-clean.txt.
- Harness verify failed at lint on existing unformatted checkout files outside
  this parser change. No unrelated source formatting was performed. Required
  later stages and same-run SDD closure were not completed.
- Two rebuilt-exporter demo cycles still fail cluster decoding. A follow-up
  type-only inspection found object clusterStatus.status, nested resources
  and integer CPUs, outside the first approved field contract. Ping, nodes,
  lock and power pass. ddae_up=0 and readyz=503 remain honest. Live status:
  output/ddae-live-test/ddae8-live-status.txt. TLS bypass was explicit; no
  trusted-TLS or supported-version certification is claimed.
- Current rows remain Implemented, not Verified. The test process stopped and
  no credentials were written by the live retest.
- Follow-up recommendation and exact candidate are in
  plans/DDAE-8-field-mapping.md; field mapping implementation awaits approval.
