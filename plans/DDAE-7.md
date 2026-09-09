# Execution Plan: DDAE-7 Windows 11 portable diagnostics

## Purpose

Deliver a self-contained test folder at `Portable/` for Windows 11 field
operators, with protected HTTP capture and offline parser analysis. This is
the Peter Chu-approved Specification v2.2.0 local implementation pass.

## Context and Constraints

- Peter Chu selected Windows 11 and root `Portable/`, and requested work to
  begin. Peter Chu subsequently approved the exact specification, plan-basis
  and test-plan digests and the encrypted-data exception on 2026-09-07.
- Preserve the locally implemented, uncommitted DDAE-6 baseline and unrelated
  user files. Its prior local tests do not close external release gates.
- Existing logs omit raw errors, URLs and bodies. Capture uses the approved
  narrow data-retention exception, not a global security-policy relaxation.
- Existing clients/DTOs, TLS, routes and normalizers are reusable. Existing
  Bash/Go integration commands are unsuitable as native Windows field tools.
- The available host is macOS ARM64. Native Windows/DDAE evidence is separate.
- No commit, tag, release, upload or external request is currently authorized.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

This heading is the repository marker contract. The basis is a candidate until
Peter Chu approves its exact digest and the specification/test digests.

### Requirement Coverage

REQ-DDAE-7-001 through REQ-DDAE-7-012 cover local implementation/testing.
REQ-DDAE-7-013 and REQ-DDAE-7-014 retain native Windows and authenticated DDAE
evidence. All rows begin Planned. Existing normal exporter contracts remain
regression obligations; new exception-path evidence does not rewrite history.

### Technical Design

Add `cmd/ddae-diagnose` and `internal/portable` in the existing Go module.
Package all field runtime dependencies beneath `Portable/`; source/governance
files need not be copied to the field machine. Use standard-library crypto and
the already locked `golang.org/x/sys/windows`; no new dependency is planned.

Subcommands: `prepare`, `self-test`, `run`, `keygen`, `decrypt`, `replay`,
`verify-bundle`. CMD wrappers select ARM64/AMD64 even under a 32-bit shell,
quote paths relative to their own directory, disable delayed expansion,
preserve exit codes and leave interactive failures visible. They never shell-
interpolate configuration/secret content or change Windows security settings.

Create a separate strict diagnostic YAML decoder; reuse DDAE validators with
an explicit configuration adapter, not the environment-overriding normal
loader. Follow specification defaults/bounds. Resolve paths relative to YAML.
Prepare is create-if-absent, never overwrite. Reject unsafe/reparse destinations,
unsupported platforms/filesystems, missing secrets and invalid key/ACL/TLS
configuration before network. Document UTF-8 file creation in Notepad and exact
existing secret newline/BOM behavior; never silently change credential bytes.

Add an explicit internal diagnostic-client constructor/observer. Normal
`NewClient`, normal YAML and the exporter CLI cannot enable capture. The
nil-observer path preserves current errors, traffic and body handling. A run
budget counts all actual HTTP attempts, including token refresh and retries.
Diagnostics instantiate no Kafka/OpenSearch client or normal durable state.

Capture only allowlisted business GETs before decoding. Read parser-input
bytes into bounded memory, preserving original read errors and noting HTTP
content decoding. Error-response capture stays within the same operation
deadline; do not repeat a request to obtain a better body. Token traffic has a
separate safe metadata-only path that cannot deliver its bodies/header values
to capture serialization. Filter authentication/cookie headers before storage.

Extract small shared production decode/normalization helpers where needed;
do not write a second forgiving parser. Live checks and replay call the same
functions. Event builders and resource normalization run in memory without
publishing or logging content. A separate bounded contract observer reports
required results shape, list completeness, safe field paths and types; it
cannot overwrite the production parser result. Audited cluster-envelope and
missing/null alert-results defects are observed, not fixed in this change.
Detail work uses validated IDs, deterministic rotating selection, explicit
counts/caps and requested/returned-ID checks; no pagination is invented.

Implement `ddaecap` format version 1 with RSA-OAEP-SHA256 key wrapping,
fresh random 256-bit run key, AES-256-GCM records and unique monotonic nonces.
Authenticate format/run/header digest/sequence plus final index. Bound lengths
before allocations and reject wrong keys, reorder/duplication and nonce
overflow. Missing final index is INCOMPLETE, even if earlier independently
authenticated records can be salvaged. No custom cipher or plaintext spool.

Keygen creates a PKCS#8 private key only in an explicit analysis location with
exclusive creation and restricted permissions; field copies carry PKIX public
PEM only. Decrypt/replay validate format/integrity and never execute stored URLs,
follow unsafe output paths or overwrite an existing extraction. Windows ACLs
grant current user/SYSTEM only; POSIX uses private modes. Fail closed when file
protection cannot be established. Host/memory compromise is outside encryption
protection; unknown business-body secrets can exist only in the raw channel.

Keep typed safe-report and encrypted-capture sinks separate. Safe results
contain fixed operation IDs, numeric attempt IDs, bounded status/type/error
values and counts/timings, never raw error strings or unknown field names.
Unique run directories, bounded step output, reserved final-summary space and
incremental record flush preserve completed work where possible. Ctrl+C cancels
workers within the configured grace; hard termination/storage failure cannot
promise a final report. Archive exact allowed reports and encrypted capture,
not the whole folder. No automatic deletion/upload or plaintext ZIP payload.

Embed synthetic self-test fixtures with a process-owned loopback TLS/token
stub; deny non-loopback network in this mode. Assert expected malformed cases
explicitly instead of confusing them with field failures. Replay requires no
DDAE credentials and disables network/publishing entirely. Keep extracted raw
analysis files private and separate from normal safe summaries.

Build both Windows architectures and the darwin/arm64 analysis helper with
Go 1.26.6/CGO disabled into ignored staging first. Publish only declared runtime
assets to `Portable/`, preserving user config/secrets/results. Build a clean
distribution ZIP from an explicit source allowlist. Verify PE machine type,
hashes/notices, complete source-content identity including uncommitted product
inputs, and exclusion of unrelated personal files. Record candidate status,
not an invented release version. Normal exporter is a separately launched
resources-only example, loopback port 9469, without raw capture.

Wire new checks into project-owned build/security scripts and add instruction
targets for new subtrees at implementation. Do not port or alter the shared
Harness baseline. Ignore generated Portable binaries/manifests/distributions,
actual config, secrets, trust, keys, results and analysis files; retain templates.
No metric/Kafka/state/normal-config migration is introduced. Preserve existing
assertions; narrow new exception tests do not weaken old raw-data protections.

### Task Breakdown

| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-DDAE-7-001 | REQ-DDAE-7-001 | approved basis and local runtime tasks | `scripts/build-portable.sh`, build-stage wiring, Portable executables/launchers | TEST-DDAE-7-001 |
| TASK-DDAE-7-002 | REQ-DDAE-7-002 | approved basis | `internal/portable/config.go`, prepare/platform code, YAML templates | TEST-DDAE-7-002 |
| TASK-DDAE-7-003 | REQ-DDAE-7-003 | TASK-DDAE-7-002 | `internal/portable/runner.go`, client/transport integration | TEST-DDAE-7-003 |
| TASK-DDAE-7-004 | REQ-DDAE-7-004 | TASK-DDAE-7-003 | production decode/normalization adapters and diagnostic contract observer | TEST-DDAE-7-004 |
| TASK-DDAE-7-005 | REQ-DDAE-7-005 | TASK-DDAE-7-003, TASK-DDAE-7-006, TASK-DDAE-7-007 | `internal/portable/capture.go`, client observer | TEST-DDAE-7-005 |
| TASK-DDAE-7-006 | REQ-DDAE-7-006 | approved basis | safe/capture sinks, auth/header exclusion and security-stage wiring | TEST-DDAE-7-006 |
| TASK-DDAE-7-007 | REQ-DDAE-7-007 | approved basis | `internal/portable/crypto.go`, keygen/decrypt and platform protection | TEST-DDAE-7-007 |
| TASK-DDAE-7-008 | REQ-DDAE-7-008 | TASK-DDAE-7-003, TASK-DDAE-7-005 | scheduling, request/storage limits and cancellation | TEST-DDAE-7-008 |
| TASK-DDAE-7-009 | REQ-DDAE-7-009 | TASK-DDAE-7-004, TASK-DDAE-7-008 | `internal/portable/report.go`, exits and transfer ZIP | TEST-DDAE-7-009 |
| TASK-DDAE-7-010 | REQ-DDAE-7-010 | TASK-DDAE-7-004, TASK-DDAE-7-007, TASK-DDAE-7-009 | self-test/replay and embedded synthetic fixtures | TEST-DDAE-7-010 |
| TASK-DDAE-7-011 | REQ-DDAE-7-011 | TASK-DDAE-7-001 | package manifest/allowlist, ignore rules, docs/templates | TEST-DDAE-7-011 |
| TASK-DDAE-7-012 | REQ-DDAE-7-012 | TASK-DDAE-7-002 through TASK-DDAE-7-011 | existing and new normal-exporter regression contracts | TEST-DDAE-7-012 |
| TASK-DDAE-7-013 | REQ-DDAE-7-013 | local candidate and authorized Windows host | `integration/portable_windows_test.go`, E2E dispatch and operator evidence | TEST-DDAE-7-013 |
| TASK-DDAE-7-014 | REQ-DDAE-7-014 | local candidate and authorized DDAE boundary | `integration/portable_ddae_test.go`, integration dispatch and safe field evidence | TEST-DDAE-7-014 |

### Verification Basis

- `test`: config/paths, TLS recording transport, parser parity, capture bytes,
  bounded scheduling, statuses, replay and regression, alongside existing
  race-enabled tests. Synthetic data and temporary files only.
- `security`: focused privacy-canary, encryption/integrity, ACL policy-adapter
  and archive-path/fault tests, plus all current security checks. Mock Windows
  ACL tests are not proof of native Windows enforcement.
- `build`: both Windows targets and analysis helper, PE/hash/notice/source
  checks, standalone clean distribution, secret/unrelated-file exclusion.
  `scripts/build.sh` retains its normal build and calls portable build checks.
- `lint`, `coverage`, `ci-policy`, supply-chain and independent-review gates
  remain; do not lower thresholds or fabricate coverage.
- `e2e`: actual native Windows launcher/ACL/self-test/capture/replay/interrupt
  evidence per architecture; cross-build/emulation cannot substitute.
- `integration`: authorized non-production DDAE authentication, selected paths,
  real body parsing and list/detail evidence, never automatic/production.
- Prepare external test entry points and wire them into current stage scripts
  while retaining inherited cases. Missing prerequisites block those stages.

The local pass implements TASK-DDAE-7-001 through TASK-DDAE-7-012 and external
test entry points; it does not execute/close external stages or claim full
`verify`/supported-release completion. Retain all existing required gates.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- [x] 2026-09-07: Read instructions, PROJECT, Harness, active DDAE-6 spec,
  plans/tests/approval records and relevant build/auth/client/log code.
- [x] 2026-09-07: Verified all three DDAE-6 digests match recorded approval.
- [x] 2026-09-07: Cross-built unchanged exporter source with Go 1.26.5,
  CGO disabled and module network lookups disabled for Windows amd64/arm64.
  Both exited 0; `file` identified x86-64/Aarch64 PE executables. This isolated
  temporary feasibility work is not new package acceptance or native evidence.
  This entry corrects the earlier mistaken 1.26.6 label: GOTOOLCHAIN=local
  selected the 1.26.5 wrapper. Version-correct DDAE-7 runs supersede it below.
- [x] 2026-09-07: Prepared candidate and Portable status/instructions draft.
- [x] 2026-09-07: Harness doctor and instruction doctor passed. The final
  sdd:digest structure/mapping check passed and candidate identities were
  recorded separately from approvals. The original DDAE-6 specification/test
  digests and pre-existing product diff fingerprint remain unchanged.
- [x] 2026-09-07: `governance:approved` correctly stopped because DDAE-7
  requirements confirmation is absent. No product implementation, new unit
  tests, complete Portable build or full verify was run past that gate.
- [x] 2026-09-07: Peter Chu confirmed all three exact candidate digests and
  approved local implementation/testing and the encrypted capture exception.
- [x] 2026-09-08: Implemented TASK-DDAE-7-001 through TASK-DDAE-7-012,
  including explicit capture-only client construction, original business body
  capture, gzip header preservation, authenticated encrypted records, protected
  file creation, isolated YAML, bounded run/reporting, synthetic self-test,
  offline extraction/replay, clean packaging and bilingual entry links.
- [x] 2026-09-08: Go 1.26.6 Harness test (race-enabled), coverage (81.8%,
  unchanged required threshold 80%), lint, build and security passed. Security
  included go vet, privacy/crypto/regression tests and govulncheck reporting
  "No vulnerabilities found". Harness doctor, instruction discovery,
  governance:approved, ci:policy and git diff --check passed.
- [x] 2026-09-08: Cross-built both Windows diagnostic/exporter pairs and the
  macOS ARM64 helper with CGO disabled. PE architecture checks, dependency
  notices, source identity and fixed 18-member distribution ZIP passed.
  `Portable/build-manifest.json` records Go 1.26.6, dirty revision and source
  Final source SHA-256 is
  `44a030c25dfc4232103658daf01c99b0fd4dca7a527902234e2688b7ce4893bb`.
- [x] 2026-09-08: Extracted the actual ZIP into
  `/private/tmp/ddae7-package-smoke.0PvyF0`, verified all hashes and ran the
  extracted analysis helper with PATH=/usr/bin:/bin (no Go). Both exited 0;
  self-test safe summary is under
  `results/20260907T164938Z-4b0b858aa9e33d940244beee/summary.txt` there.
- [x] 2026-09-08: Integration-tag package compiled with no test execution;
  Windows e2e test binaries cross-compiled for amd64/arm64. This is compilation
  evidence only. External entry points and a non-Windows fail-closed E2E guard
  are present; inherited stage cases are retained.
- [x] 2026-09-08: Final local-path review added canonical/case-insensitive
  rejection of volume, home and broad user directories before modifying ACLs.
  Synthetic path-policy tests do not exercise real Windows DACL enforcement.
  Re-ran race tests, coverage (81.8%), lint/build and security successfully.
- [x] 2026-09-08: Full `verify` with explicit integration/E2E opt-out exited 3
  at the guarded integration prerequisite, after doctor, repository/governance,
  lint, race tests, coverage and build passed. Retained report:
  `test-results/harness/runs/20260907T165630Z-verify.RP7Fah/report.json`.
  Final standalone security command exited 0; its tool-output transcript is
  retained at `test-results/ddae7-security-local.log`. This does not close
  the same-run full-verify, E2E, supply-chain or independent-review gates.
- [x] 2026-09-08: Extracted final ZIP into
  `/private/tmp/ddae7-final-smoke.Xt7I2N`; bundled verify-bundle and self-test
  both exited 0 with PATH=/usr/bin:/bin. Final safe self-test report is under
  `results/20260907T165824Z-6c96a6723dbd07615b4ac45b/` there.
  Distribution SHA-256:
  `77e2274be3d7948d030e5889f56dbac76e417b3e7b2a7633f2eeb86e353f8520`.
- [x] 2026-09-08: Reviewed new diagnostic/config/crypto/report/package paths,
  ordinary exporter isolation, CMD quoting/architecture dispatch and final
  diff. Exact approved digests remain unchanged; no new dependency/lockfile
  change or unrelated document entered the distribution. This is implementer
  review, not the required independent security/release review.
- [ ] Obtain native Windows and authorized DDAE evidence separately.

## Discoveries

- Existing debug/stdout logging is insufficient for original-body capture.
- The shared Harness is Bash/macOS/Linux tooling, not the Windows runtime.
- Prior parser/contract disagreements require separate diagnostic status layers.
- A newly added detail-ID test initially checked the decode flag incorrectly:
  production JSON decode succeeds, then BuildEvent rejects mismatched IDs.
  The corrected assertion requires decode=true AND validation=false; production
  validation was not weakened. The final suite passes this separation.
- Initial local coverage was 74.3%; adding actual bundle, transport, crypto,
  failure and replay coverage raised it to 81.8%. The threshold was unchanged.
- Finalization now writes the external completion marker only after ZIP close;
  storage/deadline failures preserve INCOMPLETE semantics and safe evidence.
  Hard OS/storage stalls remain best-effort, not a promised hard kill timer.
- Two referenced baseline guides, `HARNESS/templates/governance/README.md`
  and `HARNESS/templates/evals/README.md`, are absent. Present executable
  contracts and project instructions remain usable; do not recreate baseline
  guides or expand this change to repair them.

## Decisions

- User-selected: Windows 11, root Portable, independent test package.
- Approved by Peter Chu: dual Windows architecture, macOS analysis
  helper, encrypted business originals excluding authentication, separate key
  custody, no Kafka/OpenSearch activity and honest local-first evidence.
- Use new DDAE-7/v2.2.0 for the new packaging/data-retention boundary.
- Exact approved source bytes are unchanged. The approval-block wording remains
  as signed; current authority and implementation progress live outside it.

## Recovery and Idempotence

Use fresh test/build paths and exclusive key/run creation. Preserve user-owned
config/results/keys and all prior unrelated edits. Rollback stops diagnostics;
there is no remote mutation or existing state migration. If governed design
changes, renew affected digests and approvals before continuing implementation.

## Outcomes and Retrospective

Local implementation and the Windows distribution are present, with mapped
local stages passing. REQ-DDAE-7-001 through REQ-DDAE-7-012 are Implemented,
not Verified. Native Windows/DDAE and independent security review remain
unexecuted. `sdd:check` correctly refuses full handoff while trace rows are not
Verified; do not change statuses to hide missing external evidence.

No commit, tag, release, deployment, real DDAE call or upload was performed.
Operator-supplied credentials, CA and recipient public key remain required.
The field bundle contains no runtime settings, keys, results or unrelated
user documents. Standalone macOS self-test is not Windows validation.

Remaining external evidence: Windows 11 x64 and ARM64 execution, NTFS DACL
enforcement, actual CMD/Ctrl+C behavior, authorized authenticated DDAE bodies
and list/detail coverage, inherited Kafka/OpenSearch/deployment checks,
independent review and clean-committed-revision release supply-chain evidence.
These gates are deliberately not reported as passing in this local handoff.
