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

The normalized cluster DTO fields/types, metric names/help/types/units/labels,
cardinality, API allowlist, authentication, TLS defaults, collection deadlines,
retry policy and freshness rules are inherited without amendment. No additional
API/product version is certified by this structural compatibility fix.

Rejecting top-level JSON null explicitly avoids treating absent data as a
successful empty inventory. `[]` and `{"results":[]}` remain valid empty lists.
Missing/duplicate IDs and invalid optional quantities retain the existing
normalizer behavior. The approved follow-up adds the typed input mappings
below; no unrelated aliases or conversions are added.

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

### Approved field-mapping amendment (2.4.0)

The user replied "yes, keep going" to the candidate in
plans/DDAE-8-field-mapping.md, reviewed full-file SHA-256
ac612ed932a340e9d3b23c26fc623d9937a56ebe0f484f51f30b0e3d439f3614.
This extends the same unfinished outcome; historical 2.3.0 artifacts are
retained in plans/history/DDAE-8-v2.3.0.md. Input field mappings below supersede
only the earlier prohibition on new field aliases/conversions.

Implement typed Cluster/ResourceConfig JSON decoding near the existing DTOs.
Read clusterStatus.status from the object form, retaining the legacy string and
absent behavior. Decode nested role.resources or legacy direct quantities,
using existing integer/quantity-string handling and preserving optional values.
Reject simultaneous direct/nested fields and malformed resources/status instead
of choosing silent precedence. Keep public DTOs, normalizers and metric contracts.

| Task | Requirements | Work and planned evidence |
|---|---|---|
| TASK-DDAE-8-006 | REQ-DDAE-8-004 | Typed status decoding and synthetic string/object/invalid-status client, replay and normalization tests; TEST-DDAE-8-005 under test. |
| TASK-DDAE-8-007 | REQ-DDAE-8-005 | Typed resource decoding, integer/string CPU, optional and malformed values; exact normalized CPU/memory and metric parity tests; TEST-DDAE-8-006 under test. |
| TASK-DDAE-8-008 | REQ-DDAE-8-006 | Reject mixed resource layouts; retain ignored-metadata/privacy and parser parity tests; TEST-DDAE-8-007 under test; update adjacent docs and repeat bounded live diagnostics. |

Retain previous boundary and legacy tests except the old numeric-CPU rejection,
which is deliberately replaced with integer acceptance plus fractional,
exponent, negative and invalid-type rejection. Update synthetic tests in the
existing client, collector and portable parser files. Repeat existing local and
Harness checks; environment failures remain visible and are not waived. Repeat
two authorized resource-only live cycles with explicit diagnostic TLS bypass,
checking resource health and actual cluster metric families. No state migration,
TLS remediation, Windows diagnostic fix, Kafka, packaging or release is added.

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

## Follow-up approval (2026-09-09)

The current repository user explicitly replied "yes, keep going" to the field-mapping candidate. The original envelope-only pending/outcome notes above are historical. Requirements 004-006 and their associated implementation/test scope are now approved. No personal name is inferred.

## Final field-mapping outcome (2026-09-09)

- Implemented object status, nested resource and integer CPU input mappings,
  preserving public DTOs/metrics, legacy strings/direct fields and optional null
  values. Mixed layouts and invalid status/resource forms fail explicitly.
- Added synthetic client/replay, integer precision/optional-value, receiver
  atomicity, normalized metric and diagnostic parity tests. The previous
  numeric-CPU rejection was deliberately replaced by accepted integer cases
  plus negative/fraction/exponent/type rejections under the approved amendment.
- Pre-implementation field tests failed as expected; final targeted client,
  collector and portable parser tests pass. Full internal/ddae, collector and
  metrics package results pass in the full-suite run. go vet ./... and the
  normal Windows exporter build pass. Changed Go files are gofmt-clean and
  git diff --check passes. No unrelated product code was altered.
- Two authorized demo cycles now report all five collector success values=1,
  ddae_management_api_up=1, ddae_up=1, healthz/readyz/metrics HTTP 200. Cluster
  coordinator/worker CPU and memory families are present in both cycles.
  Raw identifiers/values were not retained in the summary. Evidence:
  output/ddae-live-test/ddae8-fields-live-status.txt and ddae8-fields-live.log.
  The temporary exporter was stopped; credentials remained process-local.
- These live checks explicitly bypassed TLS verification; CA trust remains
  unresolved. Kafka, alert/event end-to-end, packaging and supported-release
  certification remain outside this change. The portable bundle was not rebuilt.
- Full go test ./... fails in the same Windows STATE_DIR/configuration,
  file-permission and deployment-newline areas observed before this amendment.
  Evidence: output/ddae-live-test/ddae8-fields-all-tests.txt. These failures
  are not waived. Race execution remains unavailable without cgo/compiler.
- Required Harness verify remains failed at pre-existing checkout formatting
  outside this parser change; later required stages and same-run SDD closure
  therefore remain unexecuted. Evidence:
  output/ddae-live-test/ddae8-fields-verify.txt. All trace rows are Implemented,
  not Verified. No global test success, release readiness or commit is claimed.
- Final review checked read-only API behavior, typed metadata exclusion,
  quantity precision, public return type preservation, bounds, null/mixed
  inputs, replay parity and unchanged freshness/health semantics. The known
  validation limitations above remain visible instead of being masked.
