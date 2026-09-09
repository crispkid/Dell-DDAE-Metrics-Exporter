# Execution Plan: DDAE-5 configurable DDAE API path prefixes

This living plan governs the incompatible default-route correction and the
addition of independently configurable Ping and Management API prefixes. Keep
progress and observed results outside the digest-bound Plan Basis block.

## Purpose

Implement the approved `/ping` and `/v1/*` defaults with strict independent
prefix settings, deterministic route construction, RC2/PDF compatibility,
security bounds, regression coverage and complete operator documentation.

## Context and Constraints

- The current v1.0.0-rc2/DDAE-4 implementation compiles `/rest/v1` into every
  DDAE Management API GET route.
- User-supplied field validation found public 404 responses for two
  `/rest/v1` routes and protected reachable `/ping` and `/v1/*` service routes.
  It did not authenticate or validate response schemas.
- Peter Chu selected two independently configured prefixes, new defaults of
  empty for Ping and `/v1` for other API operations, and the remaining bounded
  deterministic recommendations on 2026-08-31.
- Product code must not change until Peter Chu confirms the exact
  Specification digest and approves the exact Plan Basis and Test Plan
  digests.
- Local unit/component and security testing is in the proposed implementation
  pass. Authorized authenticated DDAE integration and inherited external
  release gates remain separate.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage

- TASK-DDAE-5-001 through TASK-DDAE-5-005 cover the locally implementable
  configuration, route, security, compatibility, observability and
  documentation work for REQ-DDAE-5-001 through REQ-DDAE-5-006.
- TASK-DDAE-5-006 retains the authenticated non-production evidence required by
  REQ-DDAE-5-007. It cannot be closed by the current 401/404 observations or
  local test doubles.
- Existing DDAE-1 through DDAE-4 requirements remain regression contracts.
  Only the omitted-setting Management API GET defaults and the two new prefix
  settings change.
- `TEST_PLAN.md` defines the deterministic and external cases.
  `TRACEABILITY.md` maps each requirement to tasks, tests and scheduled Harness
  stages; every DDAE-5 row starts as `Planned`.

### Technical Design

#### Strict configuration and precedence

Add `DDAEPingPathPrefix` and `DDAEAPIPathPrefix` to the effective configuration.
Extend strict YAML v1 with `ddae.paths.ping_prefix` and
`ddae.paths.api_prefix`. Map them to `DDAE_PING_PATH_PREFIX` and
`DDAE_API_PATH_PREFIX` through the existing environment-over-YAML layering.

Use an empty Ping prefix and `/v1` API prefix when neither source provides a
value. Preserve an explicitly empty YAML or environment value. Unknown YAML
fields remain fatal.

Validate each value before application construction. Accept only empty or a
canonical ASCII absolute path namespace no longer than 128 bytes. A non-empty
prefix starts with one slash, has no trailing or repeated slash, and contains
only segments matching `[A-Za-z0-9._~-]+` other than complete `.` or `..`
segments. Reject percent encoding, backslash, whitespace, controls, query,
fragment, scheme, authority and user information. Do not trim, clean, decode
or otherwise normalize input, and do not echo an invalid value in an error.

#### Compiled suffix registry and client construction

Refactor the DDAE operation registry so each Management API operation retains
a compiled method, operation identity and suffix. Ping belongs to the Ping
prefix family; clusters, nodes, lock, power, alert list/detail and
Serviceability Log list/detail belong to the API family. At client
construction, concatenate the selected prefix and suffix exactly once and
retain the resulting immutable routes.

Keep the OAuth token route outside both families and unchanged. Keep all
Management API operations GET-only. Do not use URL reference resolution,
`path.Join`, redirects, endpoint probing or alternate-path retries.

Detail operations retain the current 1-256 byte identifier validation and
single-path-segment escaping. Route construction must preserve the current
escaped/raw-path behavior without permitting an ID to alter the prefix or
suffix.

`ApprovedOperations` must return the effective operation registry used by the
client or accept explicit prefix inputs through a bounded constructor; tests
and security policy must inspect the same route-building source rather than a
duplicated route table.

#### Compatibility, failure and observability

With `ping_prefix=/rest/v1` and `api_prefix=/rest/v1`, every Management API GET
must match v1.0.0-rc2. Preserve token behavior, typed decoders, response bounds,
retry/auth renewal, metrics, Kafka schemas/topics/keys/headers, alert/log
state, intervals and pipeline selection.

Existing bounded operation identities continue to classify errors. A failed
route affects only the owning operation/pipeline under current readiness rules
and never triggers another path. Prefixes, origins, full paths, credentials and
raw bodies do not enter runtime logs, errors, Prometheus labels or retained
test output.

Update `scripts/security-policy.sh` to assert suffix/family ownership,
fixed-token invariance, GET-only operations, prefix validation and the absence
of fallback. Remove assertions that require `/rest/v1` literals as current
defaults.

#### Documentation, deployment and rollback

Update both deployment YAML profiles, README and runbook with the exact keys,
environment overrides, empty and `/v1` defaults, grammar, route matrix and the
three complete new-default, RC2 and PDF examples. Documentation must distinguish
route reachability from authenticated response-schema compatibility.

Before upgrading an RC2 deployment that still requires `/rest/v1`, operators
set both prefixes to `/rest/v1`. Configuration-first rollback restores the old
prefixes and restarts without deleting state. Binary rollback follows a clean
stop and the prior configuration syntax.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-5-001 | REQ-DDAE-5-002, REQ-DDAE-5-003 | approved DDAE-5 basis | `internal/config/config.go`, `internal/config/yaml.go` and strict validation | TEST-DDAE-5-002, TEST-DDAE-5-003, TEST-DDAE-5-004 |
| TASK-DDAE-5-002 | REQ-DDAE-5-001, REQ-DDAE-5-002, REQ-DDAE-5-003 | TASK-DDAE-5-001 | `internal/ddae/allowlist.go`, `internal/ddae/client.go` and immutable effective routes | TEST-DDAE-5-001, TEST-DDAE-5-002, TEST-DDAE-5-004 |
| TASK-DDAE-5-003 | REQ-DDAE-5-003, REQ-DDAE-5-005 | TASK-DDAE-5-001, TASK-DDAE-5-002 | exact route security policy, no-fallback and redaction checks | TEST-DDAE-5-004, TEST-DDAE-5-005, TEST-DDAE-5-007 |
| TASK-DDAE-5-004 | REQ-DDAE-5-004, REQ-DDAE-5-005 | TASK-DDAE-5-002, TASK-DDAE-5-003 | RC2 route/data/state regression and existing readiness behavior | TEST-DDAE-5-006, TEST-DDAE-5-007 |
| TASK-DDAE-5-005 | REQ-DDAE-5-006 | TASK-DDAE-5-001 through TASK-DDAE-5-004 | deployment YAML, README, runbook and static contract coverage | TEST-DDAE-5-008 |
| TASK-DDAE-5-006 | REQ-DDAE-5-007 | TASK-DDAE-5-001 through TASK-DDAE-5-005 plus authorized external prerequisites | sanitized authenticated DDAE route and typed-response evidence | TEST-DDAE-5-009 |

### Verification Basis

- `test`: deterministic unit/component tests use strict YAML fixtures,
  environment layering, recording TLS servers, safe/unsafe IDs, golden RC2
  responses and unique redaction canaries. They cover defaults, all configured
  compatibility forms, exact token/GET paths, validation boundaries,
  no-fallback behavior and unchanged data/state contracts.
- `security`: the project policy statically and dynamically checks the compiled
  suffix families, fixed token route, GET-only boundary, prefix grammar,
  origin preservation, one-segment IDs and absence of fallback or content
  leakage.
- `integration`: an explicitly authorized non-production run authenticates and
  exercises every enabled allowlisted route with the deployment's selected
  prefixes, then validates the existing typed contracts using sanitized
  evidence. This stage remains environment-dependent.
- `lint`, `coverage`, `build` and `ci-policy` remain whole-repository local
  quality gates. Inherited `e2e`, `supply-chain`, deployment attestation and
  independent review remain supported-release gates.

No pre-implementation observation closes a DDAE-5 row. The local pass covers
TASK-DDAE-5-001 through TASK-DDAE-5-005 and TEST-DDAE-5-001 through
TEST-DDAE-5-008. TEST-DDAE-5-009 remains external authenticated integration
evidence.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-08-31: Compared the user-supplied field results with the Dell 1.5.0
  PDF and current v1.0.0-rc2 route implementation.
- [x] 2026-08-31: Peter Chu selected independent Ping/API prefixes, defaults of
  `/ping` and `/v1/*`, and the remaining recommended deterministic/security
  constraints.
- [x] 2026-08-31: Prepared Specification v2.0.0, Plan Basis, Test Plan and
  traceability as an approval candidate without changing product code.
- [x] 2026-08-31: Peter Chu confirmed Specification v2.0.0 digest
  `sha256:636bf0be364097005714d3062e164b981a0a139cbc0e1e00d93cbdc8e753ce35`
  and approved the exact Plan Basis and Test Plan digests.
- [x] 2026-09-01: Implemented TASK-DDAE-5-001 through TASK-DDAE-5-005.
- [x] 2026-09-01: Race-enabled tests, 81.5% coverage, lint, build, security and
  CI policy passed; `govulncheck` reported no vulnerabilities.
- [ ] Run TASK-DDAE-5-006 only in an explicitly authorized non-production
  environment.

## Discoveries

- The Dell PDF is internally inconsistent for Ping and differs from the tested
  service shape, so independent prefixes are necessary to express all known
  forms without arbitrary endpoints.
- A 401 response proves a protected route exists but does not prove successful
  authentication or response schema compatibility.
- The current operation registry and security script both embed full
  `/rest/v1` paths; implementation must change them from full-path literals to
  compiled suffix/family assertions without weakening the allowlist.
- YAML scalar decoding can otherwise coerce numeric or boolean values into
  strings. A dedicated path-prefix scalar type now rejects non-string YAML
  before effective configuration is built.

## Decisions

- Use a new Change ID and Specification major version because omitted-setting
  route behavior changes incompatibly.
- Keep the token path fixed and keep every Management API suffix/method
  compiled.
- Use configuration, never runtime probing, to select route namespaces.
- Preserve RC2 behavior through explicit `/rest/v1` prefixes.

## Recovery and Idempotence

The product edits are configuration and route-construction changes with no
persistent-data migration. If implementation fails, retain current state and
restore the pre-DDAE-5 binary/code path. Repeated local tests use recording
servers and temporary configuration/state. Operational rollback changes both
prefixes back to `/rest/v1` before considering binary rollback; no state file
is deleted.

## Outcomes and Retrospective

- Added independently configured `ddae.paths.ping_prefix` and
  `ddae.paths.api_prefix` settings with matching environment overrides,
  exact empty-value precedence and strict 128-byte canonical ASCII validation.
- Refactored the DDAE allowlist into fixed method/suffix families and immutable
  per-client routes. Defaults now produce `/ping` and `/v1/*` while
  `/rest/v1` settings reproduce RC2 routes; the token POST remains fixed.
- Added deterministic route-matrix, escaping, invalid-prefix, no-fallback,
  redaction, RC2 compatibility and deployment/documentation contracts.
- Updated Kubernetes/systemd YAML, README and runbook with defaults, grammar,
  route results, environment mappings, migration and rollback.
- External authenticated DDAE integration, inherited E2E, supply-chain,
  deployment attestation and independent review were not run and remain
  release gates.
