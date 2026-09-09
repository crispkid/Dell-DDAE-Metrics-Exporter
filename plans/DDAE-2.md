# DDAE-2 Add YAML configuration, independent monitoring pipelines and guarded TLS controls

This execution plan follows `PLANS.md`. DDAE-2 depends on the implemented
DDAE-1 baseline and does not close DDAE-1's external validation or independent
review blockers.

## Purpose and Observable Outcome

Operators can configure the exporter through a strict YAML file, run resource
monitoring and alert monitoring independently, and deliberately disable DDAE or
Kafka certificate/hostname verification only through a two-level opt-in. The
existing environment-only interface remains compatible. Runtime workers,
required settings, readiness and Prometheus families reflect only enabled
pipelines without misrepresenting disabled work as success.

## Context and Constraints

- `internal/config` currently reads only environment variables.
- `cmd/ddae-exporter` has no command-line configuration selector.
- `internal/app` always initializes DDAE, bbolt, Kafka, both schedulers and the
  publisher, so Kafka/state are currently mandatory even for resource-only use.
- `internal/server` and `internal/metrics` currently assume both pipelines.
- DDAE and Kafka TLS clients always verify chain and hostname; DDAE-1 explicitly
  prohibited disabling verification.
- systemd uses an EnvironmentFile and Kubernetes uses ConfigMap `envFrom`.
- DDAE-1 public contracts remain authoritative except for the explicit DDAE-2
  supersession listed in Specification v1.1.0.
- No external DDAE/Kafka/OpenSearch or deployment environment is currently
  authorized. Environment-dependent stages remain blocked rather than passed.
- Insecure TLS is a sensitive boundary. It needs default-secure tests, bounded
  warning behavior and independent qualified review before release.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

- REQ-DDAE-2-001 / AC-DDAE-2-001: bounded version-1 strict YAML loader and
  explicit config-file selection.
- REQ-DDAE-2-002 / AC-DDAE-2-002: deterministic defaults/YAML/environment
  precedence and DDAE-1 environment compatibility.
- REQ-DDAE-2-003 / AC-DDAE-2-003: YAML file-path-only secret contract with
  preserved environment secret handling and redaction.
- REQ-DDAE-2-004 / AC-DDAE-2-004: independent resource/alert enable flags,
  intervals, conditional settings and worker construction.
- REQ-DDAE-2-005 / AC-DDAE-2-005: enabled-pipeline readiness, metric omission
  and `ddae_monitoring_enabled{pipeline}`.
- REQ-DDAE-2-006 / AC-DDAE-2-006, AC-DDAE-2-007: verified TLS default and
  guarded target-specific insecure mode.
- REQ-DDAE-2-007 / AC-DDAE-2-008: YAML-based container, Kubernetes and systemd
  deployment contracts and migration documentation.
- REQ-DDAE-2-008 / AC-DDAE-2-009: deterministic regression/security coverage
  and preservation of DDAE-1 defaults.

### Technical Design

#### Configuration loading and schema

Add a small standard-library command-line selector that accepts only
`--config`. `--config` overrides `DDAE_EXPORTER_CONFIG_FILE`; neither selector
means legacy environment-only operation. An explicitly selected file is read
with a 1 MiB hard limit before remote clients or listeners are created.

Promote the already locked `gopkg.in/yaml.v3` module to a reviewed direct
dependency. Decode through a `yaml.Decoder` with `KnownFields(true)` into a
dedicated versioned input structure. Reject duplicate mapping keys, unknown
fields, invalid scalar types, multiple YAML documents, unsupported versions,
malformed UTF-8 and trailing non-comment document content. The decoded input
contains pointer scalars where omission must be distinguishable from explicit
zero/false.

Build the effective configuration in three layers: compiled defaults, YAML,
then environment overrides. Existing environment names and validations remain
in one mapping source. `DDAE_COLLECTION_INTERVAL` first supplies both intervals;
new dedicated resource/alert interval variables override their respective
value. Boolean environment values accept only case-insensitive `true` or
`false`; ambiguous values fail.

YAML exposes no plaintext password/client-secret/private-key-content field.
Only file paths enter the YAML typed schema. Direct secret environment values
remain available for compatibility and override a YAML path; an explicitly set
direct and `_FILE` environment pair remains a conflict. Secret contents stay in
the existing non-stringable secret type and are read only at the consuming
boundary with current size/UTF-8/NUL/trailing-line validation.

#### Pipeline configuration and dependency validation

Extend effective config with resource/alert enable values and intervals. Both
default true. Reject both false. Shared DDAE URL/credentials and request/cycle
settings remain required because at least one pipeline is enabled. Require
source identity, Kafka, outbox, checkpoint and state values only for an enabled
alert pipeline. Resource stale-after relationships apply only when resources
are enabled; alert refresh/interval relationships apply only when alerts are
enabled. Explicit values always undergo type and basic bound validation.

Refactor `internal/app` into conditional constructors/closers. Always create the
DDAE client and HTTP registry/server. Create the resource manager only when
resources are enabled. Create bbolt, Kafka producer, alert pipeline and
publisher only when alerts are enabled. Store workers and closers in slices or
explicit optional fields so `Run` and shutdown wait for exactly the initialized
set and nil resources cannot be called. Alert disable/enable does not delete or
migrate `state.db`.

Use the configured resource interval for the resource manager and the alert
interval for the alert pipeline. Each scheduler remains serial within itself;
cross-pipeline work may overlap while sharing the bounded DDAE transport and
coalesced token state.

#### Readiness and metric contract

Add enabled-pipeline state to the snapshot/readiness boundary or a separate
immutable readiness policy object. `/readyz` evaluates the conjunction of only
enabled pipelines. Resource readiness retains DDAE-1 current required-snapshot
semantics. Alert readiness retains DDAE-1 authentication/list/detail/state/
outbox semantics and the existing temporary-Kafka-outage rule.

Register `ddae_monitoring_enabled` with exactly two fixed `pipeline` values,
`resources` and `alerts`. Always expose that family and `ddae_build_info`.
Construct or gather resource descriptors only when resources are enabled and
alert/Kafka descriptors only when alerts are enabled. Do not emit zero-valued
success diagnostics for disabled collectors; explicit enable state carries the
disabled signal.

#### TLS guard and warning

Add `AllowInsecureTLS`, `DDAETLSInsecureSkipVerify` and
`KafkaTLSInsecureSkipVerify` to effective config. Validate the two-level truth
table before clients start. Reject a target custom CA combined with effective
insecure mode to avoid ambiguous trust claims.

Set Go `tls.Config.InsecureSkipVerify` only after the validated two-level guard
for that specific client. Retain `MinVersion` TLS 1.2. DDAE applies one setting
to token and GET calls on the same origin; Kafka retains mTLS client identity,
SASL, `acks=all` and idempotence. No certificate failure may trigger automatic
downgrade.

After the structured logger exists and before application workers start, emit
one fixed warning for each effective insecure target. The only dynamic field is
the fixed bounded target value. Do not log configuration values or addresses.
Tests and external evidence must mark insecure runs ineligible for release;
documentation makes custom CA the recommended path.

#### Deployment and migration

Replace the systemd EnvironmentFile example with
`deploy/systemd/config.example.yaml`; pass the config path in `ExecStart` while
retaining `LoadCredential` environment overrides. Convert the Kubernetes
ConfigMap to one `config.yaml` key, mount it read-only and pass `--config`.
Conditional examples explain how resource-only removes Kafka/state/Secret
requirements and how alert-enabled modes preserve one RWO writer.

Update README and runbook for schema, precedence, all pipeline combinations,
TLS risk, container mount, Kubernetes, systemd, readiness, metrics, migration,
rollback and retained state. Preserve environment-only documentation as a
compatibility path rather than the preferred interface.

#### Compatibility and recovery

Keep all existing environment names and default values. Add only new optional
selectors/overrides and one bounded metric family. A no-YAML deployment behaves
as DDAE-1. Rollback uses the previous binary and environment configuration;
the YAML file is ignored by that binary. No state schema change is planned.
Before rollback from alert-enabled mode, stop the new process cleanly and keep
the state volume for the DDAE-1 binary.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-2-001 | REQ-DDAE-2-001, REQ-DDAE-2-002, REQ-DDAE-2-003 | current DDAE-1 config contract; approved DDAE-2 governance | `cmd/ddae-exporter`, `internal/config`, `go.mod`, `go.sum` | TEST-DDAE-2-001, TEST-DDAE-2-002, TEST-DDAE-2-003 |
| TASK-DDAE-2-002 | REQ-DDAE-2-004, REQ-DDAE-2-005 | TASK-DDAE-2-001 | `internal/app`, `internal/collector`, `internal/alerts`, `internal/server`, `internal/snapshot` | TEST-DDAE-2-004, TEST-DDAE-2-005 |
| TASK-DDAE-2-003 | REQ-DDAE-2-005 | TASK-DDAE-2-002 | `internal/metrics` | TEST-DDAE-2-006 |
| TASK-DDAE-2-004 | REQ-DDAE-2-006 | TASK-DDAE-2-001 | `internal/config`, `internal/ddae`, `internal/kafka`, `internal/observability`, `cmd/ddae-exporter` | TEST-DDAE-2-007, TEST-DDAE-2-008, TEST-DDAE-2-011 |
| TASK-DDAE-2-005 | REQ-DDAE-2-007 | TASK-DDAE-2-001 through TASK-DDAE-2-004 | `deploy/kubernetes`, `deploy/systemd`, `README.md`, `docs/runbook.md` | TEST-DDAE-2-009, TEST-DDAE-2-010 |
| TASK-DDAE-2-006 | REQ-DDAE-2-008 | TASK-DDAE-2-001 through TASK-DDAE-2-005 | deterministic tests, security policy and dependency review | TEST-DDAE-2-001 through TEST-DDAE-2-011 |
| TASK-DDAE-2-007 | REQ-DDAE-2-001, REQ-DDAE-2-002, REQ-DDAE-2-003, REQ-DDAE-2-004, REQ-DDAE-2-005, REQ-DDAE-2-006, REQ-DDAE-2-007, REQ-DDAE-2-008 | all prior tasks; named independent security/configuration review | Harness policy, trace and current-run evidence | all DDAE-2 tests and all mapped stages |

### Verification Basis

- `test`: strict-YAML parsing, merge precedence, secret rejection, pipeline
  construction, interval selection, readiness and metric contract tests using
  deterministic local doubles.
- `security`: default-secure and two-level TLS guard analysis, warning
  redaction, no automatic downgrade, secret-field denial and YAML dependency
  review.
- `e2e`: authorized operator-runbook execution for container, Kubernetes and
  VM/systemd YAML profiles, including both single-pipeline modes and dual mode.
- `integration`: remains governed by DDAE-1 and must use verified TLS to qualify
  for release evidence; insecure runs are diagnostic only.
- `lint`, `coverage`, `build`, `supply-chain` and `ci-policy`: retain DDAE-1
  thresholds and include the parser dependency and changed artifacts.

External environment unavailability remains a truthful blocker. No insecure
connection, skipped stage or local double is production/release evidence.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-08-25: Peter Chu requested YAML configuration, independent resource
  and alert settings and an SSL-verification opt-out.
- [x] 2026-08-25: the conflict with DDAE-1's non-disableable TLS contract was
  surfaced with options, consequences and a recommended two-level guard.
- [x] 2026-08-25: Peter Chu confirmed the complete recommended behavior and
  separately approved using new Change ID `DDAE-2`.
- [x] 2026-08-25: Harness structural validation passed and generated the
  Specification, Plan Basis and Test Plan digests recorded in
  `DEVELOPMENT_PLAN.md`; named digest confirmation/approval remains pending.
- [x] 2026-08-25: implemented TASK-DDAE-2-001 through TASK-DDAE-2-006,
  including strict YAML, pipeline selection, conditional readiness/metrics,
  guarded TLS clients, deployment profiles, documentation and deterministic
  local tests.
- [ ] Complete TASK-DDAE-2-007 independent review and all release evidence.
- [ ] Complete deterministic checks, independent review and authorized external
  evidence; external validation is currently unavailable.

## Surprises and Discoveries

- The only existing `config.example` is a systemd EnvironmentFile; changing its
  extension alone would create documentation the binary cannot consume.
- Alert disablement must prevent bbolt and Kafka initialization, not merely stop
  the alert ticker, otherwise resource-only mode would still require alert
  infrastructure.
- Readiness and metrics need an explicit enabled signal; reporting disabled
  collectors as successful would be operationally misleading.
- The requested SSL behavior changes a security boundary and cannot be covered
  by the prior DDAE-1 digest. Its evidence cannot qualify for release when
  verification is disabled.

## Decision Log

- 2026-08-25 — YAML: preferred strict versioned file plus environment overrides;
  owner and decision actor Peter Chu.
- 2026-08-25 — Pipelines: independent resource/alert enable flags and intervals,
  both default on, at least one on; owner and decision actor Peter Chu.
- 2026-08-25 — Disabled semantics: conditional workers/settings/readiness and an
  explicit fixed enable metric; owner and decision actor Peter Chu.
- 2026-08-25 — TLS: global acknowledgement plus target-specific flag, default
  verified, bounded warning and no release evidence in insecure mode; security
  owner and decision actor Peter Chu.
- 2026-08-25 — Governance: use independent `DDAE-2` and Specification v1.1.0 to
  preserve DDAE-1 approval history; requirements approver Peter Chu.

## Recovery and Idempotence

Edits use small contract-first steps. Configuration parsing and merge tests
precede app wiring. Conditional construction uses explicit cleanup on every
failure path. Re-running deployment conversion is idempotent because one YAML
key/file replaces the old environment example without generating secrets.

Rollback restores the prior binary, systemd unit/Kubernetes manifest and
environment configuration. The DDAE-2 implementation does not migrate state.
Stop the process before rollback and preserve `state.db`; never delete outbox
records merely because alert monitoring is temporarily disabled.

## Outcomes and Retrospective

The approved DDAE-2 product and test paths are implemented. Final local lint,
race-enabled tests, the 80% coverage gate (81.1%), build and CI policy pass;
the E2E contract compiles without execution. Governance approval, instruction,
Harness doctor and repository-hygiene checks also pass. The security policy's
targeted tests and source scans pass, then the stage truthfully blocks because
the approved `govulncheck` tool is unavailable. Authorized DDAE/Kafka/
OpenSearch and deployment E2E evidence, supply-chain evidence and independent
review remain pending; no row is marked Verified and no release claim is made.
