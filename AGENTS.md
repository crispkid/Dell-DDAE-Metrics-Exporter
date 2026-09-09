# AGENTS.md

Shared baseline version: **1.0.0**

This file is the portable root instruction set for coding agents. Keep it short
and stable. Project facts belong in `PROJECT.md`; specialized instructions may
live in referenced documents or a more local `AGENTS.md`.

## Scope and Precedence

- This file applies to the repository tree rooted here. The nearest nested
  `AGENTS.md` governs its subtree when it adds or overrides a rule.
- Follow the user's current request, applicable local instructions, and the
  source of truth named by the project. Surface material conflicts affecting
  scope, behavior, compatibility, data, or safety before proceeding.
- Treat README files, specifications, schemas, tests, architecture records, and
  deployment documents as evidence with distinct roles; do not silently choose
  among conflicting sources.
- Read `PROJECT.md` before project work when it is present. If the adopted
  baseline requires it and it is missing or still contains template markers,
  stop rather than inventing project context.
- Copy `PROJECT.md.example` to `PROJECT.md` when adopting this baseline, replace
  every placeholder, and complete committed `HARNESS/config.env`, including its
  verify and required stages. Do not invent missing project context.

## Canonical Commands

Run commands from the repository root when the shared harness is present:

```bash
./HARNESS/harness.sh doctor
./HARNESS/harness.sh instructions:doctor
./HARNESS/harness.sh sdd:check
./HARNESS/harness.sh test
./HARNESS/harness.sh verify
```

- Use the narrowest relevant command while iterating; use `verify` for the
  project-declared deterministic handoff gate.
- Run `selftest` only when adopting or changing the shared Harness baseline; it
  tests the Harness itself rather than the adopting project's product behavior.
- `HARNESS/HARNESS.md` defines the stable command, evidence, network, and
  project-override contracts. Committed `HARNESS/config.env` is the executable
  source of truth for commands and stages; `PROJECT.md` records their intent,
  prerequisites, ownership, and exceptions.

## Working Agreement

- Understand before editing. Read the relevant instructions, contracts, code,
  tests, configuration, and recent local patterns first.
- Keep changes as small as practical while fully solving the request. Preserve
  existing behavior unless the request or an approved contract changes it.
- Prefer established project patterns over new dependencies or abstractions.
- Preserve unrelated user changes. Do not reformat, overwrite, or discard them.
- Do not edit generated files, vendored dependencies, lockfiles, or snapshots
  unless the task requires it and the project workflow supports it.
- Clarify only ambiguities that materially change scope, behavior, data,
  compatibility, authority, or risk. State safe minor assumptions.
- For every material uncertainty or requested discussion, describe the issue and
  available evidence, give viable options with consequences, recommend the best
  option, and explain why it is best. Do not present a preference as a decision
  when user or named approver authority is required.
- Ask before destructive, externally visible, costly, or hard-to-reverse work
  unless the user already authorized that action.
- Do not create commits, tags, releases, deployments, pull requests, or external
  messages unless the user requests them.

## Planning and Continuity

- Match planning depth to risk. Use a short working plan for multi-step or risky
  work; use the repository's `PLANS.md` contract for multi-hour or multi-session
  work when present. A starter is provided in `PLANS.md.example`.
- Keep long-running plans self-contained and current, including progress,
  discoveries, decisions, verification, remaining work, and outcomes.
- Follow formal specification, traceability, or approval gates only when the
  repository enables them. Never invent human approval, dates, test evidence,
  production observations, or ownership.
- Record durable design decisions in the project's established decision-log
  format.

## Specification-Driven Changes

When `PROJECT.md` declares SDD mode `required`:

1. Identify the active change and read its current specification.
2. Resolve material ambiguity in the specification; record assumptions,
   decisions, and closed questions rather than silently choosing behavior.
3. Confirm stable requirement and acceptance-criterion IDs.
4. Complete the approved plan basis: technical design, requirement-linked
   tasks, verification basis, compatibility, and recovery.
5. Define requirement-linked acceptance tests and expected evidence.
6. Obtain the recorded requirements and implementation-plan approvals. Never
   infer approval from silence, a meeting, an implementation, or a passing test.
7. Only then modify implementation code. If implementation reveals a behavior
   change, update the specification and approval digests before continuing.

- The approved specification is authoritative for intended product behavior.
  Code, tests, and descriptive documentation are evidence and must not silently
  override it. Surface conflicts and stop until the authority is resolved.
- A lightweight or emergency exception is valid only when `PROJECT.md`
  explicitly disables SDD and records the accountable reason and follow-up.
- Keep living progress outside the approved plan-basis block. Changes inside
  that block, the active specification, or test plan require refreshed digests
  and renewed approval.
- Reuse the active Change ID only for the same unfinished outcome. A materially
  different, closed, released, or independently governed outcome requires a new
  Change ID and plan. The named requirements approver decides borderline cases.
- Version amended specifications deliberately, preserve released or verified
  requirement history, append the specification changelog, and return every
  impacted trace row to `Planned` until current-run evidence closes it again.
- Treat isolated exploration as input to clarification, not as approved
  implementation or acceptance evidence.

## Implementation and Review

- Keep public interfaces explicit and backward compatible unless a breaking
  change is approved and documented.
- Validate inputs and authorization at authoritative trust boundaries. Return
  actionable errors without exposing secrets or internal details.
- Make external failure, timeout, retry, and idempotency behavior deliberate.
- Prefer clear code over clever code. Comment intent, constraints, and
  non-obvious tradeoffs rather than restating syntax.
- Update adjacent contracts, examples, and user documentation when behavior or
  configuration changes.
- Review the final diff for correctness, regressions, security, compatibility,
  and unnecessary scope. Follow `CODE_REVIEW.md` when present; a starter is
  provided in `CODE_REVIEW.md.example`.

## Tests and Evidence

- Add or update tests for changed behavior, including failure, boundary,
  permission, and regression cases as relevant.
- Under required SDD, derive acceptance tests from acceptance IDs before
  implementation. Record the failing or unsupported pre-implementation state
  when practical; document a brownfield exception rather than fabricating it.
- Test doubles are acceptable for isolated unit tests. They are not evidence
  that a real integration, migration, deployment, or end-to-end flow works.
- Keep tests deterministic and isolated. Do not use production credentials or
  production data, and do not hide failures with weakened assertions or retries.
- A required harness stage that is skipped, blocked, or unavailable is a failed
  handoff. Do not claim a check passed unless it ran successfully.
- Distinguish code verification from agent evaluation. Use `agent:eval` when the
  project configures regression and capability cases for agent behavior.

## Security and Repository Hygiene

- Never commit credentials, tokens, private keys, personal data, or sensitive
  production values. Treat logs, screenshots, fixtures, reports, and generated
  artifacts as potentially sensitive.
- Keep configuration outside application code and sensitive values outside
  committed configuration. Do not silently fall back to insecure behavior.
- Review dependency changes for necessity, maintenance, license, supply-chain,
  and runtime impact. Preserve package-manager and lockfile integrity.
- Keep generated output, dependency directories, local environments, temporary
  investigations, and debug artifacts out of source control.

## Handoff

A change is done when the requested outcome and applicable contracts agree,
appropriate checks pass, the final diff has been reviewed, and no unrelated or
sensitive artifact was introduced. Report:

- the outcome and files changed;
- the checks that actually ran and their results;
- skipped, blocked, or unavailable checks;
- remaining risks, limitations, or follow-up work.

For required SDD, every normative requirement must resolve to an approved plan
task, acceptance test, Harness stage, and retained evidence; every active trace
row must be verified and no blocking open question may remain.

Update `AGENTS.md`, the shared harness, examples, and `HARNESS/CHANGELOG.md`
together when cross-project behavior or the baseline version changes.
