# Project Context

## Required Core

- Project name: `Dell DDAE Metrics Exporter`
- Purpose and observable outcome: Collect DDAE cluster status, infrastructure
  node status/capacity, documented appliance operability, configured
  coordinator/worker CPU and memory, serviceability alerts and typed
  Serviceability Logs / Events through the read-only Management API.
  Expose the first four areas as stable Prometheus metrics and publish sanitized
  per-alert and per-log detail events to isolated Kafka paths for OpenSearch
  ingestion and alerting.
- Lifecycle: DDAE-2 Specification v1.1.0 is approved and implemented locally,
  dependent on the implemented but not fully verified DDAE-1 v1.0.0 baseline.
  Peter Chu confirmed DDAE-3 Specification v1.2.0 and approved its Plan Basis
  and Test Plan digests on 2026-08-26. Both batches are implemented and their
  mapped local stages pass; Batch 2 remains `Implemented` awaiting the same-run
  full verify and inherited release gates. Peter Chu confirmed DDAE-4
  Specification v1.3.0 and approved its Plan Basis and Test Plan digests on
  2026-08-28. TASK-DDAE-4-001 through TASK-DDAE-4-008 and their deterministic
  local unit/component coverage are implemented. Peter Chu confirmed DDAE-5
  Specification v2.0.0 and approved its Plan Basis and Test Plan digests on
  2026-08-31. TASK-DDAE-5-001 through TASK-DDAE-5-005 and their local
  route/configuration/security/documentation coverage were implemented on
  2026-09-01. Peter Chu confirmed DDAE-6 Specification v2.1.0 and approved its
  Plan Basis and Test Plan digests on 2026-09-02. TASK-DDAE-6-001 through
  TASK-DDAE-6-005 and their mapped local unit/component/security coverage are
  implemented. Authenticated DDAE
  integration and external DDAE, Kafka,
  OpenSearch, deployment, independent-review and inherited release gates remain
  incomplete, so no supported software release exists.
  On 2026-09-07 Peter Chu requested a self-contained Windows 11 diagnostic
  package under root `Portable/`. DDAE-7 Specification v2.2.0 and its plan/test
  basis were explicitly approved by Peter Chu on 2026-09-07. DDAE-7 is the
  active implementation authority for local portable development and its
  narrowly scoped encrypted business-data capture exception. Native Windows,
  authenticated DDAE and inherited external release gates remain separate.
  On 2026-09-08 the DDAE-7 local implementation, mapped local tests and complete
  `Portable/` distribution were built. The local pass is Implemented, not a
  Verified release; current evidence and remaining gates are in `plans/DDAE-7.md`.
- Accountable owner or role: Repository Maintainers
- Shared baseline version: `1.0.0`
- Shared baseline source and maintainer: The portable baseline is vendored in
  this repository; its original upstream location is not recorded. Repository
  Maintainers own this project's adopted copy.

## Sources of Truth

- Product behavior: `SPECIFICATION.md` contains the approved DDAE-1 v1.0.0
  baseline, the digest-approved DDAE-2 v1.1.0 amendment, the Peter
  Chu-confirmed DDAE-3 v1.2.0 amendment, and the Peter Chu-confirmed DDAE-4
  v1.3.0 amendment, plus the Peter Chu-confirmed DDAE-5 v2.0.0 default-route
  amendment with its approved Plan Basis and Test Plan digests. The Peter
  Chu-confirmed DDAE-6 v2.1.0 response-compatibility amendment and its approved
  Plan Basis and Test Plan digests are the implementation baseline. DDAE-7's
  Windows portable/capture amendment and exact plan/test digests were explicitly
  approved by Peter Chu and now govern the local implementation pass.
- Architecture and durable decisions: The approved execution plan for the
  active change plus non-normative decision summaries under `docs/decisions/`.
  The approved specification and plan remain authoritative when a summary
  conflicts with its governed source.
- API and data contracts: The active approved specification, its metric mapping
  and Kafka event contract. Applicable Dell DDAE API documentation, Prometheus
  compatibility targets and Kafka/OpenSearch consumer compatibility must be
  identified and versioned by the first product change.
- Operations or explicit non-applicability: No deployment or operations
  contract exists during bootstrap. The first product change must define the
  runtime, deployment target, health behavior, and operator runbook.
- Conflict precedence: Current user and applicable instruction authority take
  precedence. Within project artifacts, the approved active specification is
  authoritative for product behavior, committed `HARNESS/config.env` is
  authoritative for executable gates, and `PROJECT.md` is authoritative for
  project-wide intent and boundaries. The approved plan basis and test plan,
  implementation, tests, and descriptive documentation follow within their
  respective roles. Conflicts must be surfaced instead of resolved silently.

## Component Map

| Component | Path | Responsibility | Owner | Runtime/version source |
|---|---|---|---|---|
| Project governance and documentation | Repository root | Define scope, public contracts, plans, review policy, and operator-facing documentation | Repository Maintainers | `PROJECT.md` and the active SDD records |
| Portable verification Harness | `HARNESS/` | Validate instructions, repository policy, configured product stages, and retained evidence | Repository Maintainers | `HARNESS/HARNESS.md` and `HARNESS/config.env` |
| Exporter runtime | `cmd/`, `internal/` | Collect the approved DDAE Management API areas, expose cluster/node/appliance/configuration data as Prometheus metrics, publish typed per-alert details to Kafka, and optionally publish typed Serviceability Logs through an isolated disabled-by-default pipeline | Repository Maintainers | Go 1.26.6 and committed `go.mod`/`go.sum` |
| Portable diagnostics | `cmd/ddae-diagnose/`, `cmd/portable-package/`, `internal/portable/`, `Portable/` | Independent bounded read-only probe, encrypted business HTTP capture, offline parser replay and dual Windows packaging | Repository Maintainers | Approved DDAE-7 basis and generated `Portable/build-manifest.json` |
| Deployment and operations | `Dockerfile`, `deploy/`, `docs/runbook.md` | Package and operate the Kubernetes and VM/systemd profiles | Repository Maintainers | Immutable release digest plus the runbook |

Generated Harness evidence belongs under `test-results/harness/` and local
generated output belongs under `output/`; both are ignored and are not source.
No vendored dependencies are used. Sanitized fixtures live under `testdata/`;
deployment profiles live under `deploy/`.

## Cross-machine Development

Agent instructions, this project context, the complete `HARNESS/` source and
configuration, governed specifications/approval records, execution plans,
decision records and their templates are shared repository inputs. The root
`.gitignore` must not exclude them. Repository `.gitattributes` preserves LF
checkout bytes for shell scripts and approval-bound documents across hosts;
Windows command launchers use CRLF. See `docs/development-portability.md` for
the handoff inventory, bootstrap commands and local-only data boundaries.

This repository-hygiene change does not amend approved product behavior or the
Harness baseline. Historical plan notes about ignored governance files describe
the earlier state; the 2026-09-10 portability change makes those files eligible
for normal Git tracking. Commits must include both the applicable development
records and their corresponding source changes to make a handoff reproducible.

## Specification-Driven Development

- SDD mode: `required`
- SDD exemption reason: Not applicable while SDD mode is required.
- SDD exemption owner: Not applicable while SDD mode is required.
- SDD exemption follow-up: Not applicable while SDD mode is required.
- Normative specification location: `SPECIFICATION.md` after strict SDD is
  enabled.
- Changes requiring a specification: Every product change, including metric
  names, types, units, labels, scrape behavior, DDAE compatibility,
  authentication, configuration, error handling, timeout/retry behavior,
  packaging, and deployment behavior.
- Repository policy for changes that may use committed SDD mode `off`: None
  while DDAE-1 is active. Only the emergency exception below may temporarily
  change committed policy, and it cannot authorize an ungoverned handoff.
- Specification owner: Project Owner, represented by a real named actor in the
  active change records.
- Requirements approver: Project Owner, represented by a real named actor in
  each approval record.
- Design and test-plan approver: Project Owner or a designated Maintainer,
  represented by a real named actor in each approval record.
- Change ID lifecycle and same-change/new-change decision authority: Continue
  the same Change ID only for the same unfinished observable outcome. A closed,
  released, materially different, or independently governed outcome receives a
  new Change ID. The Requirements Approver decides and records borderline
  cases.
- Specification versioning policy: Use patch for non-behavioral clarification,
  minor for backward-compatible behavior, and major for removal or incompatible
  behavior.
- Requirement and acceptance-ID replacement/deprecation policy: Never reuse a
  released or verified ID for different behavior. Preserve history and add an
  explicit replacement or deprecation relationship.
- Investigation and prototype isolation/disposition policy: Keep exploration
  outside deliverable paths and acceptance evidence. Discard it or bring it
  under an approved change before delivery.
- Required SDD artifacts: `SPECIFICATION.md`, `SPEC_CHANGELOG.md`,
  `DEVELOPMENT_PLAN.md`, `TEST_PLAN.md`, `TRACEABILITY.md`, and
  `plans/<change-id>.md`.
- Emergency SDD-off authority, required committed reason, and
  retrospective-spec deadline: After strict SDD is enabled, only the Project
  Owner may authorize an exception. The repository must record the concrete
  reason, owner, recovery action, and affected scope; the specification and
  approvals must be restored before the affected change is handed off or
  released.
- SDD conflict precedence: approved specification > approved plan basis and
  approved test plan > tasks > implementation and observed test results >
  descriptive documentation. User and applicable instruction authority still
  take precedence over repository files.
- Material uncertainty protocol: Document the issue and evidence, viable
  options and consequences, the recommended option and rationale, and the
  named decision owner before changing the specification or approved plan
  basis. Record the owner's actual decision separately.

## Development and Verification Intent

`HARNESS/config.env` is the executable source of truth for commands and stages.

- Definition of Done, why each configured stage is required, and which product
  stages may close SDD acceptance evidence: During bootstrap, handoff requires
  adopted instructions, project policy, repository hygiene, and Harness
  self-tests to pass; no product stage exists and no exporter capability may be
  claimed. Before product implementation, configure at least deterministic
  lint, test, and build stages, plus integration coverage when a non-production
  DDAE test boundary becomes available.
- Integration/E2E prerequisites and isolated data: Real integration evidence
  requires an explicitly authorized non-production DDAE endpoint,
  least-privilege read-only credentials, a documented supported API version,
  an isolated Kafka test topic/broker, and isolated test data. Mocks are unit
  evidence only.
- Security, CI, supply-chain, and deployment gate rationale: Add gates when the
  corresponding implementation, dependency, CI, packaging, or deployment
  boundary is introduced. Authentication and secret-handling changes require
  security review; release artifacts require dependency and supply-chain
  review.
- Supported developer/CI environments and version files: The Harness supports
  macOS and Linux with Bash 3.2 or newer. DDAE-1 selects Go 1.26.5 and the exact
  dependency versions recorded in `plans/DDAE-1.md`; `go.mod`, `go.sum`, the CI
  toolchain and pinned OCI base become executable sources of truth after
  implementation-plan approval. DDAE-3 Batch 1 updates those executable
  sources to Go 1.26.6 under its approved security-patch requirement.
- Harness setting override policy: committed policy values are authoritative;
  callers may supply only runtime inputs explicitly consumed by project-owned
  scripts and may not weaken required stages, governance, or security policy.
- Deterministic handoff command: `./HARNESS/harness.sh verify`

## Security and Data

- Data classification: Treat DDAE inventory, configured resource data,
  appliance-operability status, alert events/payloads, endpoints, Kafka metadata, and generated
  evidence as confidential operational data unless an applicable organization
  policy is stricter. DDAE and Kafka credentials/tokens are secrets.
- Authentication/authorization authority: The DDAE system and deployment
  environment are authoritative. The exporter must use a least-privilege,
  read-only DDAE identity through the documented `dv-admin-rest` password-grant
  bearer-token flow; non-production integration must prove that the identity is
  authorized for every allowlisted GET and no mutation.
- Secret-management and redaction rules: Never commit secrets or real private
  endpoints. Inject credentials at runtime using the deployment platform's
  secret mechanism, exclude them from labels and metrics, redact them from logs
  and evidence, and fail closed when required credentials are absent or
  invalid. Alert fields may enter Kafka only through the approved versioned
  event allowlist and must never be copied wholesale from a raw DDAE response.
  The approved DDAE-7 diagnostic-only exception permits original business
  exchanges solely in opt-in encrypted artifacts with separate private-key
  custody. Token bodies/authentication headers remain excluded. Normal logs,
  metrics, Kafka, Git and safe Harness evidence retain the existing protections.
- Dependency and vulnerability policy: Add only necessary dependencies, pin
  them with the selected ecosystem's lock mechanism, review license and
  maintenance risk, and resolve applicable high-impact vulnerabilities before
  release.
- Backup, deletion, recovery, and incident owner: Git is the recovery source
  for code and governed documentation. Repository Maintainers own local
  evidence cleanup and development incidents; deployment backup, retention,
  rollback, and incident ownership must be defined before a supported release.

## Compatibility and Change Management

- Public interfaces and compatibility promise: Prometheus metric names, help,
  type, unit, label names and meanings; HTTP endpoints; configuration keys; and
  documented DDAE version support are public contracts. Kafka topic use, record
  key, headers, schema version, field names/types and lifecycle/delivery
  semantics are also public contracts. Preserve backward compatibility within
  a major release unless an approved specification explicitly defines a
  migration.
- Migration and deprecation process: Document replacements and operator action,
  preserve useful regression tests, provide a supported overlap when feasible,
  and remove an interface only through an approved incompatible change.
- Governance activation and approval enforcement: Bootstrap mode cannot
  authorize product implementation. Strict SDD and required governance must be
  committed and pass `governance:approved` before deliverable code work begins.
- Approval roles and independent-review threshold: Requirements and plans need
  the named approvals above. Changes to DDAE authentication, secret handling,
  remote access, metric identity/cardinality, Kafka schemas/delivery, or
  release/deployment behavior also require a reviewer who did not implement the
  change.

## Instruction Contract

- Instruction targets checked by Harness: The repository root. Add every new
  supported source, test, packaging, or deployment subtree when it is created.
- Instruction compatibility: Portable baseline instructions are authoritative;
  nested `AGENTS.md` files may add project-specific rules without weakening the
  root safety, SDD, or evidence contracts.
- Project-only agent rules: DDAE access must remain read-only. Treat metric
  identity and labels as public contracts; do not use unbounded values such as
  timestamps, free-form messages, request IDs, or event IDs as labels. Bound
  scrape time, retries, concurrency, and cardinality. Expose partial collection
  failures honestly and never convert stale data into an apparent successful
  scrape. Treat Kafka topic names, event keys, schemas and delivery semantics as
  public contracts; bound buffering and never silently drop alert events.
  Never use production credentials or production data in tests.

## Known Constraints

- DDAE-1 paths, metrics, Kafka schema/delivery, default limits and one-target Go
  architecture remain fixed in `SPECIFICATION.md`; DDAE-2 implements the
  approved YAML interface, independent resource/alert controls and explicit
  guarded insecure-TLS opt-in. No sanitized live DDAE or Kafka/OpenSearch integration
  evidence exists yet. A mismatch must be handled by a specification amendment
  rather than runtime guessing.
- Go 1.26.6 and the dependency versions recorded in `plans/DDAE-1.md` are the
  current implementation baseline. Kubernetes and VM/systemd are both v1
  deployment profiles. Environment-specific Kafka authentication values remain
  runtime inputs and are never committed.
- No authorized non-production DDAE test environment is recorded. Until one is
  available, only isolated unit behavior can be tested and no real integration
  claim may be made.
- Portable Harness 1.0.0 self-test line 225 does not quote the Harness path and
  therefore fails in this repository's space-containing directory. Baseline
  self-tests must run from an isolated path without spaces until a coordinated
  baseline release fixes the shared test; normal project Harness commands have
  been verified in the actual repository path.

## Current Parser Amendment

DDAE-8 (Specification 2.4.0) is the active, user-approved cluster parser fix.
The approval evidence and candidate-to-canonical transcription are recorded in
DEVELOPMENT_PLAN.md and plans/DDAE-8.md. DDAE-7 and inherited external release
gates remain incomplete. Authorized demo testing in this conversation provides
limited functional observations only, not a supported-release certification.

## Portable retirement — current user direction (2026-09-09)

The current repository user explicitly requested removal of the complete Portable
feature and confirmed removal of its package, diagnostic/packaging source,
dedicated tests and build integration, preserving historical documents and the
normal Exporter. Portable was testing-only and is neither a production nor a
pre-production deliverable. This explicit instruction supersedes earlier current
Portable component/lifecycle statements above and DDAE-7 implementation scope.
Historical approved spec/plan digests are preserved, not relabeled as approved
retirement plans. See `plans/portable-retirement.md` for scope and evidence.

## Current Query Monitoring Amendment

DDAE-9 Specification 2.5.0 is user-approved in this conversation. Its canonical
records transcribe plans/DDAE-9-query-monitoring-candidate.md, approved by OK
after queue and output clarification. DDAE-8 parser changes and Portable retirement
remain preserved. Query integration is version-bounded and not release-certified.

DDAE-9 is now implemented locally. Authorized demo query-only collection passed;
full release gates and independent review remain incomplete. See plans/DDAE-9.md.

## Current History Backfill Amendment

DDAE-10 Specification 2.6.0 is approved by the current user in this conversation.
Its canonical records transcribe plans/DDAE-10-history-backfill-candidate.md.
DDAE-9 and earlier work remain preserved. See plans/DDAE-10.md for progress.
DDAE-10 is implemented locally with synthetic and authorized Demo functional evidence.
Inherited release gates, full Harness and independent review remain incomplete.

## Current Kafka SASL Correction

DDAE-11 Specification 2.7.0 is the current user-approved query-only Kafka SASL
credential-loading correction. The user approved the linked candidate on
2026-09-11. Canonical approval identities are recorded in DEVELOPMENT_PLAN.md;
progress and evidence belong in plans/DDAE-11.md. Existing DDAE-10 and earlier
behavior, histories and incomplete release gates remain preserved. Scope is local
code, tests and documentation only; no commit, push or new release is authorized.

On 2026-09-11 the user additionally authorized local Docker Kafka testing.
The supplemental native Kafka SASL/mTLS matrix passed 19 expected outcomes,
with synthetic HTTPS DDAE inputs and 10 records read by a real consumer.
See `docs/kafka-auth-validation-2026-09-11.md` and `plans/DDAE-11.md`.
This does not close the non-production DDAE integration/E2E or release gates.


## Current Audit Corrections

DDAE-12 Specification 2.8.0 is the current user-approved audit correction and
unused-code cleanup. The user replied "同意" to the linked candidate on 2026-09-11.
Canonical approval evidence is in DEVELOPMENT_PLAN.md; execution/evidence belongs
in plans/DDAE-12.md. It preserves DDAE-11 and prior work, valid schema 1 state,
Kafka/metrics/build interfaces and inherited incomplete gates. No migration,
automatic corrupt-state repair, S2 security-policy change, commit or release is authorized.

## RC5 publication request (2026-09-11)

The current user subsequently requested committing these changes and publishing
`v1.0.0-rc5`. This authorizes the source commit and release preparation, superseding
the original DDAE-11/DDAE-12 no-commit scope. Product approval digests and required
gates remain unchanged. Release publication remains subject to those gates or a
new explicit test-only publication exception; the RC4 exception was version-specific.
