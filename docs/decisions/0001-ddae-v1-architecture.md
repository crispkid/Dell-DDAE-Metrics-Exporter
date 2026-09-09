# ADR 0001: DDAE v1 exporter architecture

- Status: Accepted through the approved DDAE-1 Plan Basis
- Change: DDAE-1
- Governing sources: `SPECIFICATION.md` v1.0.0 and the marker-bounded approved
  Plan Basis in `plans/DDAE-1.md`
- Date recorded: 2026-08-25

## Context

DDAE v1 needs bounded Prometheus collection and confidential serviceability
alert delivery without turning the exporter into a generic Management API
proxy. DDAE, Prometheus, Kafka and persistent-state failures have different
freshness and recovery semantics.

## Decision

Use one Go process per DDAE target. A compiled GET-only DDAE client and shared
in-memory token cache feed non-overlapping background collectors. Prometheus
scrapes immutable typed snapshots and never fan out to DDAE. A separate bounded
alert list/detail pipeline constructs schema-1.0 typed events, persists them to
a protected bbolt outbox and publishes to Kafka at least once with a stable
record key. Kubernetes and VM/systemd profiles both preserve one-writer state,
runtime secret injection, verified TLS and an external mTLS metrics boundary.

## Consequences

- DDAE load and scrape latency are bounded independently.
- Partial and stale metric families remain explicit.
- Kafka outages do not falsify DDAE metric state, while outbox capacity remains
  part of readiness.
- The bbolt state volume permits only one active exporter instance; Kubernetes
  uses `Recreate` and VM upgrades must stop the prior process first.
- No endpoint, metric or Kafka field may be added through runtime configuration.
- Release remains blocked until authorized integration/E2E evidence and
  independent review close the governed trace rows.
