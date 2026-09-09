# Code Review Contract

Review the requested change and its final diff; do not expand implementation
scope merely to satisfy reviewer preferences.

## Review Priorities

Review in this order:

1. Correctness and user-visible regressions.
2. Security, privacy, authorization, secret handling, and the guarantee that
   DDAE access is read-only.
3. Prometheus contracts: stable metric names/help/type/unit, label meaning and
   cardinality, duplicate series, and counter/gauge semantics.
4. Kafka alert contracts: typed field allowlist, schema/version/key, lifecycle,
   idempotency, delivery/replay, bounded buffering, and payload privacy.
5. Scrape and publish reliability: timeout budgets, cancellation, bounded concurrency,
   retry/backoff, partial failure, stale data, and honest exporter health.
6. Public configuration, DDAE-version compatibility, migration, deployment,
   and operational recovery.
7. Missing or misleading tests and verification evidence.
8. SDD consistency across requirement, acceptance, task, test,
   implementation, stage, evidence, and trace status.
9. Maintainability issues likely to cause a defect.

Style-only preferences are findings only when they violate an explicit project
contract or materially reduce clarity.

## Project-Specific Safety Checks

- Reject any DDAE mutation, excessive privilege, credential exposure, secret in
  a metric/label, or real production credential/data in tests or fixtures.
- Reject unbounded labels such as timestamps, free-form error text, request IDs,
  event IDs, or raw object identifiers without an approved cardinality bound.
- Check conversions, units, missing values, resets, overflow, timestamp
  handling, aggregation, and naming against the approved metric mapping.
- Confirm one slow or failed DDAE call cannot leak workers or hold a Prometheus
  scrape past the documented timeout budget.
- Confirm partial collection failures are observable and do not silently serve
  stale values as a successful current observation.
- Reject raw DDAE response forwarding, unapproved alert fields, Kafka secrets or
  payloads in logs, alert content in Prometheus, unbounded producer buffers, and
  any path that silently drops an alert event.
- Confirm every alert-detail ID originates from the validated list, is encoded
  as one path segment, cannot select another endpoint, and matches the returned
  detail ID.
- Confirm detail fan-out deduplicates IDs and enforces per-cycle count,
  concurrency, response-size and aggregate-deadline limits; a detail failure
  must not become an incomplete successful Kafka event.
- Confirm Kafka failure cannot exhaust resources or falsify successfully
  collected Prometheus status/capacity/configuration metrics, while producer
  failure and backlog remain observable.
- Treat mocks as unit evidence only. Integration claims must use an authorized
  non-production DDAE boundary with recorded version and an isolated Kafka test
  broker/topic with documented prerequisites.

## Evidence

- Inspect the request, applicable instructions, active specification and plan,
  changed code, adjacent contracts, metric mapping, and relevant tests.
- Verify outcomes rather than relying on an agent claim or transcript.
- Do not report a command as passing unless its successful result is available.
- Under required SDD, compare specification, approved plan-basis, test-plan,
  and traceability digests with the report. Reject a `Verified` trace row whose
  implementation, executable test, current-run stage, or evidence is absent or
  misleading.
- For a requirement amendment, verify the Change ID decision, specification
  version, append-only changelog, preserved or formally retired regression
  coverage, reset trace rows, and renewed approvals for governed byte changes.

## Findings

Each finding must include severity (`critical`, `high`, `medium`, or `low`), a
concise defect and impact, precise file and line, the triggering input/state,
and an actionable correction when it is not obvious. Prefer a small number of
defensible findings. If none are found, say so and identify verification limits
or residual risk.

## Independence

Changes to DDAE authentication, secret handling, remote access, metric
identity/cardinality, Kafka event schema/delivery, or release/deployment
behavior require a reviewer who did not implement the change. Record the real
reviewer and evidence; never infer or fabricate approval.
