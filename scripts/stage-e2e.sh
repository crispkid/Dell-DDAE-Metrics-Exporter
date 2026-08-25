#!/usr/bin/env bash
set -euo pipefail

if [[ "${DDAE_E2E_ENABLED:-}" != '1' ]]; then
  printf 'e2e blocked: set DDAE_E2E_ENABLED=1 only on authorized container, Kubernetes and VM test environments\n' >&2
  exit 3
fi

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"
go test -tags=e2e ./integration -run '^TestDeploymentRunbook$'
