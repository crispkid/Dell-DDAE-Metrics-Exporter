# Portable retirement

## Authority and scope

On 2026-09-09 the current repository user instructed: “remove Portable. it is
just for testing. not for production or even pre-produciton.” The user then
explicitly selected removal of the entire package, diagnostic/packaging code,
dedicated tests and build integration, preserving historical documents and the
normal Exporter. This is the actual authority for this removal; no separate
requirements/plan digest approval or release verification is claimed.

## Implementation and verification basis

- Remove Portable/, cmd/ddae-diagnose, cmd/portable-package, internal/portable,
  diagnostic capture APIs, isolated diagnostic configuration loader, dedicated
  tests, and portable build/security/integration/E2E hooks.
- Preserve production cluster parser fixes and their regression coverage; use
  test-only helpers instead of shipping offline replay functionality.
- Preserve DDAE-7/8 historical specifications, plans, approvals and evidence.
  Their historical Portable acceptance targets no longer describe deliverables.
- Preserve existing ignore rules for legacy confidential diagnostic artifacts.
- Verify no runtime references remain, build the exporter, run parser tests,
  go vet and the full unit suite; distinguish host limitations from removal
  regressions. Do not claim a release or complete spec audit from these checks.
- Recovery: restore the removed tracked paths and build hooks from Git history,
  reapplying any desired earlier local parser changes from retained evidence.

## Results

'Portable implementation and build/test hooks removed. No remaining code references to removed APIs were found. Production parser regression packages (internal/ddae, internal/collector, internal/metrics), exporter build, go vet ./... and git diff --check passed. Full go test failed in internal/config, internal/contract, internal/app and internal/logstate with Windows absolute-path, Unix-permission and checkout-newline failures already observed before removal. Harness verify failed before stages: evidence metadata command failed for toolchain.version. Therefore no complete handoff/release pass is claimed. Evidence: output/portable-retirement/test.jsonl, vet.txt, verify.txt. The broader requested specification audit remains incomplete.'
