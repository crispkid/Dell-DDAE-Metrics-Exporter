#!/usr/bin/env bash
set -euo pipefail

if [[ "${DDAE_INTEGRATION_ENABLED:-}" != '1' ]]; then
  printf 'integration blocked: set DDAE_INTEGRATION_ENABLED=1 only on the authorized non-production runner\n' >&2
  exit 3
fi

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"
go test -tags=integration ./integration -run '^TestAuthorizedIntegration$'
