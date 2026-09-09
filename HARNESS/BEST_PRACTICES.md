# Best-Practice Basis

Last reviewed: **2026-08-14**

This file records the rationale for baseline 1.0.0. It is not a project
execution contract and does not override `AGENTS.md`, committed
`HARNESS/config.env`, or `HARNESS/HARNESS.md`.

## Agent Instructions and Long Work

- [OpenAI: Custom instructions with AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
  documents root-to-working-directory discovery, `AGENTS.override.md`
  precedence, one selected file per directory, fallback filenames, and the
  combined project-instruction byte limit.
- [OpenAI: Using PLANS.md for multi-hour problem solving](https://developers.openai.com/cookbook/articles/codex_exec_plans)
  supports self-contained living plans with progress, discoveries, decisions,
  verification, and outcomes.

Baseline decision: keep the root AGENTS compact, place durable project intent
in PROJECT, and maintain one execution plan for long work. Formal governance
links that plan instead of duplicating steps and progress.

## Specification-Driven Development

- [GitHub Spec Kit methodology](https://github.github.com/spec-kit/) separates
  specification, technical planning, task decomposition, and implementation.
- [GitHub Spec Kit command reference](https://github.github.com/spec-kit/reference/overview.html)
  adds clarification, consistency analysis, checklists, and implementation
  convergence around the specification.
- [Kiro specifications](https://kiro.dev/docs/specs/) uses a durable
  requirements → design → tasks progression grounded in reviewable artifacts.

Baseline decision: required SDD is a committed, fail-closed adopter policy.
Human-readable Markdown is canonical; stable fields and IDs provide portable
machine checks. A named approval binds the exact specification, approved plan
basis, and test plan before implementation. Handoff then requires every active
requirement to resolve through implementation, executable tests, configured
product stages, and retained same-run evidence.

The same unfinished outcome keeps its Change ID. A closed, released, materially
different, or independently approved outcome receives a new one. Specification
versions and stable IDs preserve history, governed byte changes renew the
applicable named approvals, valuable regression coverage remains, and amended
trace rows return to `Planned`. Material product judgment belongs to the named
project decision owner rather than a portability-oriented parser.

## Agent Evaluation

- [OpenAI: Agent evals](https://developers.openai.com/api/docs/guides/agent-evals)
  and [trace grading](https://developers.openai.com/api/docs/guides/trace-grading)
  support trace-level diagnosis followed by repeatable dataset evaluation.
- [OpenAI: Sandboxes](https://developers.openai.com/api/docs/guides/agents/sandboxes)
  separates the control plane from isolated compute and emphasizes narrow
  workspace, network, secret, and artifact boundaries.
- [Harbor task format](https://harborframework.com/docs/tasks),
  [evaluation runs](https://harborframework.com/docs/run-jobs/run-evals), and
  [ATIF trajectories](https://harborframework.com/docs/agents/trajectory-format)
  provide versioned tasks, separate verifiers, per-trial results, artifacts,
  and normalized trajectories.
- [Inspect task and dataset contracts](https://inspect.aisi.org.uk/tasks.html)
  and [sandboxing](https://inspect.aisi.org.uk/sandboxing.html) support explicit
  dataset/solver/scorer separation and a clean sandbox per sample.
- [SWE-bench evaluation](https://www.swebench.com/SWE-bench/guides/datasets/)
  supports pinned repository revisions, reproducible environments, reference
  patches, and grading observable test outcomes.
- [Anthropic: Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
  distinguishes capability and regression evaluation, outcome and trace
  evidence, grader types, and multiple-trial reliability.
- [Anthropic: Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
  supports incremental work, explicit progress, baseline end-to-end checks,
  and observable verification before completion.

Baseline decision: deterministic project verification is separate from agent
evaluation. Versioned cases and results use immutable input identities,
separate verification, outcome evidence, and explicit infrastructure versus
behavioral state. Blocked or erroneous runs record structured failure without
inventing trials, graders, metrics, or thresholds.

## CI, Provenance, and Reproducibility

- [GitHub Actions secure use reference](https://docs.github.com/en/actions/reference/security/secure-use)
  supports least-privilege tokens, immutable third-party revisions, careful
  secret handling, and protected environments.
- [GitHub Actions OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)
  supports workload identity instead of long-lived cloud credentials.
- [SLSA tracks](https://slsa.dev/spec/v1.2/tracks) defines increasing source and
  build provenance trustworthiness.
- [Reproducible Builds documentation](https://reproducible-builds.org/docs/)
  covers deterministic output, managed environmental variance, checksums, and
  independent verification.
- [Python jsonschema](https://pypi.org/project/jsonschema/) supplies locked
  Draft 2020-12 conformance tests without becoming an adopter runtime
  dependency.

Baseline decision: expose explicit CI and supply-chain stages, record effective
runtime policy and stage evidence, and delegate tool-specific enforcement to
the adopting project.

## Evidence and Security Design

The evidence contract uses unique run directories, completion markers, atomic
latest pointers, current summary files, repository/configuration identities,
artifact SHA-256 and sizes, and a prefix-free dirty-state manifest. Generated
evidence is excluded from source identity only after repository-local ignore
rules and tracked-entry checks pass.

Raw commands are replaced by stable IDs. Registered secrets are redacted from
descriptive Harness metadata; structural overlap, invalid UTF-8, NUL, unsafe
paths, oversized evidence, serialization failure, dirty submodules, and invalid
result contracts fail closed. These controls preserve and validate evidence;
they are not signatures, an OS network sandbox, durable retention automation,
or protection from a same-user administrator.

## Final Baseline Decisions

- Baseline, report schema, agent case/result schemas, latest-pointer schema,
  evidence layout, and dirty-state manifest all begin at version 1.
- Committed config is the only executable project policy source; PROJECT records
  intent, owners, constraints, and exceptions without copying command strings.
- `HARNESS_VERIFY_STAGES` selects the exact project-stage set and
  `HARNESS_REQUIRED_STAGES` identifies the must-resolve subset.
- `HARNESS/ACTIVE_CHANGE` is the only active-change selector.
- The marker-bounded approved plan basis is immutable under its named approval;
  progress, discoveries, decisions, and outcomes remain living content.
- `governance:approved` validates the pre-implementation approval baseline;
  `sdd:check` validates final structure; `verify` adds observed current-run
  product-stage evidence and retained closure.
