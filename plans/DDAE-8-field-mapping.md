# DDAE-8 follow-up candidate: cluster field mappings

Status: approved by the user and implemented on 2026-09-09. The original
reviewed candidate is retained in plans/history/DDAE-8-field-mapping-candidate.md.
The following proposal text is preserved as the approved design rationale. This continues the same unfinished
cluster collection outcome and requires approval because the original DDAE-8
basis explicitly preserved field types and excluded aliases/conversions.

## Observed difference

After the envelope repair passed synthetic tests, two authorized live cycles
still failed decoding. A value-free inspection of the known fields showed:

- `clusterStatus` is an object with a string `status` member.
- `coordinator.resources.cpu` and `worker.resources.cpu` are JSON integers.
- Both `resources.memory` members are strings.
- The old direct coordinator/worker CPU and memory members are absent.

No endpoint, credentials, real identifiers or raw response values are retained.
The existing envelope fix remains implemented; no field mapping is changed yet.

## Recommended requirements and acceptance

- REQ-DDAE-8-004 / AC-DDAE-8-004: accept the legacy string clusterStatus and an
  object with a required non-null string status. Map that string through the
  unchanged status normalizer; ignore message/reason and other metadata. Reject
  malformed object status rather than treating it as a successful unknown state.
  Preserve existing absent legacy status behavior.
- REQ-DDAE-8-005 / AC-DDAE-8-005: accept legacy direct CPU/memory and the observed
  nested resources object for coordinator/worker. Decode CPU quantity strings
  and non-negative base-10 JSON integers into the existing quantity-string DTO;
  keep existing CPU/memory quantity validation and metric units. Reject other
  numeric forms/types and malformed/null resources objects. Missing optional
  quantities remain omitted rather than being fabricated as zero.
- REQ-DDAE-8-006 / AC-DDAE-8-006: reject simultaneous direct and nested resource
  representations for the same role, avoiding silent precedence and conflicting
  values. Preserve all existing route/TLS/security/metrics/freshness contracts.

## Implementation and tests

Add typed Cluster/ResourceConfig unmarshalling in internal/ddae/types.go, using
existing quantity-decoding patterns; preserve the public DTO and normalizers.
Add synthetic cases to cluster_response_test.go in ddae and collector and to
portable/parser_test.go for both complete layouts, invalid types, ambiguity,
missing quantities, bounds and production/replay parity. Check exact normalized
CPU/memory values and unchanged metric names/units. Use synthetic fixtures only.

Append the approved requirements, tests and tasks to the canonical DDAE-8 SDD
records as an amended specification version before implementing this candidate;
retain prior requirement history and reset impacted trace rows. Recalculate
identities and record the actual approval. Do not retroactively claim that this
new field behavior was covered by the first candidate's approval.

Rebuild the normal exporter and repeat two authorized resource-only demo cycles.
Check all resource collector success values, expected cluster metric families
and health/readiness; report any remaining failure without forcing readiness.
Diagnostic TLS bypass remains explicitly limited functional evidence. Run the
same local and Harness checks and retain environment failures transparently.
No TLS remediation, Windows diagnostic fixes, Kafka, packaging or release work.
Rollback is the previous binary; no stored-state migration is involved.

## Alternatives and recommendation

Keep only the envelope change: preserves the first approved scope, but this
appliance still cannot provide cluster metrics. Supporting the observed typed
fields is recommended because it addresses the newly demonstrated mismatch.
Making unknown JSON permissive or discarding status/resource fields could hide
incorrect or absent metrics and is not recommended.

## Outcome

The two authorized resource-only live cycles passed after the field mappings:
all five collectors succeeded, ddae_up=1, readyz=200 and all four configured
CPU/memory metric families were present. Diagnostic TLS bypass was explicit.
No supported release or complete Harness pass is claimed; detailed evidence
and remaining environment limitations are recorded in plans/DDAE-8.md.
