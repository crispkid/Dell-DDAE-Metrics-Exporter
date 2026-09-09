# Portable Project Harness

Baseline version: **1.0.0**

The Harness gives agents, developers, and CI one stable verification interface
while each project retains its tools. It makes no product, language, provider,
or service assumption. Research rationale is in `HARNESS/BEST_PRACTICES.md`.

The portability boundary is:

- `AGENTS.md`, `PROJECT.md.example`, this document, and starter templates are
  shared baseline files.
- `PROJECT.md`, committed `HARNESS/config.env`, `HARNESS/evals/`, execution
  plans, and active governance records are project-owned.
- Existing scripts, Make targets, tool configuration, and CI workflows remain
  the implementations of project checks.

## Quick Start

From the repository root:

```bash
cp PROJECT.md.example PROJECT.md
cp HARNESS/config.env.example HARNESS/config.env
# Copy and complete HARNESS/templates/governance for the first active change.
# Complete the required project context; delete optional sections that do not apply.
# Configure the exact project stages and commands in config.env.
./HARNESS/harness.sh selftest
./HARNESS/harness.sh doctor
./HARNESS/harness.sh sdd:digest
# Record real named approvals, implement, update traceability, then:
./HARNESS/harness.sh sdd:check
./HARNESS/harness.sh verify
```

Invoke the executable as shown; do not run it through `bash harness.sh`. Its
`/bin/bash -p` launcher deliberately ignores caller startup hooks, exported
functions, and inherited shell-option injection before trusted policy loads.

Merge `.gitignore.example` into the repository's ignore rules before the first
reported run. `doctor` fails in a Git worktree when the evidence root or a
separate agent-result root is not ignored. `selftest` validates the shared
Harness itself; run it when adopting or modifying the baseline, not as a
substitute for the project's normal tests.

The generated root must contain no tracked entries, and its ignore must come
from repository-local rules (`.gitignore` or the local
`.git/info/exclude`), not a developer's global Git excludes.

Copy and complete the optional `CODE_REVIEW.md.example` and `PLANS.md.example`
only when the project adopts those contracts.

## Stable Commands

```text
doctor                 Validate harness configuration, paths, and environment
instructions:doctor    Validate the root-to-directory instruction chain
environment:doctor     Show and validate execution prerequisites
repo:doctor            Inspect version-control hygiene
governance:doctor      Validate configured governance records
governance:approved    Validate active-change approval records
sdd:digest             Calculate active SDD approval digests
sdd:check              Validate the active strict-SDD contract
lint                   Run configured or detected lint checks
format:check           Check formatting without rewriting files
typecheck              Run configured or detected type checks
test                   Run unit/component tests
coverage               Run the project coverage gate
build                  Build project artifacts
integration            Run real integration checks
e2e                    Run end-to-end checks
security               Run the project security gate
ci:policy              Run the configured CI workflow policy gate
supply-chain           Run the configured SBOM/provenance gate
deploy:check           Validate deployment artifacts or configuration
verify                 Run the deterministic project handoff gate
agent:eval:doctor      Validate the agent evaluation dataset and runner
agent:regression       Run the configured agent regression suite
agent:capability       Run the optional capability benchmark
agent:eval             Run configured regression and capability suites
selftest               Test the shared Harness with isolated fixtures
version                Show the baseline version
help                    Show command help
```

Use the narrowest direct stage while iterating and `verify` for handoff.
`agent:eval` is separate: verification checks project artifacts; evaluation
measures whether an agent follows instructions and reliably reaches outcomes.

## Executable Configuration

Copy `config.env.example` to `config.env`, complete it, and commit it. It is
sourced by Bash and is trusted executable repository code, not dotenv data.
Review it like a build script. Adopted config runs in an isolated fail-fast Bash
and must reach `HARNESS_CONFIG_COMPLETE='1.0.0'`; the Harness imports only its
allowlisted settings. Use direct assignments for policy so an ordinary caller
or CI environment cannot silently change the deterministic gate. Pass secrets
and explicitly allowed runtime inputs through separate project-owned variables
consumed by configured scripts.

Committed `HARNESS/config.env` is the executable source of truth for stages,
commands, thresholds, network declarations, and governance enforcement.
`PROJECT.md` records intent, prerequisites, ownership, constraints, and
exceptions; it must not duplicate raw command strings or numeric settings.
Optional PROJECT sections may be deleted when their feature is disabled.
Once `PROJECT.md` is adopted, alternate `HARNESS_CONFIG_FILE` locations are
rejected; that variable remains only for isolated, pre-adoption maintainer
fixtures.

Never put credentials, personal data, production-only endpoints, or embedded
secrets in `config.env`. Prefer a project-owned script for complex or
security-sensitive commands. The Harness executes configured commands from a
mode-`0600` temporary script with `bash -e -o pipefail`: an unhandled simple
command or pipeline failure stops the stage. Bash startup hooks and exported
shell functions are ignored for this child shell. A script may opt out explicitly,
for example with a deliberate `set +e`, but must handle and return failures
itself.

Raw command text is neither echoed nor retained. Bash and child-process
diagnostics can still expose source fragments or arguments, so configured
commands must not place secrets in either.

## Verification Stage Contract

The stage lists use `lint`, `format`, `typecheck`, `test`, `coverage`, `build`,
`integration`, `e2e`, `ci-policy`, `supply-chain`, `security`, and `deploy`.

- `HARNESS_VERIFY_STAGES` is the exact ordered-independent set of project stages
  scheduled by `verify`; internal doctor, repository, and governance checks
  always run first.
- `HARNESS_REQUIRED_STAGES` must be a subset of `HARNESS_VERIFY_STAGES`. Its
  stages must resolve to a real implementation; skipped or unavailable required
  stages fail handoff.
- Any scheduled stage that runs and fails or reports blocked fails `verify`,
  whether or not it is in the required subset.
- `HARNESS_REQUIRED_STAGES=none` means no required product stage and must be
  justified in `PROJECT.md`. `HARNESS_VERIFY_STAGES=none` schedules none; when
  only the required list is `none`, scheduled optional stages may skip if no
  implementation exists, but any one that actually runs and fails is fatal.
Other common settings include `HARNESS_REQUIRED_TOOLS`,
`HARNESS_ENVIRONMENT_COMMAND`, `HARNESS_COVERAGE_THRESHOLD`, and
`HARNESS_COMMAND_TIMEOUT_SECONDS`. Timeout is zero or 1 through 86400 seconds;
a non-zero value requires `timeout` or `gtimeout`. Increment the non-secret
`HARNESS_COMMAND_ID_PREFIX` when configured command semantics change.

## Command Resolution

Each scheduled stage resolves from its `HARNESS_*_COMMAND`, then a matching root
Make target, root or shallow-child package script, conservative configured
Python/Go/Rust convention, and finally a visible skip.

Explicit commands are recommended for monorepos, nonstandard layouts, service
checks, or frozen/offline dependency rules. Use `skip` only for a scheduled,
non-required omission that is intentional and documented.

The Harness does not install dependencies, start services, or download browsers.
Detected tools can still use the network unless the execution boundary prevents it.

## Instruction Discovery

`instructions:doctor [relative-directory]` models the repository-local
root-to-target chain. In each directory it selects the first non-empty
`AGENTS.override.md`, `AGENTS.md`, or configured fallback. It reports shadowed
files, enforces the combined-byte limit, confines targets and references to the
repository, validates adopted `PROJECT.md`, and rejects an unapproved root
override.

Configure `HARNESS_INSTRUCTION_TARGETS` with every supported working subtree;
the default checks only the root. `HARNESS_INSTRUCTION_REFERENCES` lists other
required instruction documents, while `HARNESS_REQUIRED_PATHS` is reserved for
normalized, repository-contained project paths rather than duplicating the
instruction contract.

`PROJECT.md` is referenced context, not an automatically discovered instruction
file; an adopted copy must be non-empty and marker-free. Its committed config
must set `HARNESS_EXPECTED_BASELINE_VERSION`, and PROJECT, config, Harness, and
root `AGENTS.md` must all agree on that version.

## Environment and Network

`environment:doctor` records the OS and Bash version, checks required tools,
and runs the optional environment command with network-contract variables
cleared and reset to `false`. Record exact tool versions in project version
files; describe their intent in `PROJECT.md`.

`HARNESS_NETWORK_POLICY` is `deny`, `declared`, or `allow`.
`HARNESS_NETWORK_STAGES` identifies network-dependent stages. Project commands
receive `HARNESS_NETWORK_CONTRACT_ALLOWS=true|false`; evidence records the same
value and `network_enforcement=declaration-only`. This is a routing guard and
declaration, not an OS sandbox. Use CI, containers, or a sandbox when denial
must be technically enforced.

Agent validators receive separate regression and capability network flags. In
an all-suite validation, the generic flag is true only if both suite flags are
true. Service-dependent checks must fail or report blocked when credentials,
services, browsers, containers, or network prerequisites are missing; mocks do
not turn them into real integration or E2E evidence.

## Machine-Readable Evidence

Reported runs use:

```text
test-results/harness/
  runs/<run-id>/report.json
  runs/<run-id>/status.complete
  runs/<run-id>/agent-results/<suite>.json
  latest/<run-type>.json
  <run-type>.json                         # current non-authoritative summary
  agent-results/<suite>.json              # current non-authoritative summary
```

The run-owned report is canonical only when its sibling `status.complete` is a
regular non-symlink file containing `complete`. Ignore incomplete or unmarked
directories. The atomic latest pointer contains the canonical path, SHA-256,
byte size, result, and timestamps; consumers must recompute digest and size.
Fixed views are non-authoritative and may differ during concurrent publication.

Reports record run/Harness identities, timestamps, result, toolchain,
repository/configuration identities, artifacts, stages, and `effective_policy`.
Effective policy includes both stage lists, network policy/stages, timeout,
coverage, SDD, governance/approval and repository-hygiene gates, instruction
targets and size, and root-override policy. The `sdd`
record identifies the active change, approved specification version, approved
specification/plan-basis/test-plan digests, final traceability digest, and
contract state (`not-run`, `approved`, `failed`, or `verified`). A successful
strict-SDD `verify` must record `verified`; other run types may truthfully record
`not-run`. Stage records
contain status, exit code, duration, network declaration, stable command
ID/source, and detail, never raw commands. Project exit `3` means blocked; a
blocked required stage fails.

Dirty state uses a canonical length-prefixed manifest of Git status, tracked
diff identity, and each untracked regular file or symlink identity, including a
regular file's executable bit. A dirty submodule fails closed because the
superproject's `-dirty` sentinel does not identify its nested bytes. Generated
evidence roots are excluded from that snapshot so a report cannot change the
source identity it describes. The independent Git-ignore doctor still requires
those roots to be ignored, preventing accidental commits and unbounded buildup.

`HARNESS_REPORT_DIR` and agent-result paths must be normalized,
repository-contained, and free of symlink components. Required SDD forces
`HARNESS_WRITE_REPORTS=true`, because a handoff without retained current-run
stage evidence cannot be `Verified`. Outside required SDD, wrapper reports may
be disabled, but agent runs still retain their validated canonical result.
`HARNESS_REPORT_FILE` changes only the current summary basename. Evidence
directories are created mode `0700` and files `0600`, then
published files are made read-only where supported. Canonical runs are
append-only only from the Harness's point of view; projects own access control,
retention, immutable storage, signing, and independent verification.

Registered values from `HARNESS_REDACT_ENV_VARS` are replaced exactly with
`[REDACTED]` in descriptive Harness metadata. Structural values fail closed
instead of being replaced into invalid evidence. Redaction does not infer
encoded, transformed, split, or derived forms and does not sanitize console
output or project-owned artifacts. `HARNESS_REDACT_MIN_BYTES` and the evidence
string/report byte limits bound registration and output. Invalid UTF-8, NUL,
unsafe paths, limit violations, and serialization errors fail closed without
advancing latest.

Harness temporaries share one private process directory and are removed on
normal or catchable-signal exit. `SIGKILL`, host failure, or filesystem failure
can leave files for OS cleanup; sensitive CI should provide a private managed
`TMPDIR`. Reports are evidence, not proof by themselves.

## Agent Evaluation

Agent evaluation is optional. A project supplies versioned cases, a locked
`HARNESS_AGENT_CONTRACT_COMMAND`, and regression/capability runners. The Harness
has no mandatory JSON/YAML runtime parser: the project validator owns schema,
duplicate-key, cross-file, digest, manifest, path, oracle/no-op, and semantic
checks before and after runner execution.

Case and suite-result schema version 1 records immutable inputs, separate
verification, behavioral trials, graders, metrics, and retained artifacts.
Blocked or error records use structured `failure {phase, code, detail}` and may
leave unobserved collections empty; they must not invent trials, graders,
metrics, or thresholds. Case `minimum_retention_days` can only raise the
project-level evidence retention policy. The project validator records and
enforces this requirement; the shell Harness does not execute retention jobs.

Runners atomically write one complete result to the unique
`HARNESS_AGENT_RESULT_FILE`. A missing, empty, symlinked, stale, or invalid
result cannot pass or replace a prior current summary. Failed and blocked
runs must still emit a valid result once evaluation has begun. Infrastructure
retries are not additional behavioral trials; each trial needs a fresh isolated
workspace and a verifier inaccessible to the agent.

The normative adoption, environment-variable, status, integrity, metric,
artifact, and validator semantics live in `HARNESS/templates/evals/README.md`.
The schemas and examples are supporting machine contracts, not a replacement
for those cross-file rules.

## Strict SDD and Governance

Adopted projects default to `HARNESS_SDD_MODE=required`. This enforces a
two-gate workflow without inventing a language-specific specification tool:

1. Create the canonical active change and complete the active sections in
   `SPECIFICATION.md`, `SPEC_CHANGELOG.md`, `TEST_PLAN.md`, `TRACEABILITY.md`,
   `DEVELOPMENT_PLAN.md`, and `plans/<change-id>.md`.
2. Resolve every material open question. Requirements use normative
   `MUST`/`SHOULD`/`MAY` statements and stable `REQ-*` IDs; acceptance criteria,
   tasks, and tests use matching `AC-*`, `TASK-*`, and `TEST-*` IDs.
3. Run `sdd:digest`. Record the returned specification version and the three
   approval digests in the active development-plan section.
4. Obtain and record separate named requirements confirmation and plan
   approval. `governance:approved` verifies this pre-implementation baseline;
   trace rows may still be `Planned`.
5. Implement from the approved basis. If intended behavior, design basis, or
   test plan changes, update the source artifact, regenerate digests, and obtain
   renewed approval before continuing.
6. Record one `path:<repository-relative-path>` implementation reference in
   each trace row and one executable-test path in each Test Plan row. Each test
   names one scheduled product stage; the trace lists the exact stage set for
   its mapped tests and matching `stage:<stage-id>` evidence values.
   Set a row to `Verified` only after those paths exist and the selected stage
   is capable of proving the mapped acceptance criterion.
7. Run `sdd:check` to validate final structure, references, and approved
   identities. Then run `verify`: product stages execute first and the final
   SDD closure accepts `Verified` only when each referenced stage passed in that
   same run. No unresolved question or stale approval digest may remain.

Only the marker-bounded Approved Plan Basis in the execution plan is
approval-locked. Progress, discoveries, recovery notes, and outcomes stay
outside that block and remain living plan content. This avoids invalidating a
real approval merely because progress was updated.

For amendments, reuse the active Change ID only for the same unfinished
observable outcome; use a new ID and plan for a materially different, closed,
released, or independently approved outcome. Apply patch/minor/major
specification versions for non-behavioral clarification, backward-compatible
behavior, and incompatible behavior respectively. Preserve released or
verified IDs, append changelog history, preserve valuable regression tests, and
return impacted trace rows to `Planned`. Specification changes renew named
requirements confirmation; approved plan-basis or test-plan changes renew named
implementation-plan approval. Never update digests without the corresponding
real approval. The detailed decision, exploration, stage-selection, and
material-uncertainty rules are normative in
`HARNESS/templates/governance/README.md` and project-specific authority belongs
in `PROJECT.md`.

`HARNESS_SDD_MODE=off` is an explicit repository policy exception, not a
per-command escape hatch. Committed config and `PROJECT.md` must both say `off`,
and PROJECT must state a concrete accountable reason and follow-up. A user or CI
environment cannot turn off an adopted policy. PROJECT must name the exemption
owner and a concrete action or recurring review policy; `None`, `N/A`, and
`Pending` do not satisfy the contract. The Harness has no hidden
emergency bypass and never creates or infers approvals, dates, evidence, or
ownership.

The Markdown documents remain human-readable sources of truth. Stable headings,
fields, table columns, IDs, and plan markers are their machine contract; do not
reformat those elements without a coordinated baseline/schema/test update.

General governance remains available when SDD is off or before adoption.

`HARNESS_GOVERNANCE_MODE` is `auto`, `required`, or `off`.

- `auto` activates only when `HARNESS/ACTIVE_CHANGE` exists. Merely having a
  governance-named file does not activate it.
- `required` always requires the full configured governance set.
- `off` disables governance and is invalid with
  `HARNESS_REQUIRE_PLAN_APPROVAL=true`.
- Approval set to true makes `auto` fail closed and require complete governance
  plus approval rather than silently skipping.

`HARNESS/ACTIVE_CHANGE` is the only active-change selector. Change IDs use a
bounded uppercase `PREFIX-number` portable grammar; a project pattern may
narrow but not broaden that safety boundary.

The default governed set is `SPECIFICATION.md`, `SPEC_CHANGELOG.md`,
`DEVELOPMENT_PLAN.md`, `TEST_PLAN.md`, and `TRACEABILITY.md`. Files must be
regular repository-contained paths without unresolved starter markers and must
contain exact references to the active change. General governance validates
safe adoption and reference presence. Required SDD additionally validates the
active structured sections, normative requirements, ID mappings, approval
identities, digests, and trace completeness.

Approval requires separate named records for requirements confirmation and
implementation-plan approval: status, actor, role, ISO date, and evidence for
each. The development record must link `Execution Plan: plans/<change-id>.md`;
that non-empty, adopted plan must reference the same change and is the sole home
for steps and progress. The Harness never
creates or infers approval. See `HARNESS/templates/governance/README.md`.

## Test, CI, and Runtime Boundaries

- Unit/component tests may use deterministic doubles; integration and E2E must
  cross the real configured boundary and validate final state.
- Do not weaken assertions, delete valuable tests, retry away behavioral
  failures, or lower thresholds merely to pass a gate.
- Run service-dependent stages in separate CI jobs where practical. Configure
  CI-policy and supply-chain tools explicitly rather than parsing generic YAML.
- Use least-privilege tokens, immutable third-party revisions, workload
  identity, locked dependencies, and appropriate SBOM/provenance/signing.
- macOS and Linux with Bash 3.2+ are supported. Native Windows shells are not;
  use WSL. SHA-256 tooling is required. Git is optional for execution but
  required for repository identity and ignore/hygiene evidence.

## Contract Version Matrix

| Contract | Version in baseline 1.0.0 |
|---|---:|
| Harness baseline and CLI behavior | 1.0.0 |
| Harness report schema | 1 |
| Agent-eval case schema | 1 |
| Agent-eval result schema | 1 |
| Latest-pointer schema | 1 |
| Evidence storage layout | 1 |
| Dirty repository manifest | 1 |

All published contracts start at version 1 and are shipped as one coherent
baseline set.

## Baseline Maintenance

`./HARNESS/harness.sh version` prints the baseline version. Record cross-project
changes in `HARNESS/CHANGELOG.md` using semantic versioning and update the
markers in AGENTS, PROJECT example, scripts, config example, schemas, examples,
tests, and this document together. Project-only behavior stays in PROJECT,
project scripts, cases, or config and does not require a baseline release.

The shared baseline gate includes Bash 3.2/macOS and current Bash/Linux,
ShellCheck, isolated selftests, representative ecosystem fixtures, evidence
failure paths, and Draft 2020-12 positive/negative schema conformance using the
locked baseline-development dependency. They are maintainer checks, not runtime
dependencies imposed on adopters.

Modules under `HARNESS/lib/` are internal. The documented CLI commands, machine
schemas, and normative contracts are the supported interfaces.
