#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

workflow='.github/workflows/ci.yml'
[[ -f "$workflow" ]] || { printf 'missing %s\n' "$workflow" >&2; exit 1; }
for required in \
  './scripts/stage-lint.sh' \
  './scripts/stage-test.sh' \
  './scripts/stage-coverage.sh' \
  './scripts/build.sh' \
  './scripts/security-policy.sh' \
  './scripts/supply-chain.sh' \
  './scripts/stage-ci-policy.sh' \
  './scripts/stage-integration.sh' \
  './scripts/stage-e2e.sh' \
  'self-hosted' \
  'DDAE_INTEGRATION_ENABLED' \
  'DDAE_E2E_ENABLED'; do
  grep -Fq "$required" "$workflow" || { printf 'CI workflow is missing policy marker: %s\n' "$required" >&2; exit 1; }
done

if grep -Eq 'uses: [^ ]+@v[0-9]' "$workflow"; then
  printf 'CI workflow contains a mutable major-version action reference\n' >&2
  exit 1
fi

authorized_job="$(awk '
  /^  authorized-integration:/ { active=1 }
  active && /^  [A-Za-z0-9_-]+:/ && $0 !~ /^  authorized-integration:/ { exit }
  active { print }
' "$workflow")"
for required in \
  "if: github.event_name == 'workflow_dispatch' && inputs.run_integration && github.ref == 'refs/heads/main'" \
  'runs-on: [self-hosted, ddae-nonproduction]' \
  'environment: ddae-nonproduction' \
  'ref: ${{ github.sha }}' \
  'persist-credentials: false'; do
  grep -Fq "$required" <<<"$authorized_job" || {
    printf 'authorized integration policy is missing: %s\n' "$required" >&2
    exit 1
  }
done
