# Execution Plans

Use an execution plan for multi-hour or multi-session work. Store each active
plan at `plans/<change-id>.md` and link it from `DEVELOPMENT_PLAN.md` after
strict SDD is enabled.

An execution plan is a self-contained living document. A new contributor must
be able to resume from the plan, repository, and recorded evidence without
missing conversation history. It is the only detailed implementation plan for
a change; `DEVELOPMENT_PLAN.md` contains approval metadata and a link, not a
duplicate task list.

Continue a plan only for the same unfinished observable outcome. Use a new
Change ID for a materially different, closed, released, or independently
approved outcome. If implementation reveals a behavior, design-basis, or
acceptance-test change, record the discovery, pause delivery work, update the
governed source and digests, and obtain renewed approval. Isolated exploration
is not deliverable implementation or acceptance evidence.

## Required Sections

Every plan must contain:

1. Purpose and Observable Outcome.
2. Context and Constraints.
3. A marker-bounded Approved Plan Basis containing requirement coverage,
   technical design, requirement-linked tasks, and verification basis.
4. Progress with dated evidence.
5. Surprises and Discoveries.
6. Decision Log.
7. Recovery and Idempotence.
8. Outcomes and Retrospective.

For required SDD, the bytes from `SDD_APPROVED_PLAN_BASIS_BEGIN` through
`SDD_APPROVED_PLAN_BASIS_END` are approval-locked. Progress, discoveries,
decisions, and outcomes remain outside that block. Any edit to the approved
basis requires a new digest and renewed plan approval.

Each material uncertainty must record the issue and evidence, viable options,
their consequences, the recommended option and rationale, and the person or
role authorized to decide. Record the actual decision separately; a
recommendation is not approval.

## Dell DDAE Exporter Plan Requirements

Before product implementation, the plan must make these boundaries explicit:

- supported DDAE product and API versions, endpoints, authentication, required
  privileges, pagination/rate limits, and read-only guarantees;
- a metric mapping for every collector field: source, Prometheus name, help,
  type, unit, labels, missing-value behavior, and error behavior;
- a label-cardinality budget and rejection of timestamps, free-form messages,
  request/event IDs, or other unbounded label values;
- scrape timeout, per-request timeout, concurrency, retry/backoff, caching, and
  staleness behavior, including partial collector failures;
- HTTP metric and health endpoints, configuration sources, secret injection,
  logging/redaction, packaging, shutdown, and deployment behavior;
- deterministic unit tests and the exact authorized non-production DDAE
  prerequisites for integration or end-to-end evidence;
- backward compatibility, rollout, rollback, and operator-visible migration.

Update progress, discoveries, decisions, and verification at every meaningful
stopping point. Do not mark work complete from an implementation claim alone;
record observable evidence. Never place secrets, private endpoints, or
production data in a plan.

